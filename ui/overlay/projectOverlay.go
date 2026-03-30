package overlay

import (
	"claude-squad/config"
	"claude-squad/session"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	projectSourceLocal  = "local"
	projectSourceRemote = "remote"
)

type ProjectOverlay struct {
	sourcePicker *ProfilePicker
	pathPicker   *PathPicker
	nameInput    textinput.Model
	sshUser      textinput.Model
	sshHost      textinput.Model
	sshPassword  textinput.Model
	remotePath   textinput.Model
	focusIndex   int
	submitted    bool
	canceled     bool
	errorText    string
	width        int
	height       int
}

func NewProjectOverlay(initialLocalPath string) *ProjectOverlay {
	nameInput := textinput.New()
	nameInput.Placeholder = "Project name (optional)"
	nameInput.Prompt = ""
	nameInput.CharLimit = 0

	sshUser := textinput.New()
	sshUser.Placeholder = "dukebot"
	sshUser.Prompt = ""
	sshUser.CharLimit = 0

	sshHost := textinput.New()
	sshHost.Placeholder = "dukebot.local or 192.168.1.20"
	sshHost.Prompt = ""
	sshHost.CharLimit = 0

	sshPassword := textinput.New()
	sshPassword.Placeholder = "Password"
	sshPassword.Prompt = ""
	sshPassword.CharLimit = 0
	sshPassword.EchoMode = textinput.EchoPassword
	sshPassword.EchoCharacter = '•'

	remotePath := textinput.New()
	remotePath.Placeholder = "~/project or /srv/project"
	remotePath.Prompt = ""
	remotePath.CharLimit = 0

	overlay := &ProjectOverlay{
		sourcePicker: NewLabeledProfilePicker("Source", []config.Profile{
			{Name: "Local", Program: projectSourceLocal},
			{Name: "Remote", Program: projectSourceRemote},
		}),
		pathPicker:  NewPathPicker(initialLocalPath),
		nameInput:   nameInput,
		sshUser:     sshUser,
		sshHost:     sshHost,
		sshPassword: sshPassword,
		remotePath:  remotePath,
	}
	overlay.updateFocusState()
	return overlay
}

func (p *ProjectOverlay) SetSize(width, height int) {
	p.width = width
	p.height = height
	innerWidth := max(1, width-6)
	p.sourcePicker.SetWidth(innerWidth)
	p.pathPicker.SetWidth(innerWidth)
	p.nameInput.Width = innerWidth
	p.sshUser.Width = innerWidth
	p.sshHost.Width = innerWidth
	p.sshPassword.Width = innerWidth
	p.remotePath.Width = innerWidth
}

func (p *ProjectOverlay) HandleKeyPress(msg tea.KeyMsg) bool {
	p.errorText = ""

	switch msg.Type {
	case tea.KeyTab:
		if p.isLocalPathFocused() && p.pathPicker.AcceptSuggestion() {
			return false
		}
		p.setFocusIndex((p.focusIndex + 1) % p.focusStops())
		return false
	case tea.KeyShiftTab:
		p.setFocusIndex((p.focusIndex - 1 + p.focusStops()) % p.focusStops())
		return false
	case tea.KeyEsc:
		p.canceled = true
		return true
	case tea.KeyEnter:
		switch {
		case p.isEnterFocused():
			p.submitted = true
			return true
		case p.isSourceFocused():
			p.setFocusIndex(p.focusIndex + 1)
			return false
		case p.isLocalPathFocused(), p.isRemoteNameFocused(), p.isSSHUserFocused(), p.isSSHHostFocused(), p.isSSHPasswordFocused(), p.isRemotePathFocused():
			p.setFocusIndex(min(p.focusIndex+1, p.enterIndex()))
			return false
		}
	}

	switch {
	case p.isSourceFocused():
		if p.sourcePicker.HandleKeyPress(msg) {
			p.setFocusIndex(0)
		}
	case p.isLocalPathFocused():
		p.pathPicker.HandleKeyPress(msg)
	case p.isRemoteNameFocused():
		p.nameInput, _ = p.nameInput.Update(msg)
	case p.isSSHUserFocused():
		p.sshUser, _ = p.sshUser.Update(msg)
	case p.isSSHHostFocused():
		p.sshHost, _ = p.sshHost.Update(msg)
	case p.isSSHPasswordFocused():
		p.sshPassword, _ = p.sshPassword.Update(msg)
	case p.isRemotePathFocused():
		p.remotePath, _ = p.remotePath.Update(msg)
	}
	return false
}

func (p *ProjectOverlay) SetError(message string) {
	p.errorText = strings.TrimSpace(message)
}

func (p *ProjectOverlay) IsSubmitted() bool {
	return p.submitted
}

