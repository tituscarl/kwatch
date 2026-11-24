package tui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Drag selection: x/y→cell mapping through the view chrome, clamping at the
// box edges, reverse drags, partial-line spans, and click-vs-drag copy.
func TestLogsDragSelect(t *testing.T) {
	l := NewLogsModel()
	l.SetSize(80, 20) // visibleLines = 12
	var lines []string
	for i := 0; i < 30; i++ {
		lines = append(lines, fmt.Sprintf("line-%d", i))
	}
	l.lines = lines
	l.severity = make([]logSeverity, len(lines))
	l.offset = 5

	// lineNumWidth=2 → text begins at row col 5, screen x 8
	textX := logChromeCols + l.gutterCols()
	if textX != 8 {
		t.Fatalf("textX = %d, want 8", textX)
	}

	// Click on the first visible row lands on line index 5 and moves the cursor
	l.StartSelect(textX, logChromeRows)
	if l.selAnchor != 5 || l.cursor != 5 {
		t.Fatalf("anchor=%d cursor=%d, want 5", l.selAnchor, l.cursor)
	}

	// Plain click (no motion) copies nothing
	if _, ok := l.EndSelect(); ok {
		t.Fatal("click without drag should not copy")
	}

	// Drag from the text start down two rows to the right edge
	l.StartSelect(textX, logChromeRows)
	l.DragSelect(79, logChromeRows+2)
	text, ok := l.EndSelect()
	if !ok || text != "line-5\nline-6\nline-7" {
		t.Fatalf("got %q ok=%v", text, ok)
	}

	// Drag within one line grabs just that span ("ne-" of "line-5")
	l.StartSelect(textX+2, logChromeRows)
	l.DragSelect(textX+4, logChromeRows)
	text, ok = l.EndSelect()
	if !ok || text != "ne-" {
		t.Fatalf("partial got %q ok=%v", text, ok)
	}

	// Reverse drag past the top-left clamps to the first visible cell; the
	// last (anchor) line keeps only up to the drag start column
	l.StartSelect(textX, logChromeRows+3)
	l.DragSelect(0, 0)
	text, _ = l.EndSelect()
	if text != "line-5\nline-6\nline-7\nl" {
		t.Fatalf("reverse drag got %q", text)
	}

	// Click outside the log rows selects nothing
	l.StartSelect(textX, 1)
	l.DragSelect(textX, logChromeRows+2)
	if _, ok := l.EndSelect(); ok {
		t.Fatal("click on chrome should not start a selection")
	}
}

// A refresh or streamed batch mid-drag must not cancel the selection, and a
// trim must keep it on the same text.
func TestLogsSelectSurvivesUpdates(t *testing.T) {
	l := NewLogsModel()
	l.SetSize(80, 20)
	for i := 0; i < maxLogLines; i++ {
		l.lines = append(l.lines, fmt.Sprintf("line-%d", i))
	}
	l.severity = make([]logSeverity, len(l.lines))
	l.offset = 100
	textX := logChromeCols + l.gutterCols()

	l.StartSelect(textX, logChromeRows) // line-100
	l.UpdateLogs("replaced", nil)
	if l.selAnchor == -1 || l.lines[0] != "line-0" {
		t.Fatal("snapshot refresh clobbered an in-progress drag")
	}

	l.following, l.atBottom = true, true
	l.AppendLines([]string{"new-a", "new-b"}) // trims line-0, line-1
	l.DragSelect(79, logChromeRows+1)
	text, ok := l.EndSelect()
	if !ok || text != "line-100\nline-101" {
		t.Fatalf("got %q ok=%v", text, ok)
	}
}

func TestLogsTimeWindowIgnoredWhileFollowing(t *testing.T) {
	l := NewLogsModel()
	l.following = true
	l = l.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	if l.sinceHours != 0 {
		t.Fatalf("sinceHours = %d while following, want 0", l.sinceHours)
	}
}
