# Issue: Port Quickemu macOS VM Templates & Testing Plan

## 📌 Goal
Configure KubeVirt templates for **macOS** (Sonoma, Ventura, Monterey) based on **Quickemu**'s highly optimized macOS emulator designs. This requires custom QEMU command-line integration, OpenCore boot image mapping, Penryn CPU compatibility overrides, and custom display properties. Establish a testing plan for macOS templates.

## 🛠️ Design & Requirements

### 1. Custom QEMU Command-Line Arguments
KubeVirt supports injecting raw QEMU command-line arguments using the `qemu` namespace schema in the domain spec (which requires the `QEMUCmdHook` feature gate enabled in the `kubevirt` configuration):
```yaml
spec:
  template:
    spec:
      domain:
        qemu:
          commandline:
            - args:
                - "-device"
                - "isa-applesmc,osk=ourhardcodedkey..."
                - "-cpu"
                - "Penryn,kvm=on,vendor=GenuineIntel,vmware-cpuid-freq=on..."
```
We must port Quickemu's exact arguments:
- **OSK Key**: Inject the 64-character standard Apple SMC OSK key.
- **Apple Devices**: Map `isa-applesmc`, `ich9-intel-hda` audio, and optimized USB tablets.
- **CPU Overrides**: Force the CPU model to `Penryn` (or compatible Intel topology with specific flags like `sse4.2`, `popcnt`, `hypervisor` for AMD compatibilities).

### 2. Bootloader and Disk Configurations
- **OpenCore**: macOS requires the OpenCore EFI bootloader. We must mount the OpenCore image as a virtual `sata` disk (`bootOrder: 1`).
- **Disk controller**: macOS does not natively support standard `virtio-blk` disks without additional OpenCore kexts. We configure SATA/AHCI controller or NVMe emulation (`devices.disks.disk.bus: sata` or `nvme`) to maximize stability during macOS installation.

---

## 🧪 Testing Plan

### 1. Unit Tests (`templates_macos_test.go`)
- **QEMU Namespace Validation**: Verify that the generated macOS manifest utilizes the correct `qemu` XML namespace schema.
- **OSK Key Injection**: Assert that the 64-character Apple SMC OSK key is included in the QEMU arguments.
- **Penryn CPU Asserts**: Assert that the guest CPU is explicitly overridden to `Penryn` with appropriate extensions.
- **Bus Types Validation**: Verify that disks utilize the `sata` or `nvme` bus instead of the default `virtio`.

### 2. Integration Tests (`integration_macos_test.go`)
- **QEMU Hook Feature Gate Validation**: Assert that the target cluster's `KubeVirt` custom configuration has the `QEMUCmdHook` feature gate active (required to accept custom QEMU arguments).
- **Dry-Run Compilation**: Validate that the Kubernetes API accepts the `qemu` spec block and compiled YAML without returning validation hook errors.

---

## 📋 Sub-Issues & Granular Tasks Checklist

- [ ] **Sub-Issue 6.1: Custom QEMU Commandline Integration**
  - [ ] Implement macOS XML namespaces wrapper for domain specification.
  - [ ] Add the 64-character Apple SMC OSK key argument loader.
  - [ ] Configure `isa-applesmc` and Ich9 audio devices bindings.
- [ ] **Sub-Issue 6.2: Penryn CPU overrides Mapping**
  - [ ] Configure custom `Penryn` CPU models with SSE4.2, Popcnt, VMware cpuid-freq extensions.
- [ ] **Sub-Issue 6.3: Bootloader EFI and OpenCore sata drive mappings**
  - [ ] Add OpenCore AHCI/SATA SATA disk mapping templates (`sata` or `nvme` bus types).
- [ ] **Sub-Issue 6.4: Unit & Integration Tests Implementation**
  - [ ] Implement `templates_macos_test.go` asserting correct QEMU commandline structures, OSK keys, Penryn models, and SATA buses.
  - [ ] Implement `integration_macos_test.go` checking feature gate activations and compiling dry-runs.

