package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tituscarl/kwatch/internal/k8s"
)

type OverviewModel struct {
	pods        []k8s.PodInfo
	deployments []k8s.DeploymentInfo
	width       int
	height      int
}

func NewOverviewModel() OverviewModel {
	return OverviewModel{}
}

func (o *OverviewModel) UpdatePods(pods []k8s.PodInfo) {
	o.pods = pods
}

func (o *OverviewModel) UpdateDeployments(deps []k8s.DeploymentInfo) {
	o.deployments = deps
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

	podsCard := o.renderPodsCard()
	deploymentsCard := o.renderDeploymentsCard()
	healthCard := o.renderHealthCard()

	cards := lipgloss.JoinHorizontal(lipgloss.Top,
		podsCard,
		"  ",
		deploymentsCard,
		"  ",
		healthCard,
	)

	// OOMKilled alert banner
	oomEvents := k8s.GetOOMEvents(o.pods)
	if len(oomEvents) > 0 {
		oomCard := o.renderOOMCard(oomEvents)
		return lipgloss.NewStyle().Padding(1, 2).Render(
			lipgloss.JoinVertical(lipgloss.Left, cards, "", oomCard))
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(cards)
}

func (o OverviewModel) renderPodsCard() string {
	counts := map[string]int{}
	for _, p := range o.pods {
		counts[p.Status]++
	}

	var lines []string
	lines = append(lines, CardTitleStyle.Render("PODS"))
	lines = append(lines, fmt.Sprintf("Total: %d", len(o.pods)))
	lines = append(lines, "")

	if c := counts["Running"]; c > 0 {
		lines = append(lines, StyleRunning.Render(fmt.Sprintf("● Running      %d", c)))
	}
	if c := counts["Pending"] + counts["ContainerCreating"] + counts["PodInitializing"]; c > 0 {
		lines = append(lines, StylePending.Render(fmt.Sprintf("● Pending      %d", c)))
	}
	failedCount := 0
	for status, c := range counts {
		if status == "Failed" || status == "CrashLoopBackOff" || status == "Error" || status == "ImagePullBackOff" || status == "OOMKilled" {
			failedCount += c
		}
	}
	if failedCount > 0 {
		lines = append(lines, StyleFailed.Render(fmt.Sprintf("● Failed       %d", failedCount)))
	}
	if c := counts["Succeeded"] + counts["Completed"]; c > 0 {
		lines = append(lines, StyleSucceeded.Render(fmt.Sprintf("● Completed    %d", c)))
	}

	content := strings.Join(lines, "\n")
	return CardStyle.Render(content)
}

func (o OverviewModel) renderDeploymentsCard() string {
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

	var lines []string
	lines = append(lines, CardTitleStyle.Render("DEPLOYMENTS"))
	lines = append(lines, fmt.Sprintf("Total: %d", len(o.deployments)))
	lines = append(lines, "")

	if ready > 0 {
		lines = append(lines, StyleRunning.Render(fmt.Sprintf("● Available    %d", ready)))
	}
	if progressing > 0 {
		lines = append(lines, StylePending.Render(fmt.Sprintf("● Progressing  %d", progressing)))
	}
	if unavailable > 0 {
		lines = append(lines, StyleFailed.Render(fmt.Sprintf("● Unavailable  %d", unavailable)))
	}

	content := strings.Join(lines, "\n")
	return CardStyle.Render(content)
}

func (o OverviewModel) renderHealthCard() string {
	var lines []string
	lines = append(lines, CardTitleStyle.Render("HEALTH"))
	lines = append(lines, "")

	// Count issues
	var issues []string
	for _, p := range o.pods {
		switch p.Status {
		case "CrashLoopBackOff", "ImagePullBackOff", "Error", "OOMKilled", "Failed":
			issues = append(issues, fmt.Sprintf("%s: %s", p.Name, p.Status))
		}
	}
	for _, d := range o.deployments {
		if d.Available < d.Desired {
			issues = append(issues, fmt.Sprintf("%s: %d/%d ready", d.Name, d.Available, d.Desired))
		}
	}

	if len(issues) == 0 {
		lines = append(lines, StyleRunning.Render("✓ All systems healthy"))
	} else {
		lines = append(lines, StyleFailed.Render(fmt.Sprintf("⚠ %d issue(s)", len(issues))))
		lines = append(lines, "")
		for i, issue := range issues {
			if i >= 5 {
				lines = append(lines, StyleWarning.Render(fmt.Sprintf("  +%d more...", len(issues)-5)))
				break
			}
			lines = append(lines, StyleFailed.Render("  • "+issue))
		}
	}

	content := strings.Join(lines, "\n")
	return CardStyle.Render(content)
}

func (o OverviewModel) renderOOMCard(events []k8s.OOMEvent) string {
	oomStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorRed).
		Padding(1, 3).
		Width(o.width - 6)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorRed)

	var lines []string
	lines = append(lines, titleStyle.Render(fmt.Sprintf("OOMKilled: %d container(s) ran out of memory", len(events))))
	lines = append(lines, "")

	for i, e := range events {
		if i >= 8 {
			lines = append(lines, StyleWarning.Render(fmt.Sprintf("  +%d more...", len(events)-8)))
			break
		}
		detail := StyleFailed.Render(fmt.Sprintf("  %s", e.PodName))
		info := fmt.Sprintf("  container: %s", e.ContainerName)
		if e.MemLim != "" {
			info += fmt.Sprintf("  limit: %s", e.MemLim)
		}
		if e.Ago != "" {
			info += fmt.Sprintf("  (%s)", e.Ago)
		}
		if e.Restarts > 0 {
			info += fmt.Sprintf("  restarts: %d", e.Restarts)
		}
		lines = append(lines, detail)
		lines = append(lines, lipgloss.NewStyle().Foreground(colorDimText).Render(info))
	}

	return oomStyle.Render(strings.Join(lines, "\n"))
}
