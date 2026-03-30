package ui

import (
	"claude-squad/session"
	"strings"
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

func TestProjectRowRenderIsCompact(t *testing.T) {
	spin := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	renderer := &InstanceRenderer{spinner: &spin}
	renderer.setWidth(48)

	rendered := stripANSI(renderer.renderProject(&session.Project{
		ID:       "proj-1",
		Name:     "claude-squad",
		Kind:     session.ProjectKindGit,
		Sessions: []*session.Instance{{ID: "sess-1"}},
	}, true))

	lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")

	require.Len(t, lines, 2)
	require.NotEmpty(t, strings.TrimSpace(lines[0]))
	require.Contains(t, lines[0], "claude-squad")
	require.False(t, strings.HasPrefix(lines[0], " "))
	require.Contains(t, lines[1], "Git project")
}

func TestSessionRowsRenderNestedUnderProject(t *testing.T) {
	spin := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	list := NewList(&spin, false)
	list.SetSize(60, 20)
	list.SetProjects([]*session.Project{{
		ID:   "proj-1",
		Name: "claude-squad",
		Kind: session.ProjectKindGit,
		Sessions: []*session.Instance{{
			ID:    "sess-1",
			Title: "ship polish",
		}},
	}})

	rendered := stripANSI(list.String())
	for _, line := range strings.Split(rendered, "\n") {
		if strings.Contains(line, "ship polish") {
			require.True(t, strings.HasPrefix(line, "    └ "), "expected nested session prefix in %q", line)
			return
		}
	}

	t.Fatalf("session row not found in %q", rendered)
}
