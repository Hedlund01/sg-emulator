package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	primaryColor   = lipgloss.Color("#4af56783")
	secondaryColor = lipgloss.Color("#444444")
	textColor      = lipgloss.Color("#FAFAFA")
	subtleColor    = lipgloss.Color("#888888")

	// Title style
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor).
			Background(primaryColor).
			Padding(0, 2).
			MarginBottom(1)

	// Help style
	helpStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			MarginTop(1)

	// Box style for content areas
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(secondaryColor).
			Padding(1, 2)

	// Selected item style
	selectedStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	// Status message style
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFCC00")).
			MarginTop(1)

	// Input label styles
	focusedLabelStyle = lipgloss.NewStyle().
				Foreground(primaryColor)

	textLabelStyle = lipgloss.NewStyle().
			Foreground(textColor)

	blurredLabelStyle = lipgloss.NewStyle().
				Foreground(subtleColor)

	// Button styles
	focusedButtonStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)

	blurredButtonStyle = lipgloss.NewStyle().
				Foreground(subtleColor)
)
