package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tituscarl/kwatch/internal/k8s"
)

type PodsModel struct {
	pods         []k8s.PodInfo
	metrics      map[string]k8s.PodMetrics
	cursor       int
	offset       int
	width        int
	height       int
	allNS        bool
	metricsAvail bool
	filter       string
	filtering    bool
	sortCol      int  // index into sortable column names
	sortAsc      bool // true = ascending
}

func NewPodsModel(allNS bool, metricsAvail bool) PodsModel {
	return PodsModel{
		allNS:        allNS,
		metricsAvail: metricsAvail,
	}
}

func (p *PodsModel) UpdatePods(pods []k8s.PodInfo) {
	p.pods = pods
	if p.cursor >= len(pods) {
		p.cursor = max(0, len(pods)-1)
	}
}

func (p *PodsModel) UpdateMetrics(m map[string]k8s.PodMetrics) {
	p.metrics = m
}

func (p *PodsModel) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p PodsModel) SelectedPod() (k8s.PodInfo, bool) {
	filtered := p.filteredPods()
	if p.cursor < len(filtered) {
		return filtered[p.cursor], true
	}
	return k8s.PodInfo{}, false
}

func (p *PodsModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if p.filtering {
			switch {
			case key.Matches(msg, Keys.Escape):
				p.filtering = false
				p.filter = ""
				return nil
			case msg.Code == tea.KeyBackspace:
				if len(p.filter) > 0 {
					p.filter = p.filter[:len(p.filter)-1]
				}
				return nil
			case msg.Code == tea.KeyEnter:
				p.filtering = false
				return nil
			case msg.Code == tea.KeyUp, msg.Code == tea.KeyDown,
				msg.Code == tea.KeyPgUp, msg.Code == tea.KeyPgDown:
				// Allow arrow keys to navigate while filtering
			case msg.Text != "":
				p.filter += msg.Text
				p.cursor = 0
				p.offset = 0
				return nil
			}
		}

		p.handleNav(msg)
	}
	return nil
}

func (p *PodsModel) handleNav(msg tea.KeyPressMsg) {
	filtered := p.filteredPods()

	switch msg.String() {
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
			if p.cursor < p.offset {
				p.offset = p.cursor
			}
		}
	case "down", "j":
		if p.cursor < len(filtered)-1 {
			p.cursor++
			visibleRows := p.visibleRows()
			if p.cursor >= p.offset+visibleRows {
				p.offset = p.cursor - visibleRows + 1
			}
		}
	case "esc":
		if p.filter != "" {
			p.filter = ""
			p.cursor = 0
			p.offset = 0
		}
	case "/":
		p.filtering = true
		p.filter = ""
		p.cursor = 0
		p.offset = 0
	case "pgdown":
		p.cursor = min(p.cursor+p.visibleRows(), len(filtered)-1)
		if p.cursor >= p.offset+p.visibleRows() {
			p.offset = p.cursor - p.visibleRows() + 1
		}
	case "pgup":
		p.cursor = max(p.cursor-p.visibleRows(), 0)
		if p.cursor < p.offset {
			p.offset = p.cursor
		}
	case "s":
		cols := p.sortableColumns()
		p.sortCol = (p.sortCol + 1) % len(cols)
		p.sortAsc = true
		p.cursor = 0
		p.offset = 0
	case "S":
		p.sortAsc = !p.sortAsc
		p.cursor = 0
		p.offset = 0
	}
}

func (p PodsModel) View() string {
	filtered := p.filteredPods()

	if len(p.pods) == 0 {
		return lipgloss.NewStyle().
			Foreground(colorDimText).
			Padding(2, 4).
			Render("No pods found")
	}

	var b strings.Builder

	// Filter indicator
	if p.filtering {
		b.WriteString(lipgloss.NewStyle().Foreground(colorPurple).Render("Filter: "+p.filter+"█") + "\n")
	} else if p.filter != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(colorDimText).Render(fmt.Sprintf("Filter: %s (%d results)  ", p.filter, len(filtered))) + "\n")
	}

	// Header
	header := p.renderHeader()
	b.WriteString(header + "\n")

	// Rows
	visibleRows := p.visibleRows()
	end := min(p.offset+visibleRows, len(filtered))
	for i := p.offset; i < end; i++ {
		pod := filtered[i]
		row := p.renderRow(pod, i == p.cursor)
		b.WriteString(row + "\n")
	}

	// Scroll indicator
	if len(filtered) > visibleRows {
		b.WriteString(lipgloss.NewStyle().Foreground(colorDimText).Render(
			fmt.Sprintf("  showing %d-%d of %d", p.offset+1, end, len(filtered))))
	}

	return lipgloss.NewStyle().Padding(0, 1).Render(b.String())
}

