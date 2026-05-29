# Issue: Implement Live Integration Testing Harness

## 📌 Goal
Build a live integration testing harness (`go test -tags=integration`) that executes against a real/local Kubernetes cluster (like Minikube, K3s, or the active development cluster) to validate real-world KubeVirt provisioning, Tailscale operator proxying, and deletion protection safety logic.

## 🛠️ Requirements

### 1. Test Environment Setup
- **kubeconfig Detection**: Ensure integration tests utilize the active `kubeconfig` (defaulting to `~/.kube/config`).
- **Prerequisite Assertion**: Verify that KubeVirt, CDI, and Tailscale Operators are running in the test cluster before launching.

### 2. Integration Test Flow (`integration_test.go`)
- **Phase 1: Bootstrapping**:
  - Run the bootstrap logic to assert the operators are active.
  - Ensure the namespace `tailvm-integration-ns` is created and clean.
- **Phase 2: VM Provisioning**:
  - Provision a VM named `tailvm-integration-test` using a minimal cloud image (e.g. CirrOS or minimal Debian).
  - Enable nested virtualization (`host-passthrough`) and dynamic Tailscale proxying.
  - Enable Deletion Protection.
  - Assert that:
    - The `VirtualMachine` object is created in the namespace.
    - The `tailvm-protection-policy` ConfigMap is created.
    - The five proxy resources (`tailvm-integration-test-proxy` deployment/service/etc.) are deployed.
- **Phase 3: Status Tracking**:
  - Poll the cluster and wait for the VM to transition to `Running` (status.printableStatus).
  - Query the proxy pod and verify that VNC (5900), Web VNC (8080), and SSH (22) tunnels are listening.
- **Phase 4: Safety Deletion Block Validation**:
  - Attempt to delete the VM using the raw `DeleteVM()` method while `tailvm.io/protected="true"` is active.
  - Assert that the deletion CLI/TUI block intercepts the call and fails safely with a protection error.
- **Phase 5: Safety Disarm & Cleanup**:
  - Disable protection: Set `tailvm protect tailvm-integration-test off`.
  - Trigger deletion.
  - Assert that the VM, ConfigMap, and all five proxy helper resources are successfully and completely purged from the namespace.

## 📈 Success Criteria
- Execute integration tests successfully: `go test -tags=integration -v ./...`
- All created K8s resources are fully garbage-collected, leaving the test namespace completely clean.

---

## 📋 Sub-Issues & Granular Tasks Checklist

- [ ] **Sub-Issue 2.1: Test Harness & Environment Setup**
  - [ ] Implement `integration_test.go` config initialization (detecting local context).
  - [ ] Verify prerequisite check returns active operators (KubeVirt, CDI, TS Operator).
- [ ] **Sub-Issue 2.2: Live VM Creation & Deployment Assertions**
  - [ ] Provision a test VM `tailvm-integration-test` in K3s.
  - [ ] Assert the `VirtualMachine` resource is successfully deployed and has labels.
  - [ ] Assert the ConfigMap policy is created.
  - [ ] Assert five helper proxy resources are created.
- [ ] **Sub-Issue 2.3: Ingress Tunneling Verification**
  - [ ] Poll VM status until it is `Running` and `Ready`.
  - [ ] Assert the proxy container gets a valid cluster IP.
  - [ ] Assert ports 22, 5900, and 8080 are reachable on the proxy service.
- [ ] **Sub-Issue 2.4: Deletion Protection Verification**
  - [ ] Attempt raw VM deletion via client API with `tailvm.io/protected="true"`.
  - [ ] Verify the action is rejected and fails cleanly.
  - [ ] Trigger dynamic disarm patch (set protection false).
  - [ ] Run deletion and verify all VM and proxy resources are successfully garbage-collected.

