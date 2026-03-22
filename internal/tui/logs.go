package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const maxLogLines = 5000

type LogsModel struct {
	podName   string
	namespace string
	container string
	lines     []string
	offset    int
	width     int
	height    int
	err       error
	following bool
	atBottom  bool // track if user is at the bottom
}

// Messages
type LogsUpdatedMsg struct {
	Content string
	Err     error
}

type LogLineMsg struct {
	Line string
}

type LogStreamEndedMsg struct {
	Err error
}

func NewLogsModel() LogsModel {
	return LogsModel{}
}

func (l *LogsModel) Show(podName, namespace, container string) {
	l.podName = podName
	l.namespace = namespace
	l.container = container
	l.lines = nil
	l.offset = 0
	l.err = nil
	l.following = false
	l.atBottom = true
}

func (l *LogsModel) UpdateLogs(content string, err error) {
	if err != nil {
		l.err = err
		l.lines = nil
		return
	}
	l.err = nil
	l.lines = strings.Split(content, "\n")
	// Auto-scroll to bottom
	if l.atBottom {
		l.offset = l.maxOffset()
	}
}

func (l *LogsModel) AppendLine(line string) {
	l.lines = append(l.lines, line)
	// Cap total lines to prevent unbounded memory growth
	if len(l.lines) > maxLogLines {
		l.lines = l.lines[len(l.lines)-maxLogLines:]
	}
	// Auto-scroll if at bottom
	if l.atBottom {
		l.offset = l.maxOffset()
	}
}

func (l *LogsModel) SetSize(w, h int) {
	l.width = w
	l.height = h
}

func (l *LogsModel) SetFollowing(f bool) {
	l.following = f
	if f {
		l.atBottom = true
		l.offset = l.maxOffset()
	}
}

func (l LogsModel) Update(msg tea.KeyMsg) LogsModel {
	visibleLines := l.visibleLines()
	switch msg.String() {
	case "up", "k":
		if l.offset > 0 {
			l.offset--
			l.atBottom = false
		}
	case "down", "j":
		if l.offset < l.maxOffset() {
			l.offset++
		}
		if l.offset >= l.maxOffset() {
			l.atBottom = true
		}
	case "pgup":
		l.offset = max(l.offset-visibleLines, 0)
		l.atBottom = false
	case "pgdown":
		l.offset = min(l.offset+visibleLines, l.maxOffset())
		if l.offset >= l.maxOffset() {
			l.atBottom = true
		}
	case "G":
		l.offset = l.maxOffset()
		l.atBottom = true
	case "g":
		l.offset = 0
		l.atBottom = false
	case "f":
		l.following = !l.following
		if l.following {
			l.atBottom = true
			l.offset = l.maxOffset()
		}
	}
	return l
}

func (l LogsModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPurple).
		Padding(0, 1)

	title := titleStyle.Render(fmt.Sprintf("Logs: %s/%s", l.podName, l.container))

	// Mode badge
	var modeBadge string
	if l.following {
		modeBadge = lipgloss.NewStyle().
			Background(colorGreen).
			Foreground(lipgloss.Color("#000000")).
			Bold(true).
			Padding(0, 1).
			Render("FOLLOWING")
	} else {
		modeBadge = lipgloss.NewStyle().
			Background(colorDimText).
			Foreground(lipgloss.Color("#000000")).
			Bold(true).
			Padding(0, 1).
			Render("SNAPSHOT")
	}

	// Help bar
	sep := lipgloss.NewStyle().Foreground(colorSubtle).Render(" | ")
	keyStyle := lipgloss.NewStyle().Foreground(colorPurple).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(colorDimText)
	helpBar := keyStyle.Render("esc") + descStyle.Render(" close") + sep +
		keyStyle.Render("f") + descStyle.Render(" toggle follow") + sep +
		keyStyle.Render("j/k") + descStyle.Render(" scroll") + sep +
		keyStyle.Render("G") + descStyle.Render(" bottom") + sep +
		keyStyle.Render("g") + descStyle.Render(" top")

	bottomBar := helpBar + "    " + modeBadge

	if l.err != nil {
		errContent := ErrorStyle.Render(fmt.Sprintf("Error: %s", l.err))
		return lipgloss.JoinVertical(lipgloss.Left, "", title, "", errContent, "", bottomBar)
	}

	if len(l.lines) == 0 {
		noLogs := lipgloss.NewStyle().Foreground(colorDimText).Render("  No logs available")
		return lipgloss.JoinVertical(lipgloss.Left, "", title, "", noLogs, "", bottomBar)
	}

	visibleLines := l.visibleLines()
	end := min(l.offset+visibleLines, len(l.lines))
	start := l.offset

	var b strings.Builder
	lineNumWidth := len(fmt.Sprintf("%d", len(l.lines)))
	lineNumStyle := lipgloss.NewStyle().Foreground(colorDimText).Width(lineNumWidth + 1)
	logLineStyle := lipgloss.NewStyle().Foreground(colorWhite)

	for i := start; i < end; i++ {
		lineNum := lineNumStyle.Render(fmt.Sprintf("%d", i+1))
		line := l.lines[i]
		// Truncate long lines
		maxLineWidth := l.width - lineNumWidth - 6
		if maxLineWidth > 0 && len(line) > maxLineWidth {
			line = line[:maxLineWidth-3] + "..."
		}
		// Highlight warning/error lines
		styled := logLineStyle.Render(line)
		if containsLogLevel(line, "error", "fatal", "panic", "ERRO", "FATA") {
			styled = StyleFailed.Render(line)
		} else if containsLogLevel(line, "warn", "WARN") {
			styled = StyleWarning.Render(line)
		}
		b.WriteString(lineNum + " " + styled + "\n")
	}

	// Scroll indicator
	scrollInfo := lipgloss.NewStyle().Foreground(colorDimText).Render(
		fmt.Sprintf("  line %d-%d of %d", start+1, end, len(l.lines)))

	content := DetailBorderStyle.
		Width(l.width - 6).
		Height(visibleLines).
		Render(b.String())

	return lipgloss.JoinVertical(lipgloss.Left, "", title, content, scrollInfo, bottomBar)
}

func (l LogsModel) visibleLines() int {
	h := l.height - 7 // extra line for help bar
	return max(h, 1)
}

func (l LogsModel) maxOffset() int {
	return max(len(l.lines)-l.visibleLines(), 0)
}

func containsLogLevel(line string, levels ...string) bool {
	lower := strings.ToLower(line)
	for _, level := range levels {
		if strings.Contains(lower, strings.ToLower(level)) {
			return true
		}
	}
	return false
}
