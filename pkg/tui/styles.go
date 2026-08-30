package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Color palette
	primaryColor   = lipgloss.Color("#7D56F4") // Purple
	secondaryColor = lipgloss.Color("#04B575") // Emerald Green
	accentColor    = lipgloss.Color("#00D2FF") // Cyan
	warningColor   = lipgloss.Color("#FFB800") // Amber
	errorColor     = lipgloss.Color("#FF4C4C") // Red
	mutedColor     = lipgloss.Color("#6B7280") // Gray
	bgDarkColor    = lipgloss.Color("#1E1E2E")
	bgPanelColor   = lipgloss.Color("#181825")
	textLightColor = lipgloss.Color("#CDD6F4")

	// Header Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primaryColor).
			Padding(0, 2).
			MarginRight(1)

	headerSubStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)

	// Stat badges
	statBadge = lipgloss.NewStyle().
			Padding(0, 1).
			Bold(true)

	statMatched = statBadge.
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(secondaryColor)

	statUnmatched = statBadge.
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(errorColor)

	statTotal = statBadge.
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(mutedColor)

	statProto = statBadge.
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#89B4FA"))

	// Panels & Boxes
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(0, 1)

	detailPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(accentColor).
				Padding(0, 1)

	// Item selection
	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#45475A")).
				Padding(0, 1)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(textLightColor).
			Padding(0, 1)

	// Text helpers
	labelStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)

	valueStyle = lipgloss.NewStyle().
			Foreground(textLightColor)

	footerKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#313244")).
			Padding(0, 1).
			Bold(true)

	footerDescStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginRight(2)
)
