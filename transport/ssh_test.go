package transport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSSHRunnerBuildsShellWrappedCommand(t *testing.T) {
	runner := NewSSHRunnerWithConfig(SSHConfig{Username: "dukebot", Host: "dukebot.local"})
	cmd := runner.command(CommandSpec{
		Program: "git",
		Args:    []string{"-C", "/srv/repo", "status"},
	}, false)

	require.Equal(t, "ssh", cmd.Args[0])
	require.Contains(t, strings.Join(cmd.Args, " "), "dukebot@dukebot.local")
	require.Contains(t, strings.Join(cmd.Args, " "), "sh -lc")
	require.Contains(t, strings.Join(cmd.Args, " "), "git")
}

func TestEnsureAskPassScriptHandlesHostKeyConfirmation(t *testing.T) {
	manager := &SSHManager{
		socketDir: t.TempDir(),
		askpass:   filepath.Join(t.TempDir(), "askpass.sh"),
	}

	require.NoError(t, manager.ensureAskPassScript())

	content, err := os.ReadFile(manager.askpass)
	require.NoError(t, err)
	require.Contains(t, string(content), `prompt="${1:-}"`)
	require.Contains(t, string(content), `continue connecting`)
	require.Contains(t, string(content), `printf 'yes\n'`)
}

func TestAuthEnvIncludesAskPassForHostKeyPromptsWithoutPassword(t *testing.T) {
	manager := &SSHManager{
		socketDir: t.TempDir(),
		askpass:   filepath.Join(t.TempDir(), "askpass.sh"),
	}

	env := manager.authEnv(SSHConfig{Username: "dukebot", Host: "dukebot.local"})

	require.Contains(t, env, "SSH_ASKPASS="+manager.askpass)
	require.Contains(t, env, "SSH_ASKPASS_REQUIRE=force")
	require.Contains(t, env, "CLAUDE_SQUAD_SSH_KEYCHAIN_SERVICE=claude-squad:ssh:dukebot.local")
	require.Contains(t, env, "CLAUDE_SQUAD_SSH_KEYCHAIN_ACCOUNT=dukebot")
}
