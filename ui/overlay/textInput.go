package overlay

import (
	"claude-squad/config"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	tiStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)

	tiTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("62")).
			Bold(true).
			MarginBottom(1)

	tiButtonStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))

	tiFocusedButtonStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("0"))

	tiDividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

// TextInputOverlay represents a generic form overlay used for prompts and session creation.
type TextInputOverlay struct {
	titleInput    textinput.Model
	promptInput   textarea.Model
	Title         string
	FocusIndex    int
	Submitted     bool
	Canceled      bool
	OnSubmit      func()
	width         int
	height        int
	profilePicker *ProfilePicker
	branchPicker  *BranchPicker
	hasTitleInput bool
	hasPrompt     bool
}

// NewTextInputOverlay creates a prompt-only overlay with an enter button.
func NewTextInputOverlay(title string, initialValue string) *TextInputOverlay {
	promptInput := newPromptInput(initialValue)
	overlay := &TextInputOverlay{
		promptInput: promptInput,
		Title:       title,
		hasPrompt:   true,
	}
	overlay.updateFocusState()
	return overlay
}

// NewTextInputOverlayWithBranchPicker creates a prompt overlay with provider and branch pickers.
func NewTextInputOverlayWithBranchPicker(title string, initialValue string, profiles []config.Profile) *TextInputOverlay {
	promptInput := newPromptInput(initialValue)
	overlay := &TextInputOverlay{
		promptInput:   promptInput,
		Title:         title,
		profilePicker: maybeProfilePicker(profiles, 0),
		branchPicker:  NewBranchPicker(),
		hasPrompt:     true,
	}
	overlay.updateFocusState()
	return overlay
}

// NewSessionCreateOverlay creates a form for creating a new session. The form always
// captures a title and provider. When includePrompt is true, it also captures an initial
// prompt and branch selection.
func NewSessionCreateOverlay(title string, profiles []config.Profile, selectedProfile int, includePrompt bool) *TextInputOverlay {
	overlay := &TextInputOverlay{
		titleInput:    newTitleInput(),
		Title:         title,
		profilePicker: maybeProfilePicker(profiles, selectedProfile),
		hasTitleInput: true,
		hasPrompt:     includePrompt,
	}
	if includePrompt {
		overlay.promptInput = newPromptInput("")
		overlay.branchPicker = NewBranchPicker()
	}
	overlay.updateFocusState()
	return overlay
}

func maybeProfilePicker(profiles []config.Profile, selected int) *ProfilePicker {
	if len(profiles) == 0 {
		return nil
	}
	pp := NewProfilePicker(profiles)
	pp.SetSelectedIndex(selected)
	return pp
}

func newTitleInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "Session title"
	ti.Prompt = ""
	ti.CharLimit = 0
	return ti
}

func newPromptInput(initialValue string) textarea.Model {
	ti := textarea.New()
	ti.SetValue(initialValue)
	ti.ShowLineNumbers = false
	ti.Prompt = ""
	ti.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ti.CharLimit = 0
	ti.MaxHeight = 0
	return ti
}

func (t *TextInputOverlay) SetSize(width, height int) {
	t.width = width
	t.height = height
	if t.hasTitleInput {
		t.titleInput.Width = max(1, width-6)
	}
	if t.hasPrompt {
		promptHeight := max(4, min(8, height/3))
		t.promptInput.SetHeight(promptHeight)
	}
	if t.branchPicker != nil {
		t.branchPicker.SetWidth(width - 6)
	}
	if t.profilePicker != nil {
		t.profilePicker.SetWidth(width - 6)
	}
}

// Init initializes the text input overlay model.
func (t *TextInputOverlay) Init() tea.Cmd {
	return textarea.Blink
}

// View renders the model's view.
func (t *TextInputOverlay) View() string {
	return t.Render()
}

func (t *TextInputOverlay) titleInputIndex() int {
	if !t.hasTitleInput {
		return -1
	}
	return 0
}

func (t *TextInputOverlay) profilePickerIndex() int {
	if t.profilePicker == nil || !t.profilePicker.HasMultiple() {
		return -1
	}
	idx := 0
	if t.hasTitleInput {
		idx++
	}
	return idx
}

