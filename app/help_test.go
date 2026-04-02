package app

import (
	"claude-squad/session"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeneralHelpUsesDukeSquadBranding(t *testing.T) {
	content := helpTypeGeneral{}.toContent()

	require.Contains(t, content, "Duke Squad")
	require.Contains(t, content, "SSH")
	require.Contains(t, content, "Add a local folder or SSH project")
}

func TestInstanceStartHelpIncludesBranchDescription(t *testing.T) {
	content := helpTypeInstanceStart{instance: &session.Instance{
		Program:           "codex",
		ProjectKind:       session.ProjectKindGit,
		Branch:            "dev/keep-preview-live",
		BranchDescription: "Keep the preview pane updating while scrolled",
	}}.toContent()

	require.Contains(t, content, "Git branch: dev/keep-preview-live")
	require.Contains(t, content, "Branch summary: Keep the preview pane updating while scrolled")
}
