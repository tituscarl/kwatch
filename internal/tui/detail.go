package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tituscarl/kwatch/internal/k8s"
)

type DetailModel struct {
	content string
	title   string
	offset  int
	lines   int
	width   int
	height  int
}

func NewDetailModel() DetailModel {
	return DetailModel{}
}

func (d *DetailModel) SetSize(w, h int) {
	d.width = w
	d.height = h
}

func (d *DetailModel) ShowPod(pod k8s.PodInfo, metrics map[string]k8s.PodMetrics) {
	d.offset = 0
	d.title = "Pod: " + pod.Name

	// Usage lives in the metrics map, not on PodInfo
	if m, ok := metrics[pod.Namespace+"/"+pod.Name]; ok {
		pod.CPU, pod.Memory = m.CPU, m.Memory
	}

	var b strings.Builder
	b.WriteString(DetailLabelStyle.Render("Name:       ") + pod.Name + "\n")
	b.WriteString(DetailLabelStyle.Render("Namespace:  ") + pod.Namespace + "\n")
	b.WriteString(DetailLabelStyle.Render("Status:     ") + StatusStyle(pod.Status).Render(pod.Status) + "\n")
	b.WriteString(DetailLabelStyle.Render("Ready:      ") + readyStyle(pod.Ready).Render(pod.Ready) + "\n")
	b.WriteString(DetailLabelStyle.Render("Restarts:   ") + restartStyle(pod.Restarts).Render(fmt.Sprintf("%d", pod.Restarts)) + "\n")
	b.WriteString(DetailLabelStyle.Render("Age:        ") + formatAge(pod.Age) + "\n")
	b.WriteString(DetailLabelStyle.Render("Node:       ") + pod.Node + "\n")
	if pod.Deployment != "" {
		b.WriteString(DetailLabelStyle.Render("Deployment: ") + pod.Deployment + "\n")
	}

	b.WriteString(renderProblems(detailIssues(pod, metrics, time.Now()), false))

	// Resource summary
	if pod.CPU != "" || pod.Memory != "" {
		b.WriteString("\n" + DetailLabelStyle.Render("Resources (total):") + "\n")
	}
	if pod.CPU != "" {
		cpuLine := "  CPU:    " + pod.CPU + " used"
		if pod.Resources.CPUReq != "" {
			cpuLine += "  /  " + pod.Resources.CPUReq + " req"
		}
		if pod.Resources.CPULim != "" {
			cpuLine += "  /  " + pod.Resources.CPULim + " lim"
		}
		b.WriteString(cpuLine + "\n")
	}
	if pod.Memory != "" {
		memLine := "  Memory: " + pod.Memory + " used"
		if pod.Resources.MemReq != "" {
			memLine += "  /  " + pod.Resources.MemReq + " req"
		}
		if pod.Resources.MemLim != "" {
			memLine += "  /  " + pod.Resources.MemLim + " lim"
		}
		if pct, ok := podMemPct(pod, metrics); ok {
			memLine += "  " + pctStyle(pct).Render(fmt.Sprintf("(%.0f%% of limit)", pct))
		}
		b.WriteString(memLine + "\n")
	}

	if len(pod.Containers) > 0 {
		b.WriteString("\n" + DetailLabelStyle.Render("Containers:") + "\n")
		for _, c := range pod.Containers {
			readyStr := StyleFailed.Render("not ready")
			if c.Ready {
				readyStr = StyleRunning.Render("ready")
			}
			b.WriteString(fmt.Sprintf("\n  %s\n", DetailLabelStyle.Render(c.Name)))
			b.WriteString(fmt.Sprintf("    Image:    %s\n", c.Image))
			b.WriteString(fmt.Sprintf("    State:    %s\n", StatusStyle(c.State).Render(c.State)))
			b.WriteString(fmt.Sprintf("    Ready:    %s\n", readyStr))
			b.WriteString(fmt.Sprintf("    Restarts: %s\n", restartStyle(c.Restarts).Render(fmt.Sprintf("%d", c.Restarts))))
			if c.CPUReq != "" || c.CPULim != "" {
				b.WriteString(fmt.Sprintf("    CPU:      %s req / %s lim\n", c.CPUReq, c.CPULim))
			}
			if c.MemReq != "" || c.MemLim != "" {
				b.WriteString(fmt.Sprintf("    Memory:   %s req / %s lim\n", c.MemReq, c.MemLim))
			}
			if c.LastTermReason == "OOMKilled" {
				oomMsg := StyleFailed.Render("    *** OOMKilled")
				if when := termWhen(c); when != "" {
					oomMsg += StyleFailed.Render(fmt.Sprintf(" (%s)", when))
				}
				if c.MemLim != "" {
					oomMsg += StyleWarning.Render(fmt.Sprintf(" — limit was %s, consider increasing", c.MemLim))
				}
				b.WriteString(oomMsg + "\n")
			} else if c.LastTermCode != 0 && c.LastTermReason != "" {
				crashMsg := StyleFailed.Render(fmt.Sprintf("    *** Crashed (exit code %d)", c.LastTermCode))
				if when := termWhen(c); when != "" {
					crashMsg += StyleFailed.Render(fmt.Sprintf(" (%s)", when))
				}
				crashMsg += StyleWarning.Render(" — check logs for panic/error details")
				b.WriteString(crashMsg + "\n")
			}
		}
	}

	d.content = b.String()
	d.lines = strings.Count(d.content, "\n") + 1
}

