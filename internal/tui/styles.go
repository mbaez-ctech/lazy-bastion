package tui

import "github.com/charmbracelet/lipgloss"

// Color palette (k6-inspired: dark bg, violet accent, semantic traffic-light colors)
var (
	cViolet   = lipgloss.Color("#7C3AED")
	cVioletLt = lipgloss.Color("#C4B5FD")
	cGreen    = lipgloss.Color("#10B981")
	cRed      = lipgloss.Color("#EF4444")
	cYellow   = lipgloss.Color("#F59E0B")
	cGray     = lipgloss.Color("#9CA3AF")
	cDimGray  = lipgloss.Color("#4B5563")
	cWhite    = lipgloss.Color("#F9FAFB")
	cRowSel   = lipgloss.Color("#2D3748")
	cBorder   = lipgloss.Color("#374151")

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cWhite).
			Background(cViolet).
			PaddingLeft(2).
			PaddingRight(2)

	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cViolet)

	dividerStyle = lipgloss.NewStyle().
			Foreground(cBorder)

	selectedStyle = lipgloss.NewStyle().
			Background(cRowSel)

	activeStyle = lipgloss.NewStyle().
			Foreground(cGreen).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(cRed)

	startingStyle = lipgloss.NewStyle().
			Foreground(cYellow)

	idleStyle = lipgloss.NewStyle().
			Foreground(cDimGray)

	portStyle = lipgloss.NewStyle().
			Foreground(cYellow)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(cVioletLt).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(cGray)

	infoStyle = lipgloss.NewStyle().
			Foreground(cGray).
			Italic(true)

	footerStyle = lipgloss.NewStyle().
			Foreground(cGray).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(cBorder).
			PaddingTop(1).
			PaddingLeft(2)

	labelStyle = lipgloss.NewStyle().
			Foreground(cGray)
)
