package ui

import (
	"claude-squad/session"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

const readyIcon = "● "
const pausedIcon = "⏸ "

var readyStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#51bd73", Dark: "#51bd73"})

var addedLinesStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#51bd73", Dark: "#51bd73"})

var removedLinesStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#de613e"))

var pausedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"})

var titleStyle = lipgloss.NewStyle().
	Padding(1, 1, 0, 1).
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"})

var listDescStyle = lipgloss.NewStyle().
	Padding(0, 1, 1, 1).
	Foreground(lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"})

var selectedTitleStyle = lipgloss.NewStyle().
	Padding(1, 1, 0, 1).
	Background(lipgloss.Color("#dde4f0")).
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#1a1a1a"})

var selectedDescStyle = lipgloss.NewStyle().
	Padding(0, 1, 1, 1).
	Background(lipgloss.Color("#dde4f0")).
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#1a1a1a"})

var mainTitle = lipgloss.NewStyle().
	Background(lipgloss.Color("62")).
	Foreground(lipgloss.Color("230"))

var autoYesStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#dde4f0")).
	Foreground(lipgloss.Color("#1a1a1a"))

type rowKind int

const (
	rowProject rowKind = iota
	rowSession
)

type Selection struct {
	Project  *session.Project
	Instance *session.Instance
}

type listRow struct {
	kind     rowKind
	project  *session.Project
	instance *session.Instance
}

type List struct {
	projects      []*session.Project
	rows          []listRow
	selectedIdx   int
	height, width int
	renderer      *InstanceRenderer
	autoyes       bool
}

func NewList(spinner *spinner.Model, autoYes bool) *List {
	return &List{
		projects: []*session.Project{},
		rows:     []listRow{},
		renderer: &InstanceRenderer{spinner: spinner},
		autoyes:  autoYes,
	}
}

func (l *List) SetProjects(projects []*session.Project) {
	l.projects = projects
	l.rebuildRows()
}

func (l *List) GetProjects() []*session.Project {
	return l.projects
}

func (l *List) SetSize(width, height int) {
	l.width = width
	l.height = height
	l.renderer.setWidth(width)
}

func (l *List) SetSessionPreviewSize(width, height int) (err error) {
	for _, project := range l.projects {
		for _, item := range project.Sessions {
			if !item.Started() || item.Paused() {
				continue
			}

			if innerErr := item.SetPreviewSize(width, height); innerErr != nil {
				err = fmt.Errorf("%w; could not set preview size for %s: %v", err, item.Title, innerErr)
			}
		}
	}
	return
}

func (l *List) NumSessions() int {
	total := 0
	for _, project := range l.projects {
		total += len(project.Sessions)
	}
	return total
}

func (l *List) NumInstances() int {
	return l.NumSessions()
}

type InstanceRenderer struct {
	spinner *spinner.Model
	width   int
}

func (r *InstanceRenderer) setWidth(width int) {
	r.width = AdjustPreviewWidth(width)
}

func (r *InstanceRenderer) renderProject(project *session.Project, selected bool) string {
	titleS := titleStyle
	descS := listDescStyle
	if selected {
		titleS = selectedTitleStyle
		descS = selectedDescStyle
	}

	caret := "▾"
	if project.Collapsed {
		caret = "▸"
	}

	titleText := fmt.Sprintf("%s %s", caret, project.Name)
	if runewidth.StringWidth(titleText) > r.width-2 {
		titleText = runewidth.Truncate(titleText, r.width-5, "...")
	}

	metaText := projectMetaText(project)
	if runewidth.StringWidth(metaText) > r.width-2 {
		metaText = runewidth.Truncate(metaText, r.width-5, "...")
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		titleS.Render(titleText),
		descS.Render(metaText),
	)
}

func (r *InstanceRenderer) renderSession(i *session.Instance, selected bool) string {
	titleS := titleStyle
	descS := listDescStyle
	if selected {
		titleS = selectedTitleStyle
		descS = selectedDescStyle
	}

	var join string
	switch i.Status {
	case session.Running, session.Loading:
		join = fmt.Sprintf("%s ", r.spinner.View())
	case session.Ready:
		join = readyStyle.Render(readyIcon)
	case session.Paused:
		join = pausedStyle.Render(pausedIcon)
	}

	titleText := "  " + i.Title
	widthAvail := r.width - 4
	if widthAvail > 0 && runewidth.StringWidth(titleText) > widthAvail {
		titleText = runewidth.Truncate(titleText, widthAvail-3, "...")
	}

	title := titleS.Render(lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.Place(r.width-3, 1, lipgloss.Left, lipgloss.Center, titleText),
		" ",
		join,
	))

	location := i.Branch
	if location == "" {
		if i.ProjectKind == session.ProjectKindFolder {
			location = "snapshot workspace"
		} else {
			location = "starting..."
		}
	}

	stat := i.GetDiffStats()
	var diff string
	if stat != nil && stat.Error == nil && !stat.IsEmpty() {
		diff = lipgloss.JoinHorizontal(
			lipgloss.Center,
			addedLinesStyle.Background(descS.GetBackground()).Render(fmt.Sprintf("+%d", stat.Added)),
			lipgloss.Style{}.Background(descS.GetBackground()).Foreground(descS.GetForeground()).Render(","),
			removedLinesStyle.Background(descS.GetBackground()).Render(fmt.Sprintf("-%d", stat.Removed)),
		)
	}

	line := "  " + location
	if diff != "" {
		line += "  " + diff
	}
	if runewidth.StringWidth(line) > r.width-2 {
		line = runewidth.Truncate(line, r.width-5, "...")
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		descS.Render(line),
	)
}

func (l *List) String() string {
	const titleText = " Projects "
	const autoYesText = " auto-yes "

	var b strings.Builder
	b.WriteString("\n\n")

	titleWidth := AdjustPreviewWidth(l.width) + 2
	if !l.autoyes {
		b.WriteString(lipgloss.Place(titleWidth, 1, lipgloss.Left, lipgloss.Bottom, mainTitle.Render(titleText)))
	} else {
		title := lipgloss.Place(titleWidth/2, 1, lipgloss.Left, lipgloss.Bottom, mainTitle.Render(titleText))
		autoYes := lipgloss.Place(titleWidth-(titleWidth/2), 1, lipgloss.Right, lipgloss.Bottom, autoYesStyle.Render(autoYesText))
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, title, autoYes))
	}

	b.WriteString("\n\n")

	for i, row := range l.rows {
		switch row.kind {
		case rowProject:
			b.WriteString(l.renderer.renderProject(row.project, i == l.selectedIdx))
		case rowSession:
			b.WriteString(l.renderer.renderSession(row.instance, i == l.selectedIdx))
		}
		if i != len(l.rows)-1 {
			b.WriteString("\n\n")
		}
	}

	return lipgloss.Place(l.width, l.height, lipgloss.Left, lipgloss.Top, b.String())
}

