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

// logSeverity is a line's cached highlight class, computed once when the line
// is added so rendering doesn't re-scan the text on every frame.
type logSeverity uint8

const (
	sevNormal logSeverity = iota
	sevWarn
	sevError
)

type LogsModel struct {
	podName    string
	namespace  string
	container  string
	lines      []string
	severity   []logSeverity // parallel to lines; cached highlight class
	offset     int
	width      int
	height     int
	err        error
	following  bool
	atBottom   bool // track if user is at the bottom
	cursor     int  // selected line index (into l.lines in normal mode; mirrors matchCursor in filter mode)
	sinceHours int  // time window for fetches; 0 = plain tail

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

	// Mouse drag selection (display-line indices + row-relative columns;
	// anchor -1 = none)
	selAnchor    int
	selAnchorCol int
	selEnd       int
	selEndCol    int
	selDragged   bool
}

// Messages
type LogsUpdatedMsg struct {
	Content string
	Err     error
}

// LogLineMsg and LogStreamEndedMsg carry the epoch of the follow stream that
// produced them, so the app can ignore messages left over from a stream that
// was already cancelled (e.g. after toggling follow off and on quickly).
type LogLineMsg struct {
	epoch int
	Lines []string
}

type LogStreamEndedMsg struct {
	epoch int
	Err   error
}

func NewLogsModel() LogsModel {
	return LogsModel{selAnchor: -1}
}

func (l *LogsModel) Show(podName, namespace, container string) {
	l.podName = podName
	l.namespace = namespace
	l.container = container
	l.multiPod = false
	l.podCount = 0
	l.loading = true
	l.lines = nil
	l.severity = nil
	l.offset = 0
	l.err = nil
	l.following = false
	l.atBottom = true
	l.cursor = 0
	l.filtering = false
	l.filterInput = ""
	l.filterTerm = ""
	l.filterRegex = nil
	l.filterCaseSense = false
	l.matchLines = nil
	l.matchCursor = 0
	l.selAnchor = -1
	l.sinceHours = 0
}

func (l *LogsModel) ShowMultiPod(deploymentName, namespace string, podCount int) {
	l.podName = deploymentName
	l.namespace = namespace
	l.container = ""
	l.multiPod = true
	l.podCount = podCount
	l.loading = true
	l.lines = nil
	l.severity = nil
	l.offset = 0
	l.err = nil
	l.following = false
	l.atBottom = true
	l.cursor = 0
	l.filtering = false
	l.filterInput = ""
	l.filterTerm = ""
	l.filterRegex = nil
	l.filterCaseSense = false
	l.matchLines = nil
	l.matchCursor = 0
	l.selAnchor = -1
	l.sinceHours = 0
}

// SelectedLine returns the currently highlighted log line, its 1-based line
// number within the full buffer, and the pod tag (empty if single-pod).
func (l LogsModel) SelectedLine() (text string, lineNum int, podTag string, ok bool) {
	var idx int
	if l.filterTerm != "" {
		if len(l.matchLines) == 0 || l.matchCursor < 0 || l.matchCursor >= len(l.matchLines) {
			return "", 0, "", false
		}
		idx = l.matchLines[l.matchCursor]
	} else {
		if len(l.lines) == 0 || l.cursor < 0 || l.cursor >= len(l.lines) {
			return "", 0, "", false
		}
		idx = l.cursor
	}
	line := l.lines[idx]
	if l.multiPod && len(line) > 2 && line[0] == '[' {
		if end := strings.Index(line, "] "); end != -1 {
			return line[end+2:], idx + 1, line[1:end], true
		}
	}
	return line, idx + 1, "", true
}

// logChromeRows is how many rows sit above the first log line inside View's
// output: leading blank, title, box border, box padding. logChromeCols is the
// columns left of each row: box border + padding. Keep in sync with View.
const (
	logChromeRows = 4
	logChromeCols = 3
)

