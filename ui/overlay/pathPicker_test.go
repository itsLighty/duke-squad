package overlay

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProjectPathOverlayUsesPathPicker(t *testing.T) {
	overlay := NewProjectPathOverlay("Project folder", "")

	require.NotNil(t, overlay.pathPicker)
	assert.True(t, overlay.isPathPicker())
}

func TestProjectPathOverlayTabAutocompletesDirectory(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "alpha"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, "beta"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "notes.txt"), []byte("x"), 0o644))

	overlay := NewProjectPathOverlay("Project folder", filepath.Join(root, "a"))

	closed, changed := overlay.HandleKeyPress(tea.KeyMsg{Type: tea.KeyTab})

	assert.False(t, closed)
	assert.False(t, changed)
	assert.Equal(t, filepath.Join(root, "alpha")+string(os.PathSeparator), overlay.GetPathValue())
	assert.True(t, overlay.isPathPicker())
}

func TestProjectPathOverlayTabMovesToSubmitWhenAlreadyCompleted(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "alpha"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, "alpha", "child"), 0o755))

	overlay := NewProjectPathOverlay("Project folder", filepath.Join(root, "alpha")+string(os.PathSeparator))

	closed, changed := overlay.HandleKeyPress(tea.KeyMsg{Type: tea.KeyTab})

	assert.False(t, closed)
	assert.False(t, changed)
	assert.True(t, overlay.isEnterButton())
	assert.Equal(t, filepath.Join(root, "alpha")+string(os.PathSeparator), overlay.GetPathValue())
}
