package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tituscarl/kwatch/internal/k8s"
)

type OverviewModel struct {
	pods         []k8s.PodInfo
	deployments  []k8s.DeploymentInfo
	events       []k8s.EventInfo
	metrics      map[string]k8s.PodMetrics
	metricsAvail bool
	width        int
	height       int
}

func NewOverviewModel(metricsAvail bool) OverviewModel {
	return OverviewModel{metricsAvail: metricsAvail}
}

func (o *OverviewModel) UpdatePods(pods []k8s.PodInfo) {
	o.pods = pods
}

func (o *OverviewModel) UpdateDeployments(deps []k8s.DeploymentInfo) {
	o.deployments = deps
}

func (o *OverviewModel) UpdateEvents(events []k8s.EventInfo) {
	o.events = events
}

func (o *OverviewModel) UpdateMetrics(m map[string]k8s.PodMetrics) {
	o.metrics = m
}

func (o *OverviewModel) SetSize(w, h int) {
	o.width = w
	o.height = h
}

func (o OverviewModel) View() string {
	if len(o.pods) == 0 && len(o.deployments) == 0 {
		return lipgloss.NewStyle().
			Foreground(colorDimText).
			Padding(2, 4).
			Render("Waiting for data...")
	}

	// Responsive card widths
	cardWidth := (o.width - 14) / 3
	if cardWidth < 24 {
		cardWidth = 24
	}

	podsCard := o.renderPodsCard(cardWidth)
	deploymentsCard := o.renderDeploymentsCard(cardWidth)
	healthCard := o.renderHealthCard(cardWidth)

	cards := lipgloss.JoinHorizontal(lipgloss.Top,
		podsCard,
		"  ",
		deploymentsCard,
		"  ",
		healthCard,
	)

	parts := []string{cards}

	// Only show extra sections if there's enough vertical space
	cardsHeight := lipgloss.Height(cards)
	remaining := o.height - cardsHeight - 4 // padding + tab bar

	// Needs Attention section — only if there are issues and enough space
	if remaining > 6 {
		attention := o.renderAttention()
		if attention != "" {
			parts = append(parts, "", attention)
			remaining -= lipgloss.Height(attention) + 1
		}
	}

	// High Memory section — pods/deployments near memory limit
	if remaining > 6 && o.metricsAvail {
		highMem := o.renderHighMemory()
		if highMem != "" {
			parts = append(parts, "", highMem)
			remaining -= lipgloss.Height(highMem) + 1
		}
	}

	// Recent warning events — only if still have space
	if remaining > 6 {
		warnings := o.renderRecentWarnings()
		if warnings != "" {
			parts = append(parts, "", warnings)
		}
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(
		lipgloss.JoinVertical(lipgloss.Left, parts...))
}

// --- Cards ---

func (o OverviewModel) renderPodsCard(width int) string {
	running, pending, failed, completed := 0, 0, 0, 0
	for _, p := range o.pods {
		switch {
		case p.Status == "Running":
			running++
		case p.Status == "Pending" || p.Status == "ContainerCreating" || p.Status == "PodInitializing":
			pending++
		case p.Status == "Succeeded" || p.Status == "Completed":
			completed++
		case p.Status == "Failed" || p.Status == "CrashLoopBackOff" || p.Status == "Error" ||
			p.Status == "ImagePullBackOff" || p.Status == "OOMKilled" || p.Status == "ErrImagePull":
			failed++
		default:
			pending++
		}
	}

	total := len(o.pods)
	bigNum := lipgloss.NewStyle().Bold(true).Foreground(colorWhite).Render(fmt.Sprintf("%d", total))
	label := lipgloss.NewStyle().Foreground(colorDimText).Render(" pods")

	var lines []string
	lines = append(lines, CardTitleStyle.Render("PODS"))
	lines = append(lines, bigNum+label)
	lines = append(lines, "")

	if total > 0 {
		barWidth := width - 10
		lines = append(lines, renderStatusBar(barWidth, total, running, pending, failed, completed))
		lines = append(lines, "")
	}

	if running > 0 {
		lines = append(lines, StyleRunning.Render("●")+fmt.Sprintf(" Running      %d", running))
	}
	if pending > 0 {
		lines = append(lines, StylePending.Render("●")+fmt.Sprintf(" Pending      %d", pending))
	}
	if failed > 0 {
		lines = append(lines, StyleFailed.Render("●")+fmt.Sprintf(" Failed       %d", failed))
	}
	if completed > 0 {
		lines = append(lines, StyleSucceeded.Render("●")+fmt.Sprintf(" Completed    %d", completed))
	}

	return CardStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (o OverviewModel) renderDeploymentsCard(width int) string {
	var ready, progressing, unavailable int
	for _, d := range o.deployments {
		if d.Available == d.Desired && d.Desired > 0 {
			ready++
		} else if d.Available < d.Desired && d.Available > 0 {
			progressing++
		} else if d.Available == 0 && d.Desired > 0 {
			unavailable++
		}
	}

	total := len(o.deployments)
	bigNum := lipgloss.NewStyle().Bold(true).Foreground(colorWhite).Render(fmt.Sprintf("%d", total))
	label := lipgloss.NewStyle().Foreground(colorDimText).Render(" deployments")

	var lines []string
	lines = append(lines, CardTitleStyle.Render("DEPLOYMENTS"))
	lines = append(lines, bigNum+label)
	lines = append(lines, "")

	if total > 0 {
		barWidth := width - 10
		lines = append(lines, renderStatusBar(barWidth, total, ready, progressing, unavailable, 0))
		lines = append(lines, "")
	}

	if ready > 0 {
		lines = append(lines, StyleRunning.Render("●")+fmt.Sprintf(" Available    %d", ready))
	}
	if progressing > 0 {
		lines = append(lines, StylePending.Render("●")+fmt.Sprintf(" Progressing  %d", progressing))
	}
	if unavailable > 0 {
		lines = append(lines, StyleFailed.Render("●")+fmt.Sprintf(" Unavailable  %d", unavailable))
	}

	return CardStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (o OverviewModel) renderHealthCard(width int) string {
	var lines []string
	lines = append(lines, CardTitleStyle.Render("HEALTH"))
	lines = append(lines, "")

	// Count issues by type
	crashLoop, imagePull, oomKilled, otherFailed := 0, 0, 0, 0
	for _, p := range o.pods {
		switch p.Status {
		case "CrashLoopBackOff":
			crashLoop++
		case "ImagePullBackOff", "ErrImagePull":
			imagePull++
		case "OOMKilled":
			oomKilled++
		case "Error", "Failed":
			otherFailed++
		}
		if p.OOMKilled && p.Status != "OOMKilled" {
			oomKilled++
		}
	}

	issueCount := crashLoop + imagePull + oomKilled + otherFailed
	if issueCount == 0 {
		lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(colorGreen).Render("✓ All healthy"))
	} else {
		lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(colorRed).Render(
			fmt.Sprintf("⚠ %d issue(s)", issueCount)))
	}

	lines = append(lines, "")

	// Total restarts
	var totalRestarts int32
	for _, p := range o.pods {
		totalRestarts += p.Restarts
	}
	if totalRestarts > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(colorYellow).Render(
			fmt.Sprintf("↻ Restarts     %d", totalRestarts)))
	} else {
		lines = append(lines, lipgloss.NewStyle().Foreground(colorDimText).Render("↻ Restarts     0"))
	}

	// Breakdown of issue types
	if crashLoop > 0 {
		lines = append(lines, StyleFailed.Render(fmt.Sprintf("⟳ CrashLoop    %d", crashLoop)))
	}
	if imagePull > 0 {
		lines = append(lines, StyleFailed.Render(fmt.Sprintf("✗ ImagePull    %d", imagePull)))
	}
	if oomKilled > 0 {
		lines = append(lines, StyleFailed.Render(fmt.Sprintf("✗ OOMKilled    %d", oomKilled)))
	}

	return CardStyle.Width(width).Render(strings.Join(lines, "\n"))
}

