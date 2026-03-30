package ui

import (
	"claude-squad/log"
	"claude-squad/session"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	highlightColor      = lipgloss.AdaptiveColor{Light: "#2F6A62", Dark: "#8FD0C1"}
	tabSeparatorStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B9C2BC", Dark: "#4E5C57"})
	inactiveTabStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#7D847F", Dark: "#8A928D"}).Padding(0, 1)
	activeTabStyle      = lipgloss.NewStyle().Foreground(highlightColor).Bold(true).Padding(0, 1)
	tabRowStyle         = lipgloss.NewStyle()
	windowStyle         = lipgloss.NewStyle()
	projectOverviewStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#1f2622", Dark: "#d7ddd9"}).
				Align(lipgloss.Center)
)

const (
	PreviewTab int = iota
	DiffTab
	TerminalTab
)

type Tab struct {
	Name   string
	Render func(width int, height int) string
}

// TabbedWindow has tabs at the top of a pane which can be selected. The tabs
// take up one rune of height.
type TabbedWindow struct {
	tabs []string

	activeTab int
	height    int
	width     int

	preview  *PreviewPane
	diff     *DiffPane
	terminal *TerminalPane
	instance *session.Instance
	project  *session.Project
}

func NewTabbedWindow(preview *PreviewPane, diff *DiffPane, terminal *TerminalPane) *TabbedWindow {
	return &TabbedWindow{
		tabs: []string{
			"Preview",
			"Diff",
			"Terminal",
		},
		preview:  preview,
		diff:     diff,
		terminal: terminal,
	}
}

func (w *TabbedWindow) SetSelection(project *session.Project, instance *session.Instance) {
	w.project = project
	w.instance = instance
}

func (w *TabbedWindow) paneSize() (width int, height int) {
	tabHeight := tabRowStyle.GetVerticalFrameSize() + 1
	return clampDimension(w.width - windowStyle.GetHorizontalFrameSize()),
		clampDimension(w.height - tabHeight - windowStyle.GetVerticalFrameSize())
}

func (w *TabbedWindow) SetSize(width, height int) {
	w.width = clampDimension(width)
	w.height = clampDimension(height)

	contentWidth, contentHeight := w.paneSize()

	w.preview.SetSize(contentWidth, contentHeight)
	w.diff.SetSize(contentWidth, contentHeight)
	w.terminal.SetSize(contentWidth, contentHeight)
}

func (w *TabbedWindow) GetPreviewSize() (width, height int) {
	return w.preview.width, w.preview.height
}

func (w *TabbedWindow) Toggle() {
	w.activeTab = (w.activeTab + 1) % len(w.tabs)
}

// UpdatePreview updates the content of the preview pane. instance may be nil.
func (w *TabbedWindow) UpdatePreview(project *session.Project, instance *session.Instance) error {
	if w.activeTab != PreviewTab {
		return nil
	}
	return w.preview.UpdateContent(project, instance)
}

func (w *TabbedWindow) SetPreviewContent(content string) {
	if w.activeTab != PreviewTab {
		return
	}
	w.preview.SetPreviewContent(content)
}

func (w *TabbedWindow) UpdateDiff(project *session.Project, instance *session.Instance) {
	if w.activeTab != DiffTab {
		return
	}
	w.diff.SetDiff(project, instance)
}

// UpdateTerminal updates the terminal pane content. Only updates when terminal tab is active.
func (w *TabbedWindow) UpdateTerminal(project *session.Project, instance *session.Instance) error {
	if w.activeTab != TerminalTab {
		return nil
	}
	return w.terminal.UpdateContent(project, instance)
}

func (w *TabbedWindow) SetTerminalContent(content string) {
	if w.activeTab != TerminalTab {
		return
	}
	w.terminal.SetTerminalContent(content)
}

func (w *TabbedWindow) CaptureTerminalContent(instance *session.Instance) (string, error) {
	return w.terminal.CaptureContent(instance)
}

// ResetPreviewToNormalMode resets the preview pane to normal mode
func (w *TabbedWindow) ResetPreviewToNormalMode(instance *session.Instance) error {
	return w.preview.ResetToNormalMode(instance)
}

// Add these new methods for handling scroll events
func (w *TabbedWindow) ScrollUp() {
	switch w.activeTab {
	case PreviewTab:
		err := w.preview.ScrollUp(w.instance)
		if err != nil {
			log.InfoLog.Printf("tabbed window failed to scroll up: %v", err)
		}
	case DiffTab:
		w.diff.ScrollUp()
	case TerminalTab:
		if err := w.terminal.ScrollUp(); err != nil {
			log.InfoLog.Printf("tabbed window failed to scroll terminal up: %v", err)
		}
	}
}

