package overlay

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type PathPicker struct {
	input       textinput.Model
	suggestions []string
	cursor      int
	focused     bool
	width       int
}

func NewPathPicker(initialValue string) *PathPicker {
	input := textinput.New()
	input.Placeholder = "Type or paste a folder path"
	input.Prompt = ""
	input.CharLimit = 0
	input.SetValue(initialValue)

	pp := &PathPicker{input: input}
	pp.refreshSuggestions()
	return pp
}

func (pp *PathPicker) SetWidth(w int) {
	pp.width = w
	pp.input.Width = max(1, w)
}

func (pp *PathPicker) Focus() {
	pp.focused = true
	pp.input.Focus()
}

func (pp *PathPicker) Blur() {
	pp.focused = false
	pp.input.Blur()
}

func (pp *PathPicker) Value() string {
	return strings.TrimSpace(pp.input.Value())
}

func (pp *PathPicker) HandleKeyPress(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyUp:
		if pp.cursor > 0 {
			pp.cursor--
		}
		return true
	case tea.KeyDown:
		if pp.cursor < len(pp.suggestions)-1 {
			pp.cursor++
		}
		return true
	}

	before := pp.input.Value()
	pp.input, _ = pp.input.Update(msg)
	if pp.input.Value() != before {
		pp.refreshSuggestions()
		return true
	}

	return false
}

func (pp *PathPicker) AcceptSuggestion() bool {
	current, err := normalizePathValue(pp.input.Value())
	if err == nil && current != "" {
		if info, statErr := os.Stat(current); statErr == nil && info.IsDir() {
			return false
		}
	}

	if len(pp.suggestions) == 0 {
		return false
	}

	selected := pp.suggestions[pp.cursor]
	if current == strings.TrimSuffix(selected, string(os.PathSeparator)) {
		return false
	}

	pp.input.SetValue(selected)
	pp.refreshSuggestions()
	return true
}

func (pp *PathPicker) refreshSuggestions() {
	searchDir, prefix, err := suggestionSearchRoot(pp.input.Value())
	if err != nil {
		pp.suggestions = nil
		pp.cursor = 0
		return
	}

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		pp.suggestions = nil
		pp.cursor = 0
		return
	}

	prefixLower := strings.ToLower(prefix)
	suggestions := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if prefixLower != "" && !strings.HasPrefix(strings.ToLower(name), prefixLower) {
			continue
		}
		suggestions = append(suggestions, filepath.Join(searchDir, name)+string(os.PathSeparator))
	}

	sort.Slice(suggestions, func(i, j int) bool {
		return strings.ToLower(suggestions[i]) < strings.ToLower(suggestions[j])
	})

	pp.suggestions = suggestions
	if pp.cursor >= len(pp.suggestions) {
		if len(pp.suggestions) == 0 {
			pp.cursor = 0
		} else {
			pp.cursor = len(pp.suggestions) - 1
		}
	}
}

func (pp *PathPicker) Render(label string) string {
	if label == "" {
		label = "Folder"
	}

	var s strings.Builder
	s.WriteString(bpLabelStyle.Render(label))
	if pp.focused {
		s.WriteString(bpDimStyle.Render("  ↑/↓ to browse • tab to autocomplete"))
	}
	s.WriteString("\n\n")
	s.WriteString(pp.input.View())
	s.WriteString("\n\n")

	if len(pp.suggestions) == 0 {
		s.WriteString(bpDimStyle.Render("  No matching folders"))
		return s.String()
	}

	maxVisible := 5
	start := 0
	if pp.cursor >= maxVisible {
		start = pp.cursor - maxVisible + 1
	}
	end := min(start+maxVisible, len(pp.suggestions))

	for i := start; i < end; i++ {
		label := renderSuggestionLabel(pp.suggestions[i])
		switch {
		case i == pp.cursor && pp.focused:
			s.WriteString(bpSelectedStyle.Render("> " + label))
		case i == pp.cursor:
			s.WriteString("  " + label)
		default:
			s.WriteString(bpDimStyle.Render("  " + label))
		}
		if i < end-1 {
			s.WriteString("\n")
		}
	}

	return s.String()
}

func renderSuggestionLabel(path string) string {
	trimmed := strings.TrimSuffix(path, string(os.PathSeparator))
	base := filepath.Base(trimmed) + string(os.PathSeparator)
	return fmt.Sprintf("%s  %s", base, path)
}

func suggestionSearchRoot(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", "", err
		}
		return cwd, "", nil
	}

	expanded, err := expandUserPath(raw)
	if err != nil {
		return "", "", err
	}

	if raw == string(os.PathSeparator) || strings.HasSuffix(raw, string(os.PathSeparator)) {
		searchDir := filepath.Clean(expanded)
		if !filepath.IsAbs(searchDir) {
			searchDir, err = filepath.Abs(searchDir)
			if err != nil {
				return "", "", err
			}
		}
		return searchDir, "", nil
	}

	searchDir := filepath.Dir(expanded)
	if searchDir == "." {
		searchDir, err = os.Getwd()
		if err != nil {
			return "", "", err
		}
	} else if !filepath.IsAbs(searchDir) {
		searchDir, err = filepath.Abs(searchDir)
		if err != nil {
			return "", "", err
		}
	}

	return searchDir, filepath.Base(expanded), nil
}

func normalizePathValue(value string) (string, error) {
	expanded, err := expandUserPath(strings.TrimSpace(value))
	if err != nil {
		return "", err
	}
	if expanded == "" {
		return "", nil
	}
	if !filepath.IsAbs(expanded) {
		expanded, err = filepath.Abs(expanded)
		if err != nil {
			return "", err
		}
	}
	cleaned := filepath.Clean(expanded)
	if cleaned == string(os.PathSeparator) {
		return cleaned, nil
	}
	return strings.TrimSuffix(cleaned, string(os.PathSeparator)), nil
}

func expandUserPath(value string) (string, error) {
	if value != "~" && !strings.HasPrefix(value, "~"+string(os.PathSeparator)) {
		return value, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if value == "~" {
		return home, nil
	}
	return filepath.Join(home, strings.TrimPrefix(value, "~"+string(os.PathSeparator))), nil
}
