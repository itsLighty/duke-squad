package app

import (
	"claude-squad/config"
	"claude-squad/session"
	"claude-squad/ui"
	"claude-squad/ui/overlay"
	"context"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSessionProfilesSelectsDefaultProgramProfile(t *testing.T) {
	cfg := &config.Config{
		DefaultProgram: "/usr/local/bin/claude",
	}

	profiles, selected := buildSessionProfiles(cfg, cfg.GetProgram(), false)

	require.Len(t, profiles, 2)
	assert.Equal(t, 0, selected)
	assert.Equal(t, "Claude", profiles[selected].Name)
}

func TestBuildSessionProfilesPreselectsOverrideProgram(t *testing.T) {
	cfg := &config.Config{
		DefaultProgram: "/usr/local/bin/claude",
	}

	profiles, selected := buildSessionProfiles(cfg, "codex", true)

	require.Len(t, profiles, 2)
	assert.Equal(t, 1, selected)
	assert.Equal(t, "Codex", profiles[selected].Name)
	assert.Contains(t, profiles[selected].Program, "check_for_update_on_startup=false")
}

func TestBuildSessionProfilesAddsTemporaryOverrideProgramWhenMissing(t *testing.T) {
	cfg := &config.Config{
		DefaultProgram: "claude",
		Profiles: []config.Profile{
			{Name: "claude", Program: "claude"},
		},
	}

	profiles, selected := buildSessionProfiles(cfg, "codex", true)

	require.Len(t, profiles, 2)
	assert.Equal(t, 1, selected)
	assert.Equal(t, "Codex", profiles[selected].Name)
	assert.NotEmpty(t, profiles[selected].Program)
}

func TestBuildSessionProfilesAppendsCustomConfiguredProfileWithoutPathLabel(t *testing.T) {
	cfg := &config.Config{
		DefaultProgram: "claude",
		Profiles: []config.Profile{
			{Name: "/usr/local/bin/custom-agent --fast", Program: "/usr/local/bin/custom-agent --fast"},
		},
	}

	profiles, _ := buildSessionProfiles(cfg, cfg.GetProgram(), false)

	require.Len(t, profiles, 3)
	assert.Equal(t, "Custom-agent", profiles[2].Name)
	assert.Equal(t, "/usr/local/bin/custom-agent --fast", profiles[2].Program)
}

func TestNewCreateSessionInstanceUsesSelectedProgram(t *testing.T) {
	instance, err := newCreateSessionInstance("feature", ".", "codex", "ship it", "", true)
	require.NoError(t, err)

	assert.Equal(t, "codex", providerKey(instance.Program))
	assert.Equal(t, "ship it", instance.Prompt)
	assert.True(t, instance.AutoYes)
	assert.Equal(t, session.Loading, instance.Status)
}

func TestCancelCreateOverlayLeavesListUntouched(t *testing.T) {
	spin := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	list := ui.NewList(&spin, false)

	existing, err := session.NewInstance(session.InstanceOptions{
		Title:   "existing",
		Path:    t.TempDir(),
		Program: "claude",
	})
	require.NoError(t, err)

	list.AddInstance(existing)
	list.SetSelectedInstance(0)

	h := &home{
		ctx:              context.Background(),
		state:            stateCreate,
		list:             list,
		menu:             ui.NewMenu(),
		textInputOverlay: overlay.NewSessionCreateOverlay("", []config.Profile{{Name: "claude", Program: "claude"}}, 0, false),
	}

	cmd := h.cancelCreateOverlay()

	assert.NotNil(t, cmd)
	assert.Equal(t, stateDefault, h.state)
	assert.Nil(t, h.textInputOverlay)
	assert.Len(t, h.list.GetInstances(), 1)
	assert.Equal(t, "existing", h.list.GetSelectedInstance().Title)
}
