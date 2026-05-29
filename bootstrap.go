package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const (
	KubeVirtVersion = "v1.8.2"
	CDIVersion      = "v1.59.0"
)

// BootstrapCluster checks cluster state and optionally provisions missing operators.
func (cm *ClientManager) BootstrapCluster(dryRun bool) error {
	ctx := context.TODO()

	fmt.Println("🔍 Checking cluster prerequisites...")

	// 1. Check KubeVirt status
	kvInstalled := cm.checkCRDExists("virtualmachines.kubevirt.io")
	if kvInstalled {
		fmt.Println("✅ KubeVirt is already installed in the cluster.")
	} else {
		fmt.Println("⚠️  KubeVirt is NOT installed.")
		if dryRun {
			fmt.Println("👉 [Dry Run] Would install KubeVirt operator and custom resource.")
		} else {
			err := cm.installKubeVirt(ctx)
			if err != nil {
				return fmt.Errorf("failed to bootstrap KubeVirt: %v", err)
			}
		}
	}

	// 2. Check CDI status
	cdiInstalled := cm.checkCRDExists("datavolumes.cdi.kubevirt.io")
	if cdiInstalled {
		fmt.Println("✅ Containerized Data Importer (CDI) is already installed.")
	} else {
		fmt.Println("⚠️  Containerized Data Importer (CDI) is NOT installed.")
		if dryRun {
			fmt.Println("👉 [Dry Run] Would install CDI operator and custom resource.")
		} else {
			err := cm.installCDI(ctx)
			if err != nil {
				return fmt.Errorf("failed to bootstrap CDI: %v", err)
			}
		}
	}

	// 3. Check Tailscale Operator status
	tsInstalled := cm.checkNamespaceExists("tailscale")
	if tsInstalled {
		fmt.Println("✅ Tailscale operator namespace detected.")
	} else {
		fmt.Println("⚠️  Tailscale operator is NOT installed.")
		fmt.Println("💡 Tip: To enable automatic Tailscale exposure, please install the Tailscale Kubernetes Operator:")
		fmt.Println("   helm repo add tailscale https://pkgs.tailscale.com/helmcharts")
		fmt.Println("   helm repo update")
		fmt.Println("   helm install tailscale-operator tailscale/tailscale-operator --namespace tailscale --create-namespace")
	}

	return nil
}

// Helper: Check if a CRD is registered in the cluster.
func (cm *ClientManager) checkCRDExists(crdName string) bool {
	crdGVR := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	_, err := cm.DynamicClient.Resource(crdGVR).Get(context.TODO(), crdName, metav1.GetOptions{})
	return err == nil
}

// Helper: Check if a namespace exists.
func (cm *ClientManager) checkNamespaceExists(nsName string) bool {
	_, err := cm.Clientset.CoreV1().Namespaces().Get(context.TODO(), nsName, metav1.GetOptions{})
	return err == nil
}

// Downloads and applies manifests for KubeVirt
func (cm *ClientManager) installKubeVirt(ctx context.Context) error {
	fmt.Printf("📦 Downloading and applying KubeVirt %s Operator...\n", KubeVirtVersion)
	operatorURL := fmt.Sprintf("https://github.com/kubevirt/kubevirt/releases/download/%s/kubevirt-operator.yaml", KubeVirtVersion)
	crURL := fmt.Sprintf("https://github.com/kubevirt/kubevirt/releases/download/%s/kubevirt-cr.yaml", KubeVirtVersion)

	// Apply Operator
	err := cm.applyRemoteManifest(ctx, operatorURL)
	if err != nil {
		return err
	}

	fmt.Println("⏳ Waiting 15s for KubeVirt Operator to initialize...")
	time.Sleep(15 * time.Second)

	// Apply Custom Resource
	fmt.Println("📦 Downloading and applying KubeVirt Custom Resource...")
	return cm.applyRemoteManifest(ctx, crURL)
}

