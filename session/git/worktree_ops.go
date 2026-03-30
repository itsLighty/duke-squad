package git

import (
	"claude-squad/log"
	"claude-squad/transport"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Setup creates a new worktree for the session
func (g *GitWorktree) Setup() error {
	// Ensure worktrees directory exists early (can be done in parallel with branch check)
	worktreesDir, err := getWorktreeDirectoryForRunner(g.runner)
	if err != nil {
		return fmt.Errorf("failed to get worktree directory: %w", err)
	}

	if err := g.ensureDirectory(worktreesDir); err != nil {
		return err
	}

	// If this worktree uses a pre-existing branch, always set up from that branch
	// (it may exist locally or only on the remote).
	if g.isExistingBranch {
		return g.setupFromExistingBranch()
	}

	// Check if branch exists using git CLI (much faster than go-git PlainOpen)
	_, err = g.runGitCommand(g.repoPath, "show-ref", "--verify", fmt.Sprintf("refs/heads/%s", g.branchName))
	if err == nil {
		return g.setupFromExistingBranch()
	}
	return g.setupNewWorktree()
}

// setupFromExistingBranch creates a worktree from an existing branch
func (g *GitWorktree) setupFromExistingBranch() error {
	// Directory already created in Setup(), skip duplicate creation

	// Clean up any existing worktree first
	_, _ = g.runGitCommand(g.repoPath, "worktree", "remove", "-f", g.worktreePath) // Ignore error if worktree doesn't exist

	// Check if the local branch exists
	_, localErr := g.runGitCommand(g.repoPath, "show-ref", "--verify", fmt.Sprintf("refs/heads/%s", g.branchName))
	if localErr != nil {
		// Local branch doesn't exist — check if remote tracking branch exists
		_, remoteErr := g.runGitCommand(g.repoPath, "show-ref", "--verify", fmt.Sprintf("refs/remotes/origin/%s", g.branchName))
		if remoteErr != nil {
			return fmt.Errorf("branch %s not found locally or on remote", g.branchName)
		}
		// Create a local tracking branch via worktree add -b
		if _, err := g.runGitCommand(g.repoPath, "worktree", "add", "-b", g.branchName, g.worktreePath, fmt.Sprintf("origin/%s", g.branchName)); err != nil {
			return fmt.Errorf("failed to create worktree from remote branch %s: %w", g.branchName, err)
		}
		return nil
	}

	// Create a new worktree from the existing local branch
	if _, err := g.runGitCommand(g.repoPath, "worktree", "add", g.worktreePath, g.branchName); err != nil {
		return fmt.Errorf("failed to create worktree from branch %s: %w", g.branchName, err)
	}

	return nil
}

// setupNewWorktree creates a new worktree from HEAD
func (g *GitWorktree) setupNewWorktree() error {
	// Clean up any existing worktree first
	_, _ = g.runGitCommand(g.repoPath, "worktree", "remove", "-f", g.worktreePath) // Ignore error if worktree doesn't exist

	// Clean up any existing branch using git CLI (much faster than go-git PlainOpen)
	_, _ = g.runGitCommand(g.repoPath, "branch", "-D", g.branchName) // Ignore error if branch doesn't exist

	output, err := g.runGitCommand(g.repoPath, "rev-parse", "HEAD")
	if err != nil {
		if strings.Contains(err.Error(), "fatal: ambiguous argument 'HEAD'") ||
			strings.Contains(err.Error(), "fatal: not a valid object name") ||
			strings.Contains(err.Error(), "fatal: HEAD: not a valid object name") {
			return fmt.Errorf("this appears to be a brand new repository: please create an initial commit before creating an instance")
		}
		return fmt.Errorf("failed to get HEAD commit hash: %w", err)
	}
	headCommit := strings.TrimSpace(string(output))
	g.baseCommitSHA = headCommit

	// Create a new worktree from the HEAD commit
	// Otherwise, we'll inherit uncommitted changes from the previous worktree.
	// This way, we can start the worktree with a clean slate.
	// TODO: we might want to give an option to use main/master instead of the current branch.
	if _, err := g.runGitCommand(g.repoPath, "worktree", "add", "-b", g.branchName, g.worktreePath, headCommit); err != nil {
		return fmt.Errorf("failed to create worktree from commit %s: %w", headCommit, err)
	}

	return nil
}

// Cleanup removes the worktree and associated branch
func (g *GitWorktree) Cleanup() error {
	var errs []error

	// Check if worktree path exists before attempting removal
	if exists, err := g.Exists(); err == nil && exists {
		// Remove the worktree using git command
		if _, err := g.runGitCommand(g.repoPath, "worktree", "remove", "-f", g.worktreePath); err != nil {
			errs = append(errs, err)
		}
	} else if err != nil {
		// Only append error if it's not a "not exists" error
		errs = append(errs, fmt.Errorf("failed to check worktree path: %w", err))
	}

	// Delete the branch using git CLI, but skip if this is a pre-existing branch
	if !g.isExistingBranch {
		if _, err := g.runGitCommand(g.repoPath, "branch", "-D", g.branchName); err != nil {
			// Only log if it's not a "branch not found" error
			if !strings.Contains(err.Error(), "not found") {
				errs = append(errs, fmt.Errorf("failed to remove branch %s: %w", g.branchName, err))
			}
		}
	}

	// Prune the worktree to clean up any remaining references
	if err := g.Prune(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return g.combineErrors(errs)
	}

	return nil
}

// Remove removes the worktree but keeps the branch
func (g *GitWorktree) Remove() error {
	// Remove the worktree using git command
	if _, err := g.runGitCommand(g.repoPath, "worktree", "remove", "-f", g.worktreePath); err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	return nil
}

// Prune removes all working tree administrative files and directories
func (g *GitWorktree) Prune() error {
	if _, err := g.runGitCommand(g.repoPath, "worktree", "prune"); err != nil {
		return fmt.Errorf("failed to prune worktrees: %w", err)
	}
	return nil
}

func (g *GitWorktree) Exists() (bool, error) {
	if g.runner == nil || g.runner.Kind() == transport.KindLocal {
		if _, err := os.Stat(g.worktreePath); err == nil {
			return true, nil
		} else if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, err
		}
	}

	output, err := g.runner.CombinedOutput(transport.CommandSpec{
		Program: "sh",
		Args:    []string{"-lc", fmt.Sprintf("[ -d %s ] && printf yes", shellQuote(g.worktreePath))},
	})
	if err != nil {
		return false, fmt.Errorf("failed to verify remote worktree %s: %w", g.worktreePath, err)
	}
	return strings.TrimSpace(string(output)) == "yes", nil
}