func (l *List) Up() {
	if len(l.rows) == 0 {
		return
	}
	if l.selectedIdx > 0 {
		l.selectedIdx--
	}
}

func (l *List) Down() {
	if len(l.rows) == 0 {
		return
	}
	if l.selectedIdx < len(l.rows)-1 {
		l.selectedIdx++
	}
}

func (l *List) ToggleSelectedProjectCollapsed() *session.Project {
	row := l.getSelectedRow()
	if row == nil || row.project == nil || row.kind != rowProject {
		return nil
	}
	row.project.Collapsed = !row.project.Collapsed
	l.rebuildRows()
	l.SelectProject(row.project)
	return row.project
}

func (l *List) AddProject(project *session.Project) {
	l.projects = append(l.projects, project)
	l.rebuildRows()
	l.SelectProject(project)
}

func (l *List) RemoveProject(projectID string) *session.Project {
	for i, project := range l.projects {
		if project.ID != projectID {
			continue
		}
		l.projects = append(l.projects[:i], l.projects[i+1:]...)
		l.rebuildRows()
		if l.selectedIdx >= len(l.rows) && len(l.rows) > 0 {
			l.selectedIdx = len(l.rows) - 1
		}
		return project
	}
	return nil
}

// AddSession adds an instance beneath the given project and returns a compatibility no-op finalizer.
func (l *List) AddSession(projectID string, instance *session.Instance) func() {
	for _, project := range l.projects {
		if project.ID != projectID {
			continue
		}
		project.AddSession(instance)
		l.rebuildRows()
		l.SelectInstance(instance)
		return func() {}
	}
	return func() {}
}

