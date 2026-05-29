package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// VMInfo holds processed information about a VM for the UI.
type VMInfo struct {
	Name          string
	Namespace     string
	Status        string
	Ready         bool
	CPUCores      int64
	Memory        string
	IP            string
	TailscaleName string
	Protected     bool
	HostNode      string
	Age           string
}

// ClientManager wraps Kubernetes client interfaces.
type ClientManager struct {
	DynamicClient dynamic.Interface
	Clientset     kubernetes.Interface
	ContextName   string
}

var (
	gvrVM = schema.GroupVersionResource{
		Group:    "kubevirt.io",
		Version:  "v1",
		Resource: "virtualmachines",
	}
	gvrVMI = schema.GroupVersionResource{
		Group:    "kubevirt.io",
		Version:  "v1",
		Resource: "virtualmachineinstances",
	}
)

// NewClientManager initializes clients based on standard kubeconfig.
func NewClientManager() (*ClientManager, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		// Fallback to in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("could not find kubeconfig: %v", err)
		}
	}

	// Try to extract context name
	contextName := "kubernetes-context"
	rawConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{},
	).RawConfig()
	if err == nil && rawConfig.CurrentContext != "" {
		contextName = rawConfig.CurrentContext
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &ClientManager{
		DynamicClient: dynClient,
		Clientset:     clientset,
		ContextName:   contextName,
	}, nil
}

// ListVMs lists KubeVirt VMs in the namespace (empty for all).
func (cm *ClientManager) ListVMs(namespace string) ([]VMInfo, error) {
	ctx := context.TODO()

	vms, err := cm.DynamicClient.Resource(gvrVM).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var list []VMInfo
	for _, item := range vms.Items {
		name := item.GetName()
		ns := item.GetNamespace()

		// Retrieve labels and annotations
		labels := item.GetLabels()
		annotations := item.GetAnnotations()

		isProtected := false
		if labels != nil && labels["tailvm.io/protected"] == "true" {
			isProtected = true
		}

		// Find spec details
		spec, ok := item.Object["spec"].(map[string]interface{})

		// Check VM status & get current state
		statusMap, ok := item.Object["status"].(map[string]interface{})
		printableStatus := "Stopped"
		ready := false
		if ok {
			if ps, exists := statusMap["printableStatus"]; exists {
				printableStatus = fmt.Sprintf("%v", ps)
			}
			if rd, exists := statusMap["ready"]; exists {
				if rdBool, ok := rd.(bool); ok {
					ready = rdBool
				}
			}
		}

		// Attempt to fetch VMI details (IP, HostNode, etc.) if running
		ip := "N/A"
		hostNode := "N/A"
		var cpuCores int64 = 1
		memorySize := "N/A"

		// Get VM Spec configurations
		if spec != nil {
			if template, ok := spec["template"].(map[string]interface{}); ok {
				if tSpec, ok := template["spec"].(map[string]interface{}); ok {
					if domain, ok := tSpec["domain"].(map[string]interface{}); ok {
						if cpu, ok := domain["cpu"].(map[string]interface{}); ok {
							if cores, exists := cpu["cores"]; exists {
								if cVal, err := toInt64(cores); err == nil {
									cpuCores = cVal
								}
							}
						}
						if mem, ok := domain["memory"].(map[string]interface{}); ok {
							if guest, exists := mem["guest"]; exists {
								memorySize = fmt.Sprintf("%v", guest)
							}
						}
					}
				}
			}
		}

		// Retrieve runtime details from VMI
		vmi, err := cm.DynamicClient.Resource(gvrVMI).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
		if err == nil && vmi != nil {
			vmiStatus, ok := vmi.Object["status"].(map[string]interface{})
			if ok {
				if interfaces, exists := vmiStatus["interfaces"]; exists {
					if ifaceList, ok := interfaces.([]interface{}); ok && len(ifaceList) > 0 {
						if ifaceMap, ok := ifaceList[0].(map[string]interface{}); ok {
							if ipAddr, exists := ifaceMap["ipAddress"]; exists {
								ip = fmt.Sprintf("%v", ipAddr)
							}
						}
					}
				}
				if nodeName, exists := vmiStatus["nodeName"]; exists {
					hostNode = fmt.Sprintf("%v", nodeName)
				}
			}
		}

		tailscaleName := fmt.Sprintf("%s-vm", name)
		if annotations != nil && annotations["tailscale.com/hostname"] != "" {
			tailscaleName = annotations["tailscale.com/hostname"]
		}

		age := item.GetCreationTimestamp().Time
		ageStr := time.Since(age).Round(time.Minute).String()

		list = append(list, VMInfo{
			Name:          name,
			Namespace:     ns,
			Status:        printableStatus,
			Ready:         ready,
			CPUCores:      cpuCores,
			Memory:        memorySize,
			IP:            ip,
			TailscaleName: tailscaleName,
			Protected:     isProtected,
			HostNode:      hostNode,
			Age:           ageStr,
		})
	}
	return list, nil
}

