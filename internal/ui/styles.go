package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Styles holds all the styling for the UI
type Styles struct {
	App     lipgloss.Style
	Header  lipgloss.Style
	Footer  lipgloss.Style
	Table   lipgloss.Style
	Filter  lipgloss.Style
	Stats   lipgloss.Style
	Details lipgloss.Style
	Help    lipgloss.Style
}

// NewStyles creates a new Styles instance with default styling
func NewStyles() *Styles {
	// Color scheme
	var (
		primaryColor   = lipgloss.Color("#00ff88")
		secondaryColor = lipgloss.Color("#88aaff")
		accentColor    = lipgloss.Color("#ffaa00")
		backgroundColor = lipgloss.Color("#1a1a1a")
		textColor      = lipgloss.Color("#ffffff")
		mutedColor     = lipgloss.Color("#888888")
	)

	return &Styles{
		App: lipgloss.NewStyle().
			Padding(1, 2).
			Foreground(textColor).
			Background(backgroundColor),

		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Background(backgroundColor).
			Padding(0, 1).
			MarginBottom(1).
			Border(lipgloss.NormalBorder()).
			BorderForeground(primaryColor),

		Footer: lipgloss.NewStyle().
			Foreground(mutedColor).
			Background(backgroundColor).
			Padding(0, 1).
			MarginTop(1).
			Border(lipgloss.NormalBorder()).
			BorderForeground(mutedColor),

		Table: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(secondaryColor).
			Padding(1).
			MarginBottom(1),

		Filter: lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor).
			Background(backgroundColor).
			Padding(0, 1).
			MarginBottom(1).
			Border(lipgloss.NormalBorder()).
			BorderForeground(accentColor),

		Stats: lipgloss.NewStyle().
			Foreground(textColor).
			Background(backgroundColor).
			Padding(1).
			MarginBottom(1).
			Border(lipgloss.NormalBorder()).
			BorderForeground(secondaryColor),

		Details: lipgloss.NewStyle().
			Foreground(textColor).
			Background(backgroundColor).
			Padding(1).
			MarginBottom(1).
			Border(lipgloss.NormalBorder()).
			BorderForeground(secondaryColor).
			Width(80),

		Help: lipgloss.NewStyle().
			Foreground(mutedColor).
			Background(backgroundColor).
			Padding(0, 1).
			MarginTop(1),
	}
}

// TableStyles returns styles specifically for table components
func TableStyles() lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))
}
