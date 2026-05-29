package main

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientsetfake "k8s.io/client-go/kubernetes/fake"
)

// Helper: Generates a mock unstructured KubeVirt VM object for the fake dynamic client.
func newFakeVM(name, namespace, status string, isProtected bool) *unstructured.Unstructured {
	protectionLabel := "false"
	if isProtected {
		protectionLabel = "true"
	}

	rawVM := map[string]interface{}{
		"apiVersion": "kubevirt.io/v1",
		"kind":       "VirtualMachine",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"labels": map[string]interface{}{
				"app":                  "tailvm",
				"tailvm.io/protected": protectionLabel,
				"tailvm.io/pet":       "true",
			},
			"creationTimestamp": "2026-05-28T12:00:00Z",
		},
		"spec": map[string]interface{}{
			"runStrategy": "Always",
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"domain": map[string]interface{}{
						"cpu": map[string]interface{}{
							"cores": int64(4),
						},
						"memory": map[string]interface{}{
							"guest": "8Gi",
						},
					},
				},
			},
		},
		"status": map[string]interface{}{
			"printableStatus": status,
			"ready":           true,
		},
	}

	return &unstructured.Unstructured{Object: rawVM}
}

// Helper: Generates a mock unstructured KubeVirt VMI object for the fake dynamic client.
func newFakeVMI(name, namespace, ip, node string) *unstructured.Unstructured {
	rawVMI := map[string]interface{}{
		"apiVersion": "kubevirt.io/v1",
		"kind":       "VirtualMachineInstance",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"status": map[string]interface{}{
			"interfaces": []interface{}{
				map[string]interface{}{
					"ipAddress": ip,
				},
			},
			"nodeName": node,
		},
	}
	return &unstructured.Unstructured{Object: rawVMI}
}

// TestListVMs validates VM querying and status parsing via the dynamic fake client.
func TestListVMs(t *testing.T) {
	scheme := runtime.NewScheme()
	
	// Create mock unstructured resources
	vm1 := newFakeVM("noble-pet", "default", "Running", true)
	vm2 := newFakeVM("fedora-test", "default", "Stopped", false)
	vmi1 := newFakeVMI("noble-pet", "default", "10.244.1.100", "bihar")

	// Initialize fake dynamic client pre-populated with mock resources
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme, vm1, vm2, vmi1)
	clientset := clientsetfake.NewSimpleClientset()

	cm := &ClientManager{
		DynamicClient: dynClient,
		Clientset:     clientset,
		ContextName:   "fake-context",
	}

	// List VMs
	list, err := cm.ListVMs("default")
	if err != nil {
		t.Fatalf("failed to list VMs: %v", err)
	}

	if len(list) != 2 {
		t.Fatalf("expected exactly 2 VMs, got %d", len(list))
	}

	// Verify noble-pet details
	var noble VMInfo
	foundNoble := false
	for _, vm := range list {
		if vm.Name == "noble-pet" {
			noble = vm
			foundNoble = true
		}
	}

	if !foundNoble {
		t.Fatal("failed to find noble-pet in list")
	}

	if noble.Status != "Running" {
		t.Errorf("expected status Running, got %s", noble.Status)
	}

	if noble.IP != "10.244.1.100" {
		t.Errorf("expected IP 10.244.1.100, got %s", noble.IP)
	}

	if noble.HostNode != "bihar" {
		t.Errorf("expected hostNode bihar, got %s", noble.HostNode)
	}

	if !noble.Protected {
		t.Error("expected noble-pet to be flagged as protected")
	}

	// Verify fedora-test details
	foundFedora := false
	for _, vm := range list {
		if vm.Name == "fedora-test" {
			foundFedora = true
			if vm.Protected {
				t.Error("expected fedora-test to be unprotected")
			}
			if vm.Status != "Stopped" {
				t.Errorf("expected status Stopped, got %s", vm.Status)
			}
			if vm.IP != "N/A" {
				t.Errorf("expected IP to be N/A, got %s", vm.IP)
			}
		}
	}

	if !foundFedora {
		t.Error("failed to find fedora-test in list")
	}
}

