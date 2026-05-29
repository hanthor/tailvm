package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type activeState int

const (
	stateList activeState = iota
	stateCreateForm
	stateConfirmDelete
	stateLoading
)

type fieldIndex int

const (
	fieldVMName fieldIndex = iota
	fieldCPU
	fieldRAM
	fieldOS
	fieldDiskMode // "DataVolume" or "HostDisk"
	fieldDiskSrc  // URL or host path
	fieldSSHKey
	fieldProtect
)

// UI Color Palette (Catppuccin Mocha themed)
var (
	subtleColor = lipgloss.AdaptiveColor{Light: "#D9E0EE", Dark: "#1E1E2E"}
	bgColor     = lipgloss.AdaptiveColor{Light: "#EFF1F5", Dark: "#11111B"}
	textColor   = lipgloss.AdaptiveColor{Light: "#4C4F69", Dark: "#CDD6F4"}
	accentColor = lipgloss.AdaptiveColor{Light: "#8839EF", Dark: "#CBA6F7"} // Mauve
	greenColor  = lipgloss.AdaptiveColor{Light: "#40A02B", Dark: "#A6E3A1"} // Green
	redColor    = lipgloss.AdaptiveColor{Light: "#D20F39", Dark: "#F38BA8"} // Red
	yellowColor = lipgloss.AdaptiveColor{Light: "#DF8E1D", Dark: "#F9E2AF"} // Yellow
	blueColor   = lipgloss.AdaptiveColor{Light: "#1E66F5", Dark: "#89B4FA"} // Blue

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(bgColor).
			Background(accentColor).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(subtleColor)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentColor).
			Padding(1)

	warningBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(redColor).
			Padding(1)

	hudStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(blueColor).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

// UI Model
type TUIModel struct {
	client        *ClientManager
	state         activeState
	vms           []VMInfo
	err           error
	loadingMsg    string
	table         table.Model
	width         int
	height        int
	
	// Form fields
	formInputs    []textinput.Model
	activeField   fieldIndex
	osPresets     []OSTemplate
	selectedPreset int
	diskModeVal   string // "DataVolume" (default) or "HostDisk"
	
	// Delete confirmation
	selectedVM    VMInfo
	deleteConfirm textinput.Model
}

func NewTUIModel(cm *ClientManager) TUIModel {
	// Setup Table
	columns := []table.Column{
		{Title: "🛡️", Width: 3},
		{Title: "VM NAME", Width: 20},
		{Title: "STATUS", Width: 12},
		{Title: "IP ADDRESS", Width: 15},
		{Title: "TAILSCALE HOST", Width: 22},
		{Title: "CORES", Width: 6},
		{Title: "MEM", Width: 6},
		{Title: "AGE", Width: 8},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(subtleColor).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(true)
	t.SetStyles(s)

	// Setup Create Form Inputs
	inputs := make([]textinput.Model, 8)
	
	inputs[fieldVMName] = textinput.New()
	inputs[fieldVMName].Placeholder = "my-pet-vm"
	inputs[fieldVMName].Focus()

	inputs[fieldCPU] = textinput.New()
	inputs[fieldCPU].Placeholder = "4"
	inputs[fieldCPU].SetValue("4")

	inputs[fieldRAM] = textinput.New()
	inputs[fieldRAM].Placeholder = "8"
	inputs[fieldRAM].SetValue("8")

	inputs[fieldOS] = textinput.New()
	inputs[fieldOS].Placeholder = "Use arrows to change template"
	inputs[fieldOS].SetValue("Ubuntu Desktop")

	inputs[fieldDiskMode] = textinput.New()
	inputs[fieldDiskMode].Placeholder = "Use arrows to change mode (DataVolume/HostDisk)"
	inputs[fieldDiskMode].SetValue("DataVolume")

	inputs[fieldDiskSrc] = textinput.New()
	inputs[fieldDiskSrc].Placeholder = "https://releases.ubuntu.com/24.04/ubuntu-24.04-live-server-amd64.iso"
	inputs[fieldDiskSrc].SetValue("https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img")

	inputs[fieldSSHKey] = textinput.New()
	inputs[fieldSSHKey].Placeholder = "Detecting local SSH key..."
	localKey := DetectLocalSSHKeys()
	if localKey != "" {
		inputs[fieldSSHKey].SetValue(localKey)
	}

	inputs[fieldProtect] = textinput.New()
	inputs[fieldProtect].Placeholder = "Use arrows (Yes/No)"
	inputs[fieldProtect].SetValue("Yes")

	// Delete Confirmation Input
	delConfirm := textinput.New()
	delConfirm.Placeholder = "Type VM Name here"

	return TUIModel{
		client:         cm,
		state:          stateList,
		table:          t,
		formInputs:     inputs,
		osPresets:      GetOSPresets(),
		selectedPreset: 0,
		diskModeVal:    "DataVolume",
		deleteConfirm:  delConfirm,
	}
}

type tickMsg time.Time

func (m TUIModel) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.fetchVMsCmd(),
		m.tickCmd(),
	)
}

