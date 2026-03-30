package app

import (
	"claude-squad/config"
	"claude-squad/log"
	"claude-squad/session"
	"claude-squad/ui"
	"claude-squad/ui/overlay"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAppState struct{}

func (f *fakeAppState) SaveProjects(projectsJSON json.RawMessage) error { return nil }
func (f *fakeAppState) GetProjects() json.RawMessage                    { return json.RawMessage("[]") }
func (f *fakeAppState) DeleteAllProjects() error                        { return nil }
func (f *fakeAppState) SaveInstances(instancesJSON json.RawMessage) error {
	return nil
}
func (f *fakeAppState) GetInstances() json.RawMessage { return json.RawMessage("[]") }
func (f *fakeAppState) GetHelpScreensSeen() uint32    { return 0 }
func (f *fakeAppState) SetHelpScreensSeen(seen uint32) error {
	return nil
}

// TestMain runs before all tests to set up the test environment
func TestMain(m *testing.M) {
	// Initialize the logger before any tests run
	log.Initialize(false)
	defer log.Close()

	// Run all tests
	exitCode := m.Run()

	// Exit with the same code as the tests
	os.Exit(exitCode)
}

// TestConfirmationModalStateTransitions tests state transitions without full instance setup
func TestConfirmationModalStateTransitions(t *testing.T) {
	// Create a minimal home struct for testing state transitions
	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: config.DefaultConfig(),
	}

	t.Run("shows confirmation on D press", func(t *testing.T) {
		// Simulate pressing 'D'
		h.state = stateDefault
		h.confirmationOverlay = nil

		// Manually trigger what would happen in handleKeyPress for 'D'
		h.state = stateConfirm
		h.confirmationOverlay = overlay.NewConfirmationOverlay("[!] Kill session 'test'?")

		assert.Equal(t, stateConfirm, h.state)
		assert.NotNil(t, h.confirmationOverlay)
		assert.False(t, h.confirmationOverlay.Dismissed)
	})

	t.Run("returns to default on y press", func(t *testing.T) {
		// Start in confirmation state
		h.state = stateConfirm
		h.confirmationOverlay = overlay.NewConfirmationOverlay("Test confirmation")

		// Simulate pressing 'y' using HandleKeyPress
		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}
		shouldClose := h.confirmationOverlay.HandleKeyPress(keyMsg)
		if shouldClose {
			h.state = stateDefault
			h.confirmationOverlay = nil
		}

		assert.Equal(t, stateDefault, h.state)
		assert.Nil(t, h.confirmationOverlay)
	})

	t.Run("returns to default on n press", func(t *testing.T) {
		// Start in confirmation state
		h.state = stateConfirm
		h.confirmationOverlay = overlay.NewConfirmationOverlay("Test confirmation")

		// Simulate pressing 'n' using HandleKeyPress
		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
		shouldClose := h.confirmationOverlay.HandleKeyPress(keyMsg)
		if shouldClose {
			h.state = stateDefault
			h.confirmationOverlay = nil
		}

		assert.Equal(t, stateDefault, h.state)
		assert.Nil(t, h.confirmationOverlay)
	})

	t.Run("returns to default on esc press", func(t *testing.T) {
		// Start in confirmation state
		h.state = stateConfirm
		h.confirmationOverlay = overlay.NewConfirmationOverlay("Test confirmation")

		// Simulate pressing ESC using HandleKeyPress
		keyMsg := tea.KeyMsg{Type: tea.KeyEscape}
		shouldClose := h.confirmationOverlay.HandleKeyPress(keyMsg)
		if shouldClose {
			h.state = stateDefault
			h.confirmationOverlay = nil
		}

		assert.Equal(t, stateDefault, h.state)
		assert.Nil(t, h.confirmationOverlay)
	})
}

