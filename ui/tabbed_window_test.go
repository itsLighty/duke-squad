package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTabbedWindowSetSizeUsesAllocatedPaneWidth(t *testing.T) {
	window := NewTabbedWindow(NewPreviewPane(), NewDiffPane(), NewTerminalPane())

	window.SetSize(100, 40)

	expectedWidth := 100 - windowStyle.GetHorizontalFrameSize()
	expectedHeight := 40 - (activeTabStyle.GetVerticalFrameSize() + 1) - windowStyle.GetVerticalFrameSize()

	width, height := window.GetPreviewSize()
	require.Equal(t, expectedWidth, width)
	require.Equal(t, expectedHeight, height)
}

func TestTabbedWindowSetSizeClampsSmallDimensions(t *testing.T) {
	window := NewTabbedWindow(NewPreviewPane(), NewDiffPane(), NewTerminalPane())

	window.SetSize(1, 1)

	width, height := window.GetPreviewSize()
	require.Equal(t, 1, width)
	require.Equal(t, 1, height)
	require.Equal(t, 1, window.diff.width)
	require.Equal(t, 1, window.diff.height)
	require.Equal(t, 1, window.terminal.width)
	require.Equal(t, 1, window.terminal.height)
}

func TestTabbedWindowResetTransientStateClearsScrollModes(t *testing.T) {
	window := NewTabbedWindow(NewPreviewPane(), NewDiffPane(), NewTerminalPane())
	window.SetSize(80, 30)

	window.preview.isScrolling = true
	window.preview.viewport.SetContent("preview history")
	window.terminal.isScrolling = true
	window.terminal.viewport.SetContent("terminal history")
	window.diff.viewport.SetContent("one\ntwo\nthree")
	window.diff.viewport.LineDown(1)

	window.ResetTransientState()

	require.False(t, window.preview.isScrolling)
	require.NotContains(t, window.preview.viewport.View(), "preview history")
	require.True(t, strings.TrimSpace(window.preview.viewport.View()) == "")
	require.False(t, window.terminal.isScrolling)
	require.NotContains(t, window.terminal.viewport.View(), "terminal history")
	require.True(t, strings.TrimSpace(window.terminal.viewport.View()) == "")
	require.Equal(t, 0, window.diff.viewport.YOffset)
}
