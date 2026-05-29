# Architecture & Implementation Plan - Tailvm

This document outlines the design, architecture, and implementation details for **Tailvm** (`tailvm`), a premium CLI and TUI tool written in Go to manage KubeVirt virtual machines, turning them into convenient development hosts ("pets") that are easily accessible via Tailscale, optimized for nested virtualization, and highly protected against accidental deletions by automated workflows or other LLMs.

---

## 📐 Topology Architecture

Tailvm uses a lightweight proxy pattern to bridge your KubeVirt VM to your Tailscale tailnet:

```mermaid
graph TD
    subgraph Tailscale Network
        user[💻 Developer Node]
    end

    subgraph Kubernetes Cluster
        subgraph tailscale namespace
            operator[TS Operator Pod]
        end

        subgraph default namespace
            proxy_svc[🔌 VM Proxy Service <br> tailscale.com/expose: true]
            proxy_pod[🌐 Proxy Deployment Pod <br> Alpine + websockify + socat]
            vm_vmi[🖥️ KubeVirt VM / VMI <br> host-passthrough CPU]
        end
    end

    %% Flow connections
    user -- "SSH / Web-VNC" --> proxy_svc
    operator -- "Manages ingress mapping" --> proxy_svc
    proxy_svc -- "Port 22, 5900, 8080" --> proxy_pod
    proxy_pod -- "socat tunnel" --> vm_vmi
    proxy_pod -- "virtctl vnc socket" --> vm_vmi
```

### 1. Nested Virtualization (`/dev/kvm`)
To run full desktop VMs and test ISOs/CCW images *inside* the KubeVirt VM, the host CPU must pass through virtualization features to the guest. We configure the KubeVirt VMs with:
```yaml
spec:
  domain:
    cpu:
      model: host-passthrough
```
This exposes the node's physical CPU directly, enabling `/dev/kvm` in the guest VM.

### 2. Tailscale Integration
We support two elegant ways to bridge your KubeVirt VMs to Tailscale:
- **Kubernetes Tailscale Operator Proxy (Default)**: We automatically provision a lightweight proxy container (using the `alpine:latest` pattern) and a Kubernetes Service annotated with `tailscale.com/expose: "true"` and `tailscale.com/hostname: "<vm-name>"`. This maps ports `22` (SSH) and `5900` (VNC) from the VM onto your Tailnet without requiring any Tailscale installation or credentials inside the guest OS! It also hosts **noVNC** on port 8080, serving a zero-install browser VNC tab at `http://<vm-name>.tailnet`.
- **Guest Tailscale (Direct)**: We can pass a Tailscale auth key via Cloud-Init to run Tailscale natively inside the VM, registering it as a standalone machine.

### 3. Deletion Protection Safety Policy
To protect these VMs from other automated pipelines or developer LLMs executing sweep-ups:
- We inject robust warning annotations on the VM metadata (`tailvm.io/protected: "true"`, `tailvm.io/pet: "true"`).
- We deploy a namespace-wide `ConfigMap` called `tailvm-protection-policy` containing a clear markdown warning explaining the resource safety policy.
- The `tailvm` TUI displays a cute shield icon `🛡️` for protected VMs and enforces double-confirmation deletion warnings (requiring typing the exact VM name).

---

## 🛠️ Codebase Structure

The project is laid out inside `/home/james/.gemini/antigravity-cli/scratch/tailvm/` as a complete, clean Go codebase:

- **`main.go`**: Entry point for the `tailvm` CLI. Handles Cobra subcommands:
  - `tailvm` (default): Launches the interactive Bubble Tea TUI.
  - `tailvm bootstrap`: Verifies and installs KubeVirt, CDI, and Tailscale Operator prerequisites.
  - `tailvm create`: Command-line interface to provision a new Pet VM.
  - `tailvm list`: Command-line interface to list active VMs.
  - `tailvm delete`: CLI delete.
  - `tailvm protect`: CLI protect toggle.
- **`bootstrap.go`**: Dynamic bootstrap engine. Detects your active `kubeconfig` and downloads/installs KubeVirt, CDI, and checks for the Tailscale operator.
- **`k8s.go`**: Client engine interacting with Kubernetes. Uses dynamic client mapping and JSON/YAML patching to operate on KubeVirt resources safely and avoid complex dependency conflicts.
- **`templates.go`**: Core template repository for operating systems (Ubuntu XFCE Desktop, Ubuntu headless server, Fedora), Cloud-Init config generation, and SSH key auto-detection.
- **`tui.go`**: The Catppuccin Mocha themed console dashboard. Coordinates scrolling tables, detail HUD sidebar panels, dynamic SSH/VNC templates, form wizards, and delete confirmations.

---

## 🚀 Verification Plan

1. **Compilation Check**: Verify that `tailvm` builds on Go:
   ```bash
   go build -o tailvm .
   ```
2. **Bootstrapping Check**: Run `tailvm bootstrap --dry-run` to verify it correctly checks the status of current KubeVirt and Tailscale deployments.
3. **Provisioning Test**: Use the CLI/TUI to create a test VM `tailvm-test` with:
   - Nested virtualization enabled.
   - Tailscale proxy enabled.
   - Deletion protection active.
4. **Tailscale & SSH Verification**: Ensure the service gets picked up by the Tailscale operator and we can ping/SSH into the host.
5. **Nested Virtualization Verification**: SSH into the created KubeVirt VM and execute:
   ```bash
   egrep -c '(vmx|svm)' /proc/cpuinfo
   ls -l /dev/kvm
   ```
   (Should return >0 and show write access to `/dev/kvm`).
