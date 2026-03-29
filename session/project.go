package session

import (
	"claude-squad/session/git"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type ProjectKind string

const (
	ProjectKindGit    ProjectKind = "git"
	ProjectKindFolder ProjectKind = "folder"
)

type Project struct {
	ID        string
	Name      string
	RootPath  string
	Kind      ProjectKind
	CreatedAt time.Time
	UpdatedAt time.Time
	Collapsed bool
	Sessions  []*Instance
}

type ProjectData struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	RootPath  string         `json:"root_path"`
	Kind      ProjectKind    `json:"kind"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Collapsed bool           `json:"collapsed"`
	Sessions  []InstanceData `json:"sessions"`
}

func NewProject(path string) (*Project, error) {
	rootPath, kind, err := ClassifyProjectPath(path)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &Project{
		ID:        newID("proj_"),
		Name:      filepath.Base(rootPath),
		RootPath:  rootPath,
		Kind:      kind,
		CreatedAt: now,
		UpdatedAt: now,
		Sessions:  []*Instance{},
	}, nil
}

func ClassifyProjectPath(path string) (rootPath string, kind ProjectKind, err error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("project path does not exist: %s", absPath)
		}
		return "", "", fmt.Errorf("failed to stat project path: %w", err)
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("project path must be a directory: %s", absPath)
	}

	if git.IsGitRepo(absPath) {
		repoRoot, err := git.FindRepoRoot(absPath)
		if err != nil {
			return "", "", err
		}
		return repoRoot, ProjectKindGit, nil
	}

	return absPath, ProjectKindFolder, nil
}

func (p *Project) AddSession(instance *Instance) {
	instance.ProjectID = p.ID
	p.Sessions = append(p.Sessions, instance)
	p.UpdatedAt = time.Now()
}

func (p *Project) RemoveSession(instanceID string) *Instance {
	for i, instance := range p.Sessions {
		if instance.ID != instanceID {
			continue
		}
		p.Sessions = append(p.Sessions[:i], p.Sessions[i+1:]...)
		p.UpdatedAt = time.Now()
		return instance
	}
	return nil
}

func (p *Project) FindSession(instanceID string) *Instance {
	for _, instance := range p.Sessions {
		if instance.ID == instanceID {
			return instance
		}
	}
	return nil
}

func (p *Project) ToProjectData() ProjectData {
	sessions := make([]InstanceData, 0, len(p.Sessions))
	for _, instance := range p.Sessions {
		if instance.Started() {
			sessions = append(sessions, instance.ToInstanceData())
		}
	}

	return ProjectData{
		ID:        p.ID,
		Name:      p.Name,
		RootPath:  p.RootPath,
		Kind:      p.Kind,
		CreatedAt: p.CreatedAt,
		UpdatedAt: time.Now(),
		Collapsed: p.Collapsed,
		Sessions:  sessions,
	}
}

func projectFromData(data ProjectData) (*Project, error) {
	project := &Project{
		ID:        data.ID,
		Name:      data.Name,
		RootPath:  data.RootPath,
		Kind:      data.Kind,
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
		Collapsed: data.Collapsed,
		Sessions:  make([]*Instance, 0, len(data.Sessions)),
	}

	for _, sessionData := range data.Sessions {
		instance, err := FromInstanceData(sessionData)
		if err != nil {
			return nil, fmt.Errorf("failed to create instance %s: %w", sessionData.Title, err)
		}
		instance.ProjectID = project.ID
		project.Sessions = append(project.Sessions, instance)
	}

	return project, nil
}
