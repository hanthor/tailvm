package main

import (
	"io"
	"net/http"
	"regexp"
	"time"
)

// DistroCatalog defines an operating system template entry.
type DistroCatalog struct {
	ID           string
	Name         string
	Arch         string
	LatestURL    string
	FallbackURL  string
	DefaultCores int
	DefaultRAM   int
}

// GetCatalogItems returns the complete set of distro catalogs supported by tailvm.
func GetCatalogItems() []DistroCatalog {
	return []DistroCatalog{
		{
			ID:           "ubuntu-desktop",
			Name:         "Ubuntu XFCE Desktop (Noble 24.04)",
			Arch:         "amd64",
			LatestURL:    "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img",
			FallbackURL:  "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img",
			DefaultCores: 4,
			DefaultRAM:   8,
		},
		{
			ID:           "ubuntu-server",
			Name:         "Ubuntu Headless Server (Noble 24.04)",
			Arch:         "amd64",
			LatestURL:    "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img",
			FallbackURL:  "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img",
			DefaultCores: 4,
			DefaultRAM:   8,
		},
		{
			ID:           "fedora-server",
			Name:         "Fedora Server with Virt (Fedora 40)",
			Arch:         "amd64",
			LatestURL:    "https://download.fedoraproject.org/pub/fedora/linux/releases/40/Cloud/x86_64/images/Fedora-Cloud-Base-Generic.x86_64-40-1.14.qcow2",
			FallbackURL:  "https://download.fedoraproject.org/pub/fedora/linux/releases/40/Cloud/x86_64/images/Fedora-Cloud-Base-Generic.x86_64-40-1.14.qcow2",
			DefaultCores: 4,
			DefaultRAM:   8,
		},
		{
			ID:           "arch",
			Name:         "Arch Linux Developer Desktop",
			Arch:         "amd64",
			LatestURL:    "https://geo.mirror.pkgbuild.com/images/latest/Arch-Linux-x86_64-cloudimg.qcow2",
			FallbackURL:  "https://geo.mirror.pkgbuild.com/images/latest/Arch-Linux-x86_64-cloudimg.qcow2",
			DefaultCores: 4,
			DefaultRAM:   8,
		},
		{
			ID:           "nixos",
			Name:         "NixOS Minimal Cloud Server",
			Arch:         "amd64",
			LatestURL:    "https://channels.nixos.org/nixos-24.05/latest-nixos-amazon-x86_64.vhd",
			FallbackURL:  "https://channels.nixos.org/nixos-24.05/latest-nixos-amazon-x86_64.vhd",
			DefaultCores: 4,
			DefaultRAM:   8,
		},
		{
			ID:           "alpine",
			Name:         "Alpine Linux Micro Server (3.20)",
			Arch:         "amd64",
			LatestURL:    "https://alpine-cloud-images.s3.amazonaws.com/v3.20/alpine-latest-x86_64-bios.qcow2",
			FallbackURL:  "https://alpine-cloud-images.s3.amazonaws.com/v3.20/alpine-latest-x86_64-bios.qcow2",
			DefaultCores: 1,
			DefaultRAM:   1,
		},
		{
			ID:           "windows-10",
			Name:         "Windows 10 Pro (Hyper-V)",
			Arch:         "amd64",
			LatestURL:    "https://software-static.download.prss.microsoft.com/db3/win10byod/22H2/19045.2006.220908-0225.22h2_release_svc_refresh_CLIENTENTERPRISE_EVAL_x64FRE_en-us.iso",
			FallbackURL:  "https://software-static.download.prss.microsoft.com/db3/win10byod/22H2/19045.2006.220908-0225.22h2_release_svc_refresh_CLIENTENTERPRISE_EVAL_x64FRE_en-us.iso",
			DefaultCores: 4,
			DefaultRAM:   8,
		},
		{
			ID:           "windows-11",
			Name:         "Windows 11 Pro (SecureBoot & TPM v2.0)",
			Arch:         "amd64",
			LatestURL:    "https://software-static.download.prss.microsoft.com/db3/win11byod/23H2/22631.2428.231001-0608.23h2_release_svc_refresh_CLIENTENTERPRISE_EVAL_x64FRE_en-us.iso",
			FallbackURL:  "https://software-static.download.prss.microsoft.com/db3/win11byod/23H2/22631.2428.231001-0608.23h2_release_svc_refresh_CLIENTENTERPRISE_EVAL_x64FRE_en-us.iso",
			DefaultCores: 4,
			DefaultRAM:   8,
		},
		{
			ID:           "macos-sonoma",
			Name:         "macOS Sonoma (Sonoma 14)",
			Arch:         "amd64",
			LatestURL:    "https://github.com/kholia/OSX-KVM/raw/master/OpenCore/OpenCore.qcow2",
			FallbackURL:  "https://github.com/kholia/OSX-KVM/raw/master/OpenCore/OpenCore.qcow2",
			DefaultCores: 4,
			DefaultRAM:   8,
		},
		{
			ID:           "macos-ventura",
			Name:         "macOS Ventura (Ventura 13)",
			Arch:         "amd64",
			LatestURL:    "https://github.com/kholia/OSX-KVM/raw/master/OpenCore/OpenCore.qcow2",
			FallbackURL:  "https://github.com/kholia/OSX-KVM/raw/master/OpenCore/OpenCore.qcow2",
			DefaultCores: 4,
			DefaultRAM:   8,
		},
	}
}

