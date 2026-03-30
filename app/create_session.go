package app

import (
	"claude-squad/config"
	"claude-squad/session"
	"claude-squad/session/git"
	"claude-squad/ui"
	"claude-squad/ui/overlay"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
)

var builtinProviderOrder = []string{"claude", "codex"}

var builtinProviderNames = map[string]string{
	"claude": "Claude",
	"codex":  "Codex",
}

func commandForProvider(provider string) string {
	if provider == "claude" {
		if cmd, err := config.GetClaudeCommand(); err == nil && cmd != "" {
			return cmd
		}
	}
	if provider == "codex" {
		if cmd, err := config.GetProgramCommand(provider); err == nil && cmd != "" {
			return config.NormalizeProgramCommand(cmd + " -c check_for_update_on_startup=false --no-alt-screen")
		}
	}
	if cmd, err := exec.LookPath(provider); err == nil && cmd != "" {
		if provider == "codex" {
			return config.NormalizeProgramCommand(cmd + " -c check_for_update_on_startup=false --no-alt-screen")
		}
		return cmd
	}
	if provider == "codex" {
		return config.NormalizeProgramCommand("codex -c check_for_update_on_startup=false --no-alt-screen")
	}
	return provider
}

func providerKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	token := strings.Fields(value)[0]
	token = strings.Trim(token, `"'`)
	token = filepath.Base(token)
	token = strings.TrimSuffix(token, filepath.Ext(token))
	return strings.ToLower(token)
}

func displayNameForProfile(profile config.Profile) string {
	if name := builtinProviderNames[providerKey(profile.Name)]; name != "" {
		return name
	}
	if name := builtinProviderNames[providerKey(profile.Program)]; name != "" {
		return name
	}
	name := strings.TrimSpace(profile.Name)
	if name != "" && !strings.ContainsRune(name, os.PathSeparator) {
		return name
	}
	key := providerKey(profile.Program)
	if key == "" {
		key = providerKey(profile.Name)
	}
	if key == "" {
		return "Custom"
	}
	return strings.ToUpper(key[:1]) + key[1:]
}

func builtinProviderProfiles() []config.Profile {
	profiles := make([]config.Profile, 0, len(builtinProviderOrder))
	for _, provider := range builtinProviderOrder {
		profiles = append(profiles, config.Profile{
			Name:    builtinProviderNames[provider],
			Program: commandForProvider(provider),
		})
	}
	return profiles
}

func profileKeys(profiles []config.Profile) []string {
	keys := make([]string, 0, len(profiles)*2)
	for _, profile := range profiles {
		for _, key := range []string{providerKey(profile.Name), providerKey(profile.Program)} {
			if key == "" || slices.Contains(keys, key) {
				continue
			}
			keys = append(keys, key)
		}
	}
	return keys
}

func appendProfileIfMissing(profiles []config.Profile, profile config.Profile) []config.Profile {
	for _, existing := range profiles {
		if existing.Program == profile.Program || existing.Name == profile.Name {
			return profiles
		}
	}
	return append(profiles, profile)
}

