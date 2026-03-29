package session

import (
	"claude-squad/config"
	"claude-squad/log"
	"claude-squad/session/git"
	"claude-squad/session/tmux"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"
)

type Status int

const (
	Running Status = iota
	Ready
	Loading
	Paused
)

// Instance is a running instance of claude code.
type Instance struct {
	ID          string
	ProjectID   string
	ProjectKind ProjectKind

	Title     string
	Path      string
	Branch    string
	Status    Status
	Program   string
	Height    int
	Width     int
	CreatedAt time.Time
	UpdatedAt time.Time
	AutoYes   bool
	Prompt    string

	diffStats      *git.DiffStats
	selectedBranch string

	started bool

	tmuxSession *tmux.TmuxSession
	workspace   Workspace
}

func (i *Instance) ToInstanceData() InstanceData {
	data := InstanceData{
		ID:          i.ID,
		ProjectID:   i.ProjectID,
		ProjectKind: i.ProjectKind,
		Title:       i.Title,
		Path:        i.Path,
		Branch:      i.Branch,
		Status:      i.Status,
		Height:      i.Height,
		Width:       i.Width,
		CreatedAt:   i.CreatedAt,
		UpdatedAt:   time.Now(),
		Program:     i.Program,
		AutoYes:     i.AutoYes,
	}

	if i.workspace != nil {
		data.Workspace = i.workspace.ToData()
		if i.workspace.Kind() == ProjectKindGit {
			data.Worktree = GitWorktreeData{
				RepoPath:         data.Workspace.RootPath,
				WorktreePath:     data.Workspace.WorkspacePath,
				SessionName:      sessionHandleName(i.ID, i.Title),
				BranchName:       data.Workspace.BranchName,
				BaseCommitSHA:    data.Workspace.BaseCommitSHA,
				IsExistingBranch: data.Workspace.IsExistingBranch,
			}
		}
	}

	if i.diffStats != nil {
		data.DiffStats = DiffStatsData{
			Added:   i.diffStats.Added,
			Removed: i.diffStats.Removed,
			Content: i.diffStats.Content,
		}
	}

	return data
}

func FromInstanceData(data InstanceData) (*Instance, error) {
	if data.ID == "" {
		data.ID = newID("sess_")
	}
	if data.ProjectKind == "" {
		switch data.Workspace.Type {
		case ProjectKindGit, ProjectKindFolder:
			data.ProjectKind = data.Workspace.Type
		case "":
			if data.Worktree.RepoPath != "" {
				data.ProjectKind = ProjectKindGit
			}
		}
	}

	workspace, branch, err := workspaceFromData(data.Workspace, data.Worktree)
	if err != nil {
		return nil, err
	}
	if data.Branch == "" {
		data.Branch = branch
	}

	instance := &Instance{
		ID:          data.ID,
		ProjectID:   data.ProjectID,
		ProjectKind: data.ProjectKind,
		Title:       data.Title,
		Path:        data.Path,
		Branch:      data.Branch,
		Status:      data.Status,
		Height:      data.Height,
		Width:       data.Width,
		CreatedAt:   data.CreatedAt,
		UpdatedAt:   data.UpdatedAt,
		Program:     config.NormalizeProgramCommand(data.Program),
		AutoYes:     data.AutoYes,
		workspace:   workspace,
		diffStats: &git.DiffStats{
			Added:   data.DiffStats.Added,
			Removed: data.DiffStats.Removed,
			Content: data.DiffStats.Content,
		},
	}

	instance.tmuxSession = tmux.NewTmuxSession(instance.tmuxName(), instance.Program)

	if instance.Paused() {
		instance.started = true
		return instance, nil
	}

	if !instance.TmuxAlive() {
		instance.started = true
		if instance.Status == Loading {
			instance.SetStatus(Ready)
		}
		return instance, nil
	}

	if err := instance.Start(false); err != nil {
		return nil, err
	}

	return instance, nil
}

type InstanceOptions struct {
	ID          string
	ProjectID   string
	ProjectKind ProjectKind
	Title       string
	Path        string
	Program     string
	AutoYes     bool
	Branch      string
}