func (p *ProjectOverlay) IsCanceled() bool {
	return p.canceled
}

func (p *ProjectOverlay) Render() string {
	innerWidth := max(1, p.width-6)
	divider := tiDividerStyle.Render(strings.Repeat("─", innerWidth))

	var sections []string
	sections = append(sections, p.sourcePicker.Render())
	if p.isRemote() {
		sections = append(sections, tiTitleStyle.Render("Project name")+"\n"+p.nameInput.View())
		sections = append(sections, tiTitleStyle.Render("SSH username")+"\n"+p.sshUser.View())
		sections = append(sections, tiTitleStyle.Render("Host / IP")+"\n"+p.sshHost.View())
		sections = append(sections, tiTitleStyle.Render("Password")+"\n"+p.sshPassword.View())
		sections = append(sections, tiTitleStyle.Render("Remote folder")+"\n"+p.remotePath.View())
	} else {
		sections = append(sections, p.pathPicker.Render("Project folder"))
	}

	content := strings.Join(sections, "\n\n"+divider+"\n\n")
	content += "\n\n" + divider + "\n\n"
	if p.errorText != "" {
		content += bpSelectedStyle.Render(" " + p.errorText + " ")
		content += "\n\n"
	}

	enterButton := " Enter "
	if p.isEnterFocused() {
		enterButton = tiFocusedButtonStyle.Render(enterButton)
	} else {
		enterButton = tiButtonStyle.Render(enterButton)
	}
	content += enterButton
	return tiStyle.Render(content)
}

func (p *ProjectOverlay) View() string {
	return p.Render()
}

func (p *ProjectOverlay) ProjectOptions() session.ProjectOptions {
	if p.isRemote() {
		return session.ProjectOptions{
			Transport:   session.ProjectTransportSSH,
			Name:        strings.TrimSpace(p.nameInput.Value()),
			SSHUser:     strings.TrimSpace(p.sshUser.Value()),
			SSHHost:     strings.TrimSpace(p.sshHost.Value()),
			SSHPassword: p.sshPassword.Value(),
			Path:        strings.TrimSpace(p.remotePath.Value()),
		}
	}

	return session.ProjectOptions{
		Transport: session.ProjectTransportLocal,
		Path:      p.pathPicker.Value(),
	}
}

func (p *ProjectOverlay) isRemote() bool {
	return p.sourcePicker.GetSelectedProfile().Program == projectSourceRemote
}

func (p *ProjectOverlay) focusStops() int {
	return p.enterIndex() + 1
}

func (p *ProjectOverlay) enterIndex() int {
	if p.isRemote() {
		return 6
	}
	return 2
}

func (p *ProjectOverlay) setFocusIndex(idx int) {
	p.focusIndex = idx
	p.updateFocusState()
}

func (p *ProjectOverlay) updateFocusState() {
	if p.isSourceFocused() {
		p.sourcePicker.Focus()
	} else {
		p.sourcePicker.Blur()
	}

	if p.isLocalPathFocused() {
		p.pathPicker.Focus()
	} else {
		p.pathPicker.Blur()
	}

	if p.isRemoteNameFocused() {
		p.nameInput.Focus()
	} else {
		p.nameInput.Blur()
	}

	if p.isSSHUserFocused() {
		p.sshUser.Focus()
	} else {
		p.sshUser.Blur()
	}

	if p.isSSHHostFocused() {
		p.sshHost.Focus()
	} else {
		p.sshHost.Blur()
	}

	if p.isSSHPasswordFocused() {
		p.sshPassword.Focus()
	} else {
		p.sshPassword.Blur()
	}

	if p.isRemotePathFocused() {
		p.remotePath.Focus()
	} else {
		p.remotePath.Blur()
	}
}

func (p *ProjectOverlay) isSourceFocused() bool {
	return p.focusIndex == 0
}

func (p *ProjectOverlay) isLocalPathFocused() bool {
	return !p.isRemote() && p.focusIndex == 1
}

func (p *ProjectOverlay) isRemoteNameFocused() bool {
	return p.isRemote() && p.focusIndex == 1
}

func (p *ProjectOverlay) isSSHUserFocused() bool {
	return p.isRemote() && p.focusIndex == 2
}

func (p *ProjectOverlay) isSSHHostFocused() bool {
	return p.isRemote() && p.focusIndex == 3
}

func (p *ProjectOverlay) isSSHPasswordFocused() bool {
	return p.isRemote() && p.focusIndex == 4
}

func (p *ProjectOverlay) isRemotePathFocused() bool {
	return p.isRemote() && p.focusIndex == 5
}

func (p *ProjectOverlay) isEnterFocused() bool {
	return p.focusIndex == p.enterIndex()
}