// AddInstance is kept for compatibility with older tests.
func (l *List) AddInstance(instance *session.Instance) func() {
	project := &session.Project{
		ID:        newCompatProjectID(instance),
		Name:      filepathBase(instance.Path),
		RootPath:  instance.Path,
		Kind:      instance.ProjectKind,
		Transport: instance.ProjectTransport,
		SSHTarget: instance.SSHTarget,
		SSHUser:   instance.SSHUser,
		SSHHost:   instance.SSHHost,
		Sessions:  []*session.Instance{},
	}
	l.AddProject(project)
	return l.AddSession(project.ID, instance)
}

func newCompatProjectID(instance *session.Instance) string {
	if instance.ProjectID != "" {
		return instance.ProjectID
	}
	return "compat-" + instance.ID
}

func filepathBase(path string) string {
	path = strings.TrimRight(path, string(filepath.Separator))
	if path == "" {
		return "project"
	}
	parts := strings.Split(path, string(filepath.Separator))
	return parts[len(parts)-1]
}

func (l *List) RemoveSession(instanceID string) *session.Instance {
	for _, project := range l.projects {
		if instance := project.RemoveSession(instanceID); instance != nil {
			l.rebuildRows()
			if l.selectedIdx >= len(l.rows) && len(l.rows) > 0 {
				l.selectedIdx = len(l.rows) - 1
			}
			return instance
		}
	}
	return nil
}

func (l *List) Kill() {
	selected := l.GetSelectedInstance()
	if selected == nil {
		return
	}
	if err := selected.Kill(); err == nil {
		l.RemoveSession(selected.ID)
	}
}

func (l *List) Attach() (chan struct{}, error) {
	target := l.GetSelectedInstance()
	if target == nil {
		return nil, fmt.Errorf("no session selected")
	}
	return target.Attach()
}

func (l *List) GetSelectedInstance() *session.Instance {
	row := l.getSelectedRow()
	if row == nil || row.kind != rowSession {
		return nil
	}
	return row.instance
}

func (l *List) GetSelectedProject() *session.Project {
	row := l.getSelectedRow()
	if row == nil {
		return nil
	}
	return row.project
}

func (l *List) IsProjectSelected() bool {
	row := l.getSelectedRow()
	return row != nil && row.kind == rowProject
}

func (l *List) GetSelection() Selection {
	return Selection{
		Project:  l.GetSelectedProject(),
		Instance: l.GetSelectedInstance(),
	}
}

func (l *List) SetSelectedInstance(idx int) {
	sessionIdx := 0
	for rowIdx, row := range l.rows {
		if row.kind != rowSession {
			continue
		}
		if sessionIdx == idx {
			l.selectedIdx = rowIdx
			return
		}
		sessionIdx++
	}
}

func (l *List) SelectInstance(target *session.Instance) {
	for i, row := range l.rows {
		if row.instance == target {
			l.selectedIdx = i
			return
		}
	}
}

func (l *List) SelectProject(target *session.Project) {
	for i, row := range l.rows {
		if row.kind == rowProject && row.project == target {
			l.selectedIdx = i
			return
		}
	}
}

func (l *List) SelectProjectForPath(path string) {
	longestMatch := -1
	var target *session.Project
	for _, project := range l.projects {
		if project.Transport != "" && project.Transport != session.ProjectTransportLocal {
			continue
		}
		if path == project.RootPath || strings.HasPrefix(path, project.RootPath+string(filepath.Separator)) {
			if len(project.RootPath) > longestMatch {
				longestMatch = len(project.RootPath)
				target = project
			}
		}
	}
	if target != nil {
		l.SelectProject(target)
		return
	}
	if len(l.rows) > 0 {
		l.selectedIdx = 0
	}
}

func (l *List) GetInstances() []*session.Instance {
	instances := make([]*session.Instance, 0, l.NumSessions())
	for _, project := range l.projects {
		instances = append(instances, project.Sessions...)
	}
	return instances
}

func (l *List) rebuildRows() {
	rows := make([]listRow, 0)
	for _, project := range l.projects {
		rows = append(rows, listRow{kind: rowProject, project: project})
		if project.Collapsed {
			continue
		}
		for _, instance := range project.Sessions {
			rows = append(rows, listRow{kind: rowSession, project: project, instance: instance})
		}
	}
	l.rows = rows
	if l.selectedIdx >= len(l.rows) && len(l.rows) > 0 {
		l.selectedIdx = len(l.rows) - 1
	}
}

func (l *List) getSelectedRow() *listRow {
	if len(l.rows) == 0 || l.selectedIdx < 0 || l.selectedIdx >= len(l.rows) {
		return nil
	}
	return &l.rows[l.selectedIdx]
}
