package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/tituscarl/kwatch/internal/k8s"
)

type OverviewModel struct {
	pods         []k8s.PodInfo
	deployments  []k8s.DeploymentInfo
	events       []k8s.EventInfo
	metrics      map[string]k8s.PodMetrics
	metricsAvail bool
	loaded       bool // pods have arrived at least once
	width        int
	height       int
}

func NewOverviewModel(metricsAvail bool) OverviewModel {
	return OverviewModel{metricsAvail: metricsAvail}
}

func (o *OverviewModel) UpdatePods(pods []k8s.PodInfo) {
	o.loaded = true
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
	// An empty namespace used to sit on "Waiting for data..." forever;
	// only show the spinner until the first fetch lands.
	if !o.loaded {
		return loadingView("Loading cluster data...")
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

	// Needs Attention is the point of this tab, so it always renders, shrunk
	// to the rows left (box chrome + title take 6).
	if attention := o.renderAttention(remaining - 7); attention != "" {
		parts = append(parts, "", attention)
		remaining -= lipgloss.Height(attention) + 1
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
	crit, warn := 0, 0
	for _, iss := range o.issues() {
		if iss.severity == issueCritical {
			crit++
		} else {
			warn++
		}
	}
	var totalRestarts int32
	for _, p := range o.pods {
		totalRestarts += p.Restarts
	}

	// The headline takes the color of the worst issue, so the card alone
	// says whether anything is on fire.
	bold := lipgloss.NewStyle().Bold(true)
	var headline string
	switch {
	case crit > 0:
		headline = bold.Foreground(colorRed).Render(fmt.Sprintf("✗ %d critical", crit))
	case warn > 0:
		headline = bold.Foreground(colorYellow).Render(fmt.Sprintf("⚠ %d to watch", warn))
	default:
		headline = bold.Foreground(colorGreen).Render("✓ All healthy")
	}

	lines := []string{CardTitleStyle.Render("HEALTH"), headline, ""}
	if crit > 0 {
		lines = append(lines, StyleFailed.Render("●")+fmt.Sprintf(" Critical     %d", crit))
	}
	if warn > 0 {
		lines = append(lines, StylePending.Render("●")+fmt.Sprintf(" Warning      %d", warn))
	}
	restarts := lipgloss.NewStyle().Foreground(colorDimText)
	if totalRestarts > 0 {
		restarts = restarts.Foreground(colorYellow)
	}
	lines = append(lines, restarts.Render(fmt.Sprintf("↻ Restarts     %d", totalRestarts)))

	return CardStyle.Width(width).Render(strings.Join(lines, "\n"))
}

// --- Attention Section ---

const (
	issueCritical = iota
	issueWarning
)

// Pods sit in Pending/ContainerCreating for a few seconds on every rollout;
// only the ones stuck longer than this are worth flagging.
const pendingGrace = 2 * time.Minute

type issue struct {
	icon     string
	kind     string // "pod" or "deploy"
	name     string
	reason   string
	detail   string
	severity int       // issueCritical or issueWarning
	when     time.Time // when the issue (last) happened, zero if unknown
}

// issues lists everything that needs a human, critical first and most recent
// first within each severity. The Health card and the attention list both
// read from here so their counts always agree.
func (o OverviewModel) issues() []issue {
	var issues []issue
	now := time.Now()
	for _, p := range o.pods {
		issues = append(issues, podIssues(p, now)...)
	}
	for _, d := range o.deployments {
		if iss, ok := deploymentIssue(d, now); ok {
			issues = append(issues, iss)
		}
	}

	// Top restarters (pods with high restarts but currently Running)
	type restarter struct {
		name     string
		restarts int32
		when     time.Time
	}
	var restarters []restarter
	for _, p := range o.pods {
		if p.Status == "Running" && p.Restarts >= 5 {
			restarters = append(restarters, restarter{p.Name, p.Restarts, anyTermTime(p)})
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
				fmt.Sprintf("%d restarts", r.restarts), "running but unstable", issueWarning, r.when})
		}
	}

	sortIssues(issues)
	return issues
}

// sortIssues orders critical before warning, most recent first within each
// band. Issues with no known timestamp sink to the bottom of their band.
func sortIssues(issues []issue) {
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].severity != issues[j].severity {
			return issues[i].severity < issues[j].severity
		}
		return issues[i].when.After(issues[j].when)
	})
}