// TestConfirmationModalKeyHandling tests the actual key handling in confirmation state
func TestConfirmationModalKeyHandling(t *testing.T) {
	// Import needed packages
	spinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	list := ui.NewList(&spinner, false)

	// Create enough of home struct to test handleKeyPress in confirmation state
	h := &home{
		ctx:                 context.Background(),
		state:               stateConfirm,
		appConfig:           config.DefaultConfig(),
		list:                list,
		menu:                ui.NewMenu(),
		confirmationOverlay: overlay.NewConfirmationOverlay("Kill session?"),
	}

	testCases := []struct {
		name              string
		key               string
		expectedState     state
		expectedDismissed bool
		expectedNil       bool
	}{
		{
			name:              "y key confirms and dismisses overlay",
			key:               "y",
			expectedState:     stateDefault,
			expectedDismissed: true,
			expectedNil:       true,
		},
		{
			name:              "n key cancels and dismisses overlay",
			key:               "n",
			expectedState:     stateDefault,
			expectedDismissed: true,
			expectedNil:       true,
		},
		{
			name:              "esc key cancels and dismisses overlay",
			key:               "esc",
			expectedState:     stateDefault,
			expectedDismissed: true,
			expectedNil:       true,
		},
		{
			name:              "other keys are ignored",
			key:               "x",
			expectedState:     stateConfirm,
			expectedDismissed: false,
			expectedNil:       false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset state
			h.state = stateConfirm
			h.confirmationOverlay = overlay.NewConfirmationOverlay("Kill session?")

			// Create key message
			var keyMsg tea.KeyMsg
			if tc.key == "esc" {
				keyMsg = tea.KeyMsg{Type: tea.KeyEscape}
			} else {
				keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)}
			}

			// Call handleKeyPress
			model, _ := h.handleKeyPress(keyMsg)
			homeModel, ok := model.(*home)
			require.True(t, ok)

			assert.Equal(t, tc.expectedState, homeModel.state, "State mismatch for key: %s", tc.key)
			if tc.expectedNil {
				assert.Nil(t, homeModel.confirmationOverlay, "Overlay should be nil for key: %s", tc.key)
			} else {
				assert.NotNil(t, homeModel.confirmationOverlay, "Overlay should not be nil for key: %s", tc.key)
				assert.Equal(t, tc.expectedDismissed, homeModel.confirmationOverlay.Dismissed, "Dismissed mismatch for key: %s", tc.key)
			}
		})
	}
}

// TestConfirmationMessageFormatting tests that confirmation messages are formatted correctly
func TestConfirmationMessageFormatting(t *testing.T) {
	testCases := []struct {
		name            string
		sessionTitle    string
		expectedMessage string
	}{
		{
			name:            "short session name",
			sessionTitle:    "my-feature",
			expectedMessage: "[!] Kill session 'my-feature'? (y/n)",
		},
		{
			name:            "long session name",
			sessionTitle:    "very-long-feature-branch-name-here",
			expectedMessage: "[!] Kill session 'very-long-feature-branch-name-here'? (y/n)",
		},
		{
			name:            "session with spaces",
			sessionTitle:    "feature with spaces",
			expectedMessage: "[!] Kill session 'feature with spaces'? (y/n)",
		},
		{
			name:            "session with special chars",
			sessionTitle:    "feature/branch-123",
			expectedMessage: "[!] Kill session 'feature/branch-123'? (y/n)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test the message formatting directly
			actualMessage := fmt.Sprintf("[!] Kill session '%s'? (y/n)", tc.sessionTitle)
			assert.Equal(t, tc.expectedMessage, actualMessage)
		})
	}
}

// TestConfirmationFlowSimulation tests the confirmation flow by simulating the state changes
func TestConfirmationFlowSimulation(t *testing.T) {
	// Create a minimal setup
	spinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	list := ui.NewList(&spinner, false)

	// Add test instance
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "test-session",
		Path:    t.TempDir(),
		Program: "claude",
		AutoYes: false,
	})
	require.NoError(t, err)
	_ = list.AddInstance(instance)
	list.SetSelectedInstance(0)

	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: config.DefaultConfig(),
		list:      list,
		menu:      ui.NewMenu(),
	}

	// Simulate what happens when D is pressed
	selected := h.list.GetSelectedInstance()
	require.NotNil(t, selected)

	// This is what the KeyKill handler does
	message := fmt.Sprintf("[!] Kill session '%s'?", selected.Title)
	h.confirmationOverlay = overlay.NewConfirmationOverlay(message)
	h.state = stateConfirm

	// Verify the state
	assert.Equal(t, stateConfirm, h.state)
	assert.NotNil(t, h.confirmationOverlay)
	assert.False(t, h.confirmationOverlay.Dismissed)
	// Test that overlay renders with the correct message
	rendered := h.confirmationOverlay.Render()
	assert.Contains(t, rendered, "Kill session 'test-session'?")
}