// TestCreateVMAndProxy asserts full resource mutations upon VM creation.
func TestCreateVMAndProxy(t *testing.T) {
	scheme := runtime.NewScheme()
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme)
	clientset := clientsetfake.NewSimpleClientset()

	cm := &ClientManager{
		DynamicClient: dynClient,
		Clientset:     clientset,
		ContextName:   "fake-context",
	}

	name := "noble-pet"
	namespace := "default"
	cores := 4
	memoryGB := 8
	isProtected := true
	useDataVolume := false
	diskSrc := "/var/tmp/noble-pet.raw"
	cloudInit := "cloud-init-data"

	// Create VM
	err := cm.CreateVM(name, namespace, cores, memoryGB, isProtected, useDataVolume, diskSrc, cloudInit)
	if err != nil {
		t.Fatalf("failed to create VM: %v", err)
	}

	// 1. Verify VirtualMachine is successfully created in fake client
	vm, err := dynClient.Resource(gvrVM).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to find created VM in fake client: %v", err)
	}

	labels := vm.GetLabels()
	if labels["tailvm.io/protected"] != "true" {
		t.Errorf("expected protection label true, got %s", labels["tailvm.io/protected"])
	}

	// 2. Verify ConfigMap policy is created
	cmGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	policyCM, err := dynClient.Resource(cmGVR).Namespace(namespace).Get(context.TODO(), "tailvm-protection-policy", metav1.GetOptions{})
	if err != nil {
		t.Errorf("expected tailvm-protection-policy ConfigMap to be created: %v", err)
	} else {
		data, _, _ := unstructured.NestedStringMap(policyCM.Object, "data")
		if !strings.Contains(data["POLICY.md"], "TAILVM PROTECTION POLICY") {
			t.Error("expected POLICY.md data in ConfigMap")
		}
	}
}

// TestSetProtection verifies dynamic label patching.
func TestSetProtection(t *testing.T) {
	scheme := runtime.NewScheme()
	vm1 := newFakeVM("noble-pet", "default", "Running", false)
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme, vm1)

	cm := &ClientManager{
		DynamicClient: dynClient,
		Clientset:     clientsetfake.NewSimpleClientset(),
		ContextName:   "fake-context",
	}

	// Check initial protection status is false
	vm, _ := dynClient.Resource(gvrVM).Namespace("default").Get(context.TODO(), "noble-pet", metav1.GetOptions{})
	if vm.GetLabels()["tailvm.io/protected"] != "false" {
		t.Fatal("expected initial protection to be false")
	}

	// Set protection to true
	err := cm.SetProtection("noble-pet", "default", true)
	if err != nil {
		t.Fatalf("failed to set protection: %v", err)
	}

	// Verify label is updated via fake dynamic patch
	vm, _ = dynClient.Resource(gvrVM).Namespace("default").Get(context.TODO(), "noble-pet", metav1.GetOptions{})
	if vm.GetLabels()["tailvm.io/protected"] != "true" {
		t.Errorf("expected protection label to be updated to true, got %s", vm.GetLabels()["tailvm.io/protected"])
	}
}

// TestDeleteVM verifies cleanup mutations across resources.
func TestDeleteVM(t *testing.T) {
	scheme := runtime.NewScheme()
	vm1 := newFakeVM("noble-pet", "default", "Running", true)
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme, vm1)
	clientset := clientsetfake.NewSimpleClientset()

	cm := &ClientManager{
		DynamicClient: dynClient,
		Clientset:     clientset,
		ContextName:   "fake-context",
	}

	// 1. Trigger Deletion
	err := cm.DeleteVM("noble-pet", "default")
	if err != nil {
		t.Fatalf("failed to delete VM: %v", err)
	}

	// 2. Assert VM is deleted
	_, err = dynClient.Resource(gvrVM).Namespace("default").Get(context.TODO(), "noble-pet", metav1.GetOptions{})
	if err == nil {
		t.Error("expected VM noble-pet to be deleted from fake client")
	}
}
