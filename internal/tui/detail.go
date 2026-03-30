package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

func (d *DetailModel) ShowPod(pod k8s.PodInfo) {
	d.offset = 0
	d.title = "Pod: " + pod.Name

	var b strings.Builder
	b.WriteString(DetailLabelStyle.Render("Name:       ") + pod.Name + "\n")
	b.WriteString(DetailLabelStyle.Render("Namespace:  ") + pod.Namespace + "\n")
	b.WriteString(DetailLabelStyle.Render("Status:     ") + StatusStyle(pod.Status).Render(pod.Status) + "\n")
	b.WriteString(DetailLabelStyle.Render("Ready:      ") + pod.Ready + "\n")
	b.WriteString(DetailLabelStyle.Render("Restarts:   ") + fmt.Sprintf("%d", pod.Restarts) + "\n")
	b.WriteString(DetailLabelStyle.Render("Age:        ") + formatAge(pod.Age) + "\n")
	b.WriteString(DetailLabelStyle.Render("Node:       ") + pod.Node + "\n")

	// Resource summary
	b.WriteString("\n" + DetailLabelStyle.Render("Resources (total):") + "\n")
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
		b.WriteString(memLine + "\n")
	}

	if len(pod.Containers) > 0 {
		b.WriteString("\n" + DetailLabelStyle.Render("Containers:") + "\n")
		for _, c := range pod.Containers {
			readyStr := "not ready"
			if c.Ready {
				readyStr = StyleRunning.Render("ready")
			} else {
				readyStr = StyleFailed.Render("not ready")
			}
			b.WriteString(fmt.Sprintf("\n  %s\n", DetailLabelStyle.Render(c.Name)))
			b.WriteString(fmt.Sprintf("    Image:    %s\n", c.Image))
			b.WriteString(fmt.Sprintf("    State:    %s\n", c.State))
			b.WriteString(fmt.Sprintf("    Ready:    %s\n", readyStr))
			b.WriteString(fmt.Sprintf("    Restarts: %d\n", c.Restarts))
			if c.CPUReq != "" || c.CPULim != "" {
				b.WriteString(fmt.Sprintf("    CPU:      %s req / %s lim\n", c.CPUReq, c.CPULim))
			}
			if c.MemReq != "" || c.MemLim != "" {
				b.WriteString(fmt.Sprintf("    Memory:   %s req / %s lim\n", c.MemReq, c.MemLim))
			}
			if c.LastTermReason == "OOMKilled" {
				oomMsg := StyleFailed.Render("    *** OOMKilled")
				if c.LastTermAt != "" {
					oomMsg += StyleFailed.Render(fmt.Sprintf(" (%s)", c.LastTermAt))
				}
				if c.MemLim != "" {
					oomMsg += StyleWarning.Render(fmt.Sprintf(" — limit was %s, consider increasing", c.MemLim))
				}
				b.WriteString(oomMsg + "\n")
			} else if c.LastTermCode != 0 && c.LastTermReason != "" {
				crashMsg := StyleFailed.Render(fmt.Sprintf("    *** Crashed (exit code %d)", c.LastTermCode))
				if c.LastTermAt != "" {
					crashMsg += StyleFailed.Render(fmt.Sprintf(" (%s)", c.LastTermAt))
				}
				crashMsg += StyleWarning.Render(" — check logs for panic/error details")
				b.WriteString(crashMsg + "\n")
			}
		}
	}

	d.content = b.String()
	d.lines = strings.Count(d.content, "\n") + 1
}

func (d *DetailModel) ShowDeployment(dep k8s.DeploymentInfo) {
	d.offset = 0
	d.title = "Deployment: " + dep.Name

	var b strings.Builder
	b.WriteString(DetailLabelStyle.Render("Name:       ") + dep.Name + "\n")
	b.WriteString(DetailLabelStyle.Render("Namespace:  ") + dep.Namespace + "\n")
	b.WriteString(DetailLabelStyle.Render("Ready:      ") + dep.Ready + "\n")
	b.WriteString(DetailLabelStyle.Render("Up-to-date: ") + fmt.Sprintf("%d", dep.UpToDate) + "\n")
	b.WriteString(DetailLabelStyle.Render("Available:  ") + fmt.Sprintf("%d", dep.Available) + "\n")
	b.WriteString(DetailLabelStyle.Render("Desired:    ") + fmt.Sprintf("%d", dep.Desired) + "\n")
	b.WriteString(DetailLabelStyle.Render("Strategy:   ") + dep.Strategy + "\n")
	b.WriteString(DetailLabelStyle.Render("Age:        ") + formatAge(dep.Age) + "\n")
	if dep.LastDeploy > 0 {
		b.WriteString(DetailLabelStyle.Render("Deployed:   ") + formatAge(dep.LastDeploy) + " ago\n")
	}

	// Images
	if len(dep.Images) > 0 {
		b.WriteString("\n" + DetailLabelStyle.Render("Images:") + "\n")
		for _, img := range dep.Images {
			b.WriteString("  " + img + "\n")
		}
	}

	// Health status
	b.WriteString("\n" + DetailLabelStyle.Render("Health:") + "\n")
	if dep.Available == dep.Desired && dep.Desired > 0 {
		b.WriteString("  " + StyleRunning.Render("✓ All replicas available") + "\n")
	} else if dep.Available > 0 {
		b.WriteString("  " + StylePending.Render(fmt.Sprintf("⟳ %d/%d replicas available", dep.Available, dep.Desired)) + "\n")
	} else if dep.Desired > 0 {
		b.WriteString("  " + StyleFailed.Render(fmt.Sprintf("✗ 0/%d replicas available", dep.Desired)) + "\n")
	}

	d.content = b.String()
	d.lines = strings.Count(d.content, "\n") + 1
}

func (d DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
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