func (w *TabbedWindow) ScrollDown() {
	switch w.activeTab {
	case PreviewTab:
		err := w.preview.ScrollDown(w.instance)
		if err != nil {
			log.InfoLog.Printf("tabbed window failed to scroll down: %v", err)
		}
	case DiffTab:
		w.diff.ScrollDown()
	case TerminalTab:
		if err := w.terminal.ScrollDown(); err != nil {
			log.InfoLog.Printf("tabbed window failed to scroll terminal down: %v", err)
		}
	}
}

// IsInPreviewTab returns true if the preview tab is currently active
func (w *TabbedWindow) IsInPreviewTab() bool {
	return w.activeTab == PreviewTab
}

// IsInDiffTab returns true if the diff tab is currently active
func (w *TabbedWindow) IsInDiffTab() bool {
	return w.activeTab == DiffTab
}

// IsInTerminalTab returns true if the terminal tab is currently active
func (w *TabbedWindow) IsInTerminalTab() bool {
	return w.activeTab == TerminalTab
}

// GetActiveTab returns the currently active tab index
func (w *TabbedWindow) GetActiveTab() int {
	return w.activeTab
}

// AttachTerminal attaches to the terminal tmux session
func (w *TabbedWindow) AttachTerminal() (chan struct{}, error) {
	return w.terminal.Attach()
}

// CleanupTerminal closes the terminal session
func (w *TabbedWindow) CleanupTerminal() {
	w.terminal.Close()
}

// CleanupTerminalForInstance closes the cached terminal session for the given instance title.
func (w *TabbedWindow) CleanupTerminalForInstance(title string) {
	w.terminal.CloseForInstance(title)
}

// IsPreviewInScrollMode returns true if the preview pane is in scroll mode
func (w *TabbedWindow) IsPreviewInScrollMode() bool {
	return w.preview.isScrolling
}

// IsTerminalInScrollMode returns true if the terminal pane is in scroll mode
func (w *TabbedWindow) IsTerminalInScrollMode() bool {
	return w.terminal.IsScrolling()
}

// ResetTerminalToNormalMode exits scroll mode on the terminal pane
func (w *TabbedWindow) ResetTerminalToNormalMode() {
	w.terminal.ResetToNormalMode()
}

func (w *TabbedWindow) ResetTransientState() {
	w.preview.ResetScrollMode()
	w.diff.ResetScroll()
	w.terminal.ResetToNormalMode()
}

func (w *TabbedWindow) String() string {
	if w.width == 0 || w.height == 0 {
		return ""
	}

	if w.project != nil && w.instance == nil {
		return renderCenteredTextBlock(projectOverviewStyle, w.width, w.height, projectOverviewText(w.project))
	}

	contentWidth, contentHeight := w.paneSize()
	row := w.renderTabRow()
	var content string
	switch w.activeTab {
	case PreviewTab:
		content = w.preview.String()
	case DiffTab:
		content = w.diff.String()
	case TerminalTab:
		content = w.terminal.String()
	}
	window := windowStyle.Render(
		lipgloss.Place(
			contentWidth, contentHeight,
			lipgloss.Left, lipgloss.Top, content))

	return lipgloss.JoinVertical(lipgloss.Left, row, window)
}

func (w *TabbedWindow) renderTabRow() string {
	renderedTabs := make([]string, 0, len(w.tabs)*2-1)

	for i, label := range w.tabs {
		style := inactiveTabStyle
		if i == w.activeTab {
			style = activeTabStyle
		}
		renderedTabs = append(renderedTabs, style.Render(label))
		if i != len(w.tabs)-1 {
			renderedTabs = append(renderedTabs, tabSeparatorStyle.Render("·"))
		}
	}

	return tabRowStyle.Width(w.width).Render(lipgloss.JoinHorizontal(lipgloss.Left, renderedTabs...))
}

func renderCenteredTextBlock(style lipgloss.Style, width, height int, text string) string {
	availableHeight := clampDimension(height)
	lines := strings.Split(text, "\n")
	padding := max(0, availableHeight-len(lines))
	topPadding := padding / 2
	bottomPadding := padding - topPadding

	var content []string
	if topPadding > 0 {
		content = append(content, strings.Repeat("\n", topPadding))
	}
	content = append(content, text)
	if bottomPadding > 0 {
		content = append(content, strings.Repeat("\n", bottomPadding))
	}

	return style.Width(clampDimension(width)).Render(strings.Join(content, ""))
}
