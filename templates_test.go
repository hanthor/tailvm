package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGetOSPresets verifies default OS presets exist and carry correct values.
func TestGetOSPresets(t *testing.T) {
	presets := GetOSPresets()
	if len(presets) < 6 {
		t.Fatalf("expected at least 6 OS presets, got %d", len(presets))
	}

	foundDesktop := false
	foundArch := false
	foundNixOS := false
	foundAlpine := false

	for _, p := range presets {
		switch p.ID {
		case "ubuntu-desktop":
			foundDesktop = true
			if p.DefaultUser != "james" {
				t.Errorf("expected default user to be james, got %s", p.DefaultUser)
			}
		case "arch":
			foundArch = true
			if p.DefaultUser != "arch" {
				t.Errorf("expected default user to be arch, got %s", p.DefaultUser)
			}
		case "nixos":
			foundNixOS = true
			if p.DefaultUser != "nixos" {
				t.Errorf("expected default user to be nixos, got %s", p.DefaultUser)
			}
		case "alpine":
			foundAlpine = true
			if p.DefaultUser != "alpine" {
				t.Errorf("expected default user to be alpine, got %s", p.DefaultUser)
			}
		}
	}

	if !foundDesktop {
		t.Error("expected to find ubuntu-desktop preset")
	}
	if !foundArch {
		t.Error("expected to find arch preset")
	}
	if !foundNixOS {
		t.Error("expected to find nixos preset")
	}
	if !foundAlpine {
		t.Error("expected to find alpine preset")
	}
}

// TestGenerateCloudInit verifies cloud-init configuration formats.
func TestGenerateCloudInit(t *testing.T) {
	presets := GetOSPresets()
	preset := presets[0] // Ubuntu Desktop
	sshKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIG12345 james@test"

	userdata := GenerateCloudInit("james", sshKey, preset)

	if !strings.HasPrefix(userdata, "#cloud-config") {
		t.Error("expected userdata to start with #cloud-config")
	}

	if !strings.Contains(userdata, "name: james") {
		t.Error("expected userdata to contain name: james")
	}

	if !strings.Contains(userdata, sshKey) {
		t.Error("expected userdata to contain the public SSH key")
	}

	if !strings.Contains(userdata, "tigervnc-standalone-server") {
		t.Error("expected userdata to contain packages like tigervnc")
	}
}

// TestGenerateVMManifest verifies VM manifest creation for both DataVolumes and HostDisks.
// TestGenerateVMManifest verifies VM manifest creation for both DataVolumes and HostDisks.
func TestGenerateVMManifest(t *testing.T) {
	name := "test-noble"
	namespace := "default"
	cores := 4
	memoryGB := 8
	cloudInit := "cloud-init-data"

	// 1. Test CDI DataVolume pathway
	useDataVolume := true
	diskSrc := "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img"

	manifestYAML := GenerateVMManifest(name, namespace, cores, memoryGB, true, useDataVolume, diskSrc, cloudInit, "ubuntu-desktop")

	if !strings.Contains(manifestYAML, "kind: VirtualMachine") {
		t.Error("expected manifest to be of kind VirtualMachine")
	}

	if !strings.Contains(manifestYAML, "tailvm.io/protected: \"true\"") {
		t.Error("expected manifest to have protection label enabled")
	}

	if !strings.Contains(manifestYAML, "model: host-passthrough") {
		t.Error("expected CPU model host-passthrough")
	}

	if !strings.Contains(manifestYAML, "dataVolume:") {
		t.Error("expected manifest to map dataVolume volume source")
	}

	if !strings.Contains(manifestYAML, "dataVolumeTemplates:") {
		t.Error("expected manifest to contain CDI dataVolumeTemplates definition")
	}

	// 2. Test HostDisk fallback pathway
	useDataVolume = false
	diskPath := "/var/tmp/test-noble.raw"

	manifestHostDiskYAML := GenerateVMManifest(name, namespace, cores, memoryGB, false, useDataVolume, diskPath, cloudInit, "ubuntu-desktop")

	if !strings.Contains(manifestHostDiskYAML, "tailvm.io/protected: \"false\"") {
		t.Error("expected manifest to have protection label disabled")
	}

	if !strings.Contains(manifestHostDiskYAML, "hostDisk:") {
		t.Error("expected manifest to map hostDisk volume source")
	}

	if !strings.Contains(manifestHostDiskYAML, "path: /var/tmp/test-noble.raw") {
		t.Error("expected manifest to contain the custom HostDisk file path")
	}

	if strings.Contains(manifestHostDiskYAML, "dataVolumeTemplates:") {
		t.Error("expected manifest to omit CDI templates in HostDisk mode")
	}
}