func (t *TextInputOverlay) promptInputIndex() int {
	if !t.hasPrompt {
		return -1
	}
	idx := 0
	if t.hasTitleInput {
		idx++
	}
	if t.profilePicker != nil && t.profilePicker.HasMultiple() {
		idx++
	}
	return idx
}

func (t *TextInputOverlay) branchPickerIndex() int {
	if t.branchPicker == nil {
		return -1
	}
	idx := 0
	if t.hasTitleInput {
		idx++
	}
	if t.profilePicker != nil && t.profilePicker.HasMultiple() {
		idx++
	}
	if t.hasPrompt {
		idx++
	}
	return idx
}

func (t *TextInputOverlay) enterButtonIndex() int {
	idx := 0
	if t.hasTitleInput {
		idx++
	}
	if t.profilePicker != nil && t.profilePicker.HasMultiple() {
		idx++
	}
	if t.hasPrompt {
		idx++
	}
	if t.branchPicker != nil {
		idx++
	}
	return idx
}

func (t *TextInputOverlay) focusStops() int {
	return t.enterButtonIndex() + 1
}

func (t *TextInputOverlay) isTitleInput() bool {
	return t.hasTitleInput && t.FocusIndex == t.titleInputIndex()
}

func (t *TextInputOverlay) isProfilePicker() bool {
	return t.profilePicker != nil && t.profilePicker.HasMultiple() && t.FocusIndex == t.profilePickerIndex()
}

func (t *TextInputOverlay) isPromptInput() bool {
	return t.hasPrompt && t.FocusIndex == t.promptInputIndex()
}

func (t *TextInputOverlay) isBranchPicker() bool {
	return t.branchPicker != nil && t.FocusIndex == t.branchPickerIndex()
}

func (t *TextInputOverlay) isEnterButton() bool {
	return t.FocusIndex == t.enterButtonIndex()
}

func (t *TextInputOverlay) setFocusIndex(i int) {
	t.FocusIndex = i
	t.updateFocusState()
}

func (t *TextInputOverlay) updateFocusState() {
	if t.hasTitleInput {
		if t.isTitleInput() {
			t.titleInput.Focus()
		} else {
			t.titleInput.Blur()
		}
	}
	if t.hasPrompt {
		if t.isPromptInput() {
			t.promptInput.Focus()
		} else {
			t.promptInput.Blur()
		}
	}
	if t.branchPicker != nil {
		if t.isBranchPicker() {
			t.branchPicker.Focus()
		} else {
			t.branchPicker.Blur()
		}
	}
	if t.profilePicker != nil {
		if t.isProfilePicker() {
			t.profilePicker.Focus()
		} else {
			t.profilePicker.Blur()
		}
	}
}

// HandleKeyPress processes a key press and updates the state accordingly.
// Returns (shouldClose, branchFilterChanged).
func (t *TextInputOverlay) HandleKeyPress(msg tea.KeyMsg) (bool, bool) {
	switch msg.Type {
	case tea.KeyTab:
		t.setFocusIndex((t.FocusIndex + 1) % t.focusStops())
		return false, false
	case tea.KeyShiftTab:
		t.setFocusIndex((t.FocusIndex - 1 + t.focusStops()) % t.focusStops())
		return false, false
	case tea.KeyEsc:
		t.Canceled = true
		return true, false
	case tea.KeyEnter:
		switch {
		case t.isEnterButton():
			t.Submitted = true
			if t.OnSubmit != nil {
				t.OnSubmit()
			}
			return true, false
		case t.isBranchPicker():
			t.setFocusIndex(t.enterButtonIndex())
			return false, false
		case t.isProfilePicker():
			t.setFocusIndex(t.FocusIndex + 1)
			return false, false
		case t.isTitleInput():
			t.setFocusIndex(min(t.FocusIndex+1, t.enterButtonIndex()))
			return false, false
		case t.isPromptInput():
			t.promptInput, _ = t.promptInput.Update(msg)
			return false, false
		}
	default:
		switch {
		case t.isTitleInput():
			t.titleInput, _ = t.titleInput.Update(msg)
			return false, false
		case t.isPromptInput():
			t.promptInput, _ = t.promptInput.Update(msg)
			return false, false
		case t.isProfilePicker():
			if msg.Type == tea.KeyLeft || msg.Type == tea.KeyRight {
				t.profilePicker.HandleKeyPress(msg)
			}
			return false, false
		case t.isBranchPicker():
			_, filterChanged := t.branchPicker.HandleKeyPress(msg)
			return false, filterChanged
		}
	}
	return false, false
}

