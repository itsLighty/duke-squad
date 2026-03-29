package ui

import (
	"claude-squad/session"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/stretchr/testify/require"
)

func TestProjectRowRendersMetaInsteadOfRootPath(t *testing.T) {
	spin := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	list := NewList(&spin, false)
	list.SetSize(60, 20)
	list.SetProjects([]*session.Project{
		{
			ID:       "proj-1",
			Name:     "claude-squad",
			RootPath: "/Users/denizsonmez/Desktop/very-long-folder/claude-squad",
			Kind:     session.ProjectKindGit,
			Sessions: []*session.Instance{{ID: "sess-1", Title: "one"}},
		},
	})

	rendered := list.String()

	require.Contains(t, rendered, "claude-squad")
	require.Contains(t, rendered, "Git project")
	require.Contains(t, rendered, "1 session")
	require.NotContains(t, rendered, "/Users/denizsonmez/Desktop/very-long-folder/claude-squad")
}