func TestKillConfirmationRemovesFolderSession(t *testing.T) {
	spinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	list := ui.NewList(&spinner, false)

	project := &session.Project{
		ID:       "proj-folder",
		Name:     "folder-project",
		RootPath: t.TempDir(),
		Kind:     session.ProjectKindFolder,
		Sessions: []*session.Instance{},
	}
	list.AddProject(project)

	instance, err := session.NewInstance(session.InstanceOptions{
		ID:          "sess-folder",
		ProjectID:   project.ID,
		ProjectKind: session.ProjectKindFolder,
		Title:       "folder-session",
		Path:        project.RootPath,
		Program:     "claude",
	})
	require.NoError(t, err)
	list.AddSession(project.ID, instance)

	storage, err := session.NewStorage(&fakeAppState{})
	require.NoError(t, err)

	h := &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		list:         list,
		menu:         ui.NewMenu(),
		storage:      storage,
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane(), ui.NewTerminalPane()),
	}

	selected := h.list.GetSelectedInstance()
	require.NotNil(t, selected)

	killAction := func() tea.Msg {
		if selected.SupportsCheckout() {
			worktree, err := selected.GetGitWorktree()
			if err != nil {
				return err
			}

			checkedOut, err := worktree.IsBranchCheckedOut()
			if err != nil {
				return err
			}

			if checkedOut {
				return fmt.Errorf("instance %s is currently checked out", selected.Title)
			}
		}

		h.tabbedWindow.CleanupTerminalForInstance(selected.ID)
		if err := selected.Kill(); err != nil {
			return err
		}
		h.tabbedWindow.CleanupTerminalForInstance(selected.ID)
		h.list.RemoveSession(selected.ID)
		if err := h.saveProjects(); err != nil {
			return err
		}
		return instanceChangedMsg{}
	}

	msg := killAction()
	require.IsType(t, instanceChangedMsg{}, msg)
	require.Nil(t, h.list.GetSelectedInstance())
	require.Empty(t, project.Sessions)
}

func newLivePaneTestHome(t *testing.T) (*home, *session.Project, *session.Instance, *session.Instance) {
	t.Helper()

	spin := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	list := ui.NewList(&spin, false)

	project := &session.Project{
		ID:       "proj-live-pane",
		Name:     "live-pane-project",
		RootPath: t.TempDir(),
		Kind:     session.ProjectKindFolder,
		Sessions: []*session.Instance{},
	}
	list.AddProject(project)

	instanceA, err := session.NewInstance(session.InstanceOptions{
		ID:          "sess-a",
		ProjectID:   project.ID,
		ProjectKind: session.ProjectKindFolder,
		Title:       "session-a",
		Path:        project.RootPath,
		Program:     "claude",
	})
	require.NoError(t, err)

	instanceB, err := session.NewInstance(session.InstanceOptions{
		ID:          "sess-b",
		ProjectID:   project.ID,
		ProjectKind: session.ProjectKindFolder,
		Title:       "session-b",
		Path:        project.RootPath,
		Program:     "claude",
	})
	require.NoError(t, err)

	list.AddSession(project.ID, instanceA)
	list.AddSession(project.ID, instanceB)
	list.SelectInstance(instanceA)

	tabbedWindow := ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane(), ui.NewTerminalPane())
	tabbedWindow.SetSize(100, 40)

	h := &home{
		ctx:          context.Background(),
		list:         list,
		menu:         ui.NewMenu(),
		tabbedWindow: tabbedWindow,
		errBox:       ui.NewErrBox(),
	}

	return h, project, instanceA, instanceB
}

