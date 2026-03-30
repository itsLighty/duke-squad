package git

import (
	"claude-squad/transport"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// sanitizeBranchName transforms an arbitrary string into a Git branch name friendly string.
// Note: Git branch names have several rules, so this function uses a simple approach
// by allowing only a safe subset of characters.
func sanitizeBranchName(s string) string {
	// Convert to lower-case
	s = strings.ToLower(s)

	// Replace spaces with a dash
	s = strings.ReplaceAll(s, " ", "-")

	// Remove any characters not allowed in our safe subset.
	// Here we allow: letters, digits, dash, underscore, slash, and dot.
	re := regexp.MustCompile(`[^a-z0-9\-_/.]+`)
	s = re.ReplaceAllString(s, "")

	// Replace multiple dashes with a single dash (optional cleanup)
	reDash := regexp.MustCompile(`-+`)
	s = reDash.ReplaceAllString(s, "-")

	// Trim leading and trailing dashes or slashes to avoid issues
	s = strings.Trim(s, "-/")

	return s
}

// checkGHCLI checks if GitHub CLI is installed and configured
func checkGHCLI() error {
	// Check if gh is installed
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("GitHub CLI (gh) is not installed. Please install it first")
	}

	// Check if gh is authenticated
	cmd := exec.Command("gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("GitHub CLI is not configured. Please run 'gh auth login' first")
	}

	return nil
}

// IsGitRepo checks if the given path is within a git repository
func IsGitRepo(path string) bool {
	return IsGitRepoWithRunner(transport.NewLocalRunner(), path)
}

func findGitRepoRoot(path string) (string, error) {
	return findGitRepoRootWithRunner(transport.NewLocalRunner(), path)
}

func IsGitRepoWithRunner(runner transport.Runner, path string) bool {
	return runner.Run(transport.CommandSpec{
		Program: "git",
		Args:    []string{"-C", path, "rev-parse", "--show-toplevel"},
	}) == nil
}

func findGitRepoRootWithRunner(runner transport.Runner, path string) (string, error) {
	out, err := runner.Output(transport.CommandSpec{
		Program: "git",
		Args:    []string{"-C", path, "rev-parse", "--show-toplevel"},
	})
	if err != nil {
		return "", fmt.Errorf("failed to find Git repository root from path: %s", path)
	}
	return strings.TrimSpace(string(out)), nil
}

func FindRepoRoot(path string) (string, error) {
	return findGitRepoRoot(path)
}

func FindRepoRootWithRunner(runner transport.Runner, path string) (string, error) {
	return findGitRepoRootWithRunner(runner, path)
}