// podIssues returns what's wrong with a single pod, if anything.
func podIssues(p k8s.PodInfo, now time.Time) []issue {
	var issues []issue
	// Pod creation is the fallback "when" for issues without a termination
	// timestamp — a pod stuck Pending or failing to pull has been a problem
	// since it was created.
	created := now.Add(-p.Age)

	switch p.Status {
	case "CrashLoopBackOff":
		detail := fmt.Sprintf("%d restarts", p.Restarts)
		issues = append(issues, issue{"⟳", "pod", p.Name, "CrashLoopBackOff", detail, issueCritical, anyTermTime(p)})
	case "OOMKilled":
		detail := ""
		if p.Resources.MemLim != "" {
			detail = "limit " + p.Resources.MemLim
		}
		issues = append(issues, issue{"✗", "pod", p.Name, "OOMKilled", detail, issueCritical, termTime(p, true)})
	case "ImagePullBackOff", "ErrImagePull":
		detail := ""
		if len(p.Containers) > 0 {
			detail = p.Containers[0].Image
		}
		issues = append(issues, issue{"✗", "pod", p.Name, p.Status, detail, issueCritical, created})
	case "Error", "Failed":
		t := anyTermTime(p)
		if t.IsZero() {
			t = created
		}
		issues = append(issues, issue{"✗", "pod", p.Name, p.Status, "", issueCritical, t})
	case "Pending", "ContainerCreating":
		if p.Age >= pendingGrace {
			issues = append(issues, issue{"◌", "pod", p.Name, p.Status, "stuck", issueWarning, created})
		}
	}
	// Recovered OOM: running now, but was OOMKilled
	if p.OOMKilled && p.Status == "Running" {
		detail := fmt.Sprintf("%d restarts", p.Restarts)
		if p.Resources.MemLim != "" {
			detail += ", limit " + p.Resources.MemLim
		}
		issues = append(issues, issue{"⚡", "pod", p.Name, "OOMKilled (recovered)", detail, issueWarning, termTime(p, true)})
	}
	// Recovered crash: running now, but had a non-zero exit
	if p.Crashed && p.Status == "Running" {
		detail := fmt.Sprintf("%d restarts", p.Restarts)
		for _, c := range p.Containers {
			if c.LastTermCode != 0 && c.LastTermReason != "OOMKilled" {
				detail = fmt.Sprintf("exit code %d, %d restarts", c.LastTermCode, p.Restarts)
				break
			}
		}
		issues = append(issues, issue{"⚡", "pod", p.Name, "Crashed (recovered)", detail, issueWarning, termTime(p, false)})
	}
	return issues
}

// deploymentIssue reports a deployment that has fewer available replicas
// than desired.
func deploymentIssue(d k8s.DeploymentInfo, now time.Time) (issue, bool) {
	if d.Available >= d.Desired {
		return issue{}, false
	}
	// Under-replication usually starts at the last rollout
	var when time.Time
	if d.LastDeploy > 0 {
		when = now.Add(-d.LastDeploy)
	}
	detail := fmt.Sprintf("%d/%d available", d.Available, d.Desired)
	if d.Available == 0 {
		// Nothing serving at all is an outage, not a degradation
		return issue{"✗", "deploy", d.Name, "Unavailable", detail, issueCritical, when}, true
	}
	return issue{"▾", "deploy", d.Name, "Under-replicated", detail, issueWarning, when}, true
}

// podMemPct returns a pod's memory usage as a percentage of its limit; ok is
// false when there's no limit or no metrics sample.
func podMemPct(p k8s.PodInfo, metrics map[string]k8s.PodMetrics) (pct float64, ok bool) {
	m, found := metrics[p.Namespace+"/"+p.Name]
	if !found || m.MemoryBytes == 0 || p.Resources.MemLimitBytes == 0 {
		return 0, false
	}
	return float64(m.MemoryBytes) / float64(p.Resources.MemLimitBytes) * 100, true
}

