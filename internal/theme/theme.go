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

	// Primary marks state: the current outline row, selection markers.
	// It is not used for decoration.
	Primary Color

	Success Color
	Error   Color
	Warning Color

	// Running marks an in-flight execution in the gutter (orange).
	Running Color
}

// Palette holds the content colors shared by the renderer and the UI.
// Both layers must use the same values for the same role.
var Palette = struct {
	Text, Bright, Soft, Muted, Accent, Panel, Rule Color
}{
	Text:   "#e4e4e9",
	Bright: "#f4f4f8",
	Soft:   "#c8c8d0",
	Muted:  "#8a8a95",
	Accent: "#8f8af4",
	Panel:  "#1a1a21",
	Rule:   "#3a3a44",
}

// Dark is the default theme.
var Dark = Theme{
	Name:              "dark",
	Background:        "#0e0e11",
	BackgroundPanel:   "#18181e",
	BackgroundElement: "#24242c",
	Border:            "#2b2b34",
	Separator:         "#444448",
	Text:              Palette.Text,
	TextMuted:         Palette.Muted,
	Primary:           Palette.Accent,
	Success:           "#6fbf73",
	Error:             "#f05b5b",
	Warning:           "#e2b93d",
	Running:           "#ff9e64",
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
