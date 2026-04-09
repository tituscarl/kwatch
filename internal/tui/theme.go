package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

type Theme struct {
	Name    string
	Accent  color.Color // Primary accent (tabs, headers, cards)
	Green   color.Color // Running / healthy
	Yellow  color.Color // Pending / warning
	Red     color.Color // Failed / error
	Gray    color.Color // Succeeded / completed
	White   color.Color // Primary text
	Subtle  color.Color // Borders, separators
	DimText color.Color // Secondary text
	BgBar   color.Color // Header/status bar background
	BgSel   color.Color // Selected row background
}

var Themes = map[string]Theme{
	"github-dark": {
		Name:    "GitHub Dark",
		Accent:  lipgloss.Color("#58A6FF"),
		Green:   lipgloss.Color("#3FB950"),
		Yellow:  lipgloss.Color("#D29922"),
		Red:     lipgloss.Color("#F85149"),
		Gray:    lipgloss.Color("#8B949E"),
		White:   lipgloss.Color("#C9D1D9"),
		Subtle:  lipgloss.Color("#30363D"),
		DimText: lipgloss.Color("#8B949E"),
		BgBar:   lipgloss.Color("#161B22"),
		BgSel:   lipgloss.Color("#58A6FF"),
	},
	"everforest": {
		Name:    "Everforest",
		Accent:  lipgloss.Color("#A7C080"),
		Green:   lipgloss.Color("#A7C080"),
		Yellow:  lipgloss.Color("#DBBC7F"),
		Red:     lipgloss.Color("#E67E80"),
		Gray:    lipgloss.Color("#7A8478"),
		White:   lipgloss.Color("#D3C6AA"),
		Subtle:  lipgloss.Color("#3D484D"),
		DimText: lipgloss.Color("#859289"),
		BgBar:   lipgloss.Color("#232A2E"),
		BgSel:   lipgloss.Color("#A7C080"),
	},
	"dracula": {
		Name:    "Dracula",
		Accent:  lipgloss.Color("#BD93F9"),
		Green:   lipgloss.Color("#50FA7B"),
		Yellow:  lipgloss.Color("#F1FA8C"),
		Red:     lipgloss.Color("#FF5555"),
		Gray:    lipgloss.Color("#6272A4"),
		White:   lipgloss.Color("#F8F8F2"),
		Subtle:  lipgloss.Color("#44475A"),
		DimText: lipgloss.Color("#6272A4"),
		BgBar:   lipgloss.Color("#21222C"),
		BgSel:   lipgloss.Color("#BD93F9"),
	},
}

var ThemeNames = []string{"dracula", "github-dark", "everforest"}

func ApplyTheme(t Theme) {
	colorPurple = t.Accent
	colorGreen = t.Green
	colorYellow = t.Yellow
	colorRed = t.Red
	colorGray = t.Gray
	colorWhite = t.White
	colorSubtle = t.Subtle
	colorDimText = t.DimText

	// Status styles
	StyleRunning = lipgloss.NewStyle().Foreground(t.Green)
	StylePending = lipgloss.NewStyle().Foreground(t.Yellow)
	StyleFailed = lipgloss.NewStyle().Foreground(t.Red)
	StyleSucceeded = lipgloss.NewStyle().Foreground(t.Gray)
	StyleUnknown = lipgloss.NewStyle().Foreground(t.DimText)
	StyleWarning = lipgloss.NewStyle().Foreground(t.Yellow)

	// Header
	HeaderStyle = lipgloss.NewStyle().
		Background(t.Accent).
		Foreground(lipgloss.Color("#000000")).
		Bold(true).
		Padding(0, 1)

	HeaderLabelStyle = lipgloss.NewStyle().
		Foreground(t.DimText).
		Bold(false)

	HeaderValueStyle = lipgloss.NewStyle().
		Foreground(t.White).
		Bold(true)

	// Tabs
	ActiveTabStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Accent).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(t.Accent).
		Padding(0, 2)

	InactiveTabStyle = lipgloss.NewStyle().
		Foreground(t.DimText).
		Padding(0, 2)

	TabBarStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(t.Subtle)

	// Cards
	CardStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Accent).
		Padding(1, 3).
		Width(28)

	CardTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Accent).
		MarginBottom(1)

	// Table
	TableHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Accent).
		Padding(0, 1)

	TableCellStyle = lipgloss.NewStyle().
		Padding(0, 1)

	TableSelectedStyle = lipgloss.NewStyle().
		Background(t.BgSel).
		Foreground(lipgloss.Color("#000000")).
		Bold(true)

	// Status bar
	StatusBarStyle = lipgloss.NewStyle().
		Foreground(t.DimText).
		Padding(0, 1)

	StatusBarKeyStyle = lipgloss.NewStyle().
		Foreground(t.Accent).
		Bold(true)

	// Detail view
	DetailBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Accent).
		Padding(1, 2)

	DetailLabelStyle = lipgloss.NewStyle().
		Foreground(t.Accent).
		Bold(true)

	// Error
	ErrorStyle = lipgloss.NewStyle().
		Foreground(t.Red).
		Bold(true)

	// Loading
	LoadingStyle = lipgloss.NewStyle().
		Foreground(t.Accent)
}
