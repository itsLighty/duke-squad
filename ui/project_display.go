package ui

import (
	"claude-squad/session"
	"fmt"
)

func projectKindText(kind session.ProjectKind) string {
	return projectKindTextWithTransport(session.ProjectTransportLocal, kind)
}

func projectKindTextWithTransport(projectTransport session.ProjectTransport, kind session.ProjectKind) string {
	label := "Folder project"
	if kind == session.ProjectKindGit {
		label = "Git project"
	}
	if projectTransport == session.ProjectTransportSSH {
		return "Remote " + label
	}
	return label
}

func projectMetaText(project *session.Project) string {
	return fmt.Sprintf("%s • %s", projectKindTextWithTransport(project.Transport, project.Kind), sessionCountText(len(project.Sessions)))
}

func projectLocationText(project *session.Project) string {
	if project == nil {
		return ""
	}
	if project.Transport == session.ProjectTransportSSH {
		return project.DisplayLocation()
	}
	return ""
}

func legacyProjectKindText(kind session.ProjectKind) string {
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
