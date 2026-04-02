package config

import (
	"claude-squad/log"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const (
	ConfigFileName = "config.json"
	defaultProgram = "claude"
)

// GetConfigDir returns the path to the application's configuration directory
func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config home directory: %w", err)
	}
	if err := ensureConfigDirMigrated(homeDir); err != nil {
		return "", err
	}
	return getConfigDirForHome(homeDir), nil
}

// Profile represents a named program configuration
type Profile struct {
	Name    string `json:"name"`
	Program string `json:"program"`
}

// Config represents the application configuration
type Config struct {
	// DefaultProgram is the default program to run in new instances
	DefaultProgram string `json:"default_program"`
	// AutoYes is a flag to automatically accept all prompts.
	AutoYes bool `json:"auto_yes"`
	// DaemonPollInterval is the interval (ms) at which the daemon polls sessions for autoyes mode.
	DaemonPollInterval int `json:"daemon_poll_interval"`
	// BranchPrefix is the prefix used for git branches created by the application.
	BranchPrefix string `json:"branch_prefix"`
	// Profiles is a list of named program profiles.
	Profiles []Profile `json:"profiles,omitempty"`
}

// GetProgram returns the program to run. If Profiles is non-empty and
// DefaultProgram matches a profile name, that profile's Program is returned.
// Otherwise DefaultProgram is returned as-is.
func (c *Config) GetProgram() string {
	for _, p := range c.Profiles {
		if p.Name == c.DefaultProgram {
			return p.Program
		}
	}
	return c.DefaultProgram
}

// GetProfiles returns a unified list of profiles. If Profiles is defined,
// those are returned with the default profile first. Otherwise, a single
// profile is synthesized from DefaultProgram.
func (c *Config) GetProfiles() []Profile {
	if len(c.Profiles) == 0 {
		return []Profile{{Name: c.DefaultProgram, Program: c.DefaultProgram}}
	}
	// Reorder so the default profile comes first.
	profiles := make([]Profile, 0, len(c.Profiles))
	for _, p := range c.Profiles {
		if p.Name == c.DefaultProgram {
			profiles = append(profiles, p)
			break
		}
	}
	for _, p := range c.Profiles {
		if p.Name != c.DefaultProgram {
			profiles = append(profiles, p)
		}
	}
	return profiles
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	program, err := GetClaudeCommand()
	if err != nil {
		log.ErrorLog.Printf("failed to get claude command: %v", err)
		program = defaultProgram
	}

	return &Config{
		DefaultProgram:     program,
		AutoYes:            true,
		DaemonPollInterval: 1000,
		BranchPrefix:       "dev/",
	}
}

// GetClaudeCommand attempts to find the "claude" command in the user's shell
// It checks in the following order:
// 1. Shell alias resolution: using "which" command
// 2. PATH lookup
//
// If both fail, it returns an error.
func GetClaudeCommand() (string, error) {
	return GetProgramCommand("claude")
}

func preferredProgramPath(name string) string {
	if runtime.GOOS != "darwin" {
		return ""
	}

	switch name {
	case "codex":
		for _, candidate := range []string{"/opt/homebrew/bin/codex", "/usr/local/bin/codex"} {
			info, err := os.Stat(candidate)
			if err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
				return candidate
			}
		}
	}

	return ""
}

// GetProgramCommand attempts to find the given command in the user's shell.
// It checks in the following order:
// 1. Shell resolution: using "command -v"
// 2. PATH lookup
func GetProgramCommand(name string) (string, error) {
	if preferred := preferredProgramPath(name); preferred != "" {
		return preferred, nil
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash" // Default to bash if SHELL is not set
	}

	// Force the shell to load the user's profile and then run the command
	// For zsh, source .zshrc; for bash, source .bashrc
	var shellCmd string
	if strings.Contains(shell, "zsh") {
		shellCmd = fmt.Sprintf("source ~/.zshrc &>/dev/null || true; command -v %s", name)
	} else if strings.Contains(shell, "bash") {
		shellCmd = fmt.Sprintf("source ~/.bashrc &>/dev/null || true; command -v %s", name)
	} else {
		shellCmd = fmt.Sprintf("command -v %s", name)
	}

	cmd := exec.Command(shell, "-c", shellCmd)
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		path := strings.TrimSpace(string(output))
		if path != "" {
			// Check if the output is an alias definition and extract the actual path
			// Handle formats like "claude: aliased to /path/to/claude" or other shell-specific formats
			aliasRegex := regexp.MustCompile(`(?:aliased to|->|=)\s*([^\s]+)`)
			matches := aliasRegex.FindStringSubmatch(path)
			if len(matches) > 1 {
				path = matches[1]
			}
			if preferred := preferredProgramPath(name); preferred != "" {
				return preferred, nil
			}
			return path, nil
		}
	}

	// Otherwise, try to find in PATH directly
	commandPath, err := exec.LookPath(name)
	if err == nil {
		if preferred := preferredProgramPath(name); preferred != "" {
			return preferred, nil
		}
		return commandPath, nil
	}

	return "", fmt.Errorf("%s command not found in aliases or PATH", name)
}

// NormalizeProgramCommand resolves known built-in provider commands to a stable executable path
// while preserving any existing arguments.
func NormalizeProgramCommand(program string) string {
	program = strings.TrimSpace(program)
	if program == "" {
		return program
	}

	fields := strings.Fields(program)
	if len(fields) == 0 {
		return program
	}

	executable := strings.Trim(fields[0], `"'`)
	name := strings.TrimSuffix(filepath.Base(executable), filepath.Ext(executable))
	name = strings.ToLower(name)

	if name != "claude" && name != "codex" {
		return program
	}

	// Preserve explicitly pinned absolute paths unless they are the pnpm shim that exits in tmux.
	shouldResolve := executable == name || strings.Contains(executable, "/Library/pnpm/")
	if !shouldResolve {
		return program
	}

	resolved, err := GetProgramCommand(name)
	if err != nil || resolved == "" {
		resolved = executable
	}

	fields[0] = resolved
	if name == "codex" {
		if !strings.Contains(program, "check_for_update_on_startup=") {
			fields = append(fields, "-c", "check_for_update_on_startup=false")
		}
		if !strings.Contains(program, "--no-alt-screen") {
			fields = append(fields, "--no-alt-screen")
		}
	}
	return strings.Join(fields, " ")
}

func LoadConfig() *Config {
	configDir, err := GetConfigDir()
	if err != nil {
		log.ErrorLog.Printf("failed to get config directory: %v", err)
		return DefaultConfig()
	}

	configPath := filepath.Join(configDir, ConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create and save default config if file doesn't exist
			defaultCfg := DefaultConfig()
			if saveErr := saveConfig(defaultCfg); saveErr != nil {
				log.WarningLog.Printf("failed to save default config: %v", saveErr)
			}
			return defaultCfg
		}

		log.WarningLog.Printf("failed to get config file: %v", err)
		return DefaultConfig()
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		log.ErrorLog.Printf("failed to parse config file: %v", err)
		return DefaultConfig()
	}

	return &config
}

// saveConfig saves the configuration to disk
func saveConfig(config *Config) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, ConfigFileName)
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(configPath, data, 0644)
}

// SaveConfig exports the saveConfig function for use by other packages
func SaveConfig(config *Config) error {
	return saveConfig(config)
}
