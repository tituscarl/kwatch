package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type StatusBarModel struct {
	lastRefresh time.Time
	width       int
	err         error
}

func (s StatusBarModel) View() string {
	keys := []struct {
		key  string
		desc string
	}{
		{"q", "quit"},
		{"1-4", "tabs"},
		{"tab", "switch"},
		{"j/k", "navigate"},
		{"enter", "detail"},
		{"esc", "back"},
		{"/", "filter"},
	}

	var parts string
	for i, k := range keys {
		if i > 0 {
			parts += lipgloss.NewStyle().Foreground(colorSubtle).Render("  │  ")
		}
		parts += StatusBarKeyStyle.Render(k.key) + " " + StatusBarStyle.Render(k.desc)
	}

	right := ""
	if s.err != nil {
		right = ErrorStyle.Render(fmt.Sprintf("Error: %s", s.err))
	} else if !s.lastRefresh.IsZero() {
		right = StatusBarStyle.Render(fmt.Sprintf("refreshed %s ago", formatDurationShort(time.Since(s.lastRefresh))))
	}

	// Calculate spacing
	partsWidth := lipgloss.Width(parts)
	rightWidth := lipgloss.Width(right)
	gap := s.width - partsWidth - rightWidth - 2
	if gap < 1 {
		gap = 1
	}

	spacer := lipgloss.NewStyle().Width(gap).Render("")

	bar := parts + spacer + right

	return lipgloss.NewStyle().
		Width(s.width).
		Background(lipgloss.Color("#1A1A2E")).
		Padding(0, 1).
		Render(bar)
}

func formatDurationShort(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
