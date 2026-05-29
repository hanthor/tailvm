package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	namespaceFlag string
	dryRunFlag    bool
	cpuFlag       int
	memoryFlag    int
	diskSourceFlag string
	diskModeFlag  string
	sshKeyFlag    string
	protectFlag   bool
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "tailvm",
		Short: "Tailvm manages KubeVirt VM pets and bridges them over Tailscale",
		Long: `Tailvm is a premium CLI and interactive TUI tool designed to spin up 
KubeVirt Virtual Machines as dedicated, persistent developer hosts ("pets").
VMs are automatically exposed on Tailscale, support nested virtualization for GUI/ISO testing,
and carry robust safety registries to prevent automated cleanup scripts or LLMs from deleting them.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Default behavior: Launch TUI
			cm, err := NewClientManager()
			if err != nil {
				fmt.Printf("❌ Failed to initialize cluster client: %v\n", err)
				os.Exit(1)
			}

			model := NewTUIModel(cm)
			p := tea.NewProgram(model)
			if _, err := p.Run(); err != nil {
				fmt.Printf("❌ TUI execution error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	rootCmd.PersistentFlags().StringVarP(&namespaceFlag, "namespace", "n", "default", "Kubernetes Namespace")

	// 1. bootstrap command
	var bootstrapCmd = &cobra.Command{
		Use:   "bootstrap",
		Short: "Checks and installs cluster prerequisites (KubeVirt, CDI, Tailscale Operator)",
		Run: func(cmd *cobra.Command, args []string) {
			cm, err := NewClientManager()
			if err != nil {
				fmt.Printf("❌ Failed to initialize cluster client: %v\n", err)
				os.Exit(1)
			}

			err = cm.BootstrapCluster(dryRunFlag)
			if err != nil {
				fmt.Printf("❌ Bootstrapping failed: %v\n", err)
				os.Exit(1)
			}
			if !dryRunFlag {
				fmt.Println("🎉 Cluster verification and bootstrap complete!")
			}
		},
	}
	bootstrapCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Validate missing requirements without installing them")
	rootCmd.AddCommand(bootstrapCmd)

	// 2. create command
	var createCmd = &cobra.Command{
		Use:   "create [name]",
		Short: "CLI method to create a new VM pet",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			vmName := args[0]
			cm, err := NewClientManager()
			if err != nil {
				fmt.Printf("❌ Failed to initialize cluster client: %v\n", err)
				os.Exit(1)
			}

			presets := GetOSPresets()
			// Default preset is Ubuntu Desktop
			selectedPreset := presets[0]

			if sshKeyFlag == "" {
				sshKeyFlag = DetectLocalSSHKeys()
			}
			if sshKeyFlag == "" {
				fmt.Println("❌ Error: No SSH key supplied or detected. Please use --ssh-key.")
				os.Exit(1)
			}

			useDataVolume := diskModeFlag == "DataVolume"

			cloudInit := GenerateCloudInit(selectedPreset.DefaultUser, sshKeyFlag, selectedPreset)

			fmt.Printf("🛠️  Creating KubeVirt VM %s (Cores: %d, Memory: %dGB)...\n", vmName, cpuFlag, memoryFlag)
			err = cm.CreateVM(vmName, namespaceFlag, cpuFlag, memoryFlag, protectFlag, useDataVolume, diskSourceFlag, cloudInit)
			if err != nil {
				fmt.Printf("❌ VM creation failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("🎉 VM %s created! Once active, connect via:\n", vmName)
			fmt.Printf("   👉 SSH:  ssh %s@%s-vm.tailnet\n", selectedPreset.DefaultUser, vmName)
			fmt.Printf("   👉 VNC:  http://%s-vm.tailnet\n", vmName)
		},
	}
	createCmd.Flags().IntVar(&cpuFlag, "cpu", 4, "Number of CPU Cores")
	createCmd.Flags().IntVar(&memoryFlag, "memory", 8, "Guest memory size in GB")
	createCmd.Flags().StringVar(&diskSourceFlag, "disk-src", "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img", "HTTP URL (DataVolume) or HostDisk File path")
	createCmd.Flags().StringVar(&diskModeFlag, "disk-mode", "DataVolume", "Disk import type: 'DataVolume' or 'HostDisk'")
	createCmd.Flags().StringVar(&sshKeyFlag, "ssh-key", "", "Public SSH key contents")
	createCmd.Flags().BoolVar(&protectFlag, "protect", true, "Enable protective safety tags against cleanup scripts")
	rootCmd.AddCommand(createCmd)

	// 3. list command
	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "Lists active VMs and statuses",
		Run: func(cmd *cobra.Command, args []string) {
			cm, err := NewClientManager()
			if err != nil {
				fmt.Printf("❌ Failed to initialize cluster client: %v\n", err)
				os.Exit(1)
			}

			list, err := cm.ListVMs(namespaceFlag)
			if err != nil {
				fmt.Printf("❌ Failed to list VMs: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("📋 Active VMs in namespace '%s':\n\n", namespaceFlag)
			fmt.Printf("%-5s %-20s %-12s %-15s %-25s %-5s %-6s\n", "PROT", "NAME", "STATUS", "IP", "TAILSCALE HOST", "CPU", "MEM")
			fmt.Println(string(make([]byte, 95)))
			for _, vm := range list {
				prot := "🔓"
				if vm.Protected {
					prot = "🛡️"
				}
				fmt.Printf("%-5s %-20s %-12s %-15s %-25s %-5d %-6s\n", prot, vm.Name, vm.Status, vm.IP, vm.TailscaleName+".tailnet", vm.CPUCores, vm.Memory)
			}
		},
	}
	rootCmd.AddCommand(listCmd)

	// 4. delete command
	var deleteCmd = &cobra.Command{
		Use:   "delete [name]",
		Short: "Deletes a KubeVirt VM and its Tailscale proxy environment",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			vmName := args[0]
			cm, err := NewClientManager()
			if err != nil {
				fmt.Printf("❌ Failed to initialize cluster client: %v\n", err)
				os.Exit(1)
			}

			// Protection validation check
			vms, err := cm.ListVMs(namespaceFlag)
			if err == nil {
				for _, vm := range vms {
					if vm.Name == vmName && vm.Protected {
						fmt.Printf("🚨 CANCELED: VM %s is PROTECTED. To delete, you must toggle protection off first (p in TUI or tailvm protect off).\n", vmName)
						os.Exit(1)
					}
				}
			}

			err = cm.DeleteVM(vmName, namespaceFlag)
			if err != nil {
				fmt.Printf("❌ Deletion failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("🗑️  KubeVirt VM %s and its proxy environment deleted successfully.\n", vmName)
		},
	}
	rootCmd.AddCommand(deleteCmd)

	// 5. protect command
	var protectCmd = &cobra.Command{
		Use:   "protect [name] [on/off]",
		Short: "Toggles deletion safety protections on or off",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			vmName := args[0]
			action := args[1]
			cm, err := NewClientManager()
			if err != nil {
				fmt.Printf("❌ Failed to initialize cluster client: %v\n", err)
				os.Exit(1)
			}

			protect := false
			if action == "on" {
				protect = true
			} else if action != "off" {
				fmt.Println("❌ Mismatch: Action must be 'on' or 'off'")
				os.Exit(1)
			}

			err = cm.SetProtection(vmName, namespaceFlag, protect)
			if err != nil {
				fmt.Printf("❌ Protection change failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("🛡️  Protection for VM %s is now set to: %v\n", vmName, protect)
		},
	}
	rootCmd.AddCommand(protectCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
