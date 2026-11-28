package render

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/michaelishri/exmd/internal/theme"
)

// trueColorSeq is the SGR parameter string for a theme color: 38;2;R;G;B for
// a foreground, 48;2;R;G;B for a background.
func trueColorSeq(c theme.Color, bg bool) string {
	r, g, b, _ := lipgloss.Color(string(c)).RGBA()
	kind := 38
	if bg {
		kind = 48
	}
	return fmt.Sprintf("%d;2;%d;%d;%d", kind, r>>8, g>>8, b>>8)
}