func NewInstance(opts InstanceOptions) (*Instance, error) {
	t := time.Now()

	absPath, err := filepath.Abs(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	projectKind := opts.ProjectKind
	if projectKind == "" {
		if git.IsGitRepo(absPath) {
			projectKind = ProjectKindGit
		} else {
			projectKind = ProjectKindFolder
		}
	}

	id := opts.ID
	if id == "" {
		id = newID("sess_")
	}

	return &Instance{
		ID:             id,
		ProjectID:      opts.ProjectID,
		ProjectKind:    projectKind,
		Title:          opts.Title,
		Status:         Ready,
		Path:           absPath,
		Program:        config.NormalizeProgramCommand(opts.Program),
		Height:         0,
		Width:          0,
		CreatedAt:      t,
		UpdatedAt:      t,
		AutoYes:        opts.AutoYes,
		selectedBranch: opts.Branch,
	}, nil
}

func (i *Instance) tmuxName() string {
	return sessionHandleName(i.ID, i.Title)
}

func (i *Instance) RepoName() (string, error) {
	return filepath.Base(i.Path), nil
}

func (i *Instance) SetStatus(status Status) {
	i.Status = status
}

func (i *Instance) SetSelectedBranch(branch string) {
	i.selectedBranch = branch
}

func (i *Instance) Start(firstTimeSetup bool) error {
	if i.Title == "" {
		return fmt.Errorf("instance title cannot be empty")
	}

	if i.tmuxSession != nil {
		// Keep injected session for tests.
	} else {
		i.tmuxSession = tmux.NewTmuxSession(i.tmuxName(), i.Program)
	}

	if firstTimeSetup {
		workspace, branchName, err := createWorkspace(i.ProjectKind, i.Path, i.ID, i.Title, i.selectedBranch)
		if err != nil {
			return err
		}
		i.workspace = workspace
		i.Branch = branchName
	}

	var setupErr error
	defer func() {
		if setupErr != nil {
			if cleanupErr := i.Kill(); cleanupErr != nil {
				setupErr = fmt.Errorf("%v (cleanup error: %v)", setupErr, cleanupErr)
			}
		} else {
			i.started = true
		}
	}()

	if !firstTimeSetup {
		if err := i.tmuxSession.Restore(); err != nil {
			setupErr = fmt.Errorf("failed to restore existing session: %w", err)
			return setupErr
		}
	} else {
		if i.workspace == nil {
			setupErr = fmt.Errorf("workspace is not initialized")
			return setupErr
		}
		if err := i.workspace.Setup(); err != nil {
			setupErr = fmt.Errorf("failed to setup workspace: %w", err)
			return setupErr
		}

		if err := i.tmuxSession.Start(i.workspace.Path()); err != nil {
			if cleanupErr := i.workspace.Cleanup(); cleanupErr != nil {
				err = fmt.Errorf("%v (cleanup error: %v)", err, cleanupErr)
			}
			setupErr = fmt.Errorf("failed to start new session: %w", err)
			return setupErr
		}
	}

	i.SetStatus(Running)
	return nil
}

func (i *Instance) Kill() error {
	if !i.started {
		return nil
	}

	var errs []error
	if i.tmuxSession != nil {
		if err := i.tmuxSession.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close tmux session: %w", err))
		}
	}
	if i.workspace != nil {
		if err := i.workspace.Cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("failed to cleanup workspace: %w", err))
		}
	}

	return i.combineErrors(errs)
}

func (i *Instance) combineErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}

	errMsg := "multiple cleanup errors occurred:"
	for _, err := range errs {
		errMsg += "\n  - " + err.Error()
	}
	return fmt.Errorf("%s", errMsg)
}

func (i *Instance) Preview() (string, error) {
	if !i.started || i.Status == Paused || !i.TmuxAlive() {
		return "", nil
	}
	return i.tmuxSession.CapturePaneContent()
}

func (i *Instance) HasUpdated() (updated bool, hasPrompt bool) {
	if !i.started || !i.TmuxAlive() {
		return false, false
	}
	return i.tmuxSession.HasUpdated()
}

func (i *Instance) CheckAndHandleTrustPrompt() bool {
	if !i.started || i.tmuxSession == nil || !i.TmuxAlive() {
		return false
	}
	return i.tmuxSession.CheckAndHandleTrustPrompt()
}

// TapEnter sends an enter key press to the tmux session if AutoYes is enabled.
func (i *Instance) TapEnter() {
	if !i.started || !i.AutoYes {
		return
	}
	if err := i.tmuxSession.TapEnter(); err != nil {
		log.ErrorLog.Printf("error tapping enter: %v", err)
	}
}

func (i *Instance) Attach() (chan struct{}, error) {
	if !i.started {
		return nil, fmt.Errorf("cannot attach instance that has not been started")
	}
	return i.tmuxSession.Attach()
}

