package session

import (
	"claude-squad/session/git"
	"claude-squad/transport"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type ProjectKind string
type ProjectTransport string

const (
	ProjectKindGit    ProjectKind = "git"
	ProjectKindFolder ProjectKind = "folder"

	ProjectTransportLocal ProjectTransport = "local"
	ProjectTransportSSH   ProjectTransport = "ssh"
)

type Project struct {
	ID        string
	Name      string
	RootPath  string
	Kind      ProjectKind
	Transport ProjectTransport
	SSHTarget string
	SSHUser   string
	SSHHost   string
	CreatedAt time.Time
	UpdatedAt time.Time
	Collapsed bool
	Sessions  []*Instance
}

type ProjectData struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	RootPath  string           `json:"root_path"`
	Kind      ProjectKind      `json:"kind"`
	Transport ProjectTransport `json:"transport,omitempty"`
	SSHTarget string           `json:"ssh_target,omitempty"`
	SSHUser   string           `json:"ssh_user,omitempty"`
	SSHHost   string           `json:"ssh_host,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	Collapsed bool             `json:"collapsed"`
	Sessions  []InstanceData   `json:"sessions"`
}

type ProjectOptions struct {
	Transport   ProjectTransport
	Path        string
	SSHTarget   string
	SSHUser     string
	SSHHost     string
	SSHPassword string
	Name        string
}

func NewProject(path string) (*Project, error) {
	return NewProjectFromOptions(ProjectOptions{
		Transport: ProjectTransportLocal,
		Path:      path,
	})
}

func NewProjectFromOptions(opts ProjectOptions) (*Project, error) {
	transportKind := opts.Transport
	if transportKind == "" {
		transportKind = ProjectTransportLocal
	}

	sshCfg := sshConfigFromProjectOptions(opts)
	if transportKind == ProjectTransportSSH {
		if sshCfg.Username == "" {
			return nil, fmt.Errorf("ssh username cannot be empty")
		}
		if sshCfg.Host == "" {
			return nil, fmt.Errorf("host or ip cannot be empty")
		}
	}

	rootPath, kind, err := ClassifyProjectPathWithTransport(transportKind, sshCfg, opts.Path)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(opts.Name)
	if name == "" {
		name = projectBaseName(transportKind, rootPath)
	}
	name = projectDisplayName(transportKind, sshCfg.Username, name)

	now := time.Now()
	return &Project{
		ID:        newID("proj_"),
		Name:      name,
		RootPath:  rootPath,
		Kind:      kind,
		Transport: transportKind,
		SSHTarget: sshCfg.Target(),
		SSHUser:   sshCfg.Username,
		SSHHost:   sshCfg.Host,
		CreatedAt: now,
		UpdatedAt: now,
		Sessions:  []*Instance{},
	}, nil
}

func ClassifyProjectPath(path string) (rootPath string, kind ProjectKind, err error) {
	return ClassifyProjectPathWithTransport(ProjectTransportLocal, transport.SSHConfig{}, path)
}

func ClassifyProjectPathWithTransport(projectTransport ProjectTransport, sshCfg transport.SSHConfig, pathValue string) (rootPath string, kind ProjectKind, err error) {
	if projectTransport == "" {
		projectTransport = ProjectTransportLocal
	}
	switch projectTransport {
	case ProjectTransportLocal:
		return classifyLocalProjectPath(pathValue)
	case ProjectTransportSSH:
		return classifySSHProjectPath(sshCfg, pathValue)
	default:
		return "", "", fmt.Errorf("unsupported project transport: %s", projectTransport)
	}
}

func classifyLocalProjectPath(path string) (rootPath string, kind ProjectKind, err error) {
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

func classifySSHProjectPath(sshCfg transport.SSHConfig, remotePath string) (rootPath string, kind ProjectKind, err error) {
	target := sshCfg.Target()
	if target == "" {
		return "", "", fmt.Errorf("ssh target cannot be empty")
	}
	remotePath = strings.TrimSpace(remotePath)
	if remotePath == "" {
		return "", "", fmt.Errorf("remote folder cannot be empty")
	}

	runner := transport.NewSSHRunnerWithConfig(sshCfg)
	output, err := runner.Output(transport.CommandSpec{
		Program: "pwd",
		Args:    []string{"-P"},
		Dir:     remotePath,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve remote path on %s: %w", target, err)
	}

	resolved := strings.TrimSpace(string(output))
	if resolved == "" {
		return "", "", fmt.Errorf("failed to resolve remote path on %s", target)
	}

	if git.IsGitRepoWithRunner(runner, resolved) {
		repoRoot, err := git.FindRepoRootWithRunner(runner, resolved)
		if err != nil {
			return "", "", err
		}
		return repoRoot, ProjectKindGit, nil
	}

	return resolved, ProjectKindFolder, nil
}

func projectBaseName(projectTransport ProjectTransport, rootPath string) string {
	rootPath = strings.TrimSpace(rootPath)
	if rootPath == "" {
		return "project"
	}
	if projectTransport == ProjectTransportSSH {
		return path.Base(strings.TrimRight(rootPath, "/"))
	}
	return filepath.Base(rootPath)
}

func projectDisplayName(projectTransport ProjectTransport, sshUser, base string) string {
	base = strings.TrimSpace(base)
	if projectTransport != ProjectTransportSSH || strings.TrimSpace(sshUser) == "" {
		return base
	}
	suffix := " (" + strings.TrimSpace(sshUser) + ")"
	if strings.HasSuffix(base, suffix) {
		return base
	}
	return base + suffix
}

func (p *Project) AddSession(instance *Instance) {
	instance.ProjectID = p.ID
	instance.ProjectTransport = p.Transport
	instance.SSHTarget = p.SSHTarget
	instance.SSHUser = p.SSHUser
	instance.SSHHost = p.SSHHost
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
		Transport: p.Transport,
		SSHTarget: p.SSHTarget,
		SSHUser:   p.SSHUser,
		SSHHost:   p.SSHHost,
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
		Transport: data.Transport,
		SSHTarget: data.SSHTarget,
		SSHUser:   data.SSHUser,
		SSHHost:   data.SSHHost,
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
		Collapsed: data.Collapsed,
		Sessions:  make([]*Instance, 0, len(data.Sessions)),
	}
	if project.Transport == "" {
		project.Transport = ProjectTransportLocal
	}
	normalizeSSHProject(project)

	for _, sessionData := range data.Sessions {
		instance, err := FromInstanceData(sessionData)
		if err != nil {
			return nil, fmt.Errorf("failed to create instance %s: %w", sessionData.Title, err)
		}
		instance.ProjectID = project.ID
		if instance.ProjectTransport == "" {
			instance.ProjectTransport = project.Transport
		}
		if instance.SSHTarget == "" {
			instance.SSHTarget = project.SSHTarget
		}
		if instance.SSHUser == "" {
			instance.SSHUser = project.SSHUser
		}
		if instance.SSHHost == "" {
			instance.SSHHost = project.SSHHost
		}
		project.Sessions = append(project.Sessions, instance)
	}

	return project, nil
}

func (p *Project) Runner() (transport.Runner, error) {
	switch p.Transport {
	case "", ProjectTransportLocal:
		return transport.NewLocalRunner(), nil
	case ProjectTransportSSH:
		sshCfg := p.SSHConfig()
		if strings.TrimSpace(sshCfg.Target()) == "" {
			return nil, fmt.Errorf("ssh target is not configured")
		}
		return transport.NewSSHRunnerWithConfig(sshCfg), nil
	default:
		return nil, fmt.Errorf("unsupported project transport: %s", p.Transport)
	}
}

func (p *Project) LocationKey() string {
	if p.Transport == ProjectTransportSSH {
		return string(p.Transport) + "|" + p.SSHConfig().Target() + "|" + p.RootPath
	}
	return string(ProjectTransportLocal) + "||" + p.RootPath
}

func (p *Project) DisplayLocation() string {
	if p.Transport == ProjectTransportSSH {
		return fmt.Sprintf("%s:%s", p.SSHConfig().Target(), p.RootPath)
	}
	return p.RootPath
}

func (p *Project) SSHConfig() transport.SSHConfig {
	return transport.SSHConfig{
		Username: p.SSHUser,
		Host:     p.SSHHost,
	}
}

func (p *Project) SSHConfigWithPassword(password string) transport.SSHConfig {
	cfg := p.SSHConfig()
	cfg.Password = password
	return cfg
}

func normalizeSSHProject(p *Project) {
	if p == nil || p.Transport != ProjectTransportSSH {
		return
	}
	cfg := transport.ParseSSHConfig(p.SSHTarget)
	if p.SSHUser == "" {
		p.SSHUser = cfg.Username
	}
	if p.SSHHost == "" {
		p.SSHHost = cfg.Host
	}
	p.SSHTarget = transport.SSHConfig{Username: p.SSHUser, Host: p.SSHHost}.Target()
}

func sshConfigFromProjectOptions(opts ProjectOptions) transport.SSHConfig {
	cfg := transport.SSHConfig{
		Username: opts.SSHUser,
		Host:     opts.SSHHost,
		Password: opts.SSHPassword,
	}
	if cfg.Target() == "" {
		parsed := transport.ParseSSHConfig(opts.SSHTarget)
		if cfg.Username == "" {
			cfg.Username = parsed.Username
		}
		if cfg.Host == "" {
			cfg.Host = parsed.Host
		}
	}
	return cfg.Normalized()
}
