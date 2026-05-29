# Issue: Implement Comprehensive Unit Testing Coverage

## 📌 Goal
Establish a robust unit testing suite for all core Go packages (`tailvm`) to ensure 100% reliability of manifest generation, cloud-init templates, key detection, and Kubernetes API parsing.

## 🛠️ Requirements

### 1. Template & Generation Tests (`templates_test.go`)
- **OS Presets Validation**: Verify that `GetOSPresets()` returns all required configurations (Ubuntu XFCE Desktop, Ubuntu headless, Fedora) with correct default usernames and package arrays.
- **Cloud-Init Generation**: Test `GenerateCloudInit()` against multiple configurations. Assert that the generated output is a valid YAML cloud-config, user blocks are populated, SSH key is injected, and custom setup scripts are properly indented.
- **VM Manifest Parsing**: Test `GenerateVMManifest()` and verify that the CPU cores, memory limits, and `host-passthrough` CPU model are correctly injected into the generated KubeVirt YAML spec. Validate both the CDI `DataVolume` structure and the fallback `HostDisk` layout.
- **Proxy Manifest Generation**: Test `GenerateProxyManifests()` and assert that the generated multi-document YAML contains exactly five distinct resources (ServiceAccount, Role, RoleBinding, Service, Deployment) and that all port mappings (22, 5900, 8080) are correct.

### 2. SSH Key Detection Tests (`templates_test.go`)
- **Key Discovery Mocking**: Create mock SSH key files in a temporary directory, override the user home directory wrapper, and test that `DetectLocalSSHKeys()` successfully discovers keys in order of precedence (`id_ed25519.pub`, `id_rsa.pub`, `id_ecdsa.pub`).

### 3. Mock Kubernetes Client Tests (`k8s_test.go`)
- **Fake Client Setup**: Utilize `k8s.io/client-go/dynamic/fake` and `k8s.io/client-go/kubernetes/fake` to initialize mock Dynamic clients and Clientsets populated with fake cluster states.
- **Listing VMs**: Mock KubeVirt VMs in the fake dynamic client. Assert that `ListVMs()` successfully parses them, handles running status conversions, extracts IP addresses, and correctly detects deletion protection labels.
- **Creation Flow**: Assert that `CreateVM()` correctly deploys the VirtualMachine object, the namespace ConfigMap, and the helper Tailscale proxy deployment/service on the fake clientset.
- **Safe Deletion**: Assert that calling `DeleteVM()` cleans up all five helper proxy resources alongside the VirtualMachine itself.
- **Protection Toggle**: Test `SetProtection()` and verify that dynamic patch payloads correctly update the `tailvm.io/protected` label on the fake resources.

## 📈 Success Criteria
- Execute unit tests successfully: `go test -v ./...`
- Achieve > 80% statement coverage across all core modules.

---

## 📋 Sub-Issues & Granular Tasks Checklist

- [ ] **Sub-Issue 1.1: Standard Template Logic Tests**
  - [ ] Implement `TestGetOSPresets` to assert default usernames and matching ID keys.
  - [ ] Implement `TestGenerateCloudInit` to verify SSH key injection format and valid YAML.
  - [ ] Implement `TestGenerateVMManifest` to check `host-passthrough` injection and CPU/RAM allocation logic.
- [ ] **Sub-Issue 1.2: Proxy and Protection Spec Tests**
  - [ ] Implement `TestGenerateProxyManifests` to verify five distinct document resources (SA, Role, Binding, Service, Deployment) and correct ports.
  - [ ] Implement `TestGenerateNamespaceConfigMapManifest` to assert markdown policy text matches warnings.
- [ ] **Sub-Issue 1.3: SSH Local Key Detection Mocking**
  - [ ] Implement `TestDetectLocalSSHKeys` using `t.TempDir()` to mock `~/.ssh/id_ed25519.pub` and `id_rsa.pub` disk reads.
- [ ] **Sub-Issue 1.4: Dynamic Kubernetes Fake Client Mocking**
  - [ ] Set up fake dynamics using `k8s.io/client-go/dynamic/fake`.
  - [ ] Implement `TestListVMs` to assert correct extraction of status, IP, node names, and protection labels.
  - [ ] Implement `TestCreateVM` and `TestDeleteVM` to assert resource counts on mock namespaces.
  - [ ] Implement `TestSetProtection` to assert raw JSON patch commands are formatted correctly.

