package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// OSTemplate defines standard operating system characteristics.
type OSTemplate struct {
	ID          string
	Name        string
	Description string
	DefaultUser string
	Packages    []string
	SetupScript string
}

// GetOSPresets returns the premium OS presets supported by tailvm.
func GetOSPresets() []OSTemplate {
	return []OSTemplate{
		{
			ID:          "ubuntu-desktop",
			Name:        "Ubuntu XFCE Desktop (Recommended)",
			Description: "Snappy XFCE desktop, TigerVNC server, and QEMU/KVM nested virt. Perfect for GUI & ISO testing.",
			DefaultUser: "james",
			Packages: []string{
				"xfce4", "xfce4-goodies", "tigervnc-standalone-server", "tigervnc-common",
				"qemu-kvm", "libvirt-daemon-system", "libvirt-clients", "virt-manager",
				"tmux", "curl", "git", "bash-completion", "net-tools",
			},
			SetupScript: `
# Configure VNC Server
mkdir -p /home/james/.vnc
echo "vncpass" | vncpasswd -f > /home/james/.vnc/passwd
chmod 600 /home/james/.vnc/passwd
chown -R james:james /home/james/.vnc

# Create standard xstartup
cat <<'EOF' > /home/james/.vnc/xstartup
#!/bin/sh
unset SESSION_MANAGER
unset DBUS_SESSION_BUS_ADDRESS
startxfce4 &
EOF
chmod +x /home/james/.vnc/xstartup
chown james:james /home/james/.vnc/xstartup

# Configure VNC systemd service
cat <<'EOF' > /etc/systemd/system/vncserver@.service
[Unit]
Description=Remote desktop service (VNC)
After=syslog.target network.target

[Service]
Type=forking
User=james
Group=james
WorkingDirectory=/home/james

PIDFile=/home/james/.vnc/%H:%i.pid
ExecStartPre=-/usr/bin/vncserver -kill :%i > /dev/null 2>&1
ExecStart=/usr/bin/vncserver -depth 24 -geometry 1280x800 :%i
ExecStop=/usr/bin/vncserver -kill :%i

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable vncserver@1.service
systemctl start vncserver@1.service
`,
		},
		{
			ID:          "ubuntu-server",
			Name:        "Ubuntu Headless Server",
			Description: "Clean Ubuntu Server pre-configured with QEMU/KVM nested virtualization and tmux.",
			DefaultUser: "james",
			Packages: []string{
				"qemu-kvm", "libvirt-daemon-system", "libvirt-clients", "virt-manager",
				"tmux", "curl", "git", "bash-completion", "net-tools",
			},
			SetupScript: `
# Enable and start libvirt
systemctl enable --now libvirtd
`,
		},
		{
			ID:          "fedora-server",
			Name:        "Fedora Server with Virt",
			Description: "Fedora Cloud Server with virtualisation tools, libvirt, and developer utilities.",
			DefaultUser: "fedora",
			Packages: []string{
				"qemu-kvm", "libvirt-daemon-kvm", "libvirt-client", "virt-install",
				"tmux", "curl", "git", "bash-completion", "net-tools",
			},
			SetupScript: `
systemctl enable --now libvirtd
`,
		},
	}
}

