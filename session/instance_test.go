package session

import (
	"claude-squad/config"
	"claude-squad/log"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromInstanceDataWithMissingTmuxSessionLoadsAsStoppedInstance(t *testing.T) {
	log.Initialize(false)
	defer log.Close()

	tempDir := t.TempDir()
	title := fmt.Sprintf("missing-session-%d", time.Now().UnixNano())

	instance, err := FromInstanceData(InstanceData{
		Title:     title,
		Path:      tempDir,
		Branch:    "test/missing-session",
		Status:    Ready,
		Program:   "/Users/test/Library/pnpm/codex -c check_for_update_on_startup=false",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Worktree: GitWorktreeData{
			RepoPath:         tempDir,
			WorktreePath:     filepath.Join(tempDir, "worktree"),
			SessionName:      title,
			BranchName:       "test/missing-session",
			BaseCommitSHA:    "deadbeef",
			IsExistingBranch: false,
		},
	})
	require.NoError(t, err)

	assert.True(t, instance.Started())
	assert.False(t, instance.Paused())
	assert.False(t, instance.TmuxAlive())
	assert.Contains(t, instance.Program, "codex")
	assert.NotContains(t, instance.Program, "/Library/pnpm/")
}

func TestResumeStoppedCodexInstance(t *testing.T) {
	if os.Getenv("RUN_CODEX_INTEGRATION") == "" {
		t.Skip("set RUN_CODEX_INTEGRATION=1 to run Codex integration test")
	}

	log.Initialize(false)
	defer log.Close()

	codexPath, err := config.GetProgramCommand("codex")
	require.NoError(t, err)

	workdir := t.TempDir()
	initCmd := exec.Command("git", "init")
	initCmd.Dir = workdir
	require.NoError(t, initCmd.Run())

	configEmail := exec.Command("git", "config", "--local", "user.email", "test@example.com")
	configEmail.Dir = workdir
	require.NoError(t, configEmail.Run())

	configName := exec.Command("git", "config", "--local", "user.name", "Test User")
	configName.Dir = workdir
	require.NoError(t, configName.Run())

	testFile := filepath.Join(workdir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	addCmd := exec.Command("git", "add", "test.txt")
	addCmd.Dir = workdir
	require.NoError(t, addCmd.Run())

	commitCmd := exec.Command("git", "commit", "-m", "initial commit")
	commitCmd.Dir = workdir
	require.NoError(t, commitCmd.Run())

	instance, err := NewInstance(InstanceOptions{
		Title:   fmt.Sprintf("codex-resume-%d", time.Now().UnixNano()),
		Path:    workdir,
		Program: codexPath + " -c check_for_update_on_startup=false",
	})
	require.NoError(t, err)
	defer func() { _ = instance.Kill() }()

	require.NoError(t, instance.Start(true))
	require.True(t, instance.TmuxAlive())

	require.NoError(t, instance.tmuxSession.Close())
	require.False(t, instance.TmuxAlive())

	require.NoError(t, instance.Resume())
	require.True(t, instance.TmuxAlive())
}