// termWhen combines the relative age and absolute time of a container's last
// termination, e.g. "5m ago, Jul 2 14:33".
func termWhen(c k8s.ContainerInfo) string {
	s := c.LastTermAt
	if !c.LastTermTime.IsZero() {
		if s != "" {
			s += ", "
		}
		s += formatTimestamp(c.LastTermTime)
	}
	return s
}

func (d *DetailModel) ShowDeployment(dep k8s.DeploymentInfo, pods []k8s.PodInfo, metrics map[string]k8s.PodMetrics) {
	d.offset = 0
	d.title = "Deployment: " + dep.Name

	var owned []k8s.PodInfo
	for _, p := range pods {
		if p.Deployment == dep.Name && p.Namespace == dep.Namespace {
			owned = append(owned, p)
		}
	}

	var b strings.Builder
	b.WriteString(DetailLabelStyle.Render("Name:       ") + dep.Name + "\n")
	b.WriteString(DetailLabelStyle.Render("Namespace:  ") + dep.Namespace + "\n")
	b.WriteString(DetailLabelStyle.Render("Ready:      ") + readyStyle(dep.Ready).Render(dep.Ready) + "\n")
	b.WriteString(DetailLabelStyle.Render("Up-to-date: ") + fmt.Sprintf("%d", dep.UpToDate) + "\n")
	b.WriteString(DetailLabelStyle.Render("Available:  ") + fmt.Sprintf("%d", dep.Available) + "\n")
	b.WriteString(DetailLabelStyle.Render("Desired:    ") + fmt.Sprintf("%d", dep.Desired) + "\n")
	b.WriteString(DetailLabelStyle.Render("Strategy:   ") + dep.Strategy + "\n")
	b.WriteString(DetailLabelStyle.Render("Age:        ") + formatAge(dep.Age) + "\n")
	if dep.LastDeploy > 0 {
		b.WriteString(DetailLabelStyle.Render("Deployed:   ") + formatAge(dep.LastDeploy) + " ago\n")
	}

	// The deployment's own state plus anything wrong with its pods
	now := time.Now()
	var issues []issue
	if iss, ok := deploymentIssue(dep, now); ok {
		issues = append(issues, iss)
	}
	for _, p := range owned {
		issues = append(issues, detailIssues(p, metrics, now)...)
	}
	b.WriteString(renderProblems(issues, true))

	if len(dep.Images) > 0 {
		b.WriteString("\n" + DetailLabelStyle.Render("Images:") + "\n")
		for _, img := range dep.Images {
			b.WriteString("  " + img + "\n")
		}
	}

	if len(owned) > 0 {
		b.WriteString("\n" + DetailLabelStyle.Render(fmt.Sprintf("Pods (%d):", len(owned))) + "\n")
		nameW := 4
		for _, p := range owned {
			nameW = max(nameW, len(p.Name))
		}
		dim := lipgloss.NewStyle().Foreground(colorDimText)
		b.WriteString(dim.Render(fmt.Sprintf("  %-*s  %-18s%-7s%-10s%-6s%s", nameW, "NAME", "STATUS", "READY", "RESTARTS", "AGE", "MEM")) + "\n")
		for _, p := range owned {
			mem := dim.Render("-")
			if pct, ok := podMemPct(p, metrics); ok {
				mem = pctStyle(pct).Render(fmt.Sprintf("%.0f%%", pct))
			}
			b.WriteString(fmt.Sprintf("  %-*s  ", nameW, p.Name) +
				StatusStyle(p.Status).Width(18).Render(p.Status) +
				readyStyle(p.Ready).Width(7).Render(p.Ready) +
				restartStyle(p.Restarts).Width(10).Render(fmt.Sprintf("%d", p.Restarts)) +
				lipgloss.NewStyle().Width(6).Render(formatAge(p.Age)) +
				mem + "\n")
		}
	}

	d.content = b.String()
	d.lines = strings.Count(d.content, "\n") + 1
}