// displayLine returns the line shown at display index i (resolves through
// matchLines when a filter is active).
func (l LogsModel) displayLine(i int) string {
	if l.filterTerm != "" {
		return l.lines[l.matchLines[i]]
	}
	return l.lines[i]
}

// gutterCols is the width of the cursor marker + line-number gutter, i.e. the
// row column where log text begins. Keep in sync with View.
func (l LogsModel) gutterCols() int {
	return len(fmt.Sprintf("%d", len(l.lines))) + 3
}

func (l LogsModel) innerWidth() int {
	w := l.width - 12
	if w < 10 {
		w = 10
	}
	return w
}

// selCol converts a screen x into a row-relative column, clamped to the row.
func (l LogsModel) selCol(x int) int {
	return min(max(x-logChromeCols, 0), l.innerWidth()-1)
}

// StartSelect anchors a drag selection at screen x / view-relative y and
// moves the cursor to that line.
func (l *LogsModel) StartSelect(x, y int) {
	l.selAnchor = -1
	l.selDragged = false
	row := y - logChromeRows
	if row < 0 || row >= l.visibleLines() {
		return
	}
	idx := l.offset + row
	if idx >= l.displayLineCount() {
		return
	}
	l.selAnchor = idx
	l.selEnd = idx
	l.selAnchorCol = l.selCol(x)
	l.selEndCol = l.selAnchorCol
	if l.filterTerm != "" {
		l.matchCursor = idx
	} else {
		l.cursor = idx
	}
}

// DragSelect extends the selection to the cell under the mouse, clamped to
// the visible window.
func (l *LogsModel) DragSelect(x, y int) {
	if l.selAnchor == -1 {
		return
	}
	l.selDragged = true
	row := min(max(y-logChromeRows, 0), l.visibleLines()-1)
	l.selEnd = min(l.offset+row, max(l.displayLineCount()-1, 0))
	l.selEndCol = l.selCol(x)
}

// normalizedSel returns the selection endpoints ordered top-left to
// bottom-right.
func (l LogsModel) normalizedSel() (sl, sc, el, ec int) {
	sl, sc, el, ec = l.selAnchor, l.selAnchorCol, l.selEnd, l.selEndCol
	if sl > el || (sl == el && sc > ec) {
		sl, el = el, sl
		sc, ec = ec, sc
	}
	return
}

// EndSelect finishes a drag and returns the selected text: partial first and
// last lines, full lines in between, like a terminal's stream selection. A
// click without movement just moves the cursor and returns ok=false.
func (l *LogsModel) EndSelect() (string, bool) {
	sl, sc, el, ec := l.normalizedSel()
	anchored, dragged := l.selAnchor != -1, l.selDragged
	l.selAnchor = -1
	l.selDragged = false
	if !anchored || !dragged {
		return "", false
	}
	textStart := l.gutterCols()
	sel := make([]string, 0, el-sl+1)
	for i := sl; i <= el; i++ {
		text := l.displayLine(i)
		from, to := 0, ansi.StringWidth(text)
		if i == sl {
			from = max(sc-textStart, 0)
		}
		if i == el {
			to = max(ec-textStart+1, from)
		}
		// Cell offsets → byte offsets (Truncate caps at the string end)
		b1 := len(ansi.Truncate(text, from, ""))
		b2 := len(ansi.Truncate(text, to, ""))
		sel = append(sel, text[b1:b2])
	}
	return strings.Join(sel, "\n"), true
}

func (l LogsModel) inSelection(i int) bool {
	if l.selAnchor == -1 {
		return false
	}
	sl, _, el, _ := l.normalizedSel()
	return i >= sl && i <= el
}

