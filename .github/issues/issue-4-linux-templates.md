# Issue: Port Quickemu Linux VM Templates & Testing Plan

## 📌 Goal
Create a rich catalog of Linux VM templates inside `tailvm` inspired by **Quickemu**, allowing users to spin up optimized Linux desktops and servers (Ubuntu, Debian, Fedora, Arch Linux, NixOS, Alpine) on KubeVirt. Provide a dedicated unit and integration testing plan for these templates.

## 🛠️ Design & Requirements

### 1. Template Presets & Specifications
Port configuration specifications from Quickemu for the following Linux distributions:
- **Arch Linux / Manjaro**:
  - High-performance disk configurations (`virtio-blk` or `virtio-scsi`).
  - Tablet input pointer (`inputs.type="tablet"`) for absolute cursor positioning in VNC.
- **NixOS**:
  - Support for custom configuration channel hooks.
  - Setup cloud-init or custom hardware profiles suitable for virtual environments.
- **Alpine Linux**:
  - Ultra-lightweight resource footprint (512MB RAM, 1 Core).
  - Pre-configured serial console access.
- **Debian / Ubuntu / Fedora**:
  - Dual configurations: Headless server (minimal cloud-init) and Desktop environment (XFCE or native desktop).

### 2. Dynamic CDI HTTP URL Mapping
- Map each distribution to its official stable cloud image or network installation ISO URL.
- Use CDI `DataVolumes` to automate the download, decompression, and conversion of QCOW2/ISO formats directly into the VM persistent volumes.

---

## 🧪 Testing Plan

### 1. Unit Tests (`templates_linux_test.go`)
- **XML/YAML Syntax Validation**: Assert that every generated Linux VM manifest is structurally valid Kubernetes YAML.
- **Resource Limits Verification**: Assert that Alpine uses < 1GB RAM, while Arch/NixOS Desktops are allocated appropriate defaults (>= 4GB RAM, >= 2 Cores).
- **Disk Bus Types**: Verify that `virtio` is enforced for all Linux disk buses to maximize performance.
- **Tablet Device Presence**: Assert that VNC input tablet devices are included in the spec for graphical environments to prevent cursor desynchronization.

### 2. Integration Tests (`integration_linux_test.go`)
- **CDI Download Verification**: Trigger dry-runs of the dynamic CDI http sources to verify upstream URLs are reachable and return valid headers.
- **Boot Assertions**: Deploy a test Alpine VM and test NixOS VM in a real development cluster. Poll the status and verify that they reach `Running` and the guest agent successfully connects.

---

## 📋 Sub-Issues & Granular Tasks Checklist

- [ ] **Sub-Issue 4.1: NixOS Template Implementation**
  - [ ] Write NixOS KubeVirt spec definitions and default resources.
  - [ ] Add support for custom Nix config channels and cloud-init hook files.
- [ ] **Sub-Issue 4.2: Arch Linux & Alpine Presets**
  - [ ] Implement Alpine Server ultra-lightweight profile (512MB RAM).
  - [ ] Implement Arch Linux high-performance profile (Tablet cursor pointers, virtio-scsi).
- [ ] **Sub-Issue 4.3: Unit Test Suite Implementation**
  - [ ] Implement `templates_linux_test.go` and assert valid XML/YAML structures for NixOS and Arch.
  - [ ] Assert correct tablet inputs and virtio buses.
- [ ] **Sub-Issue 4.4: Integration Tests on Live Cluster**
  - [ ] Deploy test Alpine and NixOS instances.
  - [ ] Verify HTTP URL resolution via CDI.
  - [ ] Poll and assert they transition to online status successfully.

