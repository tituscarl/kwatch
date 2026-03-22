package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tituscarl/kwatch/internal/k8s"
)

type PodPickerModel struct {
	pods           []k8s.PodInfo
	deploymentName string
	cursor         int
	offset         int
	width          int
	height         int
	visible        bool
	loading        bool
}

type DeploymentPodsMsg struct {
	Pods []k8s.PodInfo
	Err  error
}

func NewPodPickerModel() PodPickerModel {
	return PodPickerModel{}
}

func (p *PodPickerModel) Show(deploymentName string) {
	p.deploymentName = deploymentName
	p.pods = nil
	p.cursor = 0
	p.offset = 0
	p.visible = true
	p.loading = true
}

func (p *PodPickerModel) Hide() {
	p.visible = false
}

func (p *PodPickerModel) UpdatePods(pods []k8s.PodInfo) {
	p.pods = pods
	p.loading = false
	// +1 for the "All Pods" row at index 0
	total := len(pods) + 1
	if p.cursor >= total {
		p.cursor = max(0, total-1)
	}
}

// IsAllPodsSelected returns true if the "All Pods" option is selected (cursor == 0).
func (p PodPickerModel) IsAllPodsSelected() bool {
	return p.cursor == 0
}

// AllPods returns all pods in the picker.
func (p PodPickerModel) AllPods() []k8s.PodInfo {
	return p.pods
}

func (p *PodPickerModel) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p PodPickerModel) SelectedPod() (k8s.PodInfo, bool) {
	// cursor 0 is "All Pods", real pods start at index 1
	idx := p.cursor - 1
	if idx >= 0 && idx < len(p.pods) {
		return p.pods[idx], true
	}
	return k8s.PodInfo{}, false
}

func (p *PodPickerModel) Update(msg tea.KeyMsg) {
	total := len(p.pods) + 1 // +1 for "All Pods" at cursor 0
	switch msg.String() {
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
			// offset only applies to pod list (cursor >= 1)
			if p.cursor >= 1 {
				podIdx := p.cursor - 1
				if podIdx < p.offset {
					p.offset = podIdx
				}
			}
		}
	case "down", "j":
		if p.cursor < total-1 {
			p.cursor++
			if p.cursor >= 1 {
				podIdx := p.cursor - 1
				vis := p.visibleRows()
				if podIdx >= p.offset+vis {
					p.offset = podIdx - vis + 1
				}
			}
		}
	}
}

func (p PodPickerModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPurple).
		Padding(0, 1)

	title := titleStyle.Render(fmt.Sprintf("Select pod from deployment: %s", p.deploymentName))
	hint := lipgloss.NewStyle().Foreground(colorDimText).Render(
		"  j/k:navigate  enter:select  esc:cancel")

	if p.loading {
		return lipgloss.JoinVertical(lipgloss.Left, "", title, hint, "",
			lipgloss.NewStyle().Foreground(colorDimText).Padding(0, 2).Render("Loading pods..."))
	}

	if len(p.pods) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, "", title, hint, "",
			lipgloss.NewStyle().Foreground(colorDimText).Padding(0, 2).Render("No pods found"))
	}

	selectedStyle := lipgloss.NewStyle().
		Background(colorPurple).
		Foreground(lipgloss.Color("#000000")).
		Bold(true)

	// "All Pods" — always visible, fixed above the table
	allPodsLabel := fmt.Sprintf("All Pods (%d pods)", len(p.pods))
	var allPodsRow string
	if p.cursor == 0 {
		allPodsRow = selectedStyle.Render("> ") +
			selectedStyle.Width(p.width-12).Padding(0, 1).Render(allPodsLabel)
	} else {
		allPodsRow = "  " + lipgloss.NewStyle().
			Foreground(colorPurple).Bold(true).Padding(0, 1).Render(allPodsLabel)
	}

	// Table header
	cols := p.columns()
	var headerParts []string
	for _, col := range cols {
		headerParts = append(headerParts, TableHeaderStyle.Width(col.width).Render(col.name))
	}
	header := "  " + lipgloss.JoinHorizontal(lipgloss.Top, headerParts...)

	// Scrollable pod rows
	var b strings.Builder
	visibleRows := p.visibleRows()
	end := min(p.offset+visibleRows, len(p.pods))

	for i := p.offset; i < end; i++ {
		pod := p.pods[i]
		selected := (p.cursor - 1) == i // cursor 1+ maps to pods[0+]
		values := []string{
			truncate(pod.Name, cols[0].width-2),
			pod.Status,
			pod.Ready,
			fmt.Sprintf("%d", pod.Restarts),
			formatAge(pod.Age),
		}

		var parts []string
		for j, col := range cols {
			if selected {
				parts = append(parts, selectedStyle.Width(col.width).Padding(0, 1).Render(values[j]))
			} else {
				style := TableCellStyle.Width(col.width)
				if col.name == "STATUS" {
					style = style.Inherit(StatusStyle(pod.Status))
				}
				parts = append(parts, style.Render(values[j]))
			}
		}

		if selected {
			b.WriteString(selectedStyle.Render("> ") + lipgloss.JoinHorizontal(lipgloss.Top, parts...) + "\n")
		} else {
			b.WriteString("  " + lipgloss.JoinHorizontal(lipgloss.Top, parts...) + "\n")
		}
	}

	if len(p.pods) > visibleRows {
		b.WriteString(lipgloss.NewStyle().Foreground(colorDimText).Render(
			fmt.Sprintf("  showing %d-%d of %d pods", p.offset+1, end, len(p.pods))))
	}

	content := DetailBorderStyle.
		Width(p.width - 6).
		Render(allPodsRow + "\n" + header + "\n" + b.String())

	return lipgloss.JoinVertical(lipgloss.Left, "", title, hint, content)
}

func (p PodPickerModel) columns() []column {
	nameWidth := p.width - 58
	if nameWidth < 20 {
		nameWidth = 20
	}
	return []column{
		{"NAME", nameWidth},
		{"STATUS", 14},
		{"READY", 8},
		{"RESTARTS", 10},
		{"AGE", 8},
	}
}

func (p PodPickerModel) visibleRows() int {
	h := p.height - 14 // account for title, hint, border, "All Pods" row, header, scroll indicator
	return max(h, 3)
}
