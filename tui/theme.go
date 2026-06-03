package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Theme Colors
	ColorPrimary   = lipgloss.Color("#5f5fd7") // Indigo/Purple
	ColorSecondary = lipgloss.Color("#7a22ff") // Light Purple
	ColorSuccess   = lipgloss.Color("#00ff87") // Mint/Green
	ColorError     = lipgloss.Color("#ff5f87") // Rose/Red
	ColorDim       = lipgloss.Color("#767676") // Medium Grey
	ColorLight     = lipgloss.Color("#e5e5e5") // Off-white

	// Styles
	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorLight).
			Background(ColorPrimary).
			Padding(0, 1)

	StyleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	StyleSelection = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSuccess)

	StyleSelectedRow = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	StyleNormalRow = lipgloss.NewStyle().
			Foreground(ColorLight)

	StyleHelp = lipgloss.NewStyle().
			Foreground(ColorDim)

	StyleError = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorError)

	StyleSuccess = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSuccess)

	StyleBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(ColorSuccess).
			Padding(0, 1)

	StyleBadgeError = StyleBadge.
			Background(ColorError)

	StyleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1)
)