// detailIssues is podIssues plus a near-limit memory warning. The overview
// has its own High Memory section, so this only applies to detail views.
func detailIssues(p k8s.PodInfo, metrics map[string]k8s.PodMetrics, now time.Time) []issue {
	issues := podIssues(p, now)
	if pct, ok := podMemPct(p, metrics); ok && pct >= 90 {
		issues = append(issues, issue{"▲", "pod", p.Name, "High memory",
			fmt.Sprintf("%.0f%% of %s limit", pct, p.Resources.MemLim), issueWarning, time.Time{}})
	}
	return issues
}

// renderProblems lists issues at the top of a detail view so the reason to
// care is visible without scrolling. withName prefixes each line with the
// object name, for views that cover several pods.
func renderProblems(issues []issue, withName bool) string {
	var b strings.Builder
	b.WriteString("\n" + DetailLabelStyle.Render("Problems:") + "\n")
	if len(issues) == 0 {
		b.WriteString("  " + StyleRunning.Render("✓ None detected") + "\n")
		return b.String()
	}
	sortIssues(issues)
	dim := lipgloss.NewStyle().Foreground(colorDimText)
	now := time.Now()
	for _, iss := range issues {
		rs := StyleFailed
		if iss.severity != issueCritical {
			rs = StyleWarning
		}
		line := "  " + rs.Render(iss.icon) + " "
		if withName {
			line += lipgloss.NewStyle().Bold(true).Render(iss.name) + "  "
		}
		line += rs.Render(iss.reason)
		if iss.detail != "" {
			line += "  " + dim.Render(iss.detail)
		}
		if !iss.when.IsZero() {
			line += dim.Render("  · " + formatAge(now.Sub(iss.when)) + " ago")
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

// readyStyle colors an "n/m" ready count: green when all ready, red otherwise.
func readyStyle(ready string) lipgloss.Style {
	if n, m, ok := strings.Cut(ready, "/"); ok && n == m {
		return StyleRunning
	}
	return StyleFailed
}

// restartStyle: dim at 0, yellow for a few, red once it looks like a loop.
func restartStyle(n int32) lipgloss.Style {
	switch {
	case n >= 5:
		return StyleFailed
	case n > 0:
		return StyleWarning
	}
	return lipgloss.NewStyle().Foreground(colorDimText)
}

// pctStyle colors a utilization percentage: red at 90+, yellow at 75+.
func pctStyle(pct float64) lipgloss.Style {
	switch {
	case pct >= 90:
		return lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	case pct >= 75:
		return StyleWarning
	}
	return StyleRunning
}

func (d DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		visibleLines := d.height - 6
		switch {
		case key.Matches(msg, Keys.Up):
			if d.offset > 0 {
				d.offset--
			}
		case key.Matches(msg, Keys.Down):
			if d.offset < d.lines-visibleLines {
				d.offset++
			}
		case key.Matches(msg, Keys.PageDown):
			d.offset = min(d.offset+visibleLines, max(0, d.lines-visibleLines))
		case key.Matches(msg, Keys.PageUp):
			d.offset = max(d.offset-visibleLines, 0)
		}
	}
	return d, nil
}

func (d DetailModel) View() string {
	titleBar := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPurple).
		Padding(0, 1).
		Render(d.title + "  " + lipgloss.NewStyle().Foreground(colorDimText).Render("(esc to close)"))

	lines := strings.Split(d.content, "\n")
	visibleLines := d.height - 6
	if visibleLines < 1 {
		visibleLines = 1
	}

	end := min(d.offset+visibleLines, len(lines))
	start := d.offset
	if start > len(lines) {
		start = len(lines)
	}

	visible := strings.Join(lines[start:end], "\n")

	contentBox := DetailBorderStyle.
		Width(d.width - 6).
		Height(visibleLines).
		Render(visible)

	return lipgloss.JoinVertical(lipgloss.Left, "", titleBar, contentBox)
}