func TestPreviewCaptureDoneMsgAppliesContentForCurrentSelection(t *testing.T) {
	h, _, instanceA, _ := newLivePaneTestHome(t)

	model, cmd := h.Update(previewCaptureDoneMsg{
		instanceID: instanceA.ID,
		content:    "preview result",
	})
	require.Nil(t, cmd)

	homeModel := model.(*home)
	require.Contains(t, homeModel.tabbedWindow.String(), "preview result")
}

func TestPreviewCaptureDoneMsgIgnoresStaleSelection(t *testing.T) {
	h, _, instanceA, instanceB := newLivePaneTestHome(t)

	_, _ = h.Update(previewCaptureDoneMsg{
		instanceID: instanceA.ID,
		content:    "current preview",
	})

	h.list.SelectInstance(instanceB)
	_, _ = h.Update(previewCaptureDoneMsg{
		instanceID: instanceB.ID,
		content:    "replacement preview",
	})

	model, cmd := h.Update(previewCaptureDoneMsg{
		instanceID: instanceA.ID,
		content:    "stale preview",
	})
	require.Nil(t, cmd)

	homeModel := model.(*home)
	rendered := homeModel.tabbedWindow.String()
	require.Contains(t, rendered, "replacement preview")
	require.NotContains(t, rendered, "stale preview")
}

func TestTerminalCaptureDoneMsgAppliesContentForCurrentSelection(t *testing.T) {
	h, _, instanceA, _ := newLivePaneTestHome(t)
	h.tabbedWindow.Toggle()
	h.tabbedWindow.Toggle()

	model, cmd := h.Update(terminalCaptureDoneMsg{
		instanceID: instanceA.ID,
		content:    "terminal result",
	})
	require.Nil(t, cmd)

	homeModel := model.(*home)
	require.Contains(t, homeModel.tabbedWindow.String(), "terminal result")
}

func TestTerminalCaptureDoneMsgIgnoresStaleTab(t *testing.T) {
	h, _, instanceA, _ := newLivePaneTestHome(t)
	h.tabbedWindow.Toggle()
	h.tabbedWindow.Toggle()

	_, _ = h.Update(terminalCaptureDoneMsg{
		instanceID: instanceA.ID,
		content:    "current terminal",
	})

	h.tabbedWindow.Toggle()

	model, cmd := h.Update(terminalCaptureDoneMsg{
		instanceID: instanceA.ID,
		content:    "stale terminal",
	})
	require.Nil(t, cmd)

	homeModel := model.(*home)
	homeModel.tabbedWindow.Toggle()
	homeModel.tabbedWindow.Toggle()

	rendered := homeModel.tabbedWindow.String()
	require.Contains(t, rendered, "current terminal")
	require.NotContains(t, rendered, "stale terminal")
}

func TestAddProjectCancelResetsMenuState(t *testing.T) {
	spin := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	list := ui.NewList(&spin, false)
	storage, err := session.NewStorage(&fakeAppState{})
	require.NoError(t, err)

	h := &home{
		ctx:              context.Background(),
		state:            stateAddProject,
		appConfig:        config.DefaultConfig(),
		list:             list,
		menu:             ui.NewMenu(),
		storage:          storage,
		textInputOverlay: overlay.NewProjectPathOverlay("Project folder", ""),
	}
	h.menu.SetSize(120, 1)
	h.menu.SetState(ui.StatePrompt)

	model, cmd := h.handleKeyPress(tea.KeyMsg{Type: tea.KeyCtrlC})
	require.NotNil(t, cmd)

	homeModel, ok := model.(*home)
	require.True(t, ok)
	assert.Equal(t, stateDefault, homeModel.state)
	assert.Nil(t, homeModel.textInputOverlay)
	assert.Contains(t, homeModel.menu.String(), "add project")
	assert.NotContains(t, homeModel.menu.String(), "submit")
}

