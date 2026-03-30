package transport

import (
	"claude-squad/config"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/creack/pty"
)

const sshKeychainServicePrefix = "duke-squad:ssh"
const sshAskPassScript = `#!/bin/sh
prompt="${1:-}"
case "$prompt" in
  *"continue connecting"*|*"yes/no"*)
    printf 'yes\n'
    exit 0
    ;;
esac
if [ -n "${DUKE_SQUAD_SSH_PASSWORD:-}" ]; then
  printf '%s\n' "$DUKE_SQUAD_SSH_PASSWORD"
  exit 0
fi
if [ -n "${DUKE_SQUAD_SSH_KEYCHAIN_SERVICE:-}" ] && [ -n "${DUKE_SQUAD_SSH_KEYCHAIN_ACCOUNT:-}" ] && command -v security >/dev/null 2>&1; then
  security find-generic-password -s "$DUKE_SQUAD_SSH_KEYCHAIN_SERVICE" -a "$DUKE_SQUAD_SSH_KEYCHAIN_ACCOUNT" -w
  exit $?
fi
exit 1
`

type SSHConfig struct {
	Username string
	Host     string
	Password string
}

func ParseSSHConfig(target string) SSHConfig {
	target = strings.TrimSpace(target)
	if target == "" {
		return SSHConfig{}
	}
	if user, host, found := strings.Cut(target, "@"); found {
		return SSHConfig{
			Username: strings.TrimSpace(user),
			Host:     strings.TrimSpace(host),
		}
	}
	return SSHConfig{Host: target}
}

func (c SSHConfig) Normalized() SSHConfig {
	c.Username = strings.TrimSpace(c.Username)
	c.Host = strings.TrimSpace(c.Host)
	return c
}

func (c SSHConfig) Target() string {
	c = c.Normalized()
	if c.Username == "" {
		return c.Host
	}
	return c.Username + "@" + c.Host
}

func (c SSHConfig) KeychainService() string {
	c = c.Normalized()
	if c.Host == "" {
		return ""
	}
	return sshKeychainServicePrefix + ":" + c.Host
}

func (c SSHConfig) KeychainAccount() string {
	return c.Normalized().Username
}

func (c SSHConfig) CanUsePasswordAuth() bool {
	return c.Password != "" || HasStoredSSHPassword(c)
}

type SSHRunner struct {
	config  SSHConfig
	manager *SSHManager
}

func NewSSHRunner(target string) *SSHRunner {
	return NewSSHRunnerWithConfig(ParseSSHConfig(target))
}

func NewSSHRunnerWithConfig(cfg SSHConfig) *SSHRunner {
	return &SSHRunner{
		config:  cfg.Normalized(),
		manager: DefaultSSHManager(),
	}
}

func (r *SSHRunner) Kind() Kind {
	return KindSSH
}

func (r *SSHRunner) Target() string {
	return r.config.Target()
}

func (r *SSHRunner) Config() SSHConfig {
	return r.config
}

func (r *SSHRunner) Run(spec CommandSpec) error {
	if err := r.manager.EnsureControlMaster(r.config); err != nil {
		return err
	}
	return r.command(spec, false).Run()
}

func (r *SSHRunner) Output(spec CommandSpec) ([]byte, error) {
	if err := r.manager.EnsureControlMaster(r.config); err != nil {
		return nil, err
	}
	return r.command(spec, false).Output()
}

func (r *SSHRunner) CombinedOutput(spec CommandSpec) ([]byte, error) {
	if err := r.manager.EnsureControlMaster(r.config); err != nil {
		return nil, err
	}
	return r.command(spec, false).CombinedOutput()
}

func (r *SSHRunner) StartPTY(spec CommandSpec) (*os.File, error) {
	if err := r.manager.EnsureControlMaster(r.config); err != nil {
		return nil, err
	}
	return pty.Start(r.command(spec, true))
}

func (r *SSHRunner) command(spec CommandSpec, forceTTY bool) *exec.Cmd {
	args := r.manager.remoteCommandArgs(r.config.Target(), forceTTY, shellCommand(spec))
	cmd := exec.Command("ssh", args...)
	cmd.Env = append(os.Environ(), r.manager.authEnv(r.config)...)
	return cmd
}

type SSHManager struct {
	socketDir string
	askpass   string
	mu        sync.Mutex
}

var (
	defaultSSHManager     *SSHManager
	defaultSSHManagerOnce sync.Once
)

