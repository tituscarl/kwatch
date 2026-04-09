package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tituscarl/kwatch/internal/k8s"
)

type EventsModel struct {
	events []k8s.EventInfo
	offset int
	width  int
	height int
}

func NewEventsModel() EventsModel {
	return EventsModel{}
}

func (e *EventsModel) UpdateEvents(events []k8s.EventInfo) {
	e.events = events
}

func (e *EventsModel) SetSize(w, h int) {
	e.width = w
	e.height = h
}

func (e *EventsModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if e.offset > 0 {
				e.offset--
			}
		case "down", "j":
			if e.offset < len(e.events)-e.visibleRows() {
				e.offset++
			}
		case "pgdown":
			e.offset = min(e.offset+e.visibleRows(), max(0, len(e.events)-e.visibleRows()))
		case "pgup":
			e.offset = max(e.offset-e.visibleRows(), 0)
		}
	}
	return nil
}

func (e EventsModel) View() string {
	if len(e.events) == 0 {
		return lipgloss.NewStyle().
			Foreground(colorDimText).
			Padding(2, 4).
			Render("No events found")
	}

	var b strings.Builder

	// Header
	typeW := 9
	reasonW := 20
	objectW := 30
	ageW := 8
	cntW := 6
	msgW := e.width - typeW - reasonW - objectW - ageW - cntW - 12
	if msgW < 20 {
		msgW = 20
	}

	header := lipgloss.JoinHorizontal(lipgloss.Top,
		TableHeaderStyle.Width(typeW).Render("TYPE"),
		TableHeaderStyle.Width(ageW).Render("AGE"),
		TableHeaderStyle.Width(cntW).Render("COUNT"),
		TableHeaderStyle.Width(reasonW).Render("REASON"),
		TableHeaderStyle.Width(objectW).Render("OBJECT"),
		TableHeaderStyle.Width(msgW).Render("MESSAGE"),
	)
	b.WriteString(header + "\n")

	// Rows
	visibleRows := e.visibleRows()
	end := min(e.offset+visibleRows, len(e.events))

	for i := e.offset; i < end; i++ {
		evt := e.events[i]

		typeStyle := TableCellStyle.Width(typeW)
		if evt.Type == "Warning" {
			typeStyle = typeStyle.Inherit(StyleWarning)
		} else {
			typeStyle = typeStyle.Inherit(StyleRunning)
		}

		row := lipgloss.JoinHorizontal(lipgloss.Top,
			typeStyle.Render(evt.Type),
			TableCellStyle.Width(ageW).Render(formatDurationShort(evt.Age)),
			TableCellStyle.Width(cntW).Render(fmt.Sprintf("%d", evt.Count)),
			TableCellStyle.Width(reasonW).Render(truncate(evt.Reason, reasonW-2)),
			TableCellStyle.Width(objectW).Render(truncate(evt.Object, objectW-2)),
			TableCellStyle.Width(msgW).Render(truncate(evt.Message, msgW-2)),
		)
		b.WriteString(row + "\n")
	}

	if len(e.events) > visibleRows {
		b.WriteString(lipgloss.NewStyle().Foreground(colorDimText).Render(
			fmt.Sprintf("  showing %d-%d of %d events", e.offset+1, end, len(e.events))))
	}

	return lipgloss.NewStyle().Padding(0, 1).Render(b.String())
}

func (e EventsModel) visibleRows() int {
	h := e.height - 4
	if h < 1 {
		h = 1
	}
	return h
}