func (g *GitWorktree) ensureDirectory(dir string) error {
	if g.runner == nil || g.runner.Kind() == transport.KindLocal {
		return os.MkdirAll(dir, 0755)
	}
	return g.runner.Run(transport.CommandSpec{
		Program: "mkdir",
		Args:    []string{"-p", dir},
	})
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func runGitCommandAt(path string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", path}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git command failed: %s (%w)", output, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func repoPathForWorktree(worktreePath string) (string, error) {
	commonDir, err := runGitCommandAt(worktreePath, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if commonDir == "" {
		return "", fmt.Errorf("git common dir not found for worktree %s", worktreePath)
	}
	if filepath.Base(commonDir) == ".git" {
		return filepath.Dir(commonDir), nil
	}
	return filepath.Dir(commonDir), nil
}

func branchNameForWorktree(worktreePath string) string {
	branchName, err := runGitCommandAt(worktreePath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		if log.ErrorLog != nil {
			log.ErrorLog.Printf("failed to resolve branch for worktree %s: %v", worktreePath, err)
		}
		return ""
	}
	if branchName == "HEAD" {
		return ""
	}
	return branchName
}

func collectManagedWorktrees(root string) ([]string, error) {
	worktreePaths := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}
		if path == root {
			return nil
		}
		if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
			worktreePaths = append(worktreePaths, path)
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return worktreePaths, nil
}

// CleanupWorktrees removes all managed worktrees and best-effort associated branches.
func CleanupWorktrees() error {
	worktreesDir, err := getWorktreeDirectory()
	if err != nil {
		return fmt.Errorf("failed to get worktree directory: %w", err)
	}

	worktreePaths, err := collectManagedWorktrees(worktreesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to discover worktrees: %w", err)
	}

	reposToPrune := make(map[string]struct{})
	var errs []error

	for _, worktreePath := range worktreePaths {
		branchName := branchNameForWorktree(worktreePath)
		repoPath, repoErr := repoPathForWorktree(worktreePath)
		if repoErr != nil {
			if log.ErrorLog != nil {
				log.ErrorLog.Printf("failed to resolve repo for worktree %s: %v", worktreePath, repoErr)
			}
			if err := os.RemoveAll(worktreePath); err != nil {
				errs = append(errs, fmt.Errorf("failed to remove worktree directory %s: %w", worktreePath, err))
			}
			continue
		}
		reposToPrune[repoPath] = struct{}{}

		if _, err := runGitCommandAt(worktreePath, "worktree", "remove", "-f", worktreePath); err != nil {
			if log.ErrorLog != nil {
				log.ErrorLog.Printf("failed to remove git worktree %s: %v", worktreePath, err)
			}
			if removeErr := os.RemoveAll(worktreePath); removeErr != nil {
				errs = append(errs, fmt.Errorf("failed to remove worktree directory %s: %w", worktreePath, removeErr))
			}
		}

		if branchName == "" {
			continue
		}
		if _, err := runGitCommandAt(repoPath, "branch", "-D", branchName); err != nil {
			if log.ErrorLog != nil {
				log.ErrorLog.Printf("failed to delete branch %s for worktree %s: %v", branchName, worktreePath, err)
			}
		}
	}

	if err := os.RemoveAll(worktreesDir); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Errorf("failed to remove worktrees directory %s: %w", worktreesDir, err))
	}

	for repoPath := range reposToPrune {
		if _, err := runGitCommandAt(repoPath, "worktree", "prune"); err != nil {
			if log.ErrorLog != nil {
				log.ErrorLog.Printf("failed to prune worktrees for repo %s: %v", repoPath, err)
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}

	if len(errs) == 1 {
		return errs[0]
	}

	msg := "multiple worktree cleanup errors occurred:"
	for _, err := range errs {
		msg += "\n  - " + err.Error()
	}
	return fmt.Errorf("%s", msg)
}