func (m TUIModel) tickCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type vmsMsg []VMInfo
type actionDoneMsg string
type errMsg error

func (m TUIModel) fetchVMsCmd() tea.Cmd {
	return func() tea.Msg {
		list, err := m.client.ListVMs("")
		if err != nil {
			return errMsg(err)
		}
		return vmsMsg(list)
	}
}

func (m TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(msg.Width - 4)
		return m, nil

	case tickMsg:
		if m.state == stateList {
			return m, tea.Batch(m.fetchVMsCmd(), m.tickCmd())
		}
		return m, m.tickCmd()

	case vmsMsg:
		m.vms = msg
		m.err = nil
		var rows []table.Row
		for _, vm := range m.vms {
			protectIcon := "🔓"
			if vm.Protected {
				protectIcon = "🛡️"
			}
			readyMark := "🔴 Stopped"
			if vm.Status == "Running" {
				if vm.Ready {
					readyMark = "🟢 Running"
				} else {
					readyMark = "🟡 Booting"
				}
			}
			rows = append(rows, table.Row{
				protectIcon,
				vm.Name,
				readyMark,
				vm.IP,
				fmt.Sprintf("%s.tailnet", vm.TailscaleName),
				fmt.Sprintf("%d", vm.CPUCores),
				vm.Memory,
				vm.Age,
			})
		}
		m.table.SetRows(rows)
		if m.state == stateLoading {
			m.state = stateList
		}

	case errMsg:
		m.err = msg
		m.state = stateList

	case actionDoneMsg:
		m.loadingMsg = ""
		m.state = stateLoading
		return m, m.fetchVMsCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.state == stateList {
				return m, tea.Quit
			}
			// Cancel forms/modals
			m.state = stateList
			return m, nil

		case "up", "k":
			if m.state == stateList {
				m.table.MoveUp(1)
			} else if m.state == stateCreateForm {
				m.prevField()
			}

		case "down", "j":
			if m.state == stateList {
				m.table.MoveDown(1)
			} else if m.state == stateCreateForm {
				m.nextField()
			}

		case "left", "h":
			if m.state == stateCreateForm {
				m.handleFormToggle(false)
			}

		case "right", "l":
			if m.state == stateCreateForm {
				m.handleFormToggle(true)
			}

		case "enter":
			if m.state == stateCreateForm {
				if m.activeField == fieldProtect {
					// Final Submit!
					return m.submitVMCreate()
				}
				m.nextField()
			} else if m.state == stateConfirmDelete {
				return m.submitVMDelete()
			}

		case "c":
			if m.state == stateList {
				m.state = stateCreateForm
				m.activeField = fieldVMName
				m.formInputs[fieldVMName].Focus()
				for i := range m.formInputs {
					if i != int(fieldVMName) {
						m.formInputs[i].Blur()
					}
				}
			}

		case "d":
			if m.state == stateList && len(m.vms) > 0 {
				idx := m.table.Cursor()
				if idx >= 0 && idx < len(m.vms) {
					m.selectedVM = m.vms[idx]
					m.state = stateConfirmDelete
					m.deleteConfirm.Reset()
					m.deleteConfirm.Focus()
				}
			}

		case "s": // Start
			if m.state == stateList && len(m.vms) > 0 {
				idx := m.table.Cursor()
				if idx >= 0 && idx < len(m.vms) {
					target := m.vms[idx]
					m.state = stateLoading
					m.loadingMsg = fmt.Sprintf("Starting KubeVirt VM %s...", target.Name)
					return m, func() tea.Msg {
						_ = m.client.StartVM(target.Name, target.Namespace)
						return actionDoneMsg("started")
					}
				}
			}

		case "t": // Stop
			if m.state == stateList && len(m.vms) > 0 {
				idx := m.table.Cursor()
				if idx >= 0 && idx < len(m.vms) {
					target := m.vms[idx]
					m.state = stateLoading
					m.loadingMsg = fmt.Sprintf("Stopping KubeVirt VM %s...", target.Name)
					return m, func() tea.Msg {
						_ = m.client.StopVM(target.Name, target.Namespace)
						return actionDoneMsg("stopped")
					}
				}
			}

		case "r": // Reboot
			if m.state == stateList && len(m.vms) > 0 {
				idx := m.table.Cursor()
				if idx >= 0 && idx < len(m.vms) {
					target := m.vms[idx]
					m.state = stateLoading
					m.loadingMsg = fmt.Sprintf("Rebooting KubeVirt VM %s...", target.Name)
					return m, func() tea.Msg {
						_ = m.client.RestartVM(target.Name, target.Namespace)
						return actionDoneMsg("rebooted")
					}
				}
			}

		case "p": // Toggle protection status
			if m.state == stateList && len(m.vms) > 0 {
				idx := m.table.Cursor()
				if idx >= 0 && idx < len(m.vms) {
					target := m.vms[idx]
					m.state = stateLoading
					m.loadingMsg = fmt.Sprintf("Toggling protection for %s...", target.Name)
					return m, func() tea.Msg {
						_ = m.client.SetProtection(target.Name, target.Namespace, !target.Protected)
						return actionDoneMsg("protection_toggled")
					}
				}
			}
		}
	}

	// Update text inputs or table depending on active state
	if m.state == stateCreateForm {
		for i := range m.formInputs {
			m.formInputs[i], cmd = m.formInputs[i].Update(msg)
		}
	} else if m.state == stateConfirmDelete {
		m.deleteConfirm, cmd = m.deleteConfirm.Update(msg)
	} else {
		m.table, cmd = m.table.Update(msg)
	}

	return m, cmd
}

