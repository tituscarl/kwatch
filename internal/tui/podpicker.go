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
	if p.cursor >= len(pods) {
		p.cursor = max(0, len(pods)-1)
	}
}

func (p *PodPickerModel) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p PodPickerModel) SelectedPod() (k8s.PodInfo, bool) {
	if p.cursor < len(p.pods) {
		return p.pods[p.cursor], true
	}
	return k8s.PodInfo{}, false
}

func (p *PodPickerModel) Update(msg tea.KeyMsg) {
	switch msg.String() {
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
			if p.cursor < p.offset {
				p.offset = p.cursor
			}
		}
	case "down", "j":
		if p.cursor < len(p.pods)-1 {
			p.cursor++
			vis := p.visibleRows()
			if p.cursor >= p.offset+vis {
				p.offset = p.cursor - vis + 1
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

	cols := p.columns()

	var b strings.Builder

	// Header
	var headerParts []string
	for _, col := range cols {
		headerParts = append(headerParts, TableHeaderStyle.Width(col.width).Render(col.name))
	}
	b.WriteString("  " + lipgloss.JoinHorizontal(lipgloss.Top, headerParts...) + "\n")

	visibleRows := p.visibleRows()
	end := min(p.offset+visibleRows, len(p.pods))

	for i := p.offset; i < end; i++ {
		pod := p.pods[i]
		values := []string{
			truncate(pod.Name, cols[0].width-2),
			pod.Status,
			pod.Ready,
			fmt.Sprintf("%d", pod.Restarts),
			formatAge(pod.Age),
		}

		var parts []string
		for j, col := range cols {
			if i == p.cursor {
				parts = append(parts, selectedStyle.Width(col.width).Padding(0, 1).Render(values[j]))
			} else {
				style := TableCellStyle.Width(col.width)
				if col.name == "STATUS" {
					style = style.Inherit(StatusStyle(pod.Status))
				}
				parts = append(parts, style.Render(values[j]))
			}
		}

		if i == p.cursor {
			b.WriteString(selectedStyle.Render("> ") + lipgloss.JoinHorizontal(lipgloss.Top, parts...) + "\n")
		} else {
			b.WriteString("  " + lipgloss.JoinHorizontal(lipgloss.Top, parts...) + "\n")
		}
	}

	if len(p.pods) > visibleRows {
		b.WriteString(lipgloss.NewStyle().Foreground(colorDimText).Render(
			fmt.Sprintf("  showing %d-%d of %d", p.offset+1, end, len(p.pods))))
	}

	content := DetailBorderStyle.
		Width(p.width - 6).
		Render(b.String())

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
	h := p.height - 10
	return max(h, 3)
}