// --- Attention Section ---

func (o OverviewModel) renderAttention() string {
	type issue struct {
		icon     string
		kind     string // "pod" or "deploy"
		name     string
		reason   string
		detail   string
		severity int // 0=critical, 1=warning
	}

	var issues []issue

	for _, p := range o.pods {
		switch p.Status {
		case "CrashLoopBackOff":
			detail := fmt.Sprintf("%d restarts", p.Restarts)
			issues = append(issues, issue{"⟳", "pod", p.Name, "CrashLoopBackOff", detail, 0})
		case "OOMKilled":
			detail := ""
			if p.Resources.MemLim != "" {
				detail = "limit: " + p.Resources.MemLim
			}
			issues = append(issues, issue{"✗", "pod", p.Name, "OOMKilled", detail, 0})
		case "ImagePullBackOff", "ErrImagePull":
			detail := ""
			if len(p.Containers) > 0 {
				detail = p.Containers[0].Image
			}
			issues = append(issues, issue{"✗", "pod", p.Name, p.Status, detail, 0})
		case "Error", "Failed":
			issues = append(issues, issue{"✗", "pod", p.Name, p.Status, "", 0})
		case "Pending", "ContainerCreating":
			issues = append(issues, issue{"◌", "pod", p.Name, p.Status, "", 1})
		}
		// Detect recovered OOM (running but was OOMKilled)
		if p.OOMKilled && p.Status == "Running" {
			detail := fmt.Sprintf("recovered, %d restarts", p.Restarts)
			if p.Resources.MemLim != "" {
				detail += ", limit: " + p.Resources.MemLim
			}
			issues = append(issues, issue{"⚡", "pod", p.Name, "OOMKilled (recovered)", detail, 1})
		}
	}

	// Under-replicated deployments
	for _, d := range o.deployments {
		if d.Available < d.Desired {
			detail := fmt.Sprintf("%d/%d available", d.Available, d.Desired)
			issues = append(issues, issue{"▾", "deploy", d.Name, "Under-replicated", detail, 1})
		}
	}

	// Top restarters (pods with high restarts but currently Running)
	type restarter struct {
		name     string
		restarts int32
	}
	var restarters []restarter
	for _, p := range o.pods {
		if p.Status == "Running" && p.Restarts >= 5 {
			restarters = append(restarters, restarter{p.Name, p.Restarts})
		}
	}
	sort.Slice(restarters, func(i, j int) bool {
		return restarters[i].restarts > restarters[j].restarts
	})
	for i, r := range restarters {
		if i >= 3 {
			break
		}
		alreadyListed := false
		for _, iss := range issues {
			if iss.name == r.name {
				alreadyListed = true
				break
			}
		}
		if !alreadyListed {
			issues = append(issues, issue{"↻", "pod", r.name,
				fmt.Sprintf("%d restarts", r.restarts), "running but unstable", 1})
		}
	}

	if len(issues) == 0 {
		return ""
	}

	// Sort: critical first, then warning
	sort.SliceStable(issues, func(i, j int) bool {
		return issues[i].severity < issues[j].severity
	})

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorRed)
	nameStyle := lipgloss.NewStyle().Foreground(colorWhite).Bold(true)
	reasonStyle := lipgloss.NewStyle().Foreground(colorRed)
	warnReasonStyle := lipgloss.NewStyle().Foreground(colorYellow)
	detailStyle := lipgloss.NewStyle().Foreground(colorDimText)
	tagStyle := lipgloss.NewStyle().
		Foreground(colorDimText).
		Bold(true)

	var lines []string
	lines = append(lines, titleStyle.Render("NEEDS ATTENTION"))
	lines = append(lines, "")

	for i, iss := range issues {
		if i >= 10 {
			lines = append(lines, detailStyle.Render(fmt.Sprintf("  +%d more...", len(issues)-10)))
			break
		}
		rs := reasonStyle
		if iss.severity > 0 {
			rs = warnReasonStyle
		}
		tag := tagStyle.Render("[pod]")
		if iss.kind == "deploy" {
			tag = tagStyle.Render("[deployment]")
		}
		line := "  " + rs.Render(iss.icon) + " " + tag + " " + nameStyle.Render(iss.name) + "  " + rs.Render(iss.reason)
		if iss.detail != "" {
			line += "  " + detailStyle.Render(iss.detail)
		}
		lines = append(lines, line)
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorSubtle).
		Padding(1, 3).
		Width(o.width - 6)

	return boxStyle.Render(strings.Join(lines, "\n"))
}