func (i *Instance) SetPreviewSize(width, height int) error {
	if !i.started || i.Status == Paused {
		return fmt.Errorf("cannot set preview size for instance that has not been started or is paused")
	}
	if !i.TmuxAlive() {
		return nil
	}
	return i.tmuxSession.SetDetachedSize(width, height)
}

func (i *Instance) GetWorktreePath() string {
	if i.workspace == nil {
		return ""
	}
	return i.workspace.Path()
}

func (i *Instance) Started() bool {
	return i.started
}

func (i *Instance) SetTitle(title string) error {
	if i.started {
		return fmt.Errorf("cannot change title of a started instance")
	}
	i.Title = title
	return nil
}

func (i *Instance) Paused() bool {
	return i.Status == Paused
}

func (i *Instance) TmuxAlive() bool {
	return i.tmuxSession != nil && i.tmuxSession.DoesSessionExist()
}

func (i *Instance) Pause() error {
	if !i.started {
		return fmt.Errorf("cannot pause instance that has not been started")
	}
	if i.Status == Paused {
		return fmt.Errorf("instance is already paused")
	}
	if i.workspace == nil {
		return fmt.Errorf("instance workspace is not initialized")
	}

	var errs []error

	if dirty, err := i.workspace.IsDirty(); err != nil {
		errs = append(errs, fmt.Errorf("failed to check if workspace is dirty: %w", err))
		log.ErrorLog.Print(err)
	} else if dirty {
		commitMsg := fmt.Sprintf("[claudesquad] update from '%s' on %s (paused)", i.Title, time.Now().Format(time.RFC822))
		if err := i.workspace.CommitChanges(commitMsg); err != nil {
			errs = append(errs, fmt.Errorf("failed to commit changes: %w", err))
			log.ErrorLog.Print(err)
			return i.combineErrors(errs)
		}
	}

	if err := i.tmuxSession.DetachSafely(); err != nil {
		errs = append(errs, fmt.Errorf("failed to detach tmux session: %w", err))
		log.ErrorLog.Print(err)
	}

	if _, err := os.Stat(i.workspace.Path()); err == nil {
		if err := i.workspace.Remove(); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove workspace: %w", err))
			log.ErrorLog.Print(err)
			return i.combineErrors(errs)
		}
		if err := i.workspace.Prune(); err != nil {
			errs = append(errs, fmt.Errorf("failed to prune workspace: %w", err))
			log.ErrorLog.Print(err)
			return i.combineErrors(errs)
		}
	}

	if err := i.combineErrors(errs); err != nil {
		log.ErrorLog.Print(err)
		return err
	}

	i.SetStatus(Paused)
	if i.workspace.SupportsCheckout() && i.workspace.BranchName() != "" {
		_ = clipboard.WriteAll(i.workspace.BranchName())
	}
	return nil
}

func (i *Instance) Resume() error {
	if !i.started {
		return fmt.Errorf("cannot resume instance that has not been started")
	}

	if i.Status != Paused {
		if i.TmuxAlive() {
			return fmt.Errorf("can only resume paused or stopped instances")
		}
		worktreePath := i.GetWorktreePath()
		if worktreePath == "" {
			return fmt.Errorf("cannot restart session: worktree path unavailable")
		}
		if _, err := os.Stat(worktreePath); err != nil {
			return fmt.Errorf("cannot restart session: worktree is missing")
		}
		if err := i.tmuxSession.Start(worktreePath); err != nil {
			log.ErrorLog.Print(err)
			return fmt.Errorf("failed to restart stopped session: %w", err)
		}
		i.SetStatus(Running)
		return nil
	}
	if i.workspace == nil {
		return fmt.Errorf("instance workspace is not initialized")
	}

	if checked, err := i.workspace.IsBranchCheckedOut(); err != nil {
		log.ErrorLog.Print(err)
		return fmt.Errorf("failed to check if branch is checked out: %w", err)
	} else if checked {
		return fmt.Errorf("cannot resume: branch is checked out, please switch to a different branch")
	}

	if err := i.workspace.Setup(); err != nil {
		log.ErrorLog.Print(err)
		return fmt.Errorf("failed to setup workspace: %w", err)
	}

	if i.tmuxSession.DoesSessionExist() {
		if err := i.tmuxSession.Restore(); err != nil {
			log.ErrorLog.Print(err)
			if err := i.tmuxSession.Start(i.workspace.Path()); err != nil {
				log.ErrorLog.Print(err)
				if cleanupErr := i.workspace.Cleanup(); cleanupErr != nil {
					err = fmt.Errorf("%v (cleanup error: %v)", err, cleanupErr)
					log.ErrorLog.Print(err)
				}
				return fmt.Errorf("failed to start new session: %w", err)
			}
		}
	} else {
		if err := i.tmuxSession.Start(i.workspace.Path()); err != nil {
			log.ErrorLog.Print(err)
			if cleanupErr := i.workspace.Cleanup(); cleanupErr != nil {
				err = fmt.Errorf("%v (cleanup error: %v)", err, cleanupErr)
				log.ErrorLog.Print(err)
			}
			return fmt.Errorf("failed to start new session: %w", err)
		}
	}

	i.SetStatus(Running)
	return nil
}

