package session

import (
	"claude-squad/transport"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	_ "unsafe"

	"github.com/stretchr/testify/require"
)

//go:linkname runBranchMetadataGenerator claude-squad/session/git.runBranchMetadataGenerator
var runBranchMetadataGenerator func(repoPath, repoName, title, prompt string) (string, error)

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
	require.Equal(t, "Duke Squad", gitLocalConfig(t, folderWorkspace.workspacePath, "user.name"))
	require.Equal(t, "duke-squad@local", gitLocalConfig(t, folderWorkspace.workspacePath, "user.email"))

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

func TestGetManagedWorkspaceDirectoryUsesDukeSquadConfigDir(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	workspaceDir, err := getManagedWorkspaceDirectory(ProjectTransportLocal)

	require.NoError(t, err)
	require.Equal(t, filepath.Join(homeDir, ".duke-squad", "managed-workspaces"), workspaceDir)
}

func TestNewGitWorkspaceUsesGeneratedMetadataForNewBranches(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	var gotRepoPath string
	var gotRepoName string
	var gotTitle string
	var gotPrompt string
	original := runBranchMetadataGenerator
	runBranchMetadataGenerator = func(repoPath, repoName, title, prompt string) (string, error) {
		gotRepoPath = repoPath
		gotRepoName = repoName
		gotTitle = title
		gotPrompt = prompt
		return `{"slug":"dev/generated-branch","description":"Draft branch metadata"}`, nil
	}
	t.Cleanup(func() {
		runBranchMetadataGenerator = original
	})

	workspace, branchName, branchDescription, err := createWorkspace(
		ProjectTransportLocal,
		"",
		"",
		"",
		ProjectKindGit,
		repoDir,
		"sess_123",
		"Implement branch metadata",
		"Use the prompt to generate a branch",
		"",
	)
	require.NoError(t, err)
	require.Equal(t, "dev/generated-branch", branchName)
	require.Equal(t, "Draft branch metadata", branchDescription)
	require.Equal(t, repoDir, gotRepoPath)
	require.Equal(t, filepath.Base(repoDir), gotRepoName)
	require.Equal(t, "Implement branch metadata", gotTitle)
	require.Equal(t, "Use the prompt to generate a branch", gotPrompt)

	require.NoError(t, workspace.Setup())
	t.Cleanup(func() {
		_ = workspace.Cleanup()
	})

	require.DirExists(t, workspace.Path())
	require.Equal(t, branchName, gitBranchAtPath(t, workspace.Path()))
	require.Contains(t, gitBranchesAtRepo(t, repoDir), branchName)
}

func TestNewGitWorkspaceSkipsMetadataForExistingBranch(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	repoDir := t.TempDir()
	originalBranch := initGitRepo(t, repoDir)
	selectedBranch := "release/1"
	runGit(t, repoDir, "checkout", "-b", selectedBranch)
	runGit(t, repoDir, "checkout", originalBranch)

	called := false
	original := runBranchMetadataGenerator
	runBranchMetadataGenerator = func(repoPath, repoName, title, prompt string) (string, error) {
		called = true
		return `{"slug":"dev/should-not-be-used","description":"Should not be generated"}`, nil
	}
	t.Cleanup(func() {
		runBranchMetadataGenerator = original
	})

	workspace, branchName, branchDescription, err := createWorkspace(
		ProjectTransportLocal,
		"",
		"",
		"",
		ProjectKindGit,
		repoDir,
		"sess_456",
		"Implement existing branch selection",
		"Prompt should be ignored",
		selectedBranch,
	)
	require.NoError(t, err)
	require.False(t, called)
	require.Equal(t, selectedBranch, branchName)
	require.Empty(t, branchDescription)

	require.NoError(t, workspace.Setup())
	t.Cleanup(func() {
		_ = workspace.Cleanup()
	})

	require.Equal(t, selectedBranch, gitBranchAtPath(t, workspace.Path()))
}

func initGitRepo(t *testing.T, repoDir string) string {
	t.Helper()

	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello"), 0644))
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial commit")

	return strings.TrimSpace(runGit(t, repoDir, "rev-parse", "--abbrev-ref", "HEAD"))
}

func runGit(t *testing.T, repoDir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	return string(output)
}

func gitBranchAtPath(t *testing.T, repoPath string) string {
	t.Helper()
	return strings.TrimSpace(runGit(t, repoPath, "rev-parse", "--abbrev-ref", "HEAD"))
}

func gitBranchesAtRepo(t *testing.T, repoPath string) string {
	t.Helper()
	return strings.TrimSpace(runGit(t, repoPath, "branch", "--list"))
}

func gitLocalConfig(t *testing.T, repoPath string, key string) string {
	t.Helper()

	cmd := exec.Command("git", "-C", repoPath, "config", "--local", "--get", key)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	return strings.TrimSpace(string(output))
}
