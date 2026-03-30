package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadStateMigratesLegacyClaudeSquadState(t *testing.T) {
	tempHome := t.TempDir()
	legacyConfigDir := filepath.Join(tempHome, ".claude-squad")
	err := os.MkdirAll(legacyConfigDir, 0755)
	require.NoError(t, err)

	legacyState := `{
		"help_screens_seen": 7,
		"instances": [],
		"projects": [{"id":"proj_1","name":"faces"}]
	}`
	legacyStatePath := filepath.Join(legacyConfigDir, StateFileName)
	err = os.WriteFile(legacyStatePath, []byte(legacyState), 0644)
	require.NoError(t, err)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", originalHome)

	state := LoadState()

	require.Equal(t, uint32(7), state.HelpScreensSeen)
	migratedStatePath := filepath.Join(tempHome, ".duke-squad", StateFileName)
	assert.FileExists(t, migratedStatePath)

	migrated, err := os.ReadFile(migratedStatePath)
	require.NoError(t, err)
	assert.JSONEq(t, legacyState, string(migrated))
}
