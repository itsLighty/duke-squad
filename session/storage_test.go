package session

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeState struct {
	projects        json.RawMessage
	instances       json.RawMessage
	helpScreensSeen uint32
}

func (f *fakeState) SaveProjects(projectsJSON json.RawMessage) error {
	f.projects = projectsJSON
	f.instances = json.RawMessage("[]")
	return nil
}

func (f *fakeState) GetProjects() json.RawMessage {
	return f.projects
}

func (f *fakeState) DeleteAllProjects() error {
	f.projects = json.RawMessage("[]")
	f.instances = json.RawMessage("[]")
	return nil
}

func (f *fakeState) SaveInstances(instancesJSON json.RawMessage) error {
	f.instances = instancesJSON
	return nil
}

func (f *fakeState) GetInstances() json.RawMessage {
	return f.instances
}

func (f *fakeState) GetHelpScreensSeen() uint32 {
	return f.helpScreensSeen
}

func (f *fakeState) SetHelpScreensSeen(seen uint32) error {
	f.helpScreensSeen = seen
	return nil
}

func TestLoadProjectsMigratesLegacyInstances(t *testing.T) {
	now := time.Now()
	legacyInstances := []InstanceData{
		{
			Title:     "one",
			Path:      "/tmp/work/repo",
			Status:    Paused,
			Program:   "claude",
			CreatedAt: now,
			UpdatedAt: now,
			Worktree: GitWorktreeData{
				RepoPath:      "/tmp/work/repo",
				WorktreePath:  "/tmp/worktrees/repo-one",
				SessionName:   "one",
				BranchName:    "user/one",
				BaseCommitSHA: "abc123",
			},
		},
		{
			Title:     "two",
			Path:      "/tmp/other/repo",
			Status:    Paused,
			Program:   "claude",
			CreatedAt: now,
			UpdatedAt: now,
			Worktree: GitWorktreeData{
				RepoPath:      "/tmp/other/repo",
				WorktreePath:  "/tmp/worktrees/repo-two",
				SessionName:   "two",
				BranchName:    "user/two",
				BaseCommitSHA: "def456",
			},
		},
	}

	instancesJSON, err := json.Marshal(legacyInstances)
	require.NoError(t, err)

	state := &fakeState{
		projects:  json.RawMessage("[]"),
		instances: instancesJSON,
	}

	storage, err := NewStorage(state)
	require.NoError(t, err)

	projects, err := storage.LoadProjects()
	require.NoError(t, err)
	require.Len(t, projects, 2)

	require.Equal(t, "repo", projects[0].Name)
	require.Equal(t, "repo (2)", projects[1].Name)
	require.Equal(t, ProjectKindGit, projects[0].Kind)
	require.Len(t, projects[0].Sessions, 1)
	require.Len(t, projects[1].Sessions, 1)
	require.NotEmpty(t, projects[0].Sessions[0].ID)
	require.Equal(t, projects[0].ID, projects[0].Sessions[0].ProjectID)
	require.NotEmpty(t, state.projects)
	require.JSONEq(t, "[]", string(state.instances))
}

func TestSaveAndLoadProjectsPreservesSSHMetadata(t *testing.T) {
	now := time.Now()
	instance, err := FromInstanceData(InstanceData{
		ID:               "sess_ssh",
		ProjectID:        "proj_ssh",
		ProjectKind:      ProjectKindGit,
		ProjectTransport: ProjectTransportSSH,
		SSHTarget:        "dukebot@dukebot.local",
		SSHUser:          "dukebot",
		SSHHost:          "dukebot.local",
		Title:            "remote-work",
		Path:             "/srv/repo",
		Branch:           "codex/remote-work",
		Status:           Paused,
		Program:          "codex",
		CreatedAt:        now,
		UpdatedAt:        now,
		Workspace: WorkspaceData{
			Type:             ProjectKindGit,
			Transport:        ProjectTransportSSH,
			SSHTarget:        "dukebot@dukebot.local",
			SSHUser:          "dukebot",
			SSHHost:          "dukebot.local",
			RootPath:         "/srv/repo",
			WorkspacePath:    "~/.claude-squad/worktrees/codex_remote",
			BranchName:       "codex/remote-work",
			BaseCommitSHA:    "abc123",
			IsExistingBranch: true,
		},
	})
	require.NoError(t, err)

	project := &Project{
		ID:        "proj_ssh",
		Name:      "remote-repo",
		RootPath:  "/srv/repo",
		Kind:      ProjectKindGit,
		Transport: ProjectTransportSSH,
		SSHTarget: "dukebot@dukebot.local",
		SSHUser:   "dukebot",
		SSHHost:   "dukebot.local",
		CreatedAt: now,
		UpdatedAt: now,
		Sessions:  []*Instance{instance},
	}

	state := &fakeState{
		projects:  json.RawMessage("[]"),
		instances: json.RawMessage("[]"),
	}
	storage, err := NewStorage(state)
	require.NoError(t, err)
	require.NoError(t, storage.SaveProjects([]*Project{project}))

	loaded, err := storage.LoadProjects()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	require.Equal(t, ProjectTransportSSH, loaded[0].Transport)
	require.Equal(t, "dukebot@dukebot.local", loaded[0].SSHTarget)
	require.Equal(t, "dukebot", loaded[0].SSHUser)
	require.Equal(t, "dukebot.local", loaded[0].SSHHost)
	require.Len(t, loaded[0].Sessions, 1)
	require.Equal(t, ProjectTransportSSH, loaded[0].Sessions[0].ProjectTransport)
	require.Equal(t, "dukebot@dukebot.local", loaded[0].Sessions[0].SSHTarget)
	require.Equal(t, "dukebot", loaded[0].Sessions[0].SSHUser)
	require.Equal(t, "dukebot.local", loaded[0].Sessions[0].SSHHost)
	require.Equal(t, "~/.claude-squad/worktrees/codex_remote", loaded[0].Sessions[0].GetWorktreePath())
}
