package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tituscarl/kwatch/internal/k8s"
)

type DeploymentsModel struct {
	deployments []k8s.DeploymentInfo
	cursor      int
	offset      int
	width       int
	height      int
	allNS       bool
	filter      string
	filtering   bool
}

func NewDeploymentsModel(allNS bool) DeploymentsModel {
	return DeploymentsModel{allNS: allNS}
}

func (d *DeploymentsModel) UpdateDeployments(deps []k8s.DeploymentInfo) {
	d.deployments = deps
	if d.cursor >= len(deps) {
		d.cursor = max(0, len(deps)-1)
	}
}

func (d *DeploymentsModel) SetSize(w, h int) {
	d.width = w
	d.height = h
}

func (d DeploymentsModel) SelectedDeployment() (k8s.DeploymentInfo, bool) {
	filtered := d.filteredDeployments()
	if d.cursor < len(filtered) {
		return filtered[d.cursor], true
	}
	return k8s.DeploymentInfo{}, false
}

func (d *DeploymentsModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if d.filtering {
			switch {
			case key.Matches(msg, Keys.Escape):
				d.filtering = false
				d.filter = ""
				return nil
			case msg.Type == tea.KeyBackspace:
				if len(d.filter) > 0 {
					d.filter = d.filter[:len(d.filter)-1]
				}
				return nil
			case msg.Type == tea.KeyEnter:
				d.filtering = false
				return nil
			case msg.Type == tea.KeyUp, msg.Type == tea.KeyDown,
				msg.Type == tea.KeyPgUp, msg.Type == tea.KeyPgDown:
				// Allow arrow keys to navigate while filtering
			case msg.Type == tea.KeyRunes:
				d.filter += string(msg.Runes)
				d.cursor = 0
				d.offset = 0
				return nil
			}
		}

		d.handleNav(msg)
	}
	return nil
}

func (d *DeploymentsModel) handleNav(msg tea.KeyMsg) {
	filtered := d.filteredDeployments()

	switch msg.String() {
	case "up", "k":
		if d.cursor > 0 {
			d.cursor--
			if d.cursor < d.offset {
				d.offset = d.cursor
			}
		}
	case "down", "j":
		if d.cursor < len(filtered)-1 {
			d.cursor++
			visibleRows := d.visibleRows()
			if d.cursor >= d.offset+visibleRows {
				d.offset = d.cursor - visibleRows + 1
			}
		}
	case "/":
		d.filtering = true
		d.filter = ""
		d.cursor = 0
		d.offset = 0
	case "pgdown":
		d.cursor = min(d.cursor+d.visibleRows(), len(filtered)-1)
		if d.cursor >= d.offset+d.visibleRows() {
			d.offset = d.cursor - d.visibleRows() + 1
		}
	case "pgup":
		d.cursor = max(d.cursor-d.visibleRows(), 0)
		if d.cursor < d.offset {
			d.offset = d.cursor
		}
	}
}

func (d DeploymentsModel) View() string {
	filtered := d.filteredDeployments()

	if len(d.deployments) == 0 {
		return lipgloss.NewStyle().
			Foreground(colorDimText).
			Padding(2, 4).
			Render("No deployments found")
	}

	var b strings.Builder

	if d.filtering {
		b.WriteString(lipgloss.NewStyle().Foreground(colorPurple).Render("Filter: "+d.filter+"█") + "\n")
	} else if d.filter != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(colorDimText).Render(fmt.Sprintf("Filter: %s (%d results)  ", d.filter, len(filtered))) + "\n")
	}

	// Header
	cols := d.columns()
	var headerParts []string
	for _, col := range cols {
		headerParts = append(headerParts, TableHeaderStyle.Width(col.width).Render(col.name))
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, headerParts...) + "\n")

	// Rows
	visibleRows := d.visibleRows()
	end := min(d.offset+visibleRows, len(filtered))
	for i := d.offset; i < end; i++ {
		dep := filtered[i]
		row := d.renderRow(dep, i == d.cursor)
		b.WriteString(row + "\n")
	}

	if len(filtered) > visibleRows {
		b.WriteString(lipgloss.NewStyle().Foreground(colorDimText).Render(
			fmt.Sprintf("  showing %d-%d of %d", d.offset+1, end, len(filtered))))
	}

	return lipgloss.NewStyle().Padding(0, 1).Render(b.String())
}

func (d DeploymentsModel) renderRow(dep k8s.DeploymentInfo, selected bool) string {
	cols := d.columns()
	values := d.rowValues(dep)

	selectedStyle := lipgloss.NewStyle().
		Background(colorPurple).
		Foreground(lipgloss.Color("#000000")).
		Bold(true)

	var parts []string
	for i, col := range cols {
		val := values[i]

		if selected {
			parts = append(parts, selectedStyle.Width(col.width).Padding(0, 1).Render(val))
		} else {
			style := TableCellStyle.Width(col.width)
			if col.name == "READY" {
				if dep.Available == dep.Desired && dep.Desired > 0 {
					style = style.Foreground(colorGreen)
				} else if dep.Available == 0 && dep.Desired > 0 {
					style = style.Foreground(colorRed)
				} else if dep.Available < dep.Desired {
					style = style.Foreground(colorYellow)
				}
			}
			parts = append(parts, style.Render(val))
		}
	}

	if selected {
		return selectedStyle.Render("> ") + lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	}
	return "  " + lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (d DeploymentsModel) columns() []column {
	cols := []column{}
	if d.allNS {
		cols = append(cols, column{"NAMESPACE", 16})
	}

	nameWidth := 30
	if d.allNS {
		nameWidth = 24
	}

	cols = append(cols,
		column{"NAME", nameWidth},
		column{"READY", 10},
		column{"UP-TO-DATE", 12},
		column{"AVAILABLE", 12},
		column{"AGE", 10},
		column{"STRATEGY", 18},
	)
	return cols
}

func (d DeploymentsModel) rowValues(dep k8s.DeploymentInfo) []string {
	vals := []string{}
	if d.allNS {
		vals = append(vals, truncate(dep.Namespace, 14))
	}
	vals = append(vals,
		truncate(dep.Name, d.columns()[len(vals)].width-2),
		dep.Ready,
		fmt.Sprintf("%d", dep.UpToDate),
		fmt.Sprintf("%d", dep.Available),
		formatAge(dep.Age),
		dep.Strategy,
	)
	return vals
}

func (d DeploymentsModel) filteredDeployments() []k8s.DeploymentInfo {
	if d.filter == "" {
		return d.deployments
	}
	filter := strings.ToLower(d.filter)
	var result []k8s.DeploymentInfo
	for _, dep := range d.deployments {
		if strings.Contains(strings.ToLower(dep.Name), filter) ||
			strings.Contains(strings.ToLower(dep.Namespace), filter) {
			result = append(result, dep)
		}
	}
	return result
}

func (d DeploymentsModel) visibleRows() int {
	h := d.height - 4
	if d.filter != "" || d.filtering {
		h--
	}
	if h < 1 {
		h = 1
	}
	return h
}
