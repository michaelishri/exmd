package render

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/michaelishri/exmd/internal/doc"
	"github.com/michaelishri/exmd/internal/theme"
)

var indexedColor = regexp.MustCompile(`\x1b\[(3[0-7]|9[0-7]|4[0-7]|10[0-7])m`)

func loadSource(t *testing.T, src string) *doc.Document {
	t.Helper()
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	res, err := doc.Load(path)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	return res.Doc
}

// TestAcceptanceThemes runs the full render + paint pipeline over documents
// that exercise each content type, at several widths, for both themes. It
// checks the properties the reader relies on rather than exact snapshots:
// full-width painted cells, no terminal-indexed colors, and no escapes at
// all in notty mode.
func TestAcceptanceThemes(t *testing.T) {
	sources := map[string]string{
		"empty": "---\ntype: runbook\n---\n",
		"prose": "---\ntype: runbook\n---\n\n# Title\n\nSome [prose](https://example.com) with `code`.\n",
		"exec":  "---\ntype: runbook\n---\n\n```bash:exec\necho hello\n```\n",
		"mixed": "---\ntype: runbook\n---\n\n# Title\n\n## 1. Section\n\n`export` here.\n\n" +
			"```bash:exec\necho hello\n```\n\n```bash\nplain fence\n```\n",
	}

	for name, src := range sources {
		d := loadSource(t, src)
		for _, width := range []int{40, 80, 120} {
			for _, th := range []theme.Theme{theme.Dark, theme.Notty} {
				t.Run(name+"/"+th.Name+"/"+strconv.Itoa(width), func(t *testing.T) {
					out := Render(d, width, th)
					painted := PaintCanvas(strings.Join(out.Lines, "\n"), width, th)
					lines := strings.Split(painted, "\n")

					if th.Name == theme.Notty.Name {
						if strings.Contains(painted, "\x1b[") {
							t.Fatalf("notty output contains escapes: %q", painted)
						}
					} else {
						canvas := backgroundEscape(th.Background)
						for i, line := range lines {
							if !strings.HasPrefix(line, canvas) {
								t.Errorf("line %d has no canvas background", i)
							}
							if !strings.HasSuffix(line, "\x1b[0m") {
								t.Errorf("line %d has no closing reset", i)
							}
							if w := ansi.StringWidth(line); w != width {
								t.Errorf("line %d width = %d, want %d", i, w, width)
							}
						}
						if indexedColor.MatchString(painted) {
							t.Error("output uses a terminal-indexed color")
						}
					}

					// Structure is independent of the palette.
					if len(out.Headings) != len(d.Headings) ||
						len(out.Blocks) != len(d.Blocks) {
						t.Error("render marks do not match the document")
					}
				})
			}
		}
	}
}
