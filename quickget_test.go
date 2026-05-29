package main

import (
	"strings"
	"testing"
)

func TestGetCatalogItems(t *testing.T) {
	items := GetCatalogItems()
	if len(items) < 10 {
		t.Fatalf("expected at least 10 catalog distro items, got %d", len(items))
	}

	foundUbuntu := false
	foundWin11 := false
	foundMac := false
	foundAlpine := false

	for _, item := range items {
		switch item.ID {
		case "ubuntu-desktop":
			foundUbuntu = true
			if item.DefaultRAM != 8 {
				t.Errorf("expected 8GB RAM for Ubuntu, got %d", item.DefaultRAM)
			}
		case "windows-11":
			foundWin11 = true
			if item.DefaultCores != 4 {
				t.Errorf("expected 4 CPU Cores for Win 11, got %d", item.DefaultCores)
			}
		case "macos-sonoma":
			foundMac = true
			if item.Arch != "amd64" {
				t.Errorf("expected amd64 architecture for macOS Sonoma, got %s", item.Arch)
			}
		case "alpine":
			foundAlpine = true
			if item.DefaultRAM != 1 {
				t.Errorf("expected 1GB RAM for Alpine, got %d", item.DefaultRAM)
			}
		}
	}

	if !foundUbuntu {
		t.Error("failed to find ubuntu-desktop in catalog")
	}
	if !foundWin11 {
		t.Error("failed to find windows-11 in catalog")
	}
	if !foundMac {
		t.Error("failed to find macos-sonoma in catalog")
	}
	if !foundAlpine {
		t.Error("failed to find alpine in catalog")
	}
}

func TestResolveCatalogURLFallback(t *testing.T) {
	// Test stable fast fallback resolution
	win11URL := ResolveCatalogURL("windows-11")
	if win11URL == "" {
		t.Error("expected windows-11 URL to be resolved successfully")
	}
	if !strings.Contains(win11URL, "win11byod") {
		t.Errorf("expected windows-11 URL to contain microsoft download path, got %s", win11URL)
	}

	macURL := ResolveCatalogURL("macos-sonoma")
	if macURL == "" {
		t.Error("expected macOS Sonoma URL to be resolved successfully")
	}
	if !strings.Contains(macURL, "OSX-KVM") {
		t.Errorf("expected macOS Sonoma URL to point to OSX-KVM boot OpenCore image, got %s", macURL)
	}

	// Test invalid ID
	invalid := ResolveCatalogURL("invalid-preset-id")
	if invalid != "" {
		t.Errorf("expected empty string for invalid preset ID, got %s", invalid)
	}
}
