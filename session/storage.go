package session

import (
	"claude-squad/config"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"
)

// InstanceData represents the serializable data of an Instance.
type InstanceData struct {
	ID               string           `json:"id"`
	ProjectID        string           `json:"project_id"`
	ProjectKind      ProjectKind      `json:"project_kind"`
	ProjectTransport ProjectTransport `json:"project_transport,omitempty"`
	SSHTarget        string           `json:"ssh_target,omitempty"`
	SSHUser          string           `json:"ssh_user,omitempty"`
	SSHHost          string           `json:"ssh_host,omitempty"`
	Title            string           `json:"title"`
	Path             string           `json:"path"`
	Branch           string           `json:"branch"`
	Status           Status           `json:"status"`
	Height           int              `json:"height"`
	Width            int              `json:"width"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	AutoYes          bool             `json:"auto_yes"`
	Program          string           `json:"program"`
	Workspace        WorkspaceData    `json:"workspace"`

	// Worktree is retained for one-shot migration from the legacy storage shape.
	Worktree  GitWorktreeData `json:"worktree"`
	DiffStats DiffStatsData   `json:"diff_stats"`
}

// GitWorktreeData represents the legacy serializable data of a GitWorktree.
type GitWorktreeData struct {
	RepoPath         string `json:"repo_path"`
	WorktreePath     string `json:"worktree_path"`
	SessionName      string `json:"session_name"`
	BranchName       string `json:"branch_name"`
	BaseCommitSHA    string `json:"base_commit_sha"`
	IsExistingBranch bool   `json:"is_existing_branch"`
}

// DiffStatsData represents the serializable data of a DiffStats.
type DiffStatsData struct {
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
	Content string `json:"content"`
}

// Storage handles saving and loading projects using the state interface.
type Storage struct {
	state config.StateManager
}

func NewStorage(state config.StateManager) (*Storage, error) {
	return &Storage{state: state}, nil
}

func (s *Storage) SaveProjects(projects []*Project) error {
	data := make([]ProjectData, 0, len(projects))
	for _, project := range projects {
		data = append(data, project.ToProjectData())
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal projects: %w", err)
	}
	return s.state.SaveProjects(jsonData)
}

func (s *Storage) LoadProjects() ([]*Project, error) {
	projectJSON := s.state.GetProjects()
	if len(projectJSON) > 0 && string(projectJSON) != "null" && string(projectJSON) != "[]" {
		var projectData []ProjectData
		if err := json.Unmarshal(projectJSON, &projectData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal projects: %w", err)
		}

		projects := make([]*Project, 0, len(projectData))
		for _, data := range projectData {
			project, err := projectFromData(data)
			if err != nil {
				return nil, err
			}
			projects = append(projects, project)
		}
		return projects, nil
	}

	projects, migrated, err := s.loadLegacyProjects()
	if err != nil {
		return nil, err
	}
	if migrated {
		if err := s.SaveProjects(projects); err != nil {
			return nil, err
		}
	}
	return projects, nil
}

func (s *Storage) loadLegacyProjects() ([]*Project, bool, error) {
	jsonData := s.state.GetInstances()
	if len(jsonData) == 0 || string(jsonData) == "null" {
		return []*Project{}, false, nil
	}

	var instancesData []InstanceData
	if err := json.Unmarshal(jsonData, &instancesData); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal legacy instances: %w", err)
	}
	if len(instancesData) == 0 {
		return []*Project{}, false, nil
	}

	projectsByPath := make(map[string]*Project)
	usedNames := make(map[string]int)
	orderedProjects := make([]*Project, 0)

	for _, data := range instancesData {
		projectRoot := data.Worktree.RepoPath
		if projectRoot == "" {
			projectRoot = data.Path
		}
		if projectRoot == "" {
			continue
		}

		project, ok := projectsByPath[projectRoot]
		if !ok {
			name := uniqueProjectName(filepath.Base(projectRoot), usedNames)
			project = &Project{
				ID:        newID("proj_"),
				Name:      name,
				RootPath:  projectRoot,
				Kind:      ProjectKindGit,
				Transport: ProjectTransportLocal,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Sessions:  []*Instance{},
			}
			projectsByPath[projectRoot] = project
			orderedProjects = append(orderedProjects, project)
		}

		if data.ID == "" {
			data.ID = newID("sess_")
		}
		data.ProjectID = project.ID
		if data.ProjectKind == "" {
			data.ProjectKind = ProjectKindGit
		}
		if data.ProjectTransport == "" {
			data.ProjectTransport = ProjectTransportLocal
		}
		if data.Workspace.Type == "" {
			data.Workspace = WorkspaceData{
				Type:             ProjectKindGit,
				Transport:        data.ProjectTransport,
				SSHTarget:        data.SSHTarget,
				SSHUser:          data.SSHUser,
				SSHHost:          data.SSHHost,
				RootPath:         data.Worktree.RepoPath,
				WorkspacePath:    data.Worktree.WorktreePath,
				BranchName:       data.Worktree.BranchName,
				BaseCommitSHA:    data.Worktree.BaseCommitSHA,
				IsExistingBranch: data.Worktree.IsExistingBranch,
			}
		}
		instance, err := FromInstanceData(data)
		if err != nil {
			return nil, false, err
		}
		project.Sessions = append(project.Sessions, instance)
	}

	return orderedProjects, true, nil
}

func uniqueProjectName(base string, used map[string]int) string {
	if base == "" {
		base = "project"
	}
	count := used[base]
	used[base] = count + 1
	if count == 0 {
		return base
	}
	return fmt.Sprintf("%s (%d)", base, count+1)
}

func (s *Storage) DeleteAllProjects() error {
	return s.state.DeleteAllProjects()
}

func (s *Storage) DeleteAllInstances() error {
	return s.DeleteAllProjects()
}

// LoadInstances flattens the project tree for legacy callers.
func (s *Storage) LoadInstances() ([]*Instance, error) {
	projects, err := s.LoadProjects()
	if err != nil {
		return nil, err
	}
	instances := make([]*Instance, 0)
	for _, project := range projects {
		instances = append(instances, project.Sessions...)
	}
	return instances, nil
}

// SaveInstances is kept for compatibility with callers that still operate on a flat session list.
func (s *Storage) SaveInstances(instances []*Instance) error {
	projectsByID := make(map[string]*Project)
	orderedProjects := make([]*Project, 0)

	for _, instance := range instances {
		projectID := instance.ProjectID
		if projectID == "" {
			projectID = "compat-" + instance.ID
		}

		project, ok := projectsByID[projectID]
		if !ok {
			project = &Project{
				ID:        projectID,
				Name:      filepath.Base(instance.Path),
				RootPath:  instance.Path,
				Kind:      instance.ProjectKind,
				Transport: instance.ProjectTransport,
				SSHTarget: instance.SSHTarget,
				SSHUser:   instance.SSHUser,
				SSHHost:   instance.SSHHost,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Sessions:  []*Instance{},
			}
			projectsByID[projectID] = project
			orderedProjects = append(orderedProjects, project)
		}
		project.Sessions = append(project.Sessions, instance)
	}

	return s.SaveProjects(orderedProjects)
}

func (s *Storage) DeleteInstance(title string) error {
	projects, err := s.LoadProjects()
	if err != nil {
		return err
	}

	found := false
	for _, project := range projects {
		filtered := project.Sessions[:0]
		for _, instance := range project.Sessions {
			if !found && instance.Title == title {
				found = true
				continue
			}
			filtered = append(filtered, instance)
		}
		project.Sessions = filtered
	}

	if !found {
		return fmt.Errorf("instance not found: %s", title)
	}
	return s.SaveProjects(projects)
}
