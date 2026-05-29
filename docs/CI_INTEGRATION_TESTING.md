# CI Integration Testing Strategy: KinD vs Minikube for KubeVirt

Running integration tests for **Tailvm** in a CI environment (like GitHub Actions) requires a live Kubernetes cluster that supports running **KubeVirt**. Since KubeVirt manages actual virtual machines, special consideration must be given to **Nested Virtualization** and hardware acceleration.

This document analyzes the differences between **KinD** and **Minikube** for KubeVirt testing and provides a production-ready GitHub Actions configuration.

---

## ⚖️ KinD vs Minikube Comparison for KubeVirt CI

| Feature | KinD (Kubernetes in Docker) 🌟 | Minikube |
| :--- | :--- | :--- |
| **Startup Speed** | **Fast** (20–40 seconds) | **Slow** (2–3 minutes) |
| **Architecture** | Nodes run as Docker containers on the host runner. | Nodes run as VMs (via KVM/VirtualBox) or Docker containers. |
| **`/dev/kvm` Access** | **Extremely Simple**. Docker containers share the host kernel. You can mount `/dev/kvm` directly from the host into the KinD node. | **Complex**. If using the `kvm2` driver, you must run a VM *inside* a VM (double-nested virtualization), which often fails on standard CI hosts. |
| **Resource Overhead** | Low memory/CPU footprint. | High overhead (especially with VM-based drivers). |
| **CI Stability** | High (few moving parts, native container namespaces). | Medium (prone to nested VM hypervisor panics on cloud runners). |

### 🏆 Recommendation: Use KinD (Kubernetes in Docker)
For KubeVirt integration testing in GitHub Actions, **KinD** is the superior choice. Because the KinD node is just a Docker container, we can pass the runner host's hardware acceleration (`/dev/kvm`) directly into the Kubernetes node with a simple volume mount, allowing KubeVirt to run hardware-accelerated VMs natively!

---

## ⚙️ How to Handle Virtualization in CI Runners

Standard GitHub-hosted runners (Ubuntu-latest) operate on virtual machines that **do not support hardware-accelerated nested virtualization** (no VMX/SVM flags are exposed). To run tests, you have two options:

### Option A: Hardware Emulation Fallback (Standard Runners)
You can configure KubeVirt to run in **software emulation mode** (QEMU emulation). It is slower, but works on standard free GitHub Actions runners.
To enable software emulation, set the `useEmulation: true` flag in the KubeVirt Custom Resource:
```yaml
apiVersion: kubevirt.io/v1
kind: KubeVirt
metadata:
  name: kubevirt
  namespace: kubevirt
spec:
  configuration:
    developerConfiguration:
      useEmulation: true  # <--- Forces software emulation
```
*Note: When using emulation, utilize ultra-lightweight test images (like CirrOS or Alpine Server 512MB presets) to prevent CI timeout.*

### Option B: Bare-Metal / Self-Hosted Runners (Hardware Accelerated)
If using self-hosted runners or cloud runners that support nested virtualization (e.g., AWS metal, Equinix, or Google Cloud custom instances with nested virt active), you can pass host acceleration directly into KinD nodes.

---

## 🚀 Production GitHub Actions CI Configuration

Create a file at `.github/workflows/integration-tests.yml` to automatically spin up KinD, install KubeVirt + CDI, and trigger your Go integration tests on every Pull Request:

```yaml
name: 🛡️ Tailvm CI Integration Tests

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  integration-test:
    name: Run Cluster Integration Tests
    runs-on: ubuntu-latest

    steps:
    - name: Check out repository
      uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.26'
        cache: true

    - name: Set up Docker Buildx
      uses: docker/setup-qemu-action@v3

    - name: Create KinD Cluster Config
      run: |
        cat <<EOF > kind-config.yaml
        kind: Cluster
        apiVersion: kind.x-k8s.io/v1alpha4
        nodes:
        - role: control-plane
          extraMounts:
          # Pass through /dev/kvm to the KinD node if it exists on the host
          - hostPath: /dev/kvm
            containerPath: /dev/kvm
        EOF

    - name: Create KinD Cluster
      uses: helm/kind-action@v1.10.0
      with:
        config: kind-config.yaml
        cluster_name: tailvm-test-cluster

    - name: Verify Cluster Connection
      run: |
        kubectl cluster-info
        kubectl get nodes -o wide

    - name: Run Tailvm Bootstrap (Installs Operators)
      run: |
        # Build tailvm binary locally
        go build -o tailvm .
        
        # Trigger bootstrapping to install KubeVirt and CDI
        # If /dev/kvm is missing on the runner, we patch KubeVirt to use emulation
        ./tailvm bootstrap
        
        # Fallback to software emulation if running on standard GitHub Actions hosts
        if [ ! -e /dev/kvm ]; then
          echo "⚠️ Hardware virtualization missing. Patching KubeVirt to software emulation..."
          kubectl patch kubevirt kubevirt -n kubevirt --type merge -p '{"spec":{"configuration":{"developerConfiguration":{"useEmulation":true}}}}'
        fi

    - name: Wait for KubeVirt components to be Ready
      run: |
        echo "⏳ Waiting for KubeVirt CR to enter Deployed phase..."
        kubectl wait --for=jsonpath='{.status.phase}'=Deployed kubevirt/kubevirt -n kubevirt --timeout=300s
        kubectl get pods -n kubevirt

    - name: Execute Live Integration Tests
      run: |
        # Run integration tests against the live KinD cluster
        go test -tags=integration -v ./...
```