// selRowRange returns the highlighted column span for display row i. First
// and last rows start/stop at the drag columns; rows in between get the full
// width.
func (l LogsModel) selRowRange(i, width int) (int, int) {
	sl, sc, el, ec := l.normalizedSel()
	from, to := 0, width
	if i == sl {
		from = min(max(sc, l.gutterCols()), width)
	}
	if i == el {
		to = min(max(ec+1, from), width)
	}
	return from, to
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
	l.selAnchor = -1
	if l.filterTerm == "" {
		l.matchLines = nil
		l.filterRegex = nil
		return
	}
	// Reuse the compiled regex — it only changes when the term or case
	// sensitivity does, and those paths clear it explicitly.
	if l.filterRegex == nil {
		l.compileFilter()
	}
	l.matchLines = l.matchLines[:0]
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
	// Don't yank lines out from under a drag; the next tick refetches anyway
	if l.selAnchor != -1 && err == nil {
		return
	}
	if err != nil {
		l.err = err
		l.lines = nil
		l.severity = nil
		l.matchLines = nil
		return
	}
	l.err = nil
	l.lines = strings.Split(content, "\n")
	l.severity = l.severity[:0]
	for _, line := range l.lines {
		l.severity = append(l.severity, classify(line))
	}
	l.refilter()
	// Auto-scroll to bottom
	if l.atBottom {
		l.offset = l.maxOffset()
		l.cursor = max(l.displayLineCount()-1, 0)
	} else if l.cursor >= l.displayLineCount() {
		l.cursor = max(l.displayLineCount()-1, 0)
	}
}

func (l *LogsModel) AppendLines(lines []string) {
	base := len(l.lines)
	l.lines = append(l.lines, lines...)
	for _, line := range lines {
		l.severity = append(l.severity, classify(line))
	}
	// Extend the match set with any new lines that match — cheaper than
	// re-running the regex over the whole buffer.
	if l.filterRegex != nil {
		for i, line := range lines {
			if l.filterRegex.MatchString(line) {
				l.matchLines = append(l.matchLines, base+i)
			}
		}
	}
	// Cap total lines to prevent unbounded memory growth.
	trimmed := 0
	shown := l.displayLineCount()
	if len(l.lines) > maxLogLines {
		trimmed = len(l.lines) - maxLogLines
		l.lines = l.lines[trimmed:]
		l.severity = l.severity[trimmed:]
		if l.filterTerm != "" {
			l.shiftMatches(trimmed)
		}
	}
	// Adjust cursor after trim (normal mode only — filter mode uses matchCursor)
	if l.filterTerm == "" && trimmed > 0 {
		l.cursor = max(l.cursor-trimmed, 0)
	}
	selecting := l.selAnchor != -1
	if selecting {
		// Keep the selection (and the rows under the mouse) pinned to the
		// same text while the trim shifts indices up.
		if shift := shown - l.displayLineCount(); shift > 0 {
			l.offset = max(l.offset-shift, 0)
			l.selAnchor -= shift
			l.selEnd = max(l.selEnd-shift, 0)
			if l.selAnchor < 0 {
				l.selAnchor = -1 // anchor line fell out of the buffer
			}
		}
	}
	// Auto-scroll if at bottom, but hold still while the user is dragging
	if l.atBottom && !selecting {
		l.offset = l.maxOffset()
		l.cursor = max(l.displayLineCount()-1, 0)
	} else if l.cursor >= l.displayLineCount() {
		l.cursor = max(l.displayLineCount()-1, 0)
	}
}

// shiftMatches re-bases match indices after the oldest `trimmed` lines were
// dropped, discarding matches that scrolled off the top.
func (l *LogsModel) shiftMatches(trimmed int) {
	dst := 0
	for _, idx := range l.matchLines {
		if ni := idx - trimmed; ni >= 0 {
			l.matchLines[dst] = ni
			dst++
		}
	}
	l.matchLines = l.matchLines[:dst]
	if l.matchCursor >= len(l.matchLines) {
		l.matchCursor = max(0, len(l.matchLines)-1)
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
		l.cursor = max(l.displayLineCount()-1, 0)
	}
}

func (l LogsModel) displayLineCount() int {
	if l.filterTerm != "" {
		return len(l.matchLines)
	}
	return len(l.lines)
}

