package git

import (
	"claude-squad/log"
	"claude-squad/transport"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// MaxBranchSearchResults is the maximum number of branches returned by SearchBranches.
const MaxBranchSearchResults = 50

// FetchBranches fetches and prunes remote-tracking branches (best-effort, won't fail if offline).
func FetchBranches(repoPath string) {
	FetchBranchesWithRunner(transport.NewLocalRunner(), repoPath)
}

func FetchBranchesWithRunner(runner transport.Runner, repoPath string) {
	_ = runner.Run(transport.CommandSpec{
		Program: "git",
		Args:    []string{"-C", repoPath, "fetch", "--prune"},
	})
}

// SearchBranches searches for branches whose name contains filter (case-insensitive),
// ordered by most recently updated first. Returns at most MaxBranchSearchResults.
// If filter is empty, returns all branches up to the limit.
func SearchBranches(repoPath, filter string) ([]string, error) {
	return SearchBranchesWithRunner(transport.NewLocalRunner(), repoPath, filter)
}

func SearchBranchesWithRunner(runner transport.Runner, repoPath, filter string) ([]string, error) {
	output, err := runner.CombinedOutput(transport.CommandSpec{
		Program: "git",
		Args: []string{"-C", repoPath, "branch", "-a",
			"--sort=-committerdate",
			"--format=%(refname:short)"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %s (%w)", output, err)
	}

	seen := make(map[string]bool)
	var branches []string
	lower := strings.ToLower(filter)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "HEAD") {
			continue
		}
		name := strings.TrimPrefix(line, "origin/")
		if seen[name] {
			continue
		}
		seen[name] = true
		if filter != "" && !strings.Contains(strings.ToLower(name), lower) {
			continue
		}
		branches = append(branches, name)
		if len(branches) >= MaxBranchSearchResults {
			break
		}
	}
	return branches, nil
}

// runGitCommand executes a git command and returns any error
func (g *GitWorktree) runGitCommand(path string, args ...string) (string, error) {
	baseArgs := []string{"-C", path}
	output, err := g.runner.CombinedOutput(transport.CommandSpec{
		Program: "git",
		Args:    append(baseArgs, args...),
	})
	if err != nil {
		return "", fmt.Errorf("git command failed: %s (%w)", output, err)
	}

	return string(output), nil
}

// PushChanges commits and pushes changes in the worktree to the remote branch
func (g *GitWorktree) PushChanges(commitMessage string, open bool) error {
	// Check if there are any changes to commit
	isDirty, err := g.IsDirty()
	if err != nil {
		return fmt.Errorf("failed to check for changes: %w", err)
	}

	if isDirty {
		// Stage all changes
		if _, err := g.runGitCommand(g.worktreePath, "add", "."); err != nil {
			log.ErrorLog.Print(err)
			return fmt.Errorf("failed to stage changes: %w", err)
		}

		// Create commit
		if _, err := g.runGitCommand(g.worktreePath, "commit", "-m", commitMessage, "--no-verify"); err != nil {
			log.ErrorLog.Print(err)
			return fmt.Errorf("failed to commit changes: %w", err)
		}
	}

	if g.runner.Kind() == transport.KindLocal {
		if err := checkGHCLI(); err != nil {
			return err
		}
		// First push the branch to remote to ensure it exists
		pushCmd := exec.Command("gh", "repo", "sync", "--source", "-b", g.branchName)
		pushCmd.Dir = g.worktreePath
		if err := pushCmd.Run(); err != nil {
			// If sync fails, try creating the branch on remote first
			gitPushCmd := exec.Command("git", "push", "-u", "origin", g.branchName)
			gitPushCmd.Dir = g.worktreePath
			if pushOutput, pushErr := gitPushCmd.CombinedOutput(); pushErr != nil {
				log.ErrorLog.Print(pushErr)
				return fmt.Errorf("failed to push branch: %s (%w)", pushOutput, pushErr)
			}
		}

		// Now sync with remote
		syncCmd := exec.Command("gh", "repo", "sync", "-b", g.branchName)
		syncCmd.Dir = g.worktreePath
		if output, err := syncCmd.CombinedOutput(); err != nil {
			log.ErrorLog.Print(err)
			return fmt.Errorf("failed to sync changes: %s (%w)", output, err)
		}
	} else {
		if _, err := g.runGitCommand(g.worktreePath, "push", "-u", "origin", g.branchName); err != nil {
			log.ErrorLog.Print(err)
			return fmt.Errorf("failed to push branch: %w", err)
		}
	}

	// Open the branch in the browser
	if open {
		if err := g.OpenBranchURL(); err != nil {
			// Just log the error but don't fail the push operation
			log.ErrorLog.Printf("failed to open branch URL: %v", err)
		}
	}

	return nil
}

// CommitChanges commits changes locally without pushing to remote
func (g *GitWorktree) CommitChanges(commitMessage string) error {
	// Check if there are any changes to commit
	isDirty, err := g.IsDirty()
	if err != nil {
		return fmt.Errorf("failed to check for changes: %w", err)
	}

	if isDirty {
		// Stage all changes
		if _, err := g.runGitCommand(g.worktreePath, "add", "."); err != nil {
			log.ErrorLog.Print(err)
			return fmt.Errorf("failed to stage changes: %w", err)
		}

		// Create commit (local only)
		if _, err := g.runGitCommand(g.worktreePath, "commit", "-m", commitMessage, "--no-verify"); err != nil {
			log.ErrorLog.Print(err)
			return fmt.Errorf("failed to commit changes: %w", err)
		}
	}

	return nil
}

// IsDirty checks if the worktree has uncommitted changes
func (g *GitWorktree) IsDirty() (bool, error) {
	output, err := g.runGitCommand(g.worktreePath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("failed to check worktree status: %w", err)
	}
	return len(output) > 0, nil
}

// IsBranchCheckedOut checks if the instance branch is currently checked out
func (g *GitWorktree) IsBranchCheckedOut() (bool, error) {
	output, err := g.runGitCommand(g.repoPath, "branch", "--show-current")
	if err != nil {
		return false, fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(output)) == g.branchName, nil
}

// OpenBranchURL opens the branch URL in the default browser
func (g *GitWorktree) OpenBranchURL() error {
	if g.runner.Kind() == transport.KindLocal {
		// Check if GitHub CLI is available
		if err := checkGHCLI(); err != nil {
			return err
		}

		cmd := exec.Command("gh", "browse", "--branch", g.branchName)
		cmd.Dir = g.worktreePath
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to open branch URL: %w", err)
		}
		return nil
	}

	url, err := g.branchBrowseURL()
	if err != nil {
		return err
	}
	cmd := browserOpenCommand(url)
	if cmd == nil {
		return fmt.Errorf("no browser opener available")
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open branch URL: %w", err)
	}
	return nil
}