// renderAttention draws the issue list as aligned columns (kind, name,
// reason, age, detail), showing at most maxRows rows.
func (o OverviewModel) renderAttention(maxRows int) string {
	issues := o.issues()
	if len(issues) == 0 {
		return ""
	}

	nameW, reasonW := 0, 0
	for _, iss := range issues {
		nameW = max(nameW, lipgloss.Width(iss.name))
		reasonW = max(reasonW, lipgloss.Width(iss.reason))
	}
	nameW = min(nameW, 48)

	critStyle := lipgloss.NewStyle().Foreground(colorRed)
	warnStyle := lipgloss.NewStyle().Foreground(colorYellow)
	nameStyle := lipgloss.NewStyle().Foreground(colorWhite).Bold(true).Width(nameW)
	dimStyle := lipgloss.NewStyle().Foreground(colorDimText)
	tagStyle := dimStyle.Bold(true).Width(7)
	ageStyle := dimStyle.Width(8).Align(lipgloss.Right)

	// Box width minus border (2) and padding (6)
	inner := max(o.width-14, 20)
	limit := max(min(10, maxRows), 2)
	shown := issues
	if len(issues) > limit {
		shown = issues[:limit-1] // leave a row for the "+N more" note
	}

	titleColor := colorYellow
	if issues[0].severity == issueCritical {
		titleColor = colorRed
	}
	lines := []string{lipgloss.NewStyle().Bold(true).Foreground(titleColor).Render("NEEDS ATTENTION"), ""}

	now := time.Now()
	for _, iss := range shown {
		rs := critStyle
		if iss.severity != issueCritical {
			rs = warnStyle
		}
		age := ""
		if !iss.when.IsZero() {
			age = formatAge(now.Sub(iss.when)) + " ago"
		}
		row := "  " + rs.Width(2).Render(iss.icon) + " " +
			tagStyle.Render(iss.kind) +
			nameStyle.Render(ansi.Truncate(iss.name, nameW, "…")) + "  " +
			rs.Width(reasonW).Render(iss.reason) +
			ageStyle.Render(age) + "  " +
			dimStyle.Render(iss.detail)
		lines = append(lines, ansi.Truncate(row, inner, "…"))
	}
	if len(shown) < len(issues) {
		lines = append(lines, dimStyle.Render(fmt.Sprintf("  +%d more...", len(issues)-len(shown))))
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(titleColor).
		Padding(1, 3).
		Width(o.width - 6)

	return boxStyle.Render(strings.Join(lines, "\n"))
}

// termTime returns when the pod's last OOM kill (oom=true) or non-OOM crash
// (oom=false) happened. Picks the most recent matching container, same
// matching rules as the detection in k8s/pods.go. Zero when unknown.
func termTime(p k8s.PodInfo, oom bool) time.Time {
	var t time.Time
	for _, c := range p.Containers {
		isOOM := c.State == "OOMKilled" || c.LastTermReason == "OOMKilled"
		if oom && !isOOM {
			continue
		}
		if !oom && (isOOM || c.LastTermCode == 0 || c.LastTermReason == "") {
			continue
		}
		if c.LastTermTime.After(t) {
			t = c.LastTermTime
		}
	}
	return t
}

// anyTermTime returns the pod's most recent container termination regardless
// of reason. Zero when none is known.
func anyTermTime(p k8s.PodInfo) time.Time {
	var t time.Time
	for _, c := range p.Containers {
		if c.LastTermTime.After(t) {
			t = c.LastTermTime
		}
	}
	return t
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
		if pct, ok := podMemPct(p, o.metrics); ok && pct >= 90 {
			entries = append(entries, highMemEntry{"pod", p.Name, pct, o.metrics[p.Namespace+"/"+p.Name].Memory, p.Resources.MemLim})
		}
	}

	// Check deployments (aggregate pods per deployment)
	byDeploy := aggregatePodMemory(o.pods, o.metrics)
	for _, dep := range o.deployments {
		agg := byDeploy[dep.Namespace+"/"+dep.Name]
		if agg.limitBytes == 0 {
			continue
		}
		pct := float64(agg.usageBytes) / float64(agg.limitBytes) * 100
		if pct >= 90 {
			entries = append(entries, highMemEntry{
				"deploy", dep.Name, pct,
				formatBytesShort(agg.usageBytes),
				formatBytesShort(agg.limitBytes),
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
		pctStr := pctStyle.Render(strings.TrimSuffix(fmt.Sprintf("%.1f", e.pct), ".0") + "%")
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
