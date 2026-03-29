package session

import (
	"claude-squad/config"
	"claude-squad/session/git"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Workspace interface {
	Kind() ProjectKind
	Path() string
	Setup() error
	Cleanup() error
	Remove() error
	Prune() error
	Diff() *git.DiffStats
	CommitChanges(commitMessage string) error
	PushChanges(commitMessage string, open bool) error
	IsDirty() (bool, error)
	IsBranchCheckedOut() (bool, error)
	BranchName() string
	BaseCommitSHA() string
	SupportsPush() bool
	SupportsBranchSelection() bool
	SupportsCheckout() bool
	ToData() WorkspaceData
}

type WorkspaceData struct {
	Type             ProjectKind `json:"type"`
	RootPath         string      `json:"root_path"`
	WorkspacePath    string      `json:"workspace_path"`
	BranchName       string      `json:"branch_name,omitempty"`
	BaseCommitSHA    string      `json:"base_commit_sha,omitempty"`
	IsExistingBranch bool        `json:"is_existing_branch,omitempty"`
}

type gitWorkspace struct {
	worktree *git.GitWorktree
}

func (g *gitWorkspace) Kind() ProjectKind {
	return ProjectKindGit
}

func (g *gitWorkspace) Path() string {
	return g.worktree.GetWorktreePath()
}

func (g *gitWorkspace) Setup() error {
	return g.worktree.Setup()
}

func (g *gitWorkspace) Cleanup() error {
	return g.worktree.Cleanup()
}

func (g *gitWorkspace) Remove() error {
	return g.worktree.Remove()
}

func (g *gitWorkspace) Prune() error {
	return g.worktree.Prune()
}

func (g *gitWorkspace) Diff() *git.DiffStats {
	return g.worktree.Diff()
}

func (g *gitWorkspace) CommitChanges(commitMessage string) error {
	return g.worktree.CommitChanges(commitMessage)
}

func (g *gitWorkspace) PushChanges(commitMessage string, open bool) error {
	return g.worktree.PushChanges(commitMessage, open)
}

func (g *gitWorkspace) IsDirty() (bool, error) {
	return g.worktree.IsDirty()
}

func (g *gitWorkspace) IsBranchCheckedOut() (bool, error) {
	return g.worktree.IsBranchCheckedOut()
}

func (g *gitWorkspace) BranchName() string {
	return g.worktree.GetBranchName()
}

func (g *gitWorkspace) BaseCommitSHA() string {
	return g.worktree.GetBaseCommitSHA()
}

func (g *gitWorkspace) SupportsPush() bool {
	return true
}

func (g *gitWorkspace) SupportsBranchSelection() bool {
	return true
}

func (g *gitWorkspace) SupportsCheckout() bool {
	return true
}

func (g *gitWorkspace) ToData() WorkspaceData {
	return WorkspaceData{
		Type:             ProjectKindGit,
		RootPath:         g.worktree.GetRepoPath(),
		WorkspacePath:    g.worktree.GetWorktreePath(),
		BranchName:       g.worktree.GetBranchName(),
		BaseCommitSHA:    g.worktree.GetBaseCommitSHA(),
		IsExistingBranch: g.worktree.IsExistingBranch(),
	}
}

func newGitWorkspace(rootPath string, sessionID string, title string, selectedBranch string) (Workspace, string, error) {
	handleName := sessionHandleName(sessionID, title)
	if selectedBranch != "" {
		worktree, err := git.NewGitWorktreeFromBranch(rootPath, selectedBranch, handleName)
		if err != nil {
			return nil, "", err
		}
		return &gitWorkspace{worktree: worktree}, selectedBranch, nil
	}

	worktree, branchName, err := git.NewGitWorktree(rootPath, handleName)
	if err != nil {
		return nil, "", err
	}
	return &gitWorkspace{worktree: worktree}, branchName, nil
}

func workspaceFromData(data WorkspaceData, legacy GitWorktreeData) (Workspace, string, error) {
	switch {
	case data.Type == ProjectKindGit:
		worktree := git.NewGitWorktreeFromStorage(
			data.RootPath,
			data.WorkspacePath,
			filepath.Base(data.WorkspacePath),
			data.BranchName,
			data.BaseCommitSHA,
			data.IsExistingBranch,
		)
		return &gitWorkspace{worktree: worktree}, data.BranchName, nil
	case data.Type == ProjectKindFolder:
		workspace := &folderWorkspace{
			rootPath:      data.RootPath,
			workspacePath: data.WorkspacePath,
			baseCommitSHA: data.BaseCommitSHA,
		}
		return workspace, "", nil
	case legacy.RepoPath != "":
		worktree := git.NewGitWorktreeFromStorage(
			legacy.RepoPath,
			legacy.WorktreePath,
			legacy.SessionName,
			legacy.BranchName,
			legacy.BaseCommitSHA,
			legacy.IsExistingBranch,
		)
		return &gitWorkspace{worktree: worktree}, legacy.BranchName, nil
	default:
		return nil, "", nil
	}
}

type folderWorkspace struct {
	rootPath      string
	workspacePath string
	baseCommitSHA string
}

func (f *folderWorkspace) Kind() ProjectKind {
	return ProjectKindFolder
}

func (f *folderWorkspace) Path() string {
	return f.workspacePath
}

func (f *folderWorkspace) Setup() error {
	if f.workspacePath == "" {
		return fmt.Errorf("folder workspace path is not set")
	}

	if _, err := os.Stat(filepath.Join(f.workspacePath, ".git")); err == nil {
		if f.baseCommitSHA == "" {
			sha, err := f.headCommit()
			if err != nil {
				return err
			}
			f.baseCommitSHA = sha
		}
		return nil
	}

	if err := os.RemoveAll(f.workspacePath); err != nil {
		return fmt.Errorf("failed to reset managed workspace: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(f.workspacePath), 0755); err != nil {
		return fmt.Errorf("failed to create managed workspace parent: %w", err)
	}
	if err := copyTree(f.rootPath, f.workspacePath); err != nil {
		return err
	}

	if err := f.gitRun("init"); err != nil {
		return err
	}
	if err := f.gitRun("config", "--local", "user.email", "claude-squad@local"); err != nil {
		return err
	}
	if err := f.gitRun("config", "--local", "user.name", "Claude Squad"); err != nil {
		return err
	}
	if err := f.gitRun("add", "."); err != nil {
		return err
	}
	if err := f.gitRun("commit", "--allow-empty", "-m", "claude-squad baseline"); err != nil {
		return err
	}

	sha, err := f.headCommit()
	if err != nil {
		return err
	}
	f.baseCommitSHA = sha
	return nil
}

func (f *folderWorkspace) Cleanup() error {
	return os.RemoveAll(f.workspacePath)
}

func (f *folderWorkspace) Remove() error {
	return nil
}

func (f *folderWorkspace) Prune() error {
	return nil
}

func (f *folderWorkspace) Diff() *git.DiffStats {
	stats := &git.DiffStats{}
	if f.baseCommitSHA == "" {
		stats.Error = fmt.Errorf("base commit SHA not set")
		return stats
	}

	if _, err := f.gitOutput("add", "-N", "."); err != nil {
		stats.Error = err
		return stats
	}

	content, err := f.gitOutput("--no-pager", "diff", f.baseCommitSHA)
	if err != nil {
		stats.Error = err
		return stats
	}

	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			stats.Added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			stats.Removed++
		}
	}
	stats.Content = content
	return stats
}

