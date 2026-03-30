package git

import (
	"claude-squad/config"
	"claude-squad/log"
	"claude-squad/transport"
	"fmt"
	"path"
	"path/filepath"
	"time"
)

func getWorktreeDirectory() (string, error) {
	return getWorktreeDirectoryForRunner(transport.NewLocalRunner())
}

func getWorktreeDirectoryForRunner(runner transport.Runner) (string, error) {
	if runner.Kind() == transport.KindSSH {
		return "~/.claude-squad/worktrees", nil
	}
	configDir, err := config.GetConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, "worktrees"), nil
}

// GitWorktree manages git worktree operations for a session
type GitWorktree struct {
	runner transport.Runner
	// Path to the repository
	repoPath string
	// Path to the worktree
	worktreePath string
	// Name of the session
	sessionName string
	// Branch name for the worktree
	branchName string
	// Base commit hash for the worktree
	baseCommitSHA string
	// isExistingBranch is true if the branch existed before the session was created.
	// When true, the branch will not be deleted on cleanup.
	isExistingBranch bool
}

func NewGitWorktreeFromStorage(runner transport.Runner, repoPath string, worktreePath string, sessionName string, branchName string, baseCommitSHA string, isExistingBranch bool) *GitWorktree {
	return &GitWorktree{
		runner:           runner,
		repoPath:         repoPath,
		worktreePath:     worktreePath,
		sessionName:      sessionName,
		branchName:       branchName,
		baseCommitSHA:    baseCommitSHA,
		isExistingBranch: isExistingBranch,
	}
}

// resolveWorktreePaths resolves the repo root and generates a unique worktree path for the given branch name.
func resolveWorktreePaths(runner transport.Runner, repoPath string, branchName string) (resolvedRepo string, worktreePath string, err error) {
	resolvedPath := repoPath
	if runner.Kind() == transport.KindLocal {
		absPath, err := filepath.Abs(repoPath)
		if err != nil {
			log.ErrorLog.Printf("git worktree path abs error, falling back to repoPath %s: %s", repoPath, err)
		} else {
			resolvedPath = absPath
		}
	}

	resolvedRepo, err = findGitRepoRootWithRunner(runner, resolvedPath)
	if err != nil {
		return "", "", err
	}

	worktreeDir, err := getWorktreeDirectoryForRunner(runner)
	if err != nil {
		return "", "", err
	}

	if runner.Kind() == transport.KindSSH {
		worktreePath = path.Join(worktreeDir, sanitizeBranchName(branchName))
	} else {
		worktreePath = filepath.Join(worktreeDir, sanitizeBranchName(branchName))
	}
	worktreePath = worktreePath + "_" + fmt.Sprintf("%x", time.Now().UnixNano())

	return resolvedRepo, worktreePath, nil
}

// NewGitWorktree creates a new GitWorktree instance
func NewGitWorktree(repoPath string, sessionName string) (tree *GitWorktree, branchname string, err error) {
	return NewGitWorktreeWithRunner(transport.NewLocalRunner(), repoPath, sessionName)
}

func NewGitWorktreeWithRunner(runner transport.Runner, repoPath string, sessionName string) (tree *GitWorktree, branchname string, err error) {
	cfg := config.LoadConfig()
	branchName := fmt.Sprintf("%s%s", cfg.BranchPrefix, sessionName)
	// Sanitize the final branch name to handle invalid characters from any source
	// (e.g., backslashes from Windows domain usernames like DOMAIN\user)
	branchName = sanitizeBranchName(branchName)

	repoPath, worktreePath, err := resolveWorktreePaths(runner, repoPath, branchName)
	if err != nil {
		return nil, "", err
	}

	return &GitWorktree{
		runner:       runner,
		repoPath:     repoPath,
		sessionName:  sessionName,
		branchName:   branchName,
		worktreePath: worktreePath,
	}, branchName, nil
}

// NewGitWorktreeFromBranch creates a new GitWorktree that uses an existing branch.
// The branch will not be deleted on cleanup.
func NewGitWorktreeFromBranch(repoPath string, branchName string, sessionName string) (*GitWorktree, error) {
	return NewGitWorktreeFromBranchWithRunner(transport.NewLocalRunner(), repoPath, branchName, sessionName)
}

func NewGitWorktreeFromBranchWithRunner(runner transport.Runner, repoPath string, branchName string, sessionName string) (*GitWorktree, error) {
	repoPath, worktreePath, err := resolveWorktreePaths(runner, repoPath, branchName)
	if err != nil {
		return nil, err
	}

	return &GitWorktree{
		runner:           runner,
		repoPath:         repoPath,
		sessionName:      sessionName,
		branchName:       branchName,
		worktreePath:     worktreePath,
		isExistingBranch: true,
	}, nil
}

// IsExistingBranch returns whether this worktree uses a pre-existing branch
func (g *GitWorktree) IsExistingBranch() bool {
	return g.isExistingBranch
}

// GetWorktreePath returns the path to the worktree
func (g *GitWorktree) GetWorktreePath() string {
	return g.worktreePath
}

// GetBranchName returns the name of the branch associated with this worktree
func (g *GitWorktree) GetBranchName() string {
	return g.branchName
}

// GetRepoPath returns the path to the repository
func (g *GitWorktree) GetRepoPath() string {
	return g.repoPath
}

// GetRepoName returns the name of the repository (last part of the repoPath).
func (g *GitWorktree) GetRepoName() string {
	if g.runner != nil && g.runner.Kind() == transport.KindSSH {
		return path.Base(g.repoPath)
	}
	return filepath.Base(g.repoPath)
}

// GetBaseCommitSHA returns the base commit SHA for the worktree
func (g *GitWorktree) GetBaseCommitSHA() string {
	return g.baseCommitSHA
}

func (g *GitWorktree) RunnerKind() transport.Kind {
	if g.runner == nil {
		return transport.KindLocal
	}
	return g.runner.Kind()
}

func (g *GitWorktree) RunnerTarget() string {
	if g.runner == nil {
		return ""
	}
	return g.runner.Target()
}

func (g *GitWorktree) RunnerSSHUser() string {
	runner, ok := g.runner.(*transport.SSHRunner)
	if !ok {
		return ""
	}
	return runner.Config().Username
}

func (g *GitWorktree) RunnerSSHHost() string {
	runner, ok := g.runner.(*transport.SSHRunner)
	if !ok {
		return ""
	}
	return runner.Config().Host
}
