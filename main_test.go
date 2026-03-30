package main

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRootCommandUsesDukeSquadBranding(t *testing.T) {
	require.Equal(t, "duke-squad", rootCmd.Use)
	require.Contains(t, rootCmd.Short, "Duke Squad")
}

func TestVersionCommandUsesForkReleaseURL(t *testing.T) {
	output := captureStdout(t, func() {
		versionCmd.Run(versionCmd, nil)
	})

	require.Contains(t, output, fmt.Sprintf("duke-squad version %s", version))
	require.Contains(t, output, fmt.Sprintf("https://github.com/itsLighty/duke-squad/releases/tag/v%s", version))
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = writer
	defer func() {
		os.Stdout = originalStdout
	}()

	fn()

	require.NoError(t, writer.Close())
	output, err := io.ReadAll(reader)
	require.NoError(t, err)

	return string(output)
}