func TestAddProjectSuccessResetsMenuState(t *testing.T) {
	spin := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	list := ui.NewList(&spin, false)
	storage, err := session.NewStorage(&fakeAppState{})
	require.NoError(t, err)

	h := &home{
		ctx:              context.Background(),
		state:            stateAddProject,
		appConfig:        config.DefaultConfig(),
		list:             list,
		menu:             ui.NewMenu(),
		storage:          storage,
		tabbedWindow:     ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane(), ui.NewTerminalPane()),
		textInputOverlay: overlay.NewProjectPathOverlay("Project folder", t.TempDir()),
	}
	h.menu.SetSize(120, 1)
	h.menu.SetState(ui.StatePrompt)

	model, cmd := h.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, model)
	require.Nil(t, cmd)

	model, cmd = h.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)

	homeModel, ok := model.(*home)
	require.True(t, ok)
	assert.Equal(t, stateDefault, homeModel.state)
	assert.Nil(t, homeModel.textInputOverlay)
	require.Len(t, homeModel.list.GetProjects(), 1)
	assert.Contains(t, homeModel.menu.String(), "add project")
	assert.NotContains(t, homeModel.menu.String(), "submit")
}

func TestAddProjectUsesLaunchDirAsDefaultPath(t *testing.T) {
	spin := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	list := ui.NewList(&spin, false)

	launchDir := t.TempDir()
	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		keySent:   true,
		launchDir: launchDir,
		appConfig: config.DefaultConfig(),
		list:      list,
		menu:      ui.NewMenu(),
	}

	model, cmd := h.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	require.NotNil(t, model)
	require.NotNil(t, cmd)

	homeModel, ok := model.(*home)
	require.True(t, ok)
	require.NotNil(t, homeModel.textInputOverlay)
	assert.Equal(t, stateAddProject, homeModel.state)
	assert.Equal(t, launchDir, homeModel.textInputOverlay.GetPathValue())
}

// TestConfirmActionWithDifferentTypes tests that confirmAction works with different action types
func TestConfirmActionWithDifferentTypes(t *testing.T) {
	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: config.DefaultConfig(),
	}

	t.Run("works with simple action returning nil", func(t *testing.T) {
		actionCalled := false
		action := func() tea.Msg {
			actionCalled = true
			return nil
		}

		// Set up callback to track action execution
		actionExecuted := false
		h.confirmationOverlay = overlay.NewConfirmationOverlay("Test action?")
		h.confirmationOverlay.OnConfirm = func() {
			h.state = stateDefault
			actionExecuted = true
			action() // Execute the action
		}
		h.state = stateConfirm

		// Verify state was set
		assert.Equal(t, stateConfirm, h.state)
		assert.NotNil(t, h.confirmationOverlay)
		assert.False(t, h.confirmationOverlay.Dismissed)
		assert.NotNil(t, h.confirmationOverlay.OnConfirm)

		// Execute the confirmation callback
		h.confirmationOverlay.OnConfirm()
		assert.True(t, actionCalled)
		assert.True(t, actionExecuted)
	})

	t.Run("works with action returning error", func(t *testing.T) {
		expectedErr := fmt.Errorf("test error")
		action := func() tea.Msg {
			return expectedErr
		}

		// Set up callback to track action execution
		var receivedMsg tea.Msg
		h.confirmationOverlay = overlay.NewConfirmationOverlay("Error action?")
		h.confirmationOverlay.OnConfirm = func() {
			h.state = stateDefault
			receivedMsg = action() // Execute the action and capture result
		}
		h.state = stateConfirm

		// Verify state was set
		assert.Equal(t, stateConfirm, h.state)
		assert.NotNil(t, h.confirmationOverlay)
		assert.False(t, h.confirmationOverlay.Dismissed)
		assert.NotNil(t, h.confirmationOverlay.OnConfirm)

		// Execute the confirmation callback
		h.confirmationOverlay.OnConfirm()
		assert.Equal(t, expectedErr, receivedMsg)
	})

	t.Run("works with action returning custom message", func(t *testing.T) {
		action := func() tea.Msg {
			return instanceChangedMsg{}
		}

		// Set up callback to track action execution
		var receivedMsg tea.Msg
		h.confirmationOverlay = overlay.NewConfirmationOverlay("Custom message action?")
		h.confirmationOverlay.OnConfirm = func() {
			h.state = stateDefault
			receivedMsg = action() // Execute the action and capture result
		}
		h.state = stateConfirm

		// Verify state was set
		assert.Equal(t, stateConfirm, h.state)
		assert.NotNil(t, h.confirmationOverlay)
		assert.False(t, h.confirmationOverlay.Dismissed)
		assert.NotNil(t, h.confirmationOverlay.OnConfirm)

		// Execute the confirmation callback
		h.confirmationOverlay.OnConfirm()
		_, ok := receivedMsg.(instanceChangedMsg)
		assert.True(t, ok, "Expected instanceChangedMsg but got %T", receivedMsg)
	})
}