// ResolveCatalogURL dynamically resolves the latest URL for an OS template using online mirror queries.
// It falls back to static URLs instantly if the mirror is slow or network is offline.
func ResolveCatalogURL(osID string) string {
	items := GetCatalogItems()
	var selected DistroCatalog
	found := false
	for _, item := range items {
		if item.ID == osID {
			selected = item
			found = true
			break
		}
	}
	if !found {
		return ""
	}

	// Dynamic resolvers based on distro patterns
	client := &http.Client{Timeout: 3 * time.Second}

	switch osID {
	case "ubuntu-desktop", "ubuntu-server":
		// Query the latest noble server image index
		resp, err := client.Get("https://cloud-images.ubuntu.com/noble/current/")
		if err == nil && resp.StatusCode == 200 {
			defer resp.Body.Close()
			bodyBytes, _ := io.ReadAll(resp.Body)
			bodyStr := string(bodyBytes)
			
			// Search for noble-server-cloudimg-amd64.img in the directory listing
			re := regexp.MustCompile(`href="([^"]*noble-server-cloudimg-amd64\.img)"`)
			matches := re.FindStringSubmatch(bodyStr)
			if len(matches) > 1 {
				return "https://cloud-images.ubuntu.com/noble/current/" + matches[1]
			}
		}

	case "arch":
		// Query geo mirror file directory to get latest build image
		resp, err := client.Get("https://geo.mirror.pkgbuild.com/images/latest/")
		if err == nil && resp.StatusCode == 200 {
			defer resp.Body.Close()
			bodyBytes, _ := io.ReadAll(resp.Body)
			bodyStr := string(bodyBytes)
			
			re := regexp.MustCompile(`href="([^"]*Arch-Linux-x86_64-cloudimg\.qcow2)"`)
			matches := re.FindStringSubmatch(bodyStr)
			if len(matches) > 1 {
				return "https://geo.mirror.pkgbuild.com/images/latest/" + matches[1]
			}
		}

	case "alpine":
		// Scrape to check if alpine latest qcow2 release versions updated
		resp, err := client.Get("https://alpine-cloud-images.s3.amazonaws.com/v3.20/")
		if err == nil && resp.StatusCode == 200 {
			defer resp.Body.Close()
			bodyBytes, _ := io.ReadAll(resp.Body)
			bodyStr := string(bodyBytes)
			
			re := regexp.MustCompile(`([^"]*alpine-latest-x86_64-bios\.qcow2)`)
			matches := re.FindAllString(bodyStr, -1)
			if len(matches) > 0 {
				// S3 bucket lists keys
				return "https://alpine-cloud-images.s3.amazonaws.com/v3.20/alpine-latest-x86_64-bios.qcow2"
			}
		}
	}

	// Fallback instantly if no custom scraper is present or if they encountered errors/timeout
	return selected.FallbackURL
}
