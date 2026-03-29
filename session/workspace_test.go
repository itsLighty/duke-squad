package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFolderWorkspaceSetupCreatesManagedGitCopy(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	sourceDir := t.TempDir()
	sourceFile := filepath.Join(sourceDir, "notes.txt")
	require.NoError(t, os.WriteFile(sourceFile, []byte("original"), 0644))

	workspace, _, err := newFolderWorkspace(sourceDir, "sess_test1234", "Folder Task")
	require.NoError(t, err)

	err = workspace.Setup()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = workspace.Cleanup()
	})

	folderWorkspace := workspace.(*folderWorkspace)
	require.DirExists(t, folderWorkspace.workspacePath)
	require.FileExists(t, filepath.Join(folderWorkspace.workspacePath, ".git", "HEAD"))
	require.FileExists(t, filepath.Join(folderWorkspace.workspacePath, "notes.txt"))
	require.NotEmpty(t, folderWorkspace.baseCommitSHA)

	workspaceFile := filepath.Join(folderWorkspace.workspacePath, "notes.txt")
	require.NoError(t, os.WriteFile(workspaceFile, []byte("changed"), 0644))

	stats := workspace.Diff()
	require.NoError(t, stats.Error)
	require.NotZero(t, stats.Added+stats.Removed)

	originalContent, err := os.ReadFile(sourceFile)
	require.NoError(t, err)
	require.Equal(t, "original", string(originalContent))
}