func (f *folderWorkspace) CommitChanges(commitMessage string) error {
	dirty, err := f.IsDirty()
	if err != nil {
		return err
	}
	if !dirty {
		return nil
	}
	if err := f.gitRun("add", "."); err != nil {
		return err
	}
	return f.gitRun("commit", "--allow-empty", "-m", commitMessage, "--no-verify")
}

func (f *folderWorkspace) PushChanges(commitMessage string, open bool) error {
	return fmt.Errorf("push is only available for git-backed projects")
}

func (f *folderWorkspace) IsDirty() (bool, error) {
	output, err := f.gitOutput("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) != "", nil
}

func (f *folderWorkspace) IsBranchCheckedOut() (bool, error) {
	return false, nil
}

func (f *folderWorkspace) BranchName() string {
	return ""
}

func (f *folderWorkspace) BaseCommitSHA() string {
	return f.baseCommitSHA
}

func (f *folderWorkspace) SupportsPush() bool {
	return false
}

func (f *folderWorkspace) SupportsBranchSelection() bool {
	return false
}

func (f *folderWorkspace) SupportsCheckout() bool {
	return false
}

func (f *folderWorkspace) ToData() WorkspaceData {
	return WorkspaceData{
		Type:          ProjectKindFolder,
		RootPath:      f.rootPath,
		WorkspacePath: f.workspacePath,
		BaseCommitSHA: f.baseCommitSHA,
	}
}

func newFolderWorkspace(rootPath string, sessionID string, title string) (Workspace, string, error) {
	workspaceRoot, err := getManagedWorkspaceDirectory()
	if err != nil {
		return nil, "", err
	}
	workspacePath := filepath.Join(workspaceRoot, sessionHandleName(sessionID, title))
	return &folderWorkspace{
		rootPath:      rootPath,
		workspacePath: workspacePath,
	}, "", nil
}

func getManagedWorkspaceDirectory() (string, error) {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "managed-workspaces"), nil
}

func cleanupManagedWorkspaces() error {
	workspaceRoot, err := getManagedWorkspaceDirectory()
	if err != nil {
		return err
	}
	if err := os.RemoveAll(workspaceRoot); err != nil {
		return err
	}
	return os.MkdirAll(workspaceRoot, 0755)
}

func CleanupManagedWorkspaces() error {
	return cleanupManagedWorkspaces()
}

func createWorkspace(kind ProjectKind, rootPath string, sessionID string, title string, selectedBranch string) (Workspace, string, error) {
	switch kind {
	case ProjectKindGit:
		return newGitWorkspace(rootPath, sessionID, title, selectedBranch)
	case ProjectKindFolder:
		return newFolderWorkspace(rootPath, sessionID, title)
	default:
		return nil, "", fmt.Errorf("unsupported project kind %q", kind)
	}
}

func (f *folderWorkspace) gitRun(args ...string) error {
	_, err := f.gitOutput(args...)
	return err
}

func (f *folderWorkspace) gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", f.workspacePath}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git command failed: %s (%w)", output, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (f *folderWorkspace) headCommit() (string, error) {
	return f.gitOutput("rev-parse", "HEAD")
}

func copyTree(src string, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source project: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("project path must be a directory")
	}
	return filepath.Walk(src, func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)

		switch mode := info.Mode(); {
		case mode.IsDir():
			return os.MkdirAll(targetPath, info.Mode().Perm())
		case mode&os.ModeSymlink != 0:
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(linkTarget, targetPath)
		default:
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(targetPath, data, info.Mode().Perm())
		}
	})
}

func workspaceDataJSON(workspace Workspace) json.RawMessage {
	if workspace == nil {
		return nil
	}
	data, _ := json.Marshal(workspace.ToData())
	return data
}