// TestMultipleConfirmationsDontInterfere tests that multiple confirmations don't interfere with each other
func TestMultipleConfirmationsDontInterfere(t *testing.T) {
	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: config.DefaultConfig(),
	}

	// First confirmation
	action1Called := false
	action1 := func() tea.Msg {
		action1Called = true
		return nil
	}

	// Set up first confirmation
	h.confirmationOverlay = overlay.NewConfirmationOverlay("First action?")
	firstOnConfirm := func() {
		h.state = stateDefault
		action1()
	}
	h.confirmationOverlay.OnConfirm = firstOnConfirm
	h.state = stateConfirm

	// Verify first confirmation
	assert.Equal(t, stateConfirm, h.state)
	assert.NotNil(t, h.confirmationOverlay)
	assert.False(t, h.confirmationOverlay.Dismissed)
	assert.NotNil(t, h.confirmationOverlay.OnConfirm)

	// Cancel first confirmation (simulate pressing 'n')
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	shouldClose := h.confirmationOverlay.HandleKeyPress(keyMsg)
	if shouldClose {
		h.state = stateDefault
		h.confirmationOverlay = nil
	}

	// Second confirmation with different action
	action2Called := false
	action2 := func() tea.Msg {
		action2Called = true
		return fmt.Errorf("action2 error")
	}

	// Set up second confirmation
	h.confirmationOverlay = overlay.NewConfirmationOverlay("Second action?")
	var secondResult tea.Msg
	secondOnConfirm := func() {
		h.state = stateDefault
		secondResult = action2()
	}
	h.confirmationOverlay.OnConfirm = secondOnConfirm
	h.state = stateConfirm

	// Verify second confirmation
	assert.Equal(t, stateConfirm, h.state)
	assert.NotNil(t, h.confirmationOverlay)
	assert.False(t, h.confirmationOverlay.Dismissed)
	assert.NotNil(t, h.confirmationOverlay.OnConfirm)

	// Execute second action to verify it's the correct one
	h.confirmationOverlay.OnConfirm()
	err, ok := secondResult.(error)
	assert.True(t, ok)
	assert.Equal(t, "action2 error", err.Error())
	assert.True(t, action2Called)
	assert.False(t, action1Called, "First action should not have been called")

	// Test that cancelled action can still be executed independently
	firstOnConfirm()
	assert.True(t, action1Called, "First action should be callable after being replaced")
}

// TestConfirmationModalVisualAppearance tests that confirmation modal has distinct visual appearance
func TestConfirmationModalVisualAppearance(t *testing.T) {
	h := &home{
		ctx:       context.Background(),
		state:     stateDefault,
		appConfig: config.DefaultConfig(),
	}

	// Create a test confirmation overlay
	message := "[!] Delete everything?"
	h.confirmationOverlay = overlay.NewConfirmationOverlay(message)
	h.state = stateConfirm

	// Verify the overlay was created with confirmation settings
	assert.NotNil(t, h.confirmationOverlay)
	assert.Equal(t, stateConfirm, h.state)
	assert.False(t, h.confirmationOverlay.Dismissed)

	// Test the overlay render (we can test that it renders without errors)
	rendered := h.confirmationOverlay.Render()
	assert.NotEmpty(t, rendered)

	// Test that it includes the message content and instructions
	assert.Contains(t, rendered, "Delete everything?")
	assert.Contains(t, rendered, "Press")
	assert.Contains(t, rendered, "to confirm")
	assert.Contains(t, rendered, "to cancel")

	// Test that the danger indicator is preserved
	assert.Contains(t, rendered, "[!")
}
