package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/michaelishri/exmd/internal/theme"
)

// TestThemeSwitchChangesCodeColors guards against glamour's global chroma
// style registry pinning code colors to the first theme rendered in the
// process. Rendering themes in sequence must produce each theme's colors.
func TestThemeSwitchChangesCodeColors(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)

	d := loadDemo(t)
	codeLine := func(out *Output) string {
		block := out.Blocks[0]
		for i := block.Line + 1; i < block.End; i++ {
			if strings.Contains(out.Lines[i], "echo") {
				return out.Lines[i]
			}
		}
		return ""
	}

	nordFirst := codeLine(Render(d, 80, theme.Nord))
	latte := codeLine(Render(d, 80, theme.CatppuccinLatte))
	nordAgain := codeLine(Render(d, 80, theme.Nord))

	if nordFirst == "" || latte == "" {
		t.Fatal("no code lines rendered")
	}
	if nordFirst == latte {
		t.Fatal("switching themes did not change the code colors")
	}
	if nordFirst != nordAgain {
		t.Fatal("returning to the first theme produced different code colors")
	}
}
