package tui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Name    string
	Accent  lipgloss.Color // Primary accent (tabs, headers, cards)
	Green   lipgloss.Color // Running / healthy
	Yellow  lipgloss.Color // Pending / warning
	Red     lipgloss.Color // Failed / error
	Gray    lipgloss.Color // Succeeded / completed
	White   lipgloss.Color // Primary text
	Subtle  lipgloss.Color // Borders, separators
	DimText lipgloss.Color // Secondary text
	BgBar   lipgloss.Color // Header/status bar background
	BgSel   lipgloss.Color // Selected row background
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
		Gray:    lipgloss.Color("#859289"),
		White:   lipgloss.Color("#D3C6AA"),
		Subtle:  lipgloss.Color("#374145"),
		DimText: lipgloss.Color("#859289"),
		BgBar:   lipgloss.Color("#272E33"),
		BgSel:   lipgloss.Color("#A7C080"),
	},
	"one-dark-pro": {
		Name:    "One Dark Pro",
		Accent:  lipgloss.Color("#61AFEF"),
		Green:   lipgloss.Color("#98C379"),
		Yellow:  lipgloss.Color("#E5C07B"),
		Red:     lipgloss.Color("#E06C75"),
		Gray:    lipgloss.Color("#5C6370"),
		White:   lipgloss.Color("#ABB2BF"),
		Subtle:  lipgloss.Color("#3E4452"),
		DimText: lipgloss.Color("#5C6370"),
		BgBar:   lipgloss.Color("#21252B"),
		BgSel:   lipgloss.Color("#61AFEF"),
	},
	"vscode-dark": {
		Name:    "VSCode Dark",
		Accent:  lipgloss.Color("#569CD6"),
		Green:   lipgloss.Color("#6A9955"),
		Yellow:  lipgloss.Color("#DCDCAA"),
		Red:     lipgloss.Color("#F44747"),
		Gray:    lipgloss.Color("#808080"),
		White:   lipgloss.Color("#D4D4D4"),
		Subtle:  lipgloss.Color("#3C3C3C"),
		DimText: lipgloss.Color("#808080"),
		BgBar:   lipgloss.Color("#252526"),
		BgSel:   lipgloss.Color("#569CD6"),
	},
}

var ThemeNames = []string{"github-dark", "everforest", "one-dark-pro", "vscode-dark"}

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