// TestGenerateVMManifestWindows verifies UEFI, SMM, TPM, and Hyper-V settings for Windows.
func TestGenerateVMManifestWindows(t *testing.T) {
	name := "win-pet"
	namespace := "default"
	cores := 4
	memoryGB := 8

	// Test Windows 11 (requires TPM)
	manifest11 := GenerateVMManifest(name, namespace, cores, memoryGB, true, true, "http://fake-win11-iso", "", "windows-11")

	if !strings.Contains(manifest11, "smm:") {
		t.Error("expected Windows 11 template to enable System Management Mode (SMM)")
	}

	if !strings.Contains(manifest11, "efi:") || !strings.Contains(manifest11, "secureBoot: true") {
		t.Error("expected Windows 11 template to enable secure UEFI boot")
	}

	if !strings.Contains(manifest11, "tpm: {}") {
		t.Error("expected Windows 11 template to enable Software TPM 2.0 emulation")
	}

	if !strings.Contains(manifest11, "hyperv:") || !strings.Contains(manifest11, "spinlocks:") {
		t.Error("expected Windows 11 template to define Hyper-V performance enlightenments")
	}

	if !strings.Contains(manifest11, "virtio-drivers") || !strings.Contains(manifest11, "quay.io/kubevirt/virtio-container-disk") {
		t.Error("expected Windows template to mount the KubeVirt VirtIO drivers container disk")
	}

	// Test Windows 10 (does NOT require TPM)
	manifest10 := GenerateVMManifest(name, namespace, cores, memoryGB, true, true, "http://fake-win10-iso", "", "windows-10")
	if strings.Contains(manifest10, "tpm: {}") {
		t.Error("expected Windows 10 template to omit Software TPM 2.0 emulation")
	}
}

// TestGenerateVMManifestMac verifies custom QEMU args, OSK key, and SATA disk configs for macOS.
func TestGenerateVMManifestMac(t *testing.T) {
	name := "mac-pet"
	namespace := "default"
	cores := 4
	memoryGB := 8

	manifest := GenerateVMManifest(name, namespace, cores, memoryGB, true, true, "http://fake-mac-opencore", "", "macos-sonoma")

	if !strings.Contains(manifest, "qemu:") || !strings.Contains(manifest, "commandline:") {
		t.Error("expected macOS template to define raw QEMU command-line configurations")
	}

	if !strings.Contains(manifest, "isa-applesmc,osk=ourhardcodedkeyisourhardcodedkeyis(c)AppleComputerInc") {
		t.Error("expected macOS template to pass the official Apple SMC OSK key")
	}

	if !strings.Contains(manifest, "Penryn") {
		t.Error("expected macOS template to override CPU model to Penryn")
	}

	if !strings.Contains(manifest, "bus: sata") {
		t.Error("expected macOS template to override rootdisk bus controller to SATA")
	}

	if strings.Contains(manifest, "cloudinitdisk") {
		t.Error("expected macOS template to omit cloud-init configurations completely")
	}
}

// TestGenerateProxyManifests verifies that exact proxy structures and ports are generated.
func TestGenerateProxyManifests(t *testing.T) {
	name := "test-vm"
	namespace := "default"

	proxyYAML := GenerateProxyManifests(name, namespace)

	// Verify all five resources are created by splitting and checking count
	parts := splitYAMLManifests(proxyYAML)
	if len(parts) != 5 {
		t.Errorf("expected exactly 5 YAML documents for the proxy registry, got %d", len(parts))
	}

	// Verify crucial ports
	if !strings.Contains(proxyYAML, "port: 22") {
		t.Error("expected proxy service to expose SSH port 22")
	}

	if !strings.Contains(proxyYAML, "port: 5900") {
		t.Error("expected proxy service to expose VNC port 5900")
	}

	if !strings.Contains(proxyYAML, "port: 80") {
		t.Error("expected proxy service to expose noVNC Web VNC port 80")
	}

	if !strings.Contains(proxyYAML, "websockify --web=/usr/share/novnc 8080") {
		t.Error("expected proxy deployment to start websockify mapping to novnc static files")
	}
}

// TestDetectLocalSSHKeys verifies SSH public key detection logic.
func TestDetectLocalSSHKeys(t *testing.T) {
	// Setup temporary home directory mocking
	tempHome := t.TempDir()
	
	// Backup original Home
	origHome := os.Getenv("HOME")
	defer func() {
		_ = os.Setenv("HOME", origHome)
	}()

	// Override Home env
	_ = os.Setenv("HOME", tempHome)

	// 1. Verify nothing is returned when no keys exist
	noKey := DetectLocalSSHKeys()
	if noKey != "" {
		t.Errorf("expected empty string when no keys exist, got %s", noKey)
	}

	// 2. Create mock SSH folder and key
	sshDir := filepath.Join(tempHome, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatal(err)
	}

	mockKeyContent := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIG12345 james@test"
	keyPath := filepath.Join(sshDir, "id_ed25519.pub")
	if err := os.WriteFile(keyPath, []byte(mockKeyContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Verify dynamic detection discovers it
	detectedKey := DetectLocalSSHKeys()
	if detectedKey != mockKeyContent {
		t.Errorf("expected detected key to match mock content, got %s", detectedKey)
	}
}

// TestGenerateNamespaceConfigMapManifest verifies warning policy ConfigMaps.
func TestGenerateNamespaceConfigMapManifest(t *testing.T) {
	namespace := "default"
	manifest := GenerateNamespaceConfigMapManifest(namespace)

	if !strings.Contains(manifest, "kind: ConfigMap") {
		t.Error("expected configmap kind")
	}

	if !strings.Contains(manifest, "name: tailvm-protection-policy") {
		t.Error("expected policy name")
	}

	if !strings.Contains(manifest, "TAILVM PROTECTION POLICY") {
		t.Error("expected policy body text")
	}
}
