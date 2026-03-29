package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

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
