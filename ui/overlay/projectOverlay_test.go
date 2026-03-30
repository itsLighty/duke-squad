package overlay

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestProjectOverlayDefaultsToLocalProject(t *testing.T) {
	overlay := NewProjectOverlay("/tmp/work")

	opts := overlay.ProjectOptions()
	require.Equal(t, "/tmp/work", opts.Path)
	require.Equal(t, "local", string(opts.Transport))
}

func TestProjectOverlaySupportsSSHFields(t *testing.T) {
	overlay := NewProjectOverlay("/tmp/work")

	overlay.HandleKeyPress(tea.KeyMsg{Type: tea.KeyRight})
	require.True(t, overlay.isRemote())

	overlay.HandleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	for _, r := range "face-swapper" {
		overlay.HandleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	overlay.HandleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	for _, r := range "dukebot" {
		overlay.HandleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	overlay.HandleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	for _, r := range "dukebot.local" {
		overlay.HandleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	overlay.HandleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	for _, r := range "supersecret" {
		overlay.HandleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	overlay.HandleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	for _, r := range "/srv/repo" {
		overlay.HandleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	opts := overlay.ProjectOptions()
	require.Equal(t, "ssh", string(opts.Transport))
	require.Equal(t, "face-swapper", opts.Name)
	require.Equal(t, "dukebot", opts.SSHUser)
	require.Equal(t, "dukebot.local", opts.SSHHost)
	require.Equal(t, "supersecret", opts.SSHPassword)
	require.Equal(t, "/srv/repo", opts.Path)
}