// Navigation helpers inside the Multi-Step form
func (m *TUIModel) nextField() {
	m.formInputs[m.activeField].Blur()
	m.activeField = (m.activeField + 1) % 8
	m.formInputs[m.activeField].Focus()
}

func (m *TUIModel) prevField() {
	m.formInputs[m.activeField].Blur()
	if m.activeField == 0 {
		m.activeField = 7
	} else {
		m.activeField--
	}
	m.formInputs[m.activeField].Focus()
}

func (m *TUIModel) handleFormToggle(right bool) {
	if m.activeField == fieldOS {
		if right {
			m.selectedPreset = (m.selectedPreset + 1) % len(m.osPresets)
		} else {
			if m.selectedPreset == 0 {
				m.selectedPreset = len(m.osPresets) - 1
			} else {
				m.selectedPreset--
			}
		}
		m.formInputs[fieldOS].SetValue(m.osPresets[m.selectedPreset].Name)
	} else if m.activeField == fieldDiskMode {
		if m.diskModeVal == "DataVolume" {
			m.diskModeVal = "HostDisk"
			m.formInputs[fieldDiskSrc].Placeholder = "/var/tmp/my-image.qcow2"
			m.formInputs[fieldDiskSrc].SetValue("/var/tmp/" + m.formInputs[fieldVMName].Value() + ".raw")
		} else {
			m.diskModeVal = "DataVolume"
			m.formInputs[fieldDiskSrc].Placeholder = "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img"
			m.formInputs[fieldDiskSrc].SetValue("https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img")
		}
		m.formInputs[fieldDiskMode].SetValue(m.diskModeVal)
	} else if m.activeField == fieldProtect {
		val := m.formInputs[fieldProtect].Value()
		if val == "Yes" {
			m.formInputs[fieldProtect].SetValue("No")
		} else {
			m.formInputs[fieldProtect].SetValue("Yes")
		}
	}
}