func buildSessionProfiles(cfg *config.Config, effectiveProgram string, programOverridden bool) ([]config.Profile, int) {
	profiles := builtinProviderProfiles()
	for _, profile := range cfg.GetProfiles() {
		if key := providerKey(profile.Program); key != "" && slices.Contains(builtinProviderOrder, key) {
			for i := range profiles {
				if providerKey(profiles[i].Name) == key || providerKey(profiles[i].Program) == key {
					profiles[i] = config.Profile{
						Name:    builtinProviderNames[key],
						Program: profile.Program,
					}
					break
				}
			}
			continue
		}
		if key := providerKey(profile.Name); key != "" && slices.Contains(builtinProviderOrder, key) {
			for i := range profiles {
				if providerKey(profiles[i].Name) == key || providerKey(profiles[i].Program) == key {
					profiles[i] = config.Profile{
						Name:    builtinProviderNames[key],
						Program: profile.Program,
					}
					break
				}
			}
			continue
		}
		profiles = appendProfileIfMissing(profiles, config.Profile{
			Name:    displayNameForProfile(profile),
			Program: profile.Program,
		})
	}

	selectedIndex := 0

	for i, profile := range profiles {
		if strings.EqualFold(profile.Name, effectiveProgram) || profile.Program == effectiveProgram {
			return profiles, i
		}
	}
	for i, profile := range profiles {
		if key := providerKey(effectiveProgram); key != "" {
			if providerKey(profile.Name) == key || providerKey(profile.Program) == key {
				return profiles, i
			}
		}
	}

	if programOverridden && effectiveProgram != "" {
		profile := config.Profile{
			Name:    displayNameForProfile(config.Profile{Program: effectiveProgram}),
			Program: effectiveProgram,
		}
		profiles = appendProfileIfMissing(profiles, profile)
		for i, existing := range profiles {
			if existing.Program == profile.Program {
				selectedIndex = i
				break
			}
		}
		return profiles, selectedIndex
	}

	if effectiveProgram != "" {
		key := providerKey(effectiveProgram)
		for i, profile := range profiles {
			if providerKey(profile.Name) == key || providerKey(profile.Program) == key {
				return profiles, i
			}
		}
	}

	if defaultKey := providerKey(cfg.DefaultProgram); defaultKey != "" {
		for i, profile := range profiles {
			if providerKey(profile.Name) == defaultKey || providerKey(profile.Program) == defaultKey {
				return profiles, i
			}
		}
	}

	return profiles, selectedIndex
}

func newCreateSessionInstance(title string, project *session.Project, program, prompt, branch string, autoYes bool) (*session.Instance, error) {
	if project == nil {
		return nil, fmt.Errorf("add a project first")
	}

	instance, err := session.NewInstance(session.InstanceOptions{
		Title:            title,
		Path:             project.RootPath,
		Program:          program,
		AutoYes:          autoYes,
		ProjectID:        project.ID,
		ProjectKind:      project.Kind,
		ProjectTransport: project.Transport,
		SSHTarget:        project.SSHTarget,
		SSHUser:          project.SSHUser,
		SSHHost:          project.SSHHost,
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
	project := m.list.GetSelectedProject()
	title := ""
	if includePrompt {
		title = "Initial prompt"
	}
	withBranchPicker := project != nil && project.Kind == session.ProjectKindGit
	return overlay.NewSessionCreateOverlay(title, profiles, selectedProfile, includePrompt, withBranchPicker)
}

func (m *home) beginCreateSession(includePrompt bool) tea.Cmd {
	project := m.list.GetSelectedProject()
	if project == nil {
		return m.handleError(fmt.Errorf("add a project first"))
	}

	m.state = stateCreate
	m.menu.SetState(ui.StateNewInstance)
	m.textInputOverlay = m.newCreateSessionOverlay(includePrompt)

	cmds := []tea.Cmd{tea.WindowSize()}
	if includePrompt && project.Kind == session.ProjectKindGit {
		runner, err := project.Runner()
		if err != nil {
			return m.handleError(err)
		}
		fetchCmd := func() tea.Msg {
			git.FetchBranchesWithRunner(runner, project.RootPath)
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
	project := m.list.GetSelectedProject()
	if project == nil {
		return m.handleError(fmt.Errorf("add a project first"))
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
		project,
		program,
		m.textInputOverlay.GetPromptValue(),
		m.textInputOverlay.GetSelectedBranch(),
		m.autoYes,
	)
	if err != nil {
		return m.handleError(err)
	}

	m.list.AddSession(project.ID, instance)
	m.textInputOverlay = nil
	m.state = stateDefault
	m.menu.SetState(ui.StateDefault)
	m.instanceStarting = true
	m.startingInstance = instance

	startCmd := runInstanceStartCmd(instance)
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