func (l LogsModel) Update(msg tea.KeyPressMsg) LogsModel {
	// A release outside the terminal never reaches us; any key drops the
	// stale selection so refreshes and auto-scroll resume.
	l.selAnchor = -1
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
				l.filterRegex = nil // force recompile for the new term
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
			l.filterRegex = nil // case prefix changed; recompile
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
		if l.filterTerm != "" {
			if l.matchCursor > 0 {
				l.matchCursor--
				if l.matchCursor < l.offset {
					l.offset = l.matchCursor
				}
				l.atBottom = false
			}
		} else if l.cursor > 0 {
			l.cursor--
			if l.cursor < l.offset {
				l.offset = l.cursor
			}
			l.atBottom = false
		}
	case "down", "j":
		if l.filterTerm != "" {
			if l.matchCursor < len(l.matchLines)-1 {
				l.matchCursor++
				if l.matchCursor >= l.offset+visibleLines {
					l.offset = l.matchCursor - visibleLines + 1
				}
				l.atBottom = l.offset >= l.maxOffset()
			}
		} else if l.cursor < len(l.lines)-1 {
			l.cursor++
			if l.cursor >= l.offset+visibleLines {
				l.offset = l.cursor - visibleLines + 1
			}
			l.atBottom = l.offset >= l.maxOffset()
		}
	case "pgup":
		l.offset = max(l.offset-visibleLines, 0)
		if l.filterTerm != "" {
			l.matchCursor = max(l.matchCursor-visibleLines, 0)
		} else {
			l.cursor = max(l.cursor-visibleLines, 0)
		}
		l.atBottom = false
	case "pgdown":
		l.offset = min(l.offset+visibleLines, l.maxOffset())
		if l.filterTerm != "" {
			l.matchCursor = min(l.matchCursor+visibleLines, max(len(l.matchLines)-1, 0))
		} else {
			l.cursor = min(l.cursor+visibleLines, max(len(l.lines)-1, 0))
		}
		if l.offset >= l.maxOffset() {
			l.atBottom = true
		}
	case "G":
		l.offset = l.maxOffset()
		if l.filterTerm != "" {
			l.matchCursor = max(len(l.matchLines)-1, 0)
		} else {
			l.cursor = max(len(l.lines)-1, 0)
		}
		l.atBottom = true
	case "g":
		l.offset = 0
		if l.filterTerm != "" {
			l.matchCursor = 0
		} else {
			l.cursor = 0
		}
		l.atBottom = false
	case "f":
		l.following = !l.following
		if l.following {
			l.atBottom = true
			l.offset = l.maxOffset()
			l.cursor = max(l.displayLineCount()-1, 0)
		}
	case "t":
		// Cycle the time window; the app refetches when this changes.
		// The follow stream doesn't use it, so leave it alone while following.
		if l.following {
			break
		}
		switch l.sinceHours {
		case 0:
			l.sinceHours = 1
		case 1:
			l.sinceHours = 6
		case 6:
			l.sinceHours = 24
		default:
			l.sinceHours = 0
		}
	}
	return l
}