// GetTitleValue returns the current title input value.
func (t *TextInputOverlay) GetTitleValue() string {
	if !t.hasTitleInput {
		return ""
	}
	return t.titleInput.Value()
}

// GetPromptValue returns the current prompt value.
func (t *TextInputOverlay) GetPromptValue() string {
	if !t.hasPrompt {
		return ""
	}
	return t.promptInput.Value()
}

// GetSelectedBranch returns the selected branch name from the branch picker.
// Returns empty string if no branch picker is present or "New branch" is selected.
func (t *TextInputOverlay) GetSelectedBranch() string {
	if t.branchPicker == nil {
		return ""
	}
	return t.branchPicker.GetSelectedBranch()
}

// GetSelectedProgram returns the program string from the selected profile.
// Returns empty string if no profile picker is present.
func (t *TextInputOverlay) GetSelectedProgram() string {
	if t.profilePicker == nil {
		return ""
	}
	return t.profilePicker.GetSelectedProfile().Program
}

// BranchFilterVersion returns the current filter version from the branch picker.
// Returns 0 if no branch picker is present.
func (t *TextInputOverlay) BranchFilterVersion() uint64 {
	if t.branchPicker == nil {
		return 0
	}
	return t.branchPicker.GetFilterVersion()
}

// BranchFilter returns the current filter text from the branch picker.
func (t *TextInputOverlay) BranchFilter() string {
	if t.branchPicker == nil {
		return ""
	}
	return t.branchPicker.GetFilter()
}

// SetBranchResults updates the branch picker with search results.
// version must match the picker's current filterVersion to be accepted.
func (t *TextInputOverlay) SetBranchResults(branches []string, version uint64) {
	if t.branchPicker == nil {
		return
	}
	t.branchPicker.SetResults(branches, version)
}

// IsSubmitted returns whether the form was submitted.
func (t *TextInputOverlay) IsSubmitted() bool {
	return t.Submitted
}

// IsCanceled returns whether the form was canceled.
func (t *TextInputOverlay) IsCanceled() bool {
	return t.Canceled
}

// SetOnSubmit sets a callback function for form submission.
func (t *TextInputOverlay) SetOnSubmit(onSubmit func()) {
	t.OnSubmit = onSubmit
}

// Render renders the text input overlay.
func (t *TextInputOverlay) Render() string {
	innerWidth := t.width - 6
	if innerWidth < 1 {
		innerWidth = 1
	}

	if t.hasTitleInput {
		t.titleInput.Width = innerWidth
	}
	if t.hasPrompt {
		t.promptInput.SetWidth(innerWidth)
	}

	divider := tiDividerStyle.Render(strings.Repeat("─", innerWidth))

	var sections []string
	if t.hasTitleInput {
		sections = append(sections, tiTitleStyle.Render("Title")+"\n"+t.titleInput.View())
	}
	if t.profilePicker != nil {
		sections = append(sections, t.profilePicker.Render())
	}
	if t.hasPrompt {
		sections = append(sections, tiTitleStyle.Render(t.Title)+"\n"+t.promptInput.View())
	} else if t.Title != "" {
		sections = append(sections, tiTitleStyle.Render(t.Title))
	}
	if t.branchPicker != nil {
		sections = append(sections, t.branchPicker.Render())
	}

	content := strings.Join(sections, "\n\n"+divider+"\n\n")
	content += "\n\n" + divider + "\n\n"

	enterButton := " Enter "
	if t.isEnterButton() {
		enterButton = tiFocusedButtonStyle.Render(enterButton)
	} else {
		enterButton = tiButtonStyle.Render(enterButton)
	}
	content += enterButton

	return tiStyle.Render(content)
}