// Downloads and applies manifests for CDI
func (cm *ClientManager) installCDI(ctx context.Context) error {
	fmt.Printf("📦 Downloading and applying CDI %s Operator...\n", CDIVersion)
	operatorURL := fmt.Sprintf("https://github.com/kubevirt/containerized-data-importer/releases/download/%s/cdi-operator.yaml", CDIVersion)
	crURL := fmt.Sprintf("https://github.com/kubevirt/containerized-data-importer/releases/download/%s/cdi-cr.yaml", CDIVersion)

	// Apply Operator
	err := cm.applyRemoteManifest(ctx, operatorURL)
	if err != nil {
		return err
	}

	fmt.Println("⏳ Waiting 15s for CDI Operator to initialize...")
	time.Sleep(15 * time.Second)

	// Apply Custom Resource
	fmt.Println("📦 Downloading and applying CDI Custom Resource...")
	return cm.applyRemoteManifest(ctx, crURL)
}

// Helper: Downloads a YAML from a URL, splits it, converts it to JSON, and applies it to the cluster dynamically.
func (cm *ClientManager) applyRemoteManifest(ctx context.Context, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch manifest: %v", err)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	manifests := splitYAMLManifests(string(data))
	for _, m := range manifests {
		if len(strings.TrimSpace(m)) == 0 {
			continue
		}

		var obj unstructured.Unstructured
		jsonStr := yamlToJSON(m)
		if jsonStr == "{}" {
			continue
		}

		if err := json.Unmarshal([]byte(jsonStr), &obj.Object); err != nil {
			// Skip comments or parsing anomalies
			continue
		}

		gvk := obj.GroupVersionKind()
		gvr := schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: strings.ToLower(gvk.Kind) + "s",
		}

		// Handle plurals and exceptions in Kubernetes Resource mapping
		if gvk.Kind == "ServiceAccount" {
			gvr.Resource = "serviceaccounts"
		} else if gvk.Kind == "Role" {
			gvr.Resource = "roles"
		} else if gvk.Kind == "RoleBinding" {
			gvr.Resource = "rolebindings"
		} else if gvk.Kind == "ClusterRole" {
			gvr.Resource = "clusterroles"
		} else if gvk.Kind == "ClusterRoleBinding" {
			gvr.Resource = "clusterrolebindings"
		} else if gvk.Kind == "CustomResourceDefinition" {
			gvr.Resource = "customresourcedefinitions"
		} else if gvk.Kind == "Deployment" {
			gvr.Resource = "deployments"
		} else if gvk.Kind == "DaemonSet" {
			gvr.Resource = "daemonsets"
		} else if gvk.Kind == "ConfigMap" {
			gvr.Resource = "configmaps"
		} else if gvk.Kind == "PriorityClass" {
			gvr.Resource = "priorityclasses"
		} else if gvk.Kind == "APIService" {
			gvr.Resource = "apiservices"
			gvr.Group = "apiregistration.k8s.io"
		} else if gvk.Kind == "ValidatingWebhookConfiguration" {
			gvr.Resource = "validatingwebhookconfigurations"
		} else if gvk.Kind == "MutatingWebhookConfiguration" {
			gvr.Resource = "mutatingwebhookconfigurations"
		} else if gvk.Kind == "KubeVirt" {
			gvr.Resource = "kubevirts"
		} else if gvk.Kind == "CDI" {
			gvr.Resource = "cdis"
		}

		// Determine if resource is cluster-scoped or namespace-scoped
		resourceClient := cm.DynamicClient.Resource(gvr)
		var namespacedClient dynamic.ResourceInterface = resourceClient

		ns := obj.GetNamespace()
		if ns != "" {
			namespacedClient = resourceClient.Namespace(ns)
		}

		_, err = namespacedClient.Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				// Object exists, let's update it by fetching the resource version
				existing, getErr := namespacedClient.Get(ctx, obj.GetName(), metav1.GetOptions{})
				if getErr == nil {
					obj.SetResourceVersion(existing.GetResourceVersion())
					_, updateErr := namespacedClient.Update(ctx, &obj, metav1.UpdateOptions{})
					if updateErr != nil {
						fmt.Printf("[Update-Error] failed to update %s %s: %v\n", gvk.Kind, obj.GetName(), updateErr)
					}
				}
			} else {
				fmt.Printf("[Apply-Error] failed to create %s %s: %v\n", gvk.Kind, obj.GetName(), err)
			}
		}
	}
	return nil
}