func (p PodsModel) renderHeader() string {
	cols := p.columns()
	activeSort := p.activeSortCol()
	var parts []string
	for _, col := range cols {
		label := col.name
		if col.name == activeSort {
			if p.sortAsc {
				label += " ▲"
			} else {
				label += " ▼"
			}
		}
		parts = append(parts, TableHeaderStyle.Width(col.width).Render(label))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (p PodsModel) renderRow(pod k8s.PodInfo, selected bool) string {
	cols := p.columns()
	values := p.rowValues(pod)

	selectedStyle := lipgloss.NewStyle().
		Background(colorPurple).
		Foreground(lipgloss.Color("#000000")).
		Bold(true)

	var parts []string
	for i, col := range cols {
		val := values[i]

		if selected {
			// Show OOM/crash warning even in selected row
			if col.name == "STATUS" {
				if pod.OOMKilled && pod.Status != "OOMKilled" {
					val = val + " OOM!"
				} else if pod.Crashed && pod.Status != "Error" && pod.Status != "Failed" {
					val = val + " CRASH!"
				}
			}
			parts = append(parts, selectedStyle.Width(col.width).Padding(0, 1).Render(val))
		} else {
			style := TableCellStyle.Width(col.width)
			if col.name == "STATUS" {
				if pod.OOMKilled && pod.Status != "OOMKilled" {
					// Pod recovered but was OOMKilled before
					val = val + StyleFailed.Render(" OOM!")
				} else if pod.Crashed && pod.Status != "Error" && pod.Status != "Failed" {
					// Pod recovered but had a crash before
					val = val + StyleWarning.Render(" CRASH!")
				}
				style = style.Inherit(StatusStyle(pod.Status))
			} else if col.name == "MEM%" {
				style = style.Inherit(memPctStyle(val))
			}
			parts = append(parts, style.Render(val))
		}
	}

	if selected {
		return selectedStyle.Render("> ") + lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	}
	return "  " + lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

type column struct {
	name  string
	width int
}

func (p PodsModel) columns() []column {
	cols := []column{}

	if p.allNS {
		cols = append(cols, column{"NAMESPACE", 16})
	}

	metricsWidth := 0
	if p.metricsAvail {
		metricsWidth = 36 // CPU(9) + MEM(9) + MEM LIM(9) + MEM%(9)
	}

	nameWidth := p.width - 60 - metricsWidth
	if p.allNS {
		nameWidth -= 16
	}
	if nameWidth < 20 {
		nameWidth = 20
	}

	cols = append(cols,
		column{"NAME", nameWidth},
		column{"STATUS", 20},
		column{"READY", 8},
		column{"RESTARTS", 10},
		column{"AGE", 10},
	)

	if p.metricsAvail {
		cols = append(cols,
			column{"CPU", 9},
			column{"MEMORY", 9},
			column{"MEM LIM", 9},
			column{"MEM%", 9},
		)
	}

	return cols
}

func (p PodsModel) rowValues(pod k8s.PodInfo) []string {
	vals := []string{}
	if p.allNS {
		vals = append(vals, truncate(pod.Namespace, 14))
	}

	cpu := ""
	mem := ""
	if p.metricsAvail && p.metrics != nil {
		key := pod.Namespace + "/" + pod.Name
		if m, ok := p.metrics[key]; ok {
			cpu = m.CPU
			mem = m.Memory
		}
	}

	vals = append(vals,
		truncate(pod.Name, p.columns()[len(vals)].width-2),
		pod.Status,
		pod.Ready,
		fmt.Sprintf("%d", pod.Restarts),
		formatAge(pod.Age),
	)

	if p.metricsAvail {
		memLim := pod.Resources.MemLim
		memPct := ""
		if mem != "" && memLim != "" {
			memPct = calcMemPct(mem, memLim)
		}
		vals = append(vals, cpu, mem, memLim, memPct)
	}

	return vals
}

func (p PodsModel) sortableColumns() []string {
	return []string{"NAME", "STATUS", "RESTARTS", "AGE"}
}

func (p PodsModel) activeSortCol() string {
	cols := p.sortableColumns()
	if p.sortCol < len(cols) {
		return cols[p.sortCol]
	}
	return ""
}

func (p PodsModel) filteredPods() []k8s.PodInfo {
	var result []k8s.PodInfo
	if p.filter == "" {
		result = make([]k8s.PodInfo, len(p.pods))
		copy(result, p.pods)
	} else {
		filter := strings.ToLower(p.filter)
		for _, pod := range p.pods {
			if strings.Contains(strings.ToLower(pod.Name), filter) ||
				strings.Contains(strings.ToLower(pod.Status), filter) ||
				strings.Contains(strings.ToLower(pod.Namespace), filter) {
				result = append(result, pod)
			}
		}
	}

	// Apply sort
	col := p.activeSortCol()
	if col != "" {
		sort.SliceStable(result, func(i, j int) bool {
			var less bool
			switch col {
			case "NAME":
				less = result[i].Name < result[j].Name
			case "STATUS":
				less = result[i].Status < result[j].Status
			case "RESTARTS":
				less = result[i].Restarts < result[j].Restarts
			case "AGE":
				less = result[i].Age < result[j].Age
			default:
				return false
			}
			if !p.sortAsc {
				return !less
			}
			return less
		})
	}

	return result
}

func (p PodsModel) visibleRows() int {
	h := p.height - 4 // header + padding + scroll indicator
	if p.filter != "" || p.filtering {
		h--
	}
	if h < 1 {
		h = 1
	}
	return h
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-2] + ".."
}

func memPctStyle(val string) lipgloss.Style {
	var pct float64
	fmt.Sscanf(val, "%f%%", &pct)
	switch {
	case pct >= 90:
		return StyleFailed
	case pct >= 70:
		return StyleWarning
	case pct > 0:
		return StyleRunning
	default:
		return StyleUnknown
	}
}

func calcMemPct(usage, limit string) string {
	usageBytes := parseMemToBytes(usage)
	limitBytes := parseMemToBytes(limit)
	if limitBytes == 0 {
		return ""
	}
	pct := float64(usageBytes) / float64(limitBytes) * 100
	return fmt.Sprintf("%.0f%%", pct)
}

func parseMemToBytes(s string) int64 {
	var val int64
	switch {
	case len(s) > 2 && s[len(s)-2:] == "Gi":
		fmt.Sscanf(s, "%dGi", &val)
		return val * 1024 * 1024 * 1024
	case len(s) > 2 && s[len(s)-2:] == "Mi":
		fmt.Sscanf(s, "%dMi", &val)
		return val * 1024 * 1024
	case len(s) > 2 && s[len(s)-2:] == "Ki":
		fmt.Sscanf(s, "%dKi", &val)
		return val * 1024
	case len(s) > 1 && s[len(s)-1:] == "B":
		fmt.Sscanf(s, "%dB", &val)
		return val
	default:
		return 0
	}
}

func formatAge(d interface{ Hours() float64 }) string {
	type durationer interface {
		Hours() float64
		Minutes() float64
		Seconds() float64
	}
	dur, ok := d.(durationer)
	if !ok {
		return "?"
	}
	hours := dur.Hours()
	switch {
	case hours >= 24*365:
		return fmt.Sprintf("%dy", int(hours/(24*365)))
	case hours >= 24:
		return fmt.Sprintf("%dd", int(hours/24))
	case hours >= 1:
		return fmt.Sprintf("%dh", int(hours))
	case dur.Minutes() >= 1:
		return fmt.Sprintf("%dm", int(dur.Minutes()))
	default:
		return fmt.Sprintf("%ds", int(dur.Seconds()))
	}
}
