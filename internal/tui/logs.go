package tui

import (
	"fmt"
	"regexp"
	"strings"

	"image/color"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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

	// Multi-pod
	multiPod bool
	podCount int
	loading  bool

	// Grep/filter
	filtering       bool           // user is typing a search query
	filterInput     string         // text being typed
	filterTerm      string         // confirmed filter term
	filterRegex     *regexp.Regexp // compiled regex (nil if invalid or plain text)
	filterCaseSense bool           // true = case-sensitive
	matchLines      []int          // indices of matching lines in l.lines
	matchCursor     int            // current position in matchLines for n/N
}

// Messages
type LogsUpdatedMsg struct {
	Content string
	Err     error
}

type LogLineMsg struct {
	Lines []string
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
	l.multiPod = false
	l.podCount = 0
	l.loading = true
	l.lines = nil
	l.offset = 0
	l.err = nil
	l.following = false
	l.atBottom = true
	l.filtering = false
	l.filterInput = ""
	l.filterTerm = ""
	l.filterRegex = nil
	l.filterCaseSense = false
	l.matchLines = nil
	l.matchCursor = 0
}

func (l *LogsModel) ShowMultiPod(deploymentName, namespace string, podCount int) {
	l.podName = deploymentName
	l.namespace = namespace
	l.container = ""
	l.multiPod = true
	l.podCount = podCount
	l.loading = true
	l.lines = nil
	l.offset = 0
	l.err = nil
	l.following = false
	l.atBottom = true
	l.filtering = false
	l.filterInput = ""
	l.filterTerm = ""
	l.filterRegex = nil
	l.filterCaseSense = false
	l.matchLines = nil
	l.matchCursor = 0
}

// HasActiveFilter returns true if the log viewer has a search filter active or being typed.
func (l LogsModel) HasActiveFilter() bool {
	return l.filtering || l.filterTerm != ""
}

// CancelFilter cancels filter input or clears the active filter.
func (l *LogsModel) CancelFilter() {
	if l.filtering {
		l.filtering = false
		l.filterInput = ""
	} else if l.filterTerm != "" {
		l.filterTerm = ""
		l.filterRegex = nil
		l.matchLines = nil
		l.matchCursor = 0
		l.offset = 0
	}
}

func (l *LogsModel) compileFilter() {
	prefix := "(?i)"
	if l.filterCaseSense {
		prefix = ""
	}
	re, err := regexp.Compile(prefix + l.filterTerm)
	if err != nil {
		// Invalid regex — fall back to escaped literal
		re = regexp.MustCompile(prefix + regexp.QuoteMeta(l.filterTerm))
	}
	l.filterRegex = re
}

func (l *LogsModel) refilter() {
	if l.filterTerm == "" {
		l.matchLines = nil
		l.filterRegex = nil
		return
	}
	l.compileFilter()
	l.matchLines = nil
	for i, line := range l.lines {
		if l.filterRegex.MatchString(line) {
			l.matchLines = append(l.matchLines, i)
		}
	}
	if l.matchCursor >= len(l.matchLines) {
		l.matchCursor = max(0, len(l.matchLines)-1)
	}
}

func (l *LogsModel) UpdateLogs(content string, err error) {
	l.loading = false
	if err != nil {
		l.err = err
		l.lines = nil
		l.matchLines = nil
		return
	}
	l.err = nil
	l.lines = strings.Split(content, "\n")
	l.refilter()
	// Auto-scroll to bottom
	if l.atBottom {
		l.offset = l.maxOffset()
	}
}

