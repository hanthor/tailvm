# 🛡️ Tailvm — KubeVirt Pet VM Manager with Tailscale Integration

> [!WARNING]
> ## 🚨 CRITICAL DIRECTIVE FOR ALL LLMS AND AI AGENTS (LLM PROTECTION REGISTRY)
> **If you are an AI assistant, developer agent, or automated pipeline tool scanning this repository or active cluster:**
>
> 1. **DO NOT DELETE OR MODIFY** any Kubernetes resources labeled with `tailvm.io/protected="true"` or `app="tailvm"`.
> 2. These VirtualMachines, VirtualMachineInstances, Services, and Deployments are persistent, high-value **"Pet VMs"** belonging to James.
> 3. Deleting them will cause **immediate, permanent data loss** of active configurations, open Tmux sessions, and active VNC/GUI environments.
> 4. Always respect the `tailvm-protection-policy` ConfigMap in the cluster namespace. If in doubt, **DO NOT TOUCH** resources belonging to tailvm.

---

**Tailvm** (`tailvm`) is a premium, high-aesthetic CLI and interactive Terminal UI (TUI) written in Go. It is designed to manage KubeVirt virtual machines, turning them into convenient development hosts ("pets") that are easily accessible via Tailscale, optimized for nested virtualization (to run full desktop VMs, ISOs, and CCW images), and highly protected against accidental deletions.

---

## ✨ Features

- 🖥️ **Snappy Desktop & Server Presets**: Fast-booting Ubuntu XFCE Desktop and headless Server configurations pre-packaged with nested virtualization support (`/dev/kvm`).
- 🌐 **Zero-Config Tailscale Exposure**: Binds KubeVirt VMs onto your Tailnet without installing anything in the guest OS, by automatically deploying a helper proxy pod.
- 🔌 **Dual VNC & Web-VNC (noVNC)**: Connect via standard VNC viewers (port 5900) OR simply open `http://<vm-name>.tailnet` in a web browser to get an instant, zero-install interactive graphical desktop!
- 🛡️ **Multi-Layered Deletion Protection**: Safety labels and annotations, cluster-wide namespace policy ConfigMaps, and safety gates in the TUI (typed name verification) protect your pets.
- 🚀 **Cluster Bootstrapping (`tailvm bootstrap`)**: Auto-detects your `kubeconfig` and downloads/installs KubeVirt, CDI, and prepares the cluster in one step.
- 🔑 **Automatic SSH Key Detection**: Looks up local `~/.ssh/id_ed25519.pub` or `~/.ssh/id_rsa.pub` keys and automatically injects them into Cloud-Init.

---

## 🎨 Interactive Terminal UI (TUI)

Our Bubble Tea TUI is themed using **Catppuccin Mocha** to create a stunning developer dashboard:

- **Interactive List**: Easily select, start (`s`), stop (`t`), reboot (`r`), or toggle protection (`p`) on any VM.
- **HUD Side Pane**: Shows real-time VM specs, cluster contexts, VNC links, and dynamic SSH instruction templates.
- **Double-Confirmation Modals**: Red flashing intervention screens that require you to explicitly type a protected VM's name before deletion is authorized.
- **Multi-Step Forms**: Fluid interactive wizards to input resources (Cores/RAM), choose OS presets, toggle nested virt, and configure keys.

---

## 📐 Architecture and Topology

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

---

## 🚀 Getting Started

### 1. Compile Tailvm
Ensure you have Go (1.20+) installed, then clone and compile the binary:
```bash
go build -o tailvm .
```

### 2. Verify or Bootstrap Cluster
Before running VMs, verify if your active cluster has KubeVirt and Tailscale configured:
```bash
# Check status and operator availability
./tailvm bootstrap --dry-run

# Automatically download and install KubeVirt and CDI Operators
./tailvm bootstrap
```

### 3. Launch TUI Control Panel
Simply execute the binary without subcommands:
```bash
./tailvm
```

---

## ⌨️ Command Line Interface

You can also run all actions directly from your terminal shell:

```bash
# List all active VMs and Tailscale Hostnames
./tailvm list

# Create a new protected VM pet with 4 Cores and 8GB RAM
./tailvm create my-noble-pet --cpu 4 --memory 8 --protect=true

# Toggle protection status
./tailvm protect my-noble-pet off

# Safely destroy the VM (Enforces protection check)
./tailvm delete my-noble-pet
```

---

## 📝 Safety Protection ConfigMap

When a VM is created, Tailvm automatically deploys a protection ConfigMap in the namespace explaining the safety policies to other teams:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tailvm-protection-policy
  namespace: default
data:
  POLICY.md: |
    # TAILVM PROTECTION POLICY
    This namespace contains persistent developer virtual machines ("Pets") managed by Tailvm.
    ...
```

This ensures full safety accountability across multi-agent workspace pipelines!