// CreateVM provisions a VM, sets its Tailscale Proxy and Deletion Protection metadata.
func (cm *ClientManager) CreateVM(name, namespace string, cpuCores, memoryGB int, isProtected bool, useDataVolume bool, diskSource, cloudInit string) error {
	ctx := context.TODO()

	// 1. Generate VM Resource Manifest
	vmManifestYAML := GenerateVMManifest(name, namespace, cpuCores, memoryGB, isProtected, useDataVolume, diskSource, cloudInit)
	
	// Parse VM YAML into dynamic object
	var vmObj unstructured.Unstructured
	if err := json.Unmarshal([]byte(yamlToJSON(vmManifestYAML)), &vmObj.Object); err != nil {
		return fmt.Errorf("parsing VM manifest error: %v", err)
	}

	_, err := cm.DynamicClient.Resource(gvrVM).Namespace(namespace).Create(ctx, &vmObj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create VM: %v", err)
	}

	// 2. Generate and Apply protection policy ConfigMap
	cmManifestYAML := GenerateNamespaceConfigMapManifest(namespace)
	var cmObj unstructured.Unstructured
	if err := json.Unmarshal([]byte(yamlToJSON(cmManifestYAML)), &cmObj.Object); err == nil {
		_, _ = cm.DynamicClient.Resource(schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "configmaps",
		}).Namespace(namespace).Create(ctx, &cmObj, metav1.CreateOptions{})
	}

	// 3. Generate and Apply Tailscale Operator Proxy Manifests
	proxyManifestYAML := GenerateProxyManifests(name, namespace)
	// Apply all elements separated by "---"
	manifests := splitYAMLManifests(proxyManifestYAML)
	for _, manifest := range manifests {
		if len(manifest) == 0 {
			continue
		}
		var obj unstructured.Unstructured
		if err := json.Unmarshal([]byte(yamlToJSON(manifest)), &obj.Object); err != nil {
			continue
		}

		gvk := obj.GroupVersionKind()
		gvr := schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: strings.ToLower(gvk.Kind) + "s",
		}
		// Special plural overrides
		if gvk.Kind == "ServiceAccount" {
			gvr.Resource = "serviceaccounts"
		} else if gvk.Kind == "RoleBinding" {
			gvr.Resource = "rolebindings"
		} else if gvk.Kind == "Deployment" {
			gvr.Resource = "deployments"
		}

		_, err = cm.DynamicClient.Resource(gvr).Namespace(namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			// Silently continue or log, proxy config is helper
			fmt.Printf("[Warning] proxy creation: %v\n", err)
		}
	}

	return nil
}

// DeleteVM destroys a VM and its Tailscale proxy environment.
func (cm *ClientManager) DeleteVM(name, namespace string) error {
	ctx := context.TODO()

	// 1. Delete KubeVirt VM
	err := cm.DynamicClient.Resource(gvrVM).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	// 2. Delete Tailscale Proxy elements
	_ = cm.Clientset.AppsV1().Deployments(namespace).Delete(ctx, fmt.Sprintf("%s-proxy", name), metav1.DeleteOptions{})
	_ = cm.Clientset.CoreV1().Services(namespace).Delete(ctx, fmt.Sprintf("%s-proxy", name), metav1.DeleteOptions{})
	_ = cm.Clientset.CoreV1().ServiceAccounts(namespace).Delete(ctx, fmt.Sprintf("tailvm-%s-proxy", name), metav1.DeleteOptions{})
	
	roleGVR := schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"}
	rbGVR := schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"}
	_ = cm.DynamicClient.Resource(roleGVR).Namespace(namespace).Delete(ctx, fmt.Sprintf("tailvm-%s-proxy-role", name), metav1.DeleteOptions{})
	_ = cm.DynamicClient.Resource(rbGVR).Namespace(namespace).Delete(ctx, fmt.Sprintf("tailvm-%s-proxy-binding", name), metav1.DeleteOptions{})

	return nil
}

// SetProtection updates the deletion protection labels and annotations.
func (cm *ClientManager) SetProtection(name, namespace string, protect bool) error {
	ctx := context.TODO()

	patchVal := "false"
	if protect {
		patchVal = "true"
	}

	patchBytes := []byte(fmt.Sprintf(`{"metadata":{"labels":{"tailvm.io/protected":"%s"}}}`, patchVal))
	_, err := cm.DynamicClient.Resource(gvrVM).Namespace(namespace).Patch(ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	return err
}

// VM Power Actions
func (cm *ClientManager) StartVM(name, namespace string) error {
	ctx := context.TODO()
	patchBytes := []byte(`{"spec":{"runStrategy":"Always"}}`)
	_, err := cm.DynamicClient.Resource(gvrVM).Namespace(namespace).Patch(ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	return err
}

func (cm *ClientManager) StopVM(name, namespace string) error {
	ctx := context.TODO()
	patchBytes := []byte(`{"spec":{"runStrategy":"Halted"}}`)
	_, err := cm.DynamicClient.Resource(gvrVM).Namespace(namespace).Patch(ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	return err
}

func (cm *ClientManager) RestartVM(name, namespace string) error {
	ctx := context.TODO()
	// To restart KubeVirt VM, we can patch it to RerunOnFailure/Always or use KubeVirt subresources.
	// Since runStrategy: Always is set, we can delete the current VMI, and KubeVirt controller will automatically spawn a new one!
	// This is the most robust, API-version-independent way to restart a VM.
	err := cm.DynamicClient.Resource(gvrVMI).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		// VM is currently stopped, let's start it
		return cm.StartVM(name, namespace)
	}
	return err
}

// Utility function to convert arbitrary interface to int64 safely
func toInt64(val interface{}) (int64, error) {
	switch v := val.(type) {
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %v to int64", val)
	}
}

// Simple YAML to JSON converter using standard Kubernetes apimachinery utilities
func yamlToJSON(yamlStr string) string {
	jsonBytes, err := yaml.ToJSON([]byte(yamlStr))
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

func splitYAMLManifests(manifestsStr string) []string {
	var list []string
	parts := strings.Split(manifestsStr, "---")
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if len(trimmed) > 0 {
			list = append(list, trimmed)
		}
	}
	return list
}