// DetectLocalSSHKeys scans standard locations for public SSH keys.
func DetectLocalSSHKeys() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	keyPaths := []string{
		filepath.Join(homeDir, ".ssh", "id_ed25519.pub"),
		filepath.Join(homeDir, ".ssh", "id_rsa.pub"),
		filepath.Join(homeDir, ".ssh", "id_ecdsa.pub"),
	}

	for _, path := range keyPaths {
		if data, err := ioutil.ReadFile(path); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

// GenerateCloudInit creates the cloud-init userdata script.
func GenerateCloudInit(username, sshKey string, preset OSTemplate) string {
	packagesList := ""
	if len(preset.Packages) > 0 {
		packagesList = "packages:\n"
		for _, pkg := range preset.Packages {
			packagesList += fmt.Sprintf("  - %s\n", pkg)
		}
	}

	return fmt.Sprintf(`#cloud-config
users:
  - name: %s
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    groups: sudo, libvirt, kvm
    ssh_authorized_keys:
      - %s

%s

runcmd:
  # Enable nested virtualization configurations inside VM
  - modprobe kvm_intel || true
  - modprobe kvm_amd || true
  - chmod 666 /dev/kvm || true
%s
  - echo "tailvm: VM provisioned successfully" >> /var/log/tailvm.log
`, username, sshKey, packagesList, indentLines(preset.SetupScript, 2))
}

// Helper to indent multiline strings
func indentLines(str string, indent int) string {
	if str == "" {
		return ""
	}
	prefix := strings.Repeat("  ", indent)
	lines := strings.Split(strings.TrimSpace(str), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

// GenerateVMManifest returns the raw YAML for the VirtualMachine object.
func GenerateVMManifest(name, namespace string, cpuCores int, memoryGuestGB int, isProtected bool, useDataVolume bool, diskSource string, cloudInitData string) string {
	protectionLabel := "false"
	if isProtected {
		protectionLabel = "true"
	}

	// Dynamic volume definition based on Hybrid mode (DataVolume or HostDisk)
	var volumeSourceYAML string
	var dataVolumeTemplateYAML string

	if useDataVolume {
		volumeSourceYAML = fmt.Sprintf(`      - dataVolume:
          name: %s-rootdisk
        name: rootdisk`, name)

		dataVolumeTemplateYAML = fmt.Sprintf(`  dataVolumeTemplates:
  - metadata:
      name: %s-rootdisk
    spec:
      storage:
        resources:
          requests:
            storage: 20Gi
        storageClassName: local-path
      source:
        http:
          url: "%s"`, name, diskSource)
	} else {
		// Default HostDisk pathway
		volumeSourceYAML = fmt.Sprintf(`      - hostDisk:
          path: %s
          type: DiskOrCreate
          capacity: 25Gi
        name: rootdisk`, diskSource)
		dataVolumeTemplateYAML = ""
	}

	warningMsg := "WARNING: This is a persistent developer PET VM managed by Tailvm. DO NOT DELETE. Deleting this VM will cause developer disruption and immediate data loss."

	return fmt.Sprintf(`apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: %s
  namespace: %s
  labels:
    app: tailvm
    tailvm.io/protected: "%s"
    tailvm.io/pet: "true"
  annotations:
    tailvm.io/warning: "%s"
    description: "%s"
spec:
  runStrategy: Always
  template:
    metadata:
      labels:
        kubevirt.io/vm: %s
        app: tailvm
    spec:
      domain:
        cpu:
          cores: %d
          model: host-passthrough
        devices:
          disks:
          - bootOrder: 1
            disk:
              bus: virtio
            name: rootdisk
          - disk:
              bus: virtio
            name: cloudinitdisk
          interfaces:
          - masquerade: {}
            name: default
        memory:
          guest: %dGi
      networks:
      - name: default
        pod: {}
      volumes:
%s
      - cloudInitNoCloud:
          userData: |
%s
        name: cloudinitdisk
%s`, name, namespace, protectionLabel, warningMsg, warningMsg, name, cpuCores, memoryGuestGB, volumeSourceYAML, indentLines(cloudInitData, 6), dataVolumeTemplateYAML)
}

// GenerateProxyManifests returns the Deployment, Service, ServiceAccount, and RBAC manifests for the Tailscale Operator Proxy.
func GenerateProxyManifests(vmName, namespace string) string {
	return fmt.Sprintf(`---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tailvm-%s-proxy
  namespace: %s
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: tailvm-%s-proxy-role
  namespace: %s
rules:
- apiGroups: ["kubevirt.io"]
  resources: ["virtualmachineinstances"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: tailvm-%s-proxy-binding
  namespace: %s
subjects:
- kind: ServiceAccount
  name: tailvm-%s-proxy
  namespace: %s
roleRef:
  kind: Role
  name: tailvm-%s-proxy-role
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: v1
kind: Service
metadata:
  name: %s-proxy
  namespace: %s
  annotations:
    tailscale.com/expose: "true"
    tailscale.com/hostname: "%s-vm"
  labels:
    app: tailvm-proxy
    vm: %s
spec:
  type: ClusterIP
  ports:
  - name: ssh
    port: 22
    targetPort: 22
  - name: vnc
    port: 5900
    targetPort: 5900
  - name: webvnc
    port: 80
    targetPort: 8080
  selector:
    app: tailvm-proxy
    vm: %s
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s-proxy
  namespace: %s
  labels:
    app: tailvm-proxy
    vm: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: tailvm-proxy
      vm: %s
  template:
    metadata:
      labels:
        app: tailvm-proxy
        vm: %s
    spec:
      serviceAccountName: tailvm-%s-proxy
      initContainers:
      - name: setup-tools
        image: alpine:latest
        command:
        - sh
        - -c
        - |
          apk add --no-cache curl >/dev/null
          curl -sSL "https://github.com/kubevirt/kubevirt/releases/download/v1.8.2/virtctl-v1.8.2-linux-amd64" -o /tmp/virtctl
          chmod +x /tmp/virtctl
        volumeMounts:
        - name: tools
          mountPath: /tmp
      containers:
      - name: proxy
        image: alpine:latest
        command:
        - sh
        - -c
        - |
          # Install deps: noVNC, websockify, socat, python3
          apk add --no-cache curl socat python3 py3-pip novnc >/dev/null 2>&1
          
          # Start virtctl vnc proxy locally on 5900
          /tmp/virtctl vnc %s -n %s --proxy-only --address=0.0.0.0 --port=5900 &
          
          # Start websockify for noVNC on 8080 mapping to 5900
          # noVNC client is at /usr/share/novnc/vnc.html or /usr/share/novnc/index.html
          # symlink index.html to vnc.html so visiting root is elegant
          ln -sf /usr/share/novnc/vnc.html /usr/share/novnc/index.html
          websockify --web=/usr/share/novnc 8080 127.0.0.1:5900 &
          
          # Wait and resolve VM IP for SSH proxying
          while true; do
            IP=$(curl -sS --cacert /var/run/secrets/kubernetes.io/serviceaccount/ca.crt \
                 -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
                 "https://kubernetes.default.svc/apis/kubevirt.io/v1/namespaces/%s/virtualmachineinstances/%s" | \
                 python3 -c "import sys,json; print(json.load(sys.stdin).get('status',{}).get('interfaces',[{}])[0].get('ipAddress',''))" 2>/dev/null)
            if [ -n "$IP" ]; then
              echo "Forwarding SSH: pod port 22 -> $IP:22"
              socat TCP-LISTEN:22,fork,reuseaddr TCP:$IP:22 &
              wait
            fi
            sleep 5
          done
        ports:
        - name: ssh
          containerPort: 22
        - name: vnc
          containerPort: 5900
        - name: webvnc
          containerPort: 8080
        volumeMounts:
        - name: tools
          mountPath: /tmp
          readOnly: true
      volumes:
      - name: tools
        emptyDir: {}
`, vmName, namespace, vmName, namespace, vmName, namespace, vmName, namespace, vmName, vmName, namespace, vmName, vmName, vmName, vmName, namespace, vmName, vmName, vmName, vmName, vmName, namespace, namespace, vmName)
}

// GenerateNamespaceConfigMapManifest returns the manifest for a namespace ConfigMap explaining the protection policy.
func GenerateNamespaceConfigMapManifest(namespace string) string {
	warningMsg := `# TAILVM PROTECTION POLICY
This namespace contains persistent developer virtual machines ("Pets") managed by Tailvm.
These machines are actively used for GUI testing, ISO execution, and continuous integration workflows by developer James.

## CRITICAL SAFETY RULES FOR ALL COLLABORATING AGENTS AND SCRIPTS:
1. **DO NOT DELETE** any VirtualMachine, VirtualMachineInstance, Service, or Deployment labeled with "tailvm.io/protected=true".
2. **DO NOT TERMINATE** the associated proxy deployments (names matching *-proxy).
3. If you need to perform maintenance or cleanups, always double-check with James or verify using the 'tailvm' tool first.

Thank you for your cooperation!
`
	return fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: tailvm-protection-policy
  namespace: %s
  labels:
    app: tailvm
    tailvm.io/pet: "true"
data:
  POLICY.md: |
%s
`, namespace, indentLines(warningMsg, 4))
}