// --- High Memory ---

func (o OverviewModel) renderHighMemory() string {
	if o.metrics == nil || len(o.metrics) == 0 {
		return ""
	}

	type highMemEntry struct {
		kind string // "pod" or "deploy"
		name string
		pct  float64
		mem  string // e.g. "450Mi"
		lim  string // e.g. "512Mi"
	}

	var entries []highMemEntry

	// Check individual pods
	for _, p := range o.pods {
		key := p.Namespace + "/" + p.Name
		m, ok := o.metrics[key]
		if !ok || m.Memory == "" || p.Resources.MemLim == "" {
			continue
		}
		usageBytes := parseMemToBytes(m.Memory)
		limitBytes := parseMemToBytes(p.Resources.MemLim)
		if limitBytes == 0 {
			continue
		}
		pct := float64(usageBytes) / float64(limitBytes) * 100
		if pct >= 90 {
			entries = append(entries, highMemEntry{"pod", p.Name, pct, m.Memory, p.Resources.MemLim})
		}
	}

	// Check deployments (aggregate pods per deployment)
	for _, dep := range o.deployments {
		var totalUsageBytes, totalLimitBytes int64
		prefix := dep.Name + "-"
		for _, pod := range o.pods {
			if pod.Namespace != dep.Namespace {
				continue
			}
			if !strings.HasPrefix(pod.Name, prefix) {
				continue
			}
			totalLimitBytes += parseMemToBytes(pod.Resources.MemLim)
			key := pod.Namespace + "/" + pod.Name
			if m, ok := o.metrics[key]; ok {
				totalUsageBytes += parseMemToBytes(m.Memory)
			}
		}
		if totalLimitBytes == 0 {
			continue
		}
		pct := float64(totalUsageBytes) / float64(totalLimitBytes) * 100
		if pct >= 90 {
			entries = append(entries, highMemEntry{
				"deploy", dep.Name, pct,
				formatBytesShort(totalUsageBytes),
				formatBytesShort(totalLimitBytes),
			})
		}
	}

	if len(entries) == 0 {
		return ""
	}

	// Sort by percentage descending
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].pct > entries[j].pct
	})

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorYellow)
	nameStyle := lipgloss.NewStyle().Foreground(colorWhite).Bold(true)
	pctStyle := lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	detailStyle := lipgloss.NewStyle().Foreground(colorDimText)
	tagStyle := lipgloss.NewStyle().Foreground(colorDimText).Bold(true)

	var lines []string
	lines = append(lines, titleStyle.Render("HIGH MEMORY USAGE"))
	lines = append(lines, "")

	for i, e := range entries {
		if i >= 8 {
			lines = append(lines, detailStyle.Render(fmt.Sprintf("  +%d more...", len(entries)-8)))
			break
		}
		tag := tagStyle.Render("[pod]")
		if e.kind == "deploy" {
			tag = tagStyle.Render("[deployment]")
		}
		pctStr := pctStyle.Render(fmt.Sprintf("%.0f%%", e.pct))
		detail := detailStyle.Render(fmt.Sprintf("%s / %s", e.mem, e.lim))
		lines = append(lines, "  "+pctStyle.Render("▲")+" "+tag+" "+nameStyle.Render(e.name)+"  "+pctStr+"  "+detail)
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorSubtle).
		Padding(1, 3).
		Width(o.width - 6)

	return boxStyle.Render(strings.Join(lines, "\n"))
}