func (g *GitWorktree) branchBrowseURL() (string, error) {
	origin, err := g.runGitCommand(g.worktreePath, "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("failed to determine remote origin: %w", err)
	}

	origin = strings.TrimSpace(origin)
	origin = strings.TrimSuffix(origin, ".git")
	host := ""
	repoPath := ""

	switch {
	case strings.HasPrefix(origin, "git@"):
		withoutUser := strings.TrimPrefix(origin, "git@")
		parts := strings.SplitN(withoutUser, ":", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("unsupported remote origin: %s", origin)
		}
		host = parts[0]
		repoPath = parts[1]
	case strings.HasPrefix(origin, "ssh://"):
		trimmed := strings.TrimPrefix(origin, "ssh://")
		parts := strings.SplitN(trimmed, "/", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("unsupported remote origin: %s", origin)
		}
		host = strings.TrimPrefix(parts[0], "git@")
		repoPath = parts[1]
	case strings.HasPrefix(origin, "https://"), strings.HasPrefix(origin, "http://"):
		trimmed := strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://")
		parts := strings.SplitN(trimmed, "/", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("unsupported remote origin: %s", origin)
		}
		host = parts[0]
		repoPath = parts[1]
	default:
		return "", fmt.Errorf("unsupported remote origin: %s", origin)
	}

	return fmt.Sprintf("https://%s/%s/tree/%s", host, strings.TrimPrefix(repoPath, "/"), g.branchName), nil
}

func browserOpenCommand(url string) *exec.Cmd {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url)
	case "linux":
		return exec.Command("xdg-open", url)
	default:
		return nil
	}
}