func DefaultSSHManager() *SSHManager {
	defaultSSHManagerOnce.Do(func() {
		configDir, err := config.GetConfigDir()
		if err != nil {
			configDir = filepath.Join(os.TempDir(), config.BinaryName)
		}
		defaultSSHManager = &SSHManager{
			socketDir: filepath.Join(configDir, "ssh"),
			askpass:   filepath.Join(configDir, "ssh", "askpass.sh"),
		}
	})
	return defaultSSHManager
}

func (m *SSHManager) EnsureControlMaster(cfg SSHConfig) error {
	cfg = cfg.Normalized()
	target := cfg.Target()
	if target == "" {
		return fmt.Errorf("ssh target is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.socketDir, 0o700); err != nil {
		return fmt.Errorf("failed to create ssh socket directory: %w", err)
	}

	if m.controlMasterAlive(target) {
		return nil
	}

	cmd := exec.Command("ssh", m.startMasterArgs(target)...)
	cmd.Env = append(os.Environ(), m.authEnv(cfg)...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) == 0 {
			return fmt.Errorf("ssh control master failed: %w", err)
		}
		return fmt.Errorf("ssh control master failed: %s (%w)", strings.TrimSpace(string(output)), err)
	}
	if !m.controlMasterAlive(target) {
		return fmt.Errorf("ssh control master did not become ready for %s", target)
	}
	return nil
}

func (m *SSHManager) remoteCommandArgs(target string, forceTTY bool, script string) []string {
	args := []string{
		"-o", "ControlMaster=auto",
		"-o", "ControlPersist=10m",
		"-S", m.socketPath(target),
	}
	if forceTTY {
		args = append(args, "-tt")
	}
	args = append(args, target, "sh", "-lc", script)
	return args
}

func (m *SSHManager) startMasterArgs(target string) []string {
	return []string{
		"-f",
		"-N",
		"-M",
		"-o", "ControlPersist=10m",
		"-S", m.socketPath(target),
		target,
	}
}

func (m *SSHManager) controlMasterAlive(target string) bool {
	cmd := exec.Command("ssh", "-S", m.socketPath(target), "-O", "check", target)
	return cmd.Run() == nil
}

func (m *SSHManager) socketPath(target string) string {
	sum := sha256.Sum256([]byte(target))
	return filepath.Join(m.socketDir, fmt.Sprintf("%x.sock", sum[:8]))
}

func (m *SSHManager) authEnv(cfg SSHConfig) []string {
	cfg = cfg.Normalized()
	if cfg.Target() == "" {
		return nil
	}
	if err := m.ensureAskPassScript(); err != nil {
		return nil
	}

	env := []string{
		"DISPLAY=duke-squad:0",
		"SSH_ASKPASS=" + m.askpass,
		"SSH_ASKPASS_REQUIRE=force",
		"DUKE_SQUAD_SSH_KEYCHAIN_SERVICE=" + cfg.KeychainService(),
		"DUKE_SQUAD_SSH_KEYCHAIN_ACCOUNT=" + cfg.KeychainAccount(),
	}
	if password := cfg.Password; password != "" {
		env = append(env, "DUKE_SQUAD_SSH_PASSWORD="+password)
	}
	return env
}

func (m *SSHManager) ensureAskPassScript() error {
	if err := os.MkdirAll(filepath.Dir(m.askpass), 0o700); err != nil {
		return err
	}
	current, err := os.ReadFile(m.askpass)
	if err == nil && string(current) == sshAskPassScript {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(m.askpass, []byte(sshAskPassScript), 0o700)
}

func StoreSSHPassword(cfg SSHConfig) error {
	cfg = cfg.Normalized()
	password := cfg.Password
	if password == "" {
		return nil
	}

	securityPath, err := exec.LookPath("security")
	if err != nil {
		return fmt.Errorf("system keychain is unavailable")
	}

	cmd := exec.Command(securityPath,
		"add-generic-password",
		"-U",
		"-s", cfg.KeychainService(),
		"-a", cfg.KeychainAccount(),
		"-w", password,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to store ssh password: %s (%w)", strings.TrimSpace(string(output)), err)
	}
	return nil
}

func HasStoredSSHPassword(cfg SSHConfig) bool {
	cfg = cfg.Normalized()
	if cfg.KeychainService() == "" || cfg.KeychainAccount() == "" {
		return false
	}
	securityPath, err := exec.LookPath("security")
	if err != nil {
		return false
	}
	cmd := exec.Command(securityPath,
		"find-generic-password",
		"-s", cfg.KeychainService(),
		"-a", cfg.KeychainAccount(),
		"-w",
	)
	return cmd.Run() == nil
}
