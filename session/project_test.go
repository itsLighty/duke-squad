package session

import (
	"os"
	"path/filepath"
	"testing"

	"claude-squad/transport"
	"github.com/stretchr/testify/require"
)

func TestClassifyProjectPathRejectsMissingPath(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing")

	_, _, err := ClassifyProjectPath(missingPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not exist")
}

func TestClassifyProjectPathRejectsFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "project.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("not a directory"), 0644))

	_, _, err := ClassifyProjectPath(filePath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be a directory")
}

func TestClassifySSHProjectPathDetectsGitRepo(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	sshPath := filepath.Join(binDir, "ssh")
	require.NoError(t, os.WriteFile(sshPath, []byte(`#!/bin/sh
set -eu
socket=""
mode=""
while [ $# -gt 0 ]; do
	case "$1" in
		-S)
			socket="$2"
			shift 2
			;;
		-O)
			mode="$2"
			shift 2
			;;
		-M)
			mode="master"
			shift
			;;
		-f|-N|-tt)
			shift
			;;
		-o)
			shift 2
			;;
		*)
			break
			;;
	esac
done
if [ "$mode" = "check" ]; then
	[ -n "$socket" ] && [ -e "$socket" ] && exit 0
	exit 255
fi
if [ "$mode" = "master" ]; then
	mkdir -p "$(dirname "$socket")"
	: > "$socket"
	exit 0
fi
target="$1"
shift
script="${3:-}"
case "$script" in
	*"pwd"* )
		printf "/srv/repo\n"
		exit 0
		;;
	*"rev-parse"*"--show-toplevel"* )
		printf "/srv/repo\n"
		exit 0
		;;
esac
exit 1
`), 0o755))

	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	rootPath, kind, err := ClassifyProjectPathWithTransport(ProjectTransportSSH, transport.SSHConfig{
		Username: "dukebot",
		Host:     "dukebot.local",
		Password: "secret",
	}, "~/repo")
	require.NoError(t, err)
	require.Equal(t, "/srv/repo", rootPath)
	require.Equal(t, ProjectKindGit, kind)
}
