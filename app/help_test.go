package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeneralHelpUsesDukeSquadBranding(t *testing.T) {
	content := helpTypeGeneral{}.toContent()

	require.Contains(t, content, "Duke Squad")
	require.Contains(t, content, "SSH")
	require.Contains(t, content, "Add a local folder or SSH project")
}
