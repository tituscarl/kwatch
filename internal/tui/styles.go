package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	colorPurple  = lipgloss.Color("#7D56F4")
	colorGreen   = lipgloss.Color("#00CC66")
	colorYellow  = lipgloss.Color("#FFAA00")
	colorRed     = lipgloss.Color("#FF4444")
	colorGray    = lipgloss.Color("#666666")
	colorWhite   = lipgloss.Color("#FAFAFA")
	colorSubtle  = lipgloss.Color("#383838")
	colorDimText = lipgloss.Color("#888888")

	// Status styles
	StyleRunning   = lipgloss.NewStyle().Foreground(colorGreen)
	StylePending   = lipgloss.NewStyle().Foreground(colorYellow)
	StyleFailed    = lipgloss.NewStyle().Foreground(colorRed)
	StyleSucceeded = lipgloss.NewStyle().Foreground(colorGray)
	StyleUnknown   = lipgloss.NewStyle().Foreground(colorDimText)
	StyleWarning   = lipgloss.NewStyle().Foreground(colorYellow)

	// Header
	HeaderStyle = lipgloss.NewStyle().
			Background(colorPurple).
			Foreground(colorWhite).
			Bold(true).
			Padding(0, 1)

	HeaderLabelStyle = lipgloss.NewStyle().
				Foreground(colorDimText).
				Bold(false)

	HeaderValueStyle = lipgloss.NewStyle().
				Foreground(colorWhite).
				Bold(true)

	// Tabs
	ActiveTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPurple).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(colorPurple).
			Padding(0, 2)

	InactiveTabStyle = lipgloss.NewStyle().
				Foreground(colorDimText).
				Padding(0, 2)

	TabBarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(colorSubtle)

	// Cards (for overview)
	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPurple).
			Padding(1, 3).
			Width(28)

	CardTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPurple).
			MarginBottom(1)

	// Table
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPurple).
				Padding(0, 1)

	TableCellStyle = lipgloss.NewStyle().
			Padding(0, 1)

	TableSelectedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#7D56F4")).
				Foreground(colorWhite).
				Bold(true)

	// Status bar
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(colorDimText).
			Padding(0, 1)

	StatusBarKeyStyle = lipgloss.NewStyle().
				Foreground(colorPurple).
				Bold(true)

	// Detail view
	DetailBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPurple).
				Padding(1, 2)

	DetailLabelStyle = lipgloss.NewStyle().
				Foreground(colorPurple).
				Bold(true)

	// Error
	ErrorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	// Loading
	LoadingStyle = lipgloss.NewStyle().
			Foreground(colorPurple)
)

func StatusStyle(status string) lipgloss.Style {
	switch status {
	case "Running":
		return StyleRunning
	case "Pending", "ContainerCreating", "PodInitializing":
		return StylePending
	case "Failed", "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull", "Error", "OOMKilled":
		return StyleFailed
	case "Succeeded", "Completed":
		return StyleSucceeded
	case "Terminating":
		return StyleWarning
	default:
		if len(status) > 5 && status[:5] == "Init:" {
			return StylePending
		}
		return StyleUnknown
	}
}