func (i *Instance) UpdateDiffStats() error {
	if !i.started {
		i.diffStats = nil
		return nil
	}
	if i.Status == Paused {
		return nil
	}
	if i.workspace == nil {
		i.diffStats = nil
		return nil
	}

	stats := i.workspace.Diff()
	if stats.Error != nil {
		if strings.Contains(stats.Error.Error(), "base commit SHA not set") {
			i.diffStats = nil
			return nil
		}
		return fmt.Errorf("failed to get diff stats: %w", stats.Error)
	}

	i.diffStats = stats
	return nil
}

// ComputeDiff runs the expensive git diff I/O and returns the result without
// mutating instance state. Safe to call from a background goroutine.
func (i *Instance) ComputeDiff() *git.DiffStats {
	if !i.started || i.Status == Paused || i.workspace == nil {
		return nil
	}
	stats := i.workspace.Diff()
	if stats != nil && stats.Error != nil && strings.Contains(stats.Error.Error(), "base commit SHA not set") {
		return nil
	}
	return stats
}

// SetDiffStats sets the diff statistics on the instance. Should be called from
// the main event loop to avoid data races with View.
func (i *Instance) SetDiffStats(stats *git.DiffStats) {
	i.diffStats = stats
}

func (i *Instance) GetDiffStats() *git.DiffStats {
	return i.diffStats
}

func (i *Instance) SendPrompt(prompt string) error {
	if !i.started {
		return fmt.Errorf("instance not started")
	}
	if i.tmuxSession == nil {
		return fmt.Errorf("tmux session not initialized")
	}
	if err := i.tmuxSession.SendKeys(prompt); err != nil {
		return fmt.Errorf("error sending keys to tmux session: %w", err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := i.tmuxSession.TapEnter(); err != nil {
		return fmt.Errorf("error tapping enter: %w", err)
	}
	return nil
}

func (i *Instance) PreviewFullHistory() (string, error) {
	if !i.started || i.Status == Paused || !i.TmuxAlive() {
		return "", nil
	}
	return i.tmuxSession.CapturePaneContentWithOptions("-", "-")
}

func (i *Instance) SetTmuxSession(session *tmux.TmuxSession) {
	i.tmuxSession = session
}

func (i *Instance) SendKeys(keys string) error {
	if !i.started || i.Status == Paused || !i.TmuxAlive() {
		return fmt.Errorf("cannot send keys to instance that has not been started or is paused")
	}
	return i.tmuxSession.SendKeys(keys)
}

func (i *Instance) SupportsPush() bool {
	if i.workspace != nil {
		return i.workspace.SupportsPush()
	}
	return i.ProjectKind == ProjectKindGit
}

func (i *Instance) SupportsBranchSelection() bool {
	if i.workspace != nil {
		return i.workspace.SupportsBranchSelection()
	}
	return i.ProjectKind == ProjectKindGit
}

func (i *Instance) SupportsCheckout() bool {
	if i.workspace != nil {
		return i.workspace.SupportsCheckout()
	}
	return i.ProjectKind == ProjectKindGit
}

func (i *Instance) PushChanges(commitMessage string, open bool) error {
	if i.workspace == nil {
		return fmt.Errorf("workspace is not initialized")
	}
	return i.workspace.PushChanges(commitMessage, open)
}

func (i *Instance) GetGitWorktree() (*git.GitWorktree, error) {
	workspace, ok := i.workspace.(*gitWorkspace)
	if !ok {
		return nil, fmt.Errorf("instance is not backed by a git worktree")
	}
	return workspace.worktree, nil
}
