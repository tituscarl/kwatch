package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type LineDetailModel struct {
	content      string
	title        string
	offset       int
	width        int
	height       int
	wrappedLines []string
}

func NewLineDetailModel() LineDetailModel {
	return LineDetailModel{}
}

func (d *LineDetailModel) SetSize(w, h int) {
	d.width = w
	d.height = h
	d.rewrap()
}

func (d *LineDetailModel) Show(line string, lineNum int, podTag string) {
	d.content = line
	if podTag != "" {
		d.title = fmt.Sprintf("Line %d · [%s]", lineNum, podTag)
	} else {
		d.title = fmt.Sprintf("Line %d", lineNum)
	}
	d.offset = 0
	d.rewrap()
}

func (d *LineDetailModel) rewrap() {
	innerWidth := d.width - 12
	if innerWidth < 10 {
		innerWidth = 10
	}
	if d.content == "" {
		d.wrappedLines = nil
		return
	}
	wrapped := ansi.Hardwrap(d.content, innerWidth, false)
	d.wrappedLines = strings.Split(wrapped, "\n")
}

func (d LineDetailModel) visibleLines() int {
	h := d.height - 6
	if h < 1 {
		h = 1
	}
	return h
}

func (d LineDetailModel) Update(msg tea.Msg) (LineDetailModel, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		visible := d.visibleLines()
		maxOff := max(len(d.wrappedLines)-visible, 0)
		switch {
		case key.Matches(k, Keys.Up):
			if d.offset > 0 {
				d.offset--
			}
		case key.Matches(k, Keys.Down):
			if d.offset < maxOff {
				d.offset++
			}
		case key.Matches(k, Keys.PageUp):
			d.offset = max(d.offset-visible, 0)
		case key.Matches(k, Keys.PageDown):
			d.offset = min(d.offset+visible, maxOff)
		}
		switch k.String() {
		case "g":
			d.offset = 0
		case "G":
			d.offset = maxOff
		}
	}
	return d, nil
}

func (d LineDetailModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorPurple).Padding(0, 1)
	title := titleStyle.Render(d.title)

	sep := lipgloss.NewStyle().Foreground(colorSubtle).Render(" | ")
	keyStyle := lipgloss.NewStyle().Foreground(colorPurple).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(colorDimText)
	helpBar := keyStyle.Render("esc") + descStyle.Render(" close") + sep +
		keyStyle.Render("j/k") + descStyle.Render(" scroll") + sep +
		keyStyle.Render("g/G") + descStyle.Render(" top/bottom")

	visible := d.visibleLines()
	end := min(d.offset+visible, len(d.wrappedLines))
	start := d.offset
	if start > len(d.wrappedLines) {
		start = len(d.wrappedLines)
	}

	body := ""
	if len(d.wrappedLines) > 0 {
		body = strings.Join(d.wrappedLines[start:end], "\n")
	}

	contentBox := DetailBorderStyle.
		Width(d.width - 6).
		Height(visible).
		Render(body)

	var scrollInfo string
	if len(d.wrappedLines) > visible {
		scrollInfo = lipgloss.NewStyle().Foreground(colorDimText).Render(
			fmt.Sprintf("  row %d-%d of %d", start+1, end, len(d.wrappedLines)))
	}

	parts := []string{"", title, contentBox}
	if scrollInfo != "" {
		parts = append(parts, scrollInfo)
	}
	parts = append(parts, helpBar)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
