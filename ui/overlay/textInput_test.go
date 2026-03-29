package overlay

import (
	"claude-squad/config"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSessionCreateOverlayWithoutPromptIncludesProviderChoice(t *testing.T) {
	overlay := NewSessionCreateOverlay("", []config.Profile{
		{Name: "claude", Program: "claude"},
		{Name: "codex", Program: "codex"},
	}, 0, false)

	assert.True(t, overlay.hasTitleInput)
	assert.False(t, overlay.hasPrompt)
	assert.Nil(t, overlay.branchPicker)
	assert.Equal(t, "claude", overlay.GetSelectedProgram())
	assert.Equal(t, overlay.titleInputIndex(), overlay.FocusIndex)
}

func TestNewSessionCreateOverlayWithPromptIncludesPromptAndBranchPicker(t *testing.T) {
	overlay := NewSessionCreateOverlay("Initial prompt", []config.Profile{
		{Name: "claude", Program: "claude"},
		{Name: "codex", Program: "codex"},
	}, 1, true)

	assert.True(t, overlay.hasPrompt)
	require.NotNil(t, overlay.branchPicker)
	assert.Equal(t, "codex", overlay.GetSelectedProgram())
	assert.Equal(t, overlay.titleInputIndex(), overlay.FocusIndex)
}

func TestSessionCreateOverlayCanSwitchProviderFromOverrideSelection(t *testing.T) {
	overlay := NewSessionCreateOverlay("", []config.Profile{
		{Name: "claude", Program: "claude"},
		{Name: "codex", Program: "codex"},
	}, 1, false)

	overlay.setFocusIndex(overlay.profilePickerIndex())
	closed, branchChanged := overlay.HandleKeyPress(tea.KeyMsg{Type: tea.KeyLeft})

	assert.False(t, closed)
	assert.False(t, branchChanged)
	assert.Equal(t, "claude", overlay.GetSelectedProgram())
}
