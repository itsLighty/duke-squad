package ui

import (
	"claude-squad/session"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

var previewPaneStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"})

type PreviewPane struct {
	width  int
	height int

	previewState previewState
	isScrolling  bool
	viewport     viewport.Model
}

type previewState struct {
	// fallback is true if the preview pane is displaying fallback text
	fallback bool
	// text is the text displayed in the preview pane
	text string
}

func NewPreviewPane() *PreviewPane {
	return &PreviewPane{
		viewport: viewport.New(0, 0),
	}
}

func (p *PreviewPane) SetSize(width, maxHeight int) {
	p.width = clampDimension(width)
	p.height = clampDimension(maxHeight)
	p.viewport.Width = p.width
	p.viewport.Height = p.height
}

// setFallbackState sets the preview state with fallback text and a message
func (p *PreviewPane) setFallbackState(message string) {
	p.previewState = previewState{
		fallback: true,
		text:     lipgloss.JoinVertical(lipgloss.Center, FallBackText, "", message),
	}
}

func (p *PreviewPane) setScrollViewportContent(content string) {
	wasAtBottom := p.viewport.AtBottom()
	offset := p.viewport.YOffset

	p.viewport.SetContent(content)
	if wasAtBottom {
		p.viewport.GotoBottom()
		return
	}
	p.viewport.SetYOffset(offset)
}

// Updates the preview pane content with the tmux pane content
func (p *PreviewPane) UpdateContent(project *session.Project, instance *session.Instance) error {
	switch {
	case project == nil && instance == nil:
		p.setFallbackState("No projects yet. Add one with 'a' to start tracking work.")
		return nil
	case project != nil && instance == nil:
		p.setFallbackState(projectOverviewText(project))
		return nil
	case instance.Status == session.Loading:
		p.setFallbackState("Setting up workspace...")
		return nil
	case instance.Status == session.Paused:
		p.setFallbackState(lipgloss.JoinVertical(lipgloss.Center,
			"Session is paused. Press 'r' to resume.",
			"",
			lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{
					Light: "#FFD700",
					Dark:  "#FFD700",
				}).
				Render(fmt.Sprintf(
					"The instance can be checked out at '%s' (copied to your clipboard)",
					instance.Branch,
				)),
		))
		return nil
	case !instance.TmuxAlive():
		p.setFallbackState("Session is not running. Press 'r' to restart.")
		return nil
	}

	var content string
	var err error

	if p.isScrolling {
		// Keep the scrolled preview live instead of freezing it.
		content, err = instance.PreviewFullHistory()
		if err != nil {
			return err
		}
		p.setScrollViewportContent(content)
		return nil
	}

	// In normal mode, use the usual preview
	content, err = instance.Preview()
	if err != nil {
		return err
	}

	// Always update the preview state with content, even if empty
	// This ensures that newly created instances will display their content immediately
	if len(content) == 0 && !instance.Started() {
		p.setFallbackState("Please enter a name for the instance.")
	} else {
		// Update the preview state with the current content
		p.previewState = previewState{
			fallback: false,
			text:     content,
		}
	}

	return nil
}

// SetPreviewContent updates the preview content directly from an async capture.
// It does nothing while scroll mode is active to avoid clobbering viewport state.
func (p *PreviewPane) SetPreviewContent(content string) {
	if p.isScrolling {
		return
	}
	p.previewState = previewState{
		fallback: false,
		text:     content,
	}
}

// Returns the preview pane content as a string.
func (p *PreviewPane) String() string {
	if p.width == 0 || p.height == 0 {
		return strings.Repeat("\n", p.height)
	}

	if p.previewState.fallback {
		availableHeight := p.height

		fallbackLines := len(strings.Split(p.previewState.text, "\n"))

		totalPadding := availableHeight - fallbackLines
		topPadding := 0
		bottomPadding := 0
		if totalPadding > 0 {
			topPadding = totalPadding / 2
			bottomPadding = totalPadding - topPadding // accounts for odd numbers
		}

		var lines []string
		if topPadding > 0 {
			lines = append(lines, strings.Repeat("\n", topPadding))
		}
		lines = append(lines, p.previewState.text)
		if bottomPadding > 0 {
			lines = append(lines, strings.Repeat("\n", bottomPadding))
		}

		return previewPaneStyle.
			Width(p.width).
			Align(lipgloss.Center).
			Render(strings.Join(lines, ""))
	}

	// If in copy mode, use the viewport to display scrollable content
	if p.isScrolling {
		return p.viewport.View()
	}

	// Normal mode display
	// Calculate available height accounting for border and margin
	availableHeight := p.height - 1 //  1 for ellipsis

	lines := strings.Split(p.previewState.text, "\n")

	// Truncate if we have more lines than available height
	if availableHeight > 0 {
		if len(lines) > availableHeight {
			lines = lines[:availableHeight]
			lines = append(lines, "...")
		} else {
			// Pad with empty lines to fill available height
			padding := availableHeight - len(lines)
			lines = append(lines, make([]string, padding)...)
		}
	}

	content := strings.Join(lines, "\n")
	rendered := previewPaneStyle.Width(p.width).Render(content)
	return rendered
}

// ScrollUp scrolls up in the viewport
func (p *PreviewPane) ScrollUp(instance *session.Instance) error {
	if instance == nil || instance.Status == session.Paused || !instance.TmuxAlive() {
		return nil
	}

	if !p.isScrolling {
		// Entering scroll mode - capture entire pane content including scrollback history
		content, err := instance.PreviewFullHistory()
		if err != nil {
			return err
		}
		p.viewport.SetContent(content)

		// Position the viewport at the bottom initially
		p.viewport.GotoBottom()

		p.isScrolling = true
	}

	// Already in scroll mode, just scroll the viewport
	p.viewport.LineUp(1)
	return nil
}

// ScrollDown scrolls down in the viewport
func (p *PreviewPane) ScrollDown(instance *session.Instance) error {
	if instance == nil || instance.Status == session.Paused || !instance.TmuxAlive() {
		return nil
	}

	if !p.isScrolling {
		// Entering scroll mode - capture entire pane content including scrollback history
		content, err := instance.PreviewFullHistory()
		if err != nil {
			return err
		}
		p.viewport.SetContent(content)

		// Position the viewport at the bottom initially
		p.viewport.GotoBottom()

		p.isScrolling = true
		return nil
	}

	// Already in copy mode, just scroll the viewport
	p.viewport.LineDown(1)
	return nil
}

func (p *PreviewPane) ResetScrollMode() {
	if !p.isScrolling {
		return
	}
	p.isScrolling = false
	p.viewport.SetContent("")
	p.viewport.GotoTop()
}

// ResetToNormalMode exits scroll mode and returns to normal mode
func (p *PreviewPane) ResetToNormalMode(instance *session.Instance) error {
	if !p.isScrolling {
		return nil
	}

	p.ResetScrollMode()

	if instance == nil || instance.Status == session.Paused || !instance.TmuxAlive() {
		return nil
	}

	content, err := instance.Preview()
	if err != nil {
		return err
	}
	p.previewState.text = content

	return nil
}
