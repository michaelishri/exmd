// Package theme holds the design tokens for the md-exec TUI. The visual
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
	Background           Color
	BackgroundPanel      Color
	BackgroundPanelMuted Color
	BackgroundElement    Color

	// Border is the one hairline separator color used in the UI (above the
	// status bar).
	Border Color

	Text      Color
	TextMuted Color

	// Primary marks state: the current outline row, selection markers.
	// It is not used for decoration.
	Primary Color

	Success Color
	Error   Color
	Warning Color
}

// Dark is the default theme.
var Dark = Theme{
	Name:                 "dark",
	Background:           "#0e0e11",
	BackgroundPanel:      "#18181e",
	BackgroundPanelMuted: "#131317",
	BackgroundElement:    "#24242c",
	Border:               "#2b2b34",
	Text:                 "#e4e4e9",
	TextMuted:            "#87878f",
	Primary:              "#8f8af4",
	Success:              "#6fbf73",
	Error:                "#f05b5b",
	Warning:              "#e2b93d",
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