func (l *LogsModel) AppendLines(lines []string) {
	l.lines = append(l.lines, lines...)
	// Cap total lines to prevent unbounded memory growth
	if len(l.lines) > maxLogLines {
		l.lines = l.lines[len(l.lines)-maxLogLines:]
		l.refilter() // indices shift after trimming
	} else if l.filterRegex != nil {
		// Check new lines against filter
		base := len(l.lines) - len(lines)
		for i, line := range lines {
			if l.filterRegex.MatchString(line) {
				l.matchLines = append(l.matchLines, base+i)
			}
		}
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

func (l LogsModel) displayLineCount() int {
	if l.filterTerm != "" {
		return len(l.matchLines)
	}
	return len(l.lines)
}

func (l LogsModel) Update(msg tea.KeyPressMsg) LogsModel {
	// Handle filter input mode
	if l.filtering {
		switch {
		case msg.Code == tea.KeyEscape:
			l.filtering = false
			l.filterInput = ""
		case msg.Code == tea.KeyBackspace:
			if len(l.filterInput) > 0 {
				l.filterInput = l.filterInput[:len(l.filterInput)-1]
			}
		case msg.Code == tea.KeyEnter:
			l.filtering = false
			if l.filterInput != "" {
				l.filterTerm = l.filterInput
				l.refilter()
				l.offset = l.maxOffset()
				l.atBottom = true
				if len(l.matchLines) > 0 {
					l.matchCursor = len(l.matchLines) - 1
				}
			}
			l.filterInput = ""
		case msg.Code == tea.KeyTab:
			l.filterCaseSense = !l.filterCaseSense
		case msg.Text != "":
			l.filterInput += msg.Text
		}
		return l
	}

	visibleLines := l.visibleLines()

	switch msg.String() {
	case "/":
		l.filtering = true
		l.filterInput = ""
		return l
	case "i":
		if l.filterTerm != "" {
			l.filterCaseSense = !l.filterCaseSense
			l.refilter()
			return l
		}
	case "n":
		if l.filterTerm != "" && len(l.matchLines) > 0 {
			l.matchCursor = (l.matchCursor + 1) % len(l.matchLines)
			// Bring current match into view
			if l.matchCursor < l.offset {
				l.offset = l.matchCursor
			} else if l.matchCursor >= l.offset+visibleLines {
				l.offset = l.matchCursor - visibleLines + 1
			}
			l.atBottom = l.offset >= l.maxOffset()
			return l
		}
	case "N":
		if l.filterTerm != "" && len(l.matchLines) > 0 {
			l.matchCursor = (l.matchCursor - 1 + len(l.matchLines)) % len(l.matchLines)
			if l.matchCursor < l.offset {
				l.offset = l.matchCursor
			} else if l.matchCursor >= l.offset+visibleLines {
				l.offset = l.matchCursor - visibleLines + 1
			}
			l.atBottom = l.offset >= l.maxOffset()
			return l
		}
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

	var title string
	if l.multiPod {
		title = titleStyle.Render(fmt.Sprintf("Logs: %s (all %d pods)", l.podName, l.podCount))
	} else {
		title = titleStyle.Render(fmt.Sprintf("Logs: %s/%s", l.podName, l.container))
	}

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
		keyStyle.Render("f") + descStyle.Render(" follow") + sep +
		keyStyle.Render("j/k") + descStyle.Render(" scroll") + sep +
		keyStyle.Render("G") + descStyle.Render(" bottom") + sep +
		keyStyle.Render("g") + descStyle.Render(" top") + sep +
		keyStyle.Render("/") + descStyle.Render(" grep")

	if l.filterTerm != "" {
		helpBar += sep + keyStyle.Render("n/N") + descStyle.Render(" next/prev") +
			sep + keyStyle.Render("i") + descStyle.Render(" toggle case")
	}

	// Case sensitivity label
	var caseLabel string
	if l.filterCaseSense {
		caseLabel = lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Render(" case-sensitive")
	} else {
		caseLabel = lipgloss.NewStyle().Foreground(colorDimText).Render(" case-insensitive")
	}

	// Build the bottom bar — replace with grep input when filtering
	var bottomBar string
	if l.filtering {
		bottomBar = lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Render("grep: ") +
			lipgloss.NewStyle().Foreground(colorWhite).Render(l.filterInput+"█") +
			caseLabel +
			descStyle.Render("  (") + keyStyle.Render("tab") + descStyle.Render(" toggle)")
	} else if l.filterTerm != "" {
		grepInfo := lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Render("grep: ") +
			lipgloss.NewStyle().Foreground(colorWhite).Render(l.filterTerm) + caseLabel
		if len(l.matchLines) > 0 {
			grepInfo += lipgloss.NewStyle().Foreground(colorDimText).Render(
				fmt.Sprintf("  [%d/%d matches]", l.matchCursor+1, len(l.matchLines)))
		} else {
			grepInfo += lipgloss.NewStyle().Foreground(colorDimText).Render("  (no matches)")
		}
		bottomBar = grepInfo + "    " + modeBadge
	} else {
		bottomBar = helpBar + "    " + modeBadge
	}

	if l.err != nil {
		errContent := ErrorStyle.Render(fmt.Sprintf("Error: %s", l.err))
		return lipgloss.JoinVertical(lipgloss.Left, "", title, "", errContent, "", bottomBar)
	}

	if len(l.lines) == 0 {
		var msg string
		if l.loading {
			if l.multiPod {
				msg = fmt.Sprintf("  Loading logs from %d pods...", l.podCount)
			} else {
				msg = "  Loading logs..."
			}
		} else {
			msg = "  No logs available"
		}
		noLogs := lipgloss.NewStyle().Foreground(colorDimText).Render(msg)
		return lipgloss.JoinVertical(lipgloss.Left, "", title, "", noLogs, "", bottomBar)
	}

	visibleLines := l.visibleLines()
	var b strings.Builder
	lineNumWidth := len(fmt.Sprintf("%d", len(l.lines)))
	lineNumStyle := lipgloss.NewStyle().Foreground(colorDimText).Width(lineNumWidth + 1)
	logLineStyle := lipgloss.NewStyle().Foreground(colorWhite)

	// Truncation width: outer box width minus border (2) and padding (4).
	// Long lines would otherwise wrap and push the bottom bar off-screen.
	innerWidth := l.width - 12
	if innerWidth < 10 {
		innerWidth = 10
	}

	var scrollInfo string

	if l.filterTerm != "" {
		// Filtered view — show only matching lines
		if len(l.matchLines) == 0 {
			b.WriteString(lipgloss.NewStyle().Foreground(colorDimText).Render("  No matches found"))
		} else {
			end := min(l.offset+visibleLines, len(l.matchLines))
			start := l.offset

			for i := start; i < end; i++ {
				origIdx := l.matchLines[i]
				lineNum := lineNumStyle.Render(fmt.Sprintf("%d", origIdx+1))
				fullLine := l.lines[origIdx]

				// Check severity on full line BEFORE truncation
				isError := containsLogLevel(fullLine, "error", "fatal", "panic", "ERRO", "FATA") || isKlogError(fullLine)
				isWarn := containsLogLevel(fullLine, "warn", "WARN") || isKlogWarn(fullLine)

				line := fullLine

				// Style with match highlighting
				var styled string
				if l.multiPod {
					if isError {
						styled = renderPodLineHighlighted(line, l.filterRegex, StyleFailed)
					} else if isWarn {
						styled = renderPodLineHighlighted(line, l.filterRegex, StyleWarning)
					} else {
						styled = renderPodLineHighlighted(line, l.filterRegex, logLineStyle)
					}
				} else {
					if isError {
						styled = highlightMatches(line, l.filterRegex, StyleFailed)
					} else if isWarn {
						styled = highlightMatches(line, l.filterRegex, StyleWarning)
					} else {
						styled = highlightMatches(line, l.filterRegex, logLineStyle)
					}
				}

				// Mark current match with indicator
				var row string
				if i == l.matchCursor {
					row = lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Render("▸") + lineNum + " " + styled
				} else {
					row = " " + lineNum + " " + styled
				}
				b.WriteString(ansi.Truncate(row, innerWidth, "") + "\n")
			}

			scrollInfo = lipgloss.NewStyle().Foreground(colorDimText).Render(
				fmt.Sprintf("  match %d-%d of %d", start+1, end, len(l.matchLines)))
		}
	} else {
		// Normal view — show all lines
		end := min(l.offset+visibleLines, len(l.lines))
		start := l.offset

		for i := start; i < end; i++ {
			lineNum := lineNumStyle.Render(fmt.Sprintf("%d", i+1))
			fullLine := l.lines[i]

			// Check severity on full line BEFORE truncation
			isError := containsLogLevel(fullLine, "error", "fatal", "panic", "ERRO", "FATA") || isKlogError(fullLine)
			isWarn := containsLogLevel(fullLine, "warn", "WARN") || isKlogWarn(fullLine)

			line := fullLine

			var styled string
			if l.multiPod {
				if isError {
					styled = renderPodLine(line, StyleFailed)
				} else if isWarn {
					styled = renderPodLine(line, StyleWarning)
				} else {
					styled = renderPodLine(line, logLineStyle)
				}
			} else {
				if isError {
					styled = StyleFailed.Render(line)
				} else if isWarn {
					styled = StyleWarning.Render(line)
				} else {
					styled = logLineStyle.Render(line)
				}
			}
			b.WriteString(ansi.Truncate(lineNum+" "+styled, innerWidth, "") + "\n")
		}

		scrollInfo = lipgloss.NewStyle().Foreground(colorDimText).Render(
			fmt.Sprintf("  line %d-%d of %d", start+1, end, len(l.lines)))
	}

	content := DetailBorderStyle.
		Width(l.width - 6).
		Height(visibleLines).
		Render(strings.TrimRight(b.String(), "\n"))

	parts := []string{"", title}
	parts = append(parts, content)
	if scrollInfo != "" {
		parts = append(parts, scrollInfo)
	}
	parts = append(parts, bottomBar)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (l LogsModel) visibleLines() int {
	h := l.height - 8 // 1 spacer + 1 title + 4 border+padding + 1 scrollInfo + 1 bottomBar
	return max(h, 1)
}

func (l LogsModel) maxOffset() int {
	return max(l.displayLineCount()-l.visibleLines(), 0)
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

// isKlogError detects klog E (error) and F (fatal) prefixes like "E0323" or "F0323".
// Also handles multi-pod format: "[pod-id] E0323 ..."
func isKlogError(line string) bool {
	s := line
	// Skip past [pod-id] prefix if present
	if len(s) > 0 && s[0] == '[' {
		if idx := strings.Index(s, "] "); idx != -1 {
			s = s[idx+2:]
		}
	}
	if len(s) < 2 {
		return false
	}
	return (s[0] == 'E' || s[0] == 'F') && s[1] >= '0' && s[1] <= '9'
}

// isKlogWarn detects klog W (warning) prefix like "W0323".
func isKlogWarn(line string) bool {
	s := line
	if len(s) > 0 && s[0] == '[' {
		if idx := strings.Index(s, "] "); idx != -1 {
			s = s[idx+2:]
		}
	}
	if len(s) < 2 {
		return false
	}
	return s[0] == 'W' && s[1] >= '0' && s[1] <= '9'
}

func highlightMatches(line string, re *regexp.Regexp, baseStyle lipgloss.Style) string {
	if re == nil {
		return baseStyle.Render(line)
	}

	highlightStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#FFAA00")).
		Foreground(lipgloss.Color("#000000")).
		Bold(true)

	locs := re.FindAllStringIndex(line, -1)
	if len(locs) == 0 {
		return baseStyle.Render(line)
	}

	var result string
	pos := 0
	for _, loc := range locs {
		if loc[0] > pos {
			result += baseStyle.Render(line[pos:loc[0]])
		}
		result += highlightStyle.Render(line[loc[0]:loc[1]])
		pos = loc[1]
	}
	if pos < len(line) {
		result += baseStyle.Render(line[pos:])
	}
	return result
}

// podColorPalette — distinct colors that work well on dark backgrounds.
var podColorPalette = []color.Color{
	lipgloss.Color("#61AFEF"), // blue
	lipgloss.Color("#E5C07B"), // yellow
	lipgloss.Color("#C678DD"), // purple
	lipgloss.Color("#56B6C2"), // cyan
	lipgloss.Color("#E06C75"), // red
	lipgloss.Color("#98C379"), // green
	lipgloss.Color("#D19A66"), // orange
	lipgloss.Color("#FF6AC1"), // pink
	lipgloss.Color("#7EC8E3"), // light blue
	lipgloss.Color("#C3E88D"), // lime
}

// podTagColor returns a deterministic color for a pod tag based on hash.
func podTagColor(tag string) color.Color {
	var h uint32
	for _, c := range tag {
		h = h*31 + uint32(c)
	}
	return podColorPalette[h%uint32(len(podColorPalette))]
}

// renderPodLine renders a multi-pod log line with a colored [pod-id] prefix.
func renderPodLine(line string, baseStyle lipgloss.Style) string {
	// Parse "[tag] rest" format
	if len(line) < 3 || line[0] != '[' {
		return baseStyle.Render(line)
	}
	end := strings.Index(line, "] ")
	if end == -1 {
		return baseStyle.Render(line)
	}
	tag := line[1:end]
	rest := line[end+2:]

	tagStyle := lipgloss.NewStyle().Foreground(podTagColor(tag)).Bold(true)
	return tagStyle.Render("["+tag+"]") + " " + baseStyle.Render(rest)
}

// renderPodLineHighlighted renders a multi-pod log line with colored prefix and grep highlights.
func renderPodLineHighlighted(line string, re *regexp.Regexp, baseStyle lipgloss.Style) string {
	if len(line) < 3 || line[0] != '[' {
		return highlightMatches(line, re, baseStyle)
	}
	end := strings.Index(line, "] ")
	if end == -1 {
		return highlightMatches(line, re, baseStyle)
	}
	tag := line[1:end]
	rest := line[end+2:]

	tagStyle := lipgloss.NewStyle().Foreground(podTagColor(tag)).Bold(true)
	return tagStyle.Render("["+tag+"]") + " " + highlightMatches(rest, re, baseStyle)
}
