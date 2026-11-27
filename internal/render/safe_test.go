package render

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/michaelishri/exmd/internal/theme"
)

// hostileSource carries escape sequences in every place a document can hold
// text: prose, headings, a display fence, an executable block, and the title.
const hostileSource = "---\ntype: runbook\ntitle: \"t\\e]0;pwned\\a\"\n---\n" +
	"# Head \x1b]52;c;aGFja2Vk\x07 one\n\n" +
	"Text \x1b[31mred\x1b[0m and \x1b]8;;http://evil\x07link\x1b]8;;\x07 \x9b2J end.\n\n" +
	"```bash\necho \x1b[2J display\n```\n\n" +
	"```bash:exec\necho \x1b[8mhidden\x1b[28m shown\n```\n\n" +
	"~~~\ntilde \x07 fence\n~~~\n\nafter \x1b[1Kfence\n"

// sgrOnly matches the escapes the renderer itself emits: SGR styling only.
var sgrOnly = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

// TestRenderNeutralizesControlCharacters is the acceptance test for hostile
// documents: no escape from the source reaches the terminal, code shows a
// visible marker where a control character was, and prose drops it.
func TestRenderNeutralizesControlCharacters(t *testing.T) {
	d := loadSource(t, hostileSource)
	for _, th := range []theme.Theme{theme.Dark, theme.Notty} {
		out := Render(d, 80, th)
		joined := strings.Join(out.Lines, "\n")
		painted := PaintCanvas(joined, 80, th)

		rest := sgrOnly.ReplaceAllString(painted, "")
		for _, r := range rest {
			if (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
				t.Fatalf("%s: control %U reached the output:\n%q", th.Name, r, rest)
			}
		}

		plain := ansi.Strip(joined)
		for _, want := range []string{"echo ^[[2J display", "^[[8mhidden^[[28m shown", "tilde ^G fence"} {
			if !strings.Contains(plain, want) {
				t.Errorf("%s: code should show %q, got:\n%s", th.Name, want, plain)
			}
		}
		for _, banned := range []string{"52;c", "http://evil", "[31m", "[1K"} {
			if strings.Contains(plain, banned) {
				t.Errorf("%s: prose kept %q:\n%s", th.Name, banned, plain)
			}
		}
		if !strings.Contains(plain, "red and link") || !strings.Contains(plain, "after fence") {
			t.Errorf("%s: prose text lost around stripped escapes:\n%s", th.Name, plain)
		}
	}
}

// TestSafeLinesKeepsLineCount guards the line mapping the renderer depends on.
func TestSafeLinesKeepsLineCount(t *testing.T) {
	in := strings.Split(hostileSource, "\n")
	if got := safeLines(in); len(got) != len(in) {
		t.Fatalf("safeLines changed the line count: %d → %d", len(in), len(got))
	}
}