func (l LogsModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPurple).
		Padding(0, 1)

	window := ""
	if l.sinceHours > 0 && !l.following {
		window = fmt.Sprintf(" · last %dh", l.sinceHours)
	}
	var title string
	if l.multiPod {
		title = titleStyle.Render(fmt.Sprintf("Logs: %s (all %d pods)%s", l.podName, l.podCount, window))
	} else {
		title = titleStyle.Render(fmt.Sprintf("Logs: %s/%s%s", l.podName, l.container, window))
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
		keyStyle.Render("enter") + descStyle.Render(" view line") + sep +
		keyStyle.Render("f") + descStyle.Render(" follow") + sep +
		keyStyle.Render("j/k") + descStyle.Render(" scroll") + sep +
		keyStyle.Render("G") + descStyle.Render(" bottom") + sep +
		keyStyle.Render("g") + descStyle.Render(" top") + sep +
		keyStyle.Render("c") + descStyle.Render(" copy") + sep +
		keyStyle.Render("t") + descStyle.Render(" window") + sep +
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
				msg = fmt.Sprintf("  %s Loading logs from %d pods...", spinnerFrame, l.podCount)
			} else {
				msg = "  " + spinnerFrame + " Loading logs..."
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
	cursorStyle := lipgloss.NewStyle().Foreground(colorPurple).Bold(true)

	// Truncation width: outer box width minus border (2) and padding (4).
	// Long lines would otherwise wrap and push the bottom bar off-screen.
	innerWidth := l.innerWidth()

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
				line := l.lines[origIdx]
				base := severityStyle(l.severity[origIdx], logLineStyle)

				var styled string
				if l.multiPod {
					styled = renderPodLineHighlighted(line, l.filterRegex, base)
				} else {
					styled = highlightMatches(line, l.filterRegex, base)
				}

				// Mark current match with indicator
				var row string
				if i == l.matchCursor {
					row = cursorStyle.Render("▸") + lineNum + " " + styled
				} else {
					row = " " + lineNum + " " + styled
				}
				selFrom, selTo := 0, 0
				if l.inSelection(i) {
					selFrom, selTo = l.selRowRange(i, innerWidth)
				} else if i == l.matchCursor {
					selTo = innerWidth
				}
				b.WriteString(renderLogRow(row, innerWidth, selFrom, selTo) + "\n")
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
			line := l.lines[i]
			base := severityStyle(l.severity[i], logLineStyle)

			var styled string
			if l.multiPod {
				styled = renderPodLine(line, base)
			} else {
				styled = base.Render(line)
			}
			var row string
			if i == l.cursor {
				row = cursorStyle.Render("▸") + lineNum + " " + styled
			} else {
				row = " " + lineNum + " " + styled
			}
			selFrom, selTo := 0, 0
			if l.inSelection(i) {
				selFrom, selTo = l.selRowRange(i, innerWidth)
			} else if i == l.cursor {
				selTo = innerWidth
			}
			b.WriteString(renderLogRow(row, innerWidth, selFrom, selTo) + "\n")
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

// Severity keywords, kept lowercase so a line only has to be lowercased once.
var (
	errorKeywords = []string{"error", "fatal", "panic", "erro", "fata"}
	warnKeywords  = []string{"warn"}
)

// classify determines a line's highlight severity. Error outranks warning, and
// klog-style level prefixes (E/F/W) are checked alongside substring keywords.
func classify(line string) logSeverity {
	lower := strings.ToLower(line)
	if isKlogError(line) || containsAny(lower, errorKeywords) {
		return sevError
	}
	if isKlogWarn(line) || containsAny(lower, warnKeywords) {
		return sevWarn
	}
	return sevNormal
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// paintBg lays a background bar under an already-styled string. The inline
// SGR resets would punch holes in a wrapped background, so the bar is
// re-applied after every reset instead.
func paintBg(s string) string {
	bg := ansi.Style{}.BackgroundColor(colorSubtle).String()
	return bg + strings.ReplaceAll(s, "\x1b[m", "\x1b[m"+bg) + "\x1b[m"
}

// renderLogRow truncates a row to width and paints a background over the
// [selFrom, selTo) column span — pass (0, width) for a full bar, or an empty
// span for none.
func renderLogRow(row string, width, selFrom, selTo int) string {
	row = ansi.Truncate(row, width, "")
	if selFrom >= selTo {
		return row
	}
	if pad := width - ansi.StringWidth(row); pad > 0 {
		row += strings.Repeat(" ", pad)
	}
	if selFrom <= 0 && selTo >= width {
		return paintBg(row)
	}
	return ansi.Cut(row, 0, selFrom) + paintBg(ansi.Cut(row, selFrom, selTo)) + ansi.Cut(row, selTo, width)
}

// severityStyle maps a cached severity to its display style, falling back to
// the supplied base style for normal lines.
func severityStyle(sev logSeverity, base lipgloss.Style) lipgloss.Style {
	switch sev {
	case sevError:
		return StyleFailed
	case sevWarn:
		return StyleWarning
	default:
		return base
	}
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
