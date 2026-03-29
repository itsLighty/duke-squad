package app

import (
	"claude-squad/config"
	"claude-squad/session"
	"claude-squad/session/git"
	"claude-squad/ui"
	"claude-squad/ui/overlay"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
)

func buildSessionProfiles(cfg *config.Config, effectiveProgram string, programOverridden bool) ([]config.Profile, int) {
	profiles := cfg.GetProfiles()
	selectedIndex := 0

	for i, profile := range profiles {
		if profile.Name == effectiveProgram {
			return profiles, i
		}
	}
	for i, profile := range profiles {
		if profile.Program == effectiveProgram {
			return profiles, i
		}
	}

	if effectiveProgram != "" {
		profiles = append(profiles, config.Profile{
			Name:    effectiveProgram,
			Program: effectiveProgram,
		})
		selectedIndex = len(profiles) - 1
	}

	return profiles, selectedIndex
}

func newCreateSessionInstance(title, path, program, prompt, branch string, autoYes bool) (*session.Instance, error) {
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   title,
		Path:    path,
		Program: program,
		AutoYes: autoYes,
	})
	if err != nil {
		return nil, err
	}

	if branch != "" {
		instance.SetSelectedBranch(branch)
	}
	instance.Prompt = prompt
	instance.AutoYes = autoYes
	instance.SetStatus(session.Loading)

	return instance, nil
}

func (m *home) newCreateSessionOverlay(includePrompt bool) *overlay.TextInputOverlay {
	profiles, selectedProfile := buildSessionProfiles(m.appConfig, m.program, m.programOverridden)
	title := ""
	if includePrompt {
		title = "Initial prompt"
	}
	return overlay.NewSessionCreateOverlay(title, profiles, selectedProfile, includePrompt)
}

func (m *home) beginCreateSession(includePrompt bool) tea.Cmd {
	m.state = stateCreate
	m.menu.SetState(ui.StateNewInstance)
	m.textInputOverlay = m.newCreateSessionOverlay(includePrompt)

	cmds := []tea.Cmd{tea.WindowSize()}
	if includePrompt {
		fetchCmd := func() tea.Msg {
			currentDir, _ := os.Getwd()
			git.FetchBranches(currentDir)
			return nil
		}
		cmds = append(cmds, fetchCmd, m.runBranchSearch("", m.textInputOverlay.BranchFilterVersion()))
	}

	return tea.Batch(cmds...)
}

func (m *home) submitCreateSession() tea.Cmd {
	if m.textInputOverlay == nil {
		return nil
	}

	title := strings.TrimSpace(m.textInputOverlay.GetTitleValue())
	if title == "" {
		return m.handleError(fmt.Errorf("title cannot be empty"))
	}
	if runewidth.StringWidth(title) > 32 {
		return m.handleError(fmt.Errorf("title cannot be longer than 32 characters"))
	}

	program := m.textInputOverlay.GetSelectedProgram()
	if program == "" {
		program = m.program
	}

	instance, err := newCreateSessionInstance(
		title,
		".",
		program,
		m.textInputOverlay.GetPromptValue(),
		m.textInputOverlay.GetSelectedBranch(),
		m.autoYes,
	)
	if err != nil {
		return m.handleError(err)
	}

	finalize := m.list.AddInstance(instance)
	m.list.SelectInstance(instance)
	m.textInputOverlay = nil
	m.state = stateDefault
	m.menu.SetState(ui.StateDefault)

	startCmd := func() tea.Msg {
		err := instance.Start(true)
		return instanceStartedMsg{
			instance: instance,
			err:      err,
			finalize: finalize,
		}
	}

	return tea.Batch(tea.WindowSize(), m.instanceChanged(), startCmd)
}

func (m *home) cancelCreateOverlay() tea.Cmd {
	m.textInputOverlay = nil
	m.state = stateDefault
	return tea.Sequence(
		tea.WindowSize(),
		func() tea.Msg {
			m.menu.SetState(ui.StateDefault)
			return nil
		},
	)
}
