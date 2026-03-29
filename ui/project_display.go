package ui

import (
	"claude-squad/session"
	"fmt"
)

func projectKindText(kind session.ProjectKind) string {
	if kind == session.ProjectKindGit {
		return "Git project"
	}
	return "Folder project"
}

func sessionCountText(count int) string {
	if count == 1 {
		return "1 session"
	}
	return fmt.Sprintf("%d sessions", count)
}

func projectMetaText(project *session.Project) string {
	return fmt.Sprintf("%s • %s", projectKindText(project.Kind), sessionCountText(len(project.Sessions)))
}