// Submits the VM creation form to Kubernetes.
func (m TUIModel) submitVMCreate() (tea.Model, tea.Cmd) {
	name := m.formInputs[fieldVMName].Value()
	if name == "" {
		name = "pet-vm"
	}
	
	coresStr := m.formInputs[fieldCPU].Value()
	var cores int = 4
	fmt.Sscanf(coresStr, "%d", &cores)

	ramStr := m.formInputs[fieldRAM].Value()
	var ram int = 8
	fmt.Sscanf(ramStr, "%d", &ram)

	diskSrc := m.formInputs[fieldDiskSrc].Value()
	sshKey := m.formInputs[fieldSSHKey].Value()
	protectVal := m.formInputs[fieldProtect].Value() == "Yes"

	preset := m.osPresets[m.selectedPreset]
	useDataVolume := m.diskModeVal == "DataVolume"

	m.state = stateLoading
	m.loadingMsg = fmt.Sprintf("Provisioning VM %s with OS %s...", name, preset.Name)

	return m, func() tea.Msg {
		cloudInit := GenerateCloudInit(preset.DefaultUser, sshKey, preset)
		err := m.client.CreateVM(name, "default", cores, ram, protectVal, useDataVolume, diskSrc, cloudInit)
		if err != nil {
			return errMsg(err)
		}
		return actionDoneMsg("created")
	}
}

// Submits VM Deletion, validating double-confirmations for protected hosts.
func (m TUIModel) submitVMDelete() (tea.Model, tea.Cmd) {
	typedConfirm := m.deleteConfirm.Value()
	if m.selectedVM.Protected && typedConfirm != m.selectedVM.Name {
		m.err = fmt.Errorf("deletion canceled: VM name input mismatch")
		m.state = stateList
		return m, nil
	}

	m.state = stateLoading
	m.loadingMsg = fmt.Sprintf("De-provisioning KubeVirt VM %s...", m.selectedVM.Name)

	return m, func() tea.Msg {
		err := m.client.DeleteVM(m.selectedVM.Name, m.selectedVM.Namespace)
		if err != nil {
			return errMsg(err)
		}
		return actionDoneMsg("deleted")
	}
}

