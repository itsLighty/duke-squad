package git

import (
	"claude-squad/transport"
	"fmt"
	"os"
	"os/exec"
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

func TestCleanupWorktreesDoesNotDependOnCurrentDirectory(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	repoDir := t.TempDir()
	runGit := func(path string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", path}, args...)...)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))
		return string(output)
	}

	runGit(repoDir, "init")
	runGit(repoDir, "config", "user.email", "test@example.com")
	runGit(repoDir, "config", "user.name", "Test User")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello"), 0644))
	runGit(repoDir, "add", ".")
	runGit(repoDir, "commit", "-m", "initial commit")

	worktree, branchName, err := NewGitWorktree(repoDir, "sess_test-cleanup")
	require.NoError(t, err)
	require.NoError(t, worktree.Setup())
	require.DirExists(t, worktree.GetWorktreePath())

	otherDir := t.TempDir()
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(otherDir))
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	require.NoError(t, CleanupWorktrees())
	_, err = os.Stat(worktree.GetWorktreePath())
	require.True(t, os.IsNotExist(err))

	branches := runGit(repoDir, "branch", "--list", branchName)
	require.Empty(t, branches)
}

func TestGitWorktreeExistsReturnsRunnerErrorForRemoteChecks(t *testing.T) {
	worktree := NewGitWorktreeFromStorage(
		stubRunner{
			combinedOutput: func(spec transport.CommandSpec) ([]byte, error) {
				return nil, fmt.Errorf("permission denied")
			},
		},
		"/srv/repo",
		"/srv/worktree",
		"session",
		"codex/session",
		"abc123",
		false,
	)

	exists, err := worktree.Exists()

	require.False(t, exists)
	require.ErrorContains(t, err, "permission denied")
}
