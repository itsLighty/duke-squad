package session

import (
	"os"
	"path/filepath"
	"testing"

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
