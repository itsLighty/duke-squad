package session

import (
	"claude-squad/transport"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubRunner struct {
	combinedOutput func(spec transport.CommandSpec) ([]byte, error)
}

func (s stubRunner) Kind() transport.Kind {
	return transport.KindSSH
}

func (s stubRunner) Target() string {
	return "dukebot@dukebot.local"
}

func (s stubRunner) Run(spec transport.CommandSpec) error {
	return nil
}

func (s stubRunner) Output(spec transport.CommandSpec) ([]byte, error) {
	return nil, nil
}

func (s stubRunner) CombinedOutput(spec transport.CommandSpec) ([]byte, error) {
	if s.combinedOutput != nil {
		return s.combinedOutput(spec)
	}
	return nil, nil
}

func (s stubRunner) StartPTY(spec transport.CommandSpec) (*os.File, error) {
	return nil, nil
}

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

func TestRemoteFolderWorkspaceExistsReturnsRunnerError(t *testing.T) {
	workspace := &remoteFolderWorkspace{
		runner: stubRunner{
			combinedOutput: func(spec transport.CommandSpec) ([]byte, error) {
				return nil, fmt.Errorf("permission denied")
			},
		},
		workspacePath: "/srv/managed/faceswap",
	}

	exists, err := workspace.Exists()

	require.False(t, exists)
	require.ErrorContains(t, err, "permission denied")
}
