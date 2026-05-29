# Issue: Port Quickemu Windows VM Templates & Testing Plan

## 📌 Goal
Implement KubeVirt templates for **Windows 10** and **Windows 11** based on **Quickemu** specifications, enabling optimized performance, full TPM v2.0 emulation, UEFI firmware bootloaders, and integrated VirtIO drivers. Establish a testing plan for Windows configurations.

## 🛠️ Design & Requirements

### 1. KubeVirt Windows Template Specifications
To run Windows 10/11 efficiently on KubeVirt, the VM manifest must incorporate Quickemu-equivalent configurations:
- **UEFI & SMM**: Windows requires UEFI bootloader. SMM (System Management Mode) must be enabled:
  ```yaml
  spec:
    template:
      spec:
        domain:
          firmware:
            bootloader:
              efi:
                secureBoot: true
          features:
            smm:
              enabled: true
  ```
- **TPM v2.0 Emulation (Windows 11)**: Enable software TPM emulation in KubeVirt:
  ```yaml
  spec:
    template:
      spec:
        domain:
          devices:
            tpm: {}
  ```
- **Hyper-V Enlightenments**: Inject CPU hyper-v extensions to significantly improve Windows guest performance:
  - `relaxed`, `vapic`, `spinlocks` (threshold: 8191), `vpindex`, `runtime`, `synic`, `synictimer`, `reset`, `vendorid` (custom ID).

### 2. VirtIO Drivers Auto-Mount
- Windows installers do not natively package VirtIO storage and network drivers.
- We must automatically download the Fedora VirtIO Win ISO (`virtio-win.iso`) and mount it as a secondary CD-ROM drive (`bootOrder: 2` or `bus: sata`) alongside the installation media, allowing Windows to load drivers during setup.

---

## 🧪 Testing Plan

### 1. Unit Tests (`templates_windows_test.go`)
- **Firmware Asserts**: Verify that SMM is enabled and UEFI bootloaders are present in generated Windows templates.
- **TPM Verification**: Assert that Windows 11 templates include the `tpm` device, whereas Windows 10 templates can omit it.
- **Hyper-V Feature Auditing**: Assert that all critical Hyper-V performance extensions are enabled.
- **Driver ISO Presence**: Assert that the `virtio-win` disk is present and correctly mapped as a SATA CD-ROM.

### 2. Integration Tests (`integration_windows_test.go`)
- **VirtIO Win ISO Availability**: Run a test validating the Fedora VirtIO Win ISO download URL is active and valid.
- **Spec Verification**: Trigger a Dry-Run VM apply to the cluster API and verify KubeVirt validates the SMM, UEFI, and TPM configurations successfully.
- **Driver Mounting Verification**: Spin up a mock Windows container/pod and assert the secondary SATA driver disk mount is detected in the VM manifest lifecycle.

---

## 📋 Sub-Issues & Granular Tasks Checklist

- [ ] **Sub-Issue 5.1: Windows 10 & 11 Template Construction**
  - [ ] Implement Windows 10 base template structure with standard SATA CD-ROM mounts.
  - [ ] Implement Windows 11 base template incorporating software TPM v2.0 emulation blocks.
  - [ ] Add UEFI bootloader configuration with SecureBoot parameters and SMM feature mappings.
- [ ] **Sub-Issue 5.2: Hyper-V Enlightenment Feature Set Integration**
  - [ ] Map CPU extenders: relaxed, vapic, spinlocks (8191), vpindex, runtime, synic, synictimer, reset, vendorid.
- [ ] **Sub-Issue 5.3: VirtIO ISO Dynamic Mount Engine**
  - [ ] Implement Fedora VirtIO Win ISO resolver hook.
  - [ ] Auto-inject secondary disk block utilizing AHCI/SATA bus.
- [ ] **Sub-Issue 5.4: Unit & Integration Tests Implementation**
  - [ ] Implement `templates_windows_test.go` verifying SMM, UEFI, TPM, and SATA drivers configurations.
  - [ ] Verify KubeVirt dry-run API validation passes.

