# TailVM

Spin up and manage QEMU/KubeVirt virtual machines with secure VNC access over Tailscale.

## Backends

| Backend | What it does |
|---------|-------------|
| **qemu** (default) | Local QEMU/KVM VMs managed via systemd user services. VNC bound to your Tailscale IP. |
| **kubevirt** | Kubernetes VMs via KubeVirt CRDs. VNC via `virtctl`. |

## Quick start

```bash
# Install
cp tailvm ~/.local/bin/tailvm
chmod +x ~/.local/bin/tailvm

# Local QEMU VM from an ISO
tailvm create myvm --iso https://releases.ubuntu.com/noble/ubuntu-24.04-live-server-amd64.iso
tailvm start myvm          # also spawns virt-viewer
tailvm list                # show all VMs
tailvm viewer myvm         # reconnect

# KubeVirt VM from a container disk
tailvm create mykubevm --kubevirt --container-disk quay.io/containerdisks/ubuntu:24.04
tailvm start mykubevm      # uses virtctl start
tailvm viewer mykubevm     # uses virtctl vnc
tailvm delete mykubevm
```

## Requirements

### QEMU backend
- `qemu` (provides `qemu-system-x86_64`, `qemu-img`)
- `tailscale`
- `flatpak` + `org.virt_manager.virt-viewer` (for VNC viewer)
- Linux with KVM support

### KubeVirt backend
- `kubectl` configured for a cluster with KubeVirt installed
- `virtctl` (`brew install kubevirt-cli`)

## Commands

```
create  NAME         Create a VM
start   NAME         Start a VM
stop    NAME         Stop a VM
list                 List all VMs (both backends)
info    NAME         Show VM details
delete  NAME         Delete VM and its files/disks
viewer  NAME         Launch VNC viewer
eject   NAME         Eject ISO (QEMU only)
logs    NAME         Tail VM logs
```

### Create flags

| Flag | Backend | Description |
|------|---------|-------------|
| `--kubevirt`, `-k` | both | Use KubeVirt backend instead of local QEMU |
| `--mem 4G` | both | Memory allocation (default: 4G) |
| `--cpu 2` | both | CPU cores (default: 2) |
| `--disk 20G` | both | Disk size (default: 20G) |
| `--iso URL` | qemu | ISO to boot from |
| `--qcow URL` | qemu | QCOW2 template to copy |
| `--container-disk IMG` | kubevirt | Container disk image |
| `--pvc NAME` | kubevirt | Existing PVC to use |
| `--node NAME` | kubevirt | Schedule on specific node |
| `--namespace NS` | kubevirt | Kubernetes namespace (default: default) |
| `--cloud-init-password` | kubevirt | Cloud-init password (random if omitted) |

## KubeVirt defaults

When you run `tailvm create myvm --kubevirt` with no other flags, tailvm:
- Creates a 20G empty PVC (`myvm-disk`)
- Attaches cloud-init with a random password (shown once)
- Lets the scheduler pick a node

When `--container-disk` is given and `--disk` is also given, an additional data PVC
is created (`myvm-data`).

## How backends are tracked

The VM name is stored in `~/.local/share/tailvm/registry.json`. After `create`,
all subsequent commands (`start`, `stop`, `viewer`, …) know which backend to use
— no need to repeat `--kubevirt`.

`tailvm list` shows VMs from both backends together with a `BACKEND` column.

## License

MIT