// --- Recent Warnings ---

func (o OverviewModel) renderRecentWarnings() string {
	var warnings []k8s.EventInfo
	for _, e := range o.events {
		if e.Type == "Warning" {
			warnings = append(warnings, e)
		}
	}

	if len(warnings) == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorYellow)
	reasonStyle := lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	objStyle := lipgloss.NewStyle().Foreground(colorWhite)
	msgStyle := lipgloss.NewStyle().Foreground(colorDimText)
	ageStyle := lipgloss.NewStyle().Foreground(colorDimText)

	var lines []string
	lines = append(lines, titleStyle.Render("RECENT WARNINGS"))
	lines = append(lines, "")

	shown := 0
	for _, w := range warnings {
		if shown >= 5 {
			lines = append(lines, msgStyle.Render(fmt.Sprintf("  +%d more (see Events tab)", len(warnings)-5)))
			break
		}

		age := formatAge(w.Age)
		countStr := ""
		if w.Count > 1 {
			countStr = fmt.Sprintf(" (x%d)", w.Count)
		}

		lines = append(lines, "  "+reasonStyle.Render(w.Reason)+" "+objStyle.Render(w.Object)+
			ageStyle.Render(" "+age+countStr))

		// Truncate message
		msg := w.Message
		maxMsgLen := o.width - 16
		if maxMsgLen > 80 {
			maxMsgLen = 80
		}
		if len(msg) > maxMsgLen {
			msg = msg[:maxMsgLen-3] + "..."
		}
		lines = append(lines, "    "+msgStyle.Render(msg))

		shown++
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorSubtle).
		Padding(1, 3).
		Width(o.width - 6)

	return boxStyle.Render(strings.Join(lines, "\n"))
}

// --- Helpers ---

func renderStatusBar(width, total, green, yellow, red, gray int) string {
	if total == 0 || width <= 0 {
		return ""
	}

	gw := green * width / total
	yw := yellow * width / total
	rw := red * width / total
	grw := gray * width / total

	if green > 0 && gw == 0 {
		gw = 1
	}
	if yellow > 0 && yw == 0 {
		yw = 1
	}
	if red > 0 && rw == 0 {
		rw = 1
	}
	if gray > 0 && grw == 0 {
		grw = 1
	}

	used := gw + yw + rw + grw
	if used < width {
		diff := width - used
		switch {
		case green >= yellow && green >= red && green >= gray:
			gw += diff
		case yellow >= green && yellow >= red && yellow >= gray:
			yw += diff
		case red >= green && red >= yellow && red >= gray:
			rw += diff
		default:
			grw += diff
		}
	}

	var bar string
	if gw > 0 {
		bar += lipgloss.NewStyle().Background(colorGreen).Foreground(colorGreen).Render(strings.Repeat("█", gw))
	}
	if yw > 0 {
		bar += lipgloss.NewStyle().Background(colorYellow).Foreground(colorYellow).Render(strings.Repeat("█", yw))
	}
	if rw > 0 {
		bar += lipgloss.NewStyle().Background(colorRed).Foreground(colorRed).Render(strings.Repeat("█", rw))
	}
	if grw > 0 {
		bar += lipgloss.NewStyle().Background(colorGray).Foreground(colorGray).Render(strings.Repeat("█", grw))
	}

	return bar
}
