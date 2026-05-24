package tui

import "charm.land/lipgloss/v2"

// Slate palette — Indigo + Emerald + Amber
var (
	colorPrimary    = lipgloss.Color("#6366F1") // Indigo 500 — titles, spinner, filled progress
	colorSecondary  = lipgloss.Color("#818CF8") // Indigo 400 — labels, status line, JSON keys
	colorPath       = lipgloss.Color("#94A3B8") // Slate 400 — file/dir paths
	colorAccent     = lipgloss.Color("#FBBF24") // Amber 400 — empty state, search match highlight
	colorSuccess    = lipgloss.Color("#34D399") // Emerald 400 — JSON strings
	colorNumber     = lipgloss.Color("#FB923C") // Orange 400 — JSON numbers
	colorError      = lipgloss.Color("#F87171") // Red 400 — JSON keywords (true/false/null)
	colorMuted      = lipgloss.Color("#64748B") // Slate 500 — help text, counts, dim info
	colorSurface    = lipgloss.Color("#334155") // Slate 700 — dividers, empty progress bar
	colorText       = lipgloss.Color("#CBD5E1") // Slate 300 — regular rows, body text
	colorTextDim    = lipgloss.Color("#94A3B8") // Slate 400 — JSON punctuation
	colorSelectedFG = lipgloss.Color("#F8FAFC") // Slate 50 — selected row foreground
	colorSelectedBG = lipgloss.Color("#3730A3") // Indigo 800 — selected row background
)

// progressGradientHex is a 5-step Indigo 800→300 gradient for the progress bar fill.
var progressGradientHex = []string{"#3730A3", "#4F46E5", "#6366F1", "#818CF8", "#A5B4FC"}
