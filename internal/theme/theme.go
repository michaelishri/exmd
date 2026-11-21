// Package theme holds the design tokens for the exmd TUI. The visual
// language follows the OpenCode TUI: no borders, separation by background
// shades, muted chrome, and color reserved for state.
package theme

import (
	"fmt"
	"strings"
)

// Color is a theme color token.
type Color string

// Theme is a set of named color tokens.
type Theme struct {
	Name string

	// Background shades, darkest to lightest. The canvas itself is the
	// terminal's own background; panels and elements sit one or two shades
	// above it.
	Background        Color
	BackgroundPanel   Color
	BackgroundElement Color

	// Border is the one hairline separator color used in the UI (above the
	// status bar).
	Border Color
	// Separator is a darkened chrome tone for hairline separators, halfway
	// between TextMuted and black.
	Separator Color

	Text      Color
	TextMuted Color
	Bright    Color
	Soft      Color

	// Accent and Rule are the content-layer equivalents of Primary and
	// Separator: links, inline code and horizontal rules.
	Accent Color
	Rule   Color

	// CodePanel is the surface of a display-only fence; ExecPanel is the
	// raised surface of an executable block and its inline output.
	CodePanel Color
	ExecPanel Color

	// Primary marks state: the current outline row, selection markers.
	// It is not used for decoration.
	Primary Color

	Success Color
	Error   Color
	Warning Color

	// Running marks an in-flight execution in the gutter (orange).
	Running Color

	// The bright state variants mark the block that currently holds the
	// cursor. They are derived from the state colors, never hard-coded at
	// the point of use.
	SuccessBright Color
	ErrorBright   Color
	RunningBright Color
}

// Dark is the default theme. Every color the application emits comes from
// one of these tokens — including the canvas and the content renderer's
// palette — so no layer can fall back to the terminal's own colors.
var Dark = Theme{
	Name:              "dark",
	Background:        "#0e0e11",
	BackgroundPanel:   "#18181e",
	BackgroundElement: "#24242c",
	Border:            "#2b2b34",
	Separator:         "#444448",
	Text:              "#e4e4e9",
	TextMuted:         "#8a8a95",
	Bright:            "#f4f4f8",
	Soft:              "#c8c8d0",
	Accent:            "#8f8af4",
	Rule:              "#3a3a44",
	CodePanel:         "#1a1a21",
	ExecPanel:         "#1e2130",
	Primary:           "#8f8af4",
	Success:           "#6fbf73",
	Error:             "#f05b5b",
	Warning:           "#e2b93d",
	Running:           "#ff9e64",
	SuccessBright:     "#a5e8a8",
	ErrorBright:       "#ff9b9b",
	RunningBright:     "#ffc9a3",
}

// Notty is the fallback for terminals without color support: every token is
// empty, so styles render as plain text.
var Notty = Theme{
	Name: "notty",
}

// Resolve maps a --theme flag value to a Theme. "auto" (the default, and the
// empty string) picks Dark when the terminal supports color and Notty
// otherwise; "dark" and "notty" force their tokens regardless. Matching is
// case-insensitive.
func Resolve(name string, colorSupported bool) (Theme, error) {
	switch strings.ToLower(name) {
	case "", "auto":
		if colorSupported {
			return Dark, nil
		}
		return Notty, nil
	case "dark":
		return Dark, nil
	case "notty":
		return Notty, nil
	default:
		return Theme{}, fmt.Errorf("unknown theme %q (valid: auto, dark, notty)", name)
	}
}