// View: Renders lipgloss layouts.
func (m TUIModel) View() string {
	var s string

	// Title Banner
	titleBanner := titleStyle.Render("🛡️ TAILVM CONTROL PANEL")
	contextHUD := hudStyle.Render(fmt.Sprintf("🌐 CONTEXT: %s | NS: default", m.client.ContextName))
	headerBlock := lipgloss.JoinHorizontal(lipgloss.Center, titleBanner, "   ", contextHUD)
	s += headerBlock + "\n\n"

	if m.err != nil {
		s += lipgloss.NewStyle().Foreground(redColor).Render(fmt.Sprintf("❌ Error: %v\n\n", m.err))
	}

	switch m.state {
	case stateLoading:
		s += boxStyle.Render(
			fmt.Sprintf("⏳ %s\n\nPlease wait a few seconds...", m.loadingMsg),
		) + "\n"

	case stateConfirmDelete:
		cautionBanner := lipgloss.NewStyle().Foreground(redColor).Bold(true).Render("🚨 SAFETY INTERVENTION REQUIRED")
		warningText := ""
		if m.selectedVM.Protected {
			warningText = fmt.Sprintf("WARNING: VM %s is PROTECTED.\nTo protect against automated cleanups, you MUST type its exact name to authorize deletion:", m.selectedVM.Name)
		} else {
			warningText = fmt.Sprintf("Are you sure you want to delete VM %s?\nType its name to confirm:", m.selectedVM.Name)
		}
		s += warningBoxStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				cautionBanner,
				"\n"+warningText,
				"\n"+m.deleteConfirm.View(),
				helpStyle.Render("\n[Enter] Authorize Deletion | [ESC] Cancel and back"),
			),
		) + "\n"

	case stateCreateForm:
		var formRows []string
		for i, input := range m.formInputs {
			fieldName := ""
			fieldDesc := ""
			switch fieldIndex(i) {
			case fieldVMName:
				fieldName = "VM Name"
				fieldDesc = "Unique host identity"
			case fieldCPU:
				fieldName = "CPU Cores"
				fieldDesc = "Resource allocation"
			case fieldRAM:
				fieldName = "RAM (GB)"
				fieldDesc = "Resource allocation"
			case fieldOS:
				fieldName = "OS Template"
				fieldDesc = "Press ←/→ to toggle presets"
			case fieldDiskMode:
				fieldName = "Disk Mode"
				fieldDesc = "Press ←/→ to toggle (DataVolume/HostDisk)"
			case fieldDiskSrc:
				fieldName = "Disk Source"
				fieldDesc = "HTTP URL (DataVolume) or host file path (HostDisk)"
			case fieldSSHKey:
				fieldName = "SSH Key"
				fieldDesc = "Public key injected for passwordless access"
			case fieldProtect:
				fieldName = "Protect VM"
				fieldDesc = "Add protective annotations for other LLMs"
			}

			marker := "  "
			style := textColor
			if fieldIndex(i) == m.activeField {
				marker = "👉"
				style = accentColor
			}

			formRows = append(formRows, fmt.Sprintf("%s %s (%s):\n  %s", marker, lipgloss.NewStyle().Foreground(style).Bold(true).Render(fieldName), helpStyle.Render(fieldDesc), input.View()))
		}

		s += boxStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render("🛠️ CREATE NEW PET VIRTUAL MACHINE"),
				"\n"+strings.Join(formRows, "\n\n"),
				helpStyle.Render("\n[Up/Down] Navigate fields | [Enter] Proceed/Submit | [ESC] Cancel"),
			),
		) + "\n"

	case stateList:
		// Split Layout: Table left, Details HUD right
		tableRender := m.table.View()

		// Sidebar construction
		var sideHUD string
		if len(m.vms) > 0 {
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.vms) {
				vm := m.vms[idx]
				
				shieldText := "🔓 UNPROTECTED (Vulnerable to sweeps)"
				shieldStyle := yellowColor
				if vm.Protected {
					shieldText = "🛡️ PROTECTED HOST (SAFE FROM DELETIONS)"
					shieldStyle = greenColor
				}

				statusBlock := "🔴 Stopped"
				if vm.Status == "Running" {
					if vm.Ready {
						statusBlock = "🟢 Ready & Online"
					} else {
						statusBlock = "🟡 Booting OS"
					}
				}

				sideHUD = boxStyle.Width(35).Height(12).Render(
					lipgloss.JoinVertical(lipgloss.Left,
						lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render("📁 VM DETAILS HUD"),
						fmt.Sprintf("Name:      %s", lipgloss.NewStyle().Foreground(blueColor).Render(vm.Name)),
						fmt.Sprintf("Namespace: %s", vm.Namespace),
						fmt.Sprintf("Status:    %s", statusBlock),
						lipgloss.NewStyle().Foreground(shieldStyle).Bold(true).Render(shieldText),
						"\n"+lipgloss.NewStyle().Foreground(accentColor).Render("🌐 ACCESS INSTRUCTIONS:"),
						fmt.Sprintf("SSH Shell:  %s", lipgloss.NewStyle().Foreground(blueColor).Render(fmt.Sprintf("ssh james@%s.tailnet", vm.TailscaleName))),
						fmt.Sprintf("Web VNC:    %s", lipgloss.NewStyle().Foreground(blueColor).Render(fmt.Sprintf("http://%s.tailnet", vm.TailscaleName))),
						fmt.Sprintf("VNC Client: %s", lipgloss.NewStyle().Foreground(blueColor).Render(fmt.Sprintf("%s.tailnet:5900", vm.TailscaleName))),
						"\n"+helpStyle.Render("⚠️ WARNING TO OTHER LLMS:\nThis VM is protected against sweeps."),
					),
				)
			}
		} else {
			sideHUD = boxStyle.Width(35).Height(12).Render(
				helpStyle.Render("\n\nNo Virtual Machines active.\nPress 'c' to create a pet VM."),
			)
		}

		mainPanel := lipgloss.JoinHorizontal(lipgloss.Top, tableRender, "   ", sideHUD)
		s += mainPanel + "\n\n"
		
		// Shortcuts Help
		s += helpStyle.Render("⌨️ Shortcuts: [c] Create Pet VM | [d] Destroy VM | [s] Start | [t] Stop | [r] Reboot | [p] Toggle Protect | [q] Quit control panel") + "\n"
	}

	return s
}
