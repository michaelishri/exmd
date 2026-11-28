package render

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/michaelishri/exmd/internal/theme"
)

// escapeRE matches an SGR sequence (group 1 holds its parameters) or any OSC
// sequence, such as a glamour hyperlink.
var escapeRE = regexp.MustCompile(`\x1b\[([0-9;:]*)m|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)

// unpaintedCell returns the first printed character drawn without an
// explicit background, tracking SGR state the way a terminal does.
func unpaintedCell(line string) (string, bool) {
	hasBG := false
	pos := 0
	check := func(text string) (string, bool) {
		if text != "" && !hasBG {
			return text, true
		}
		return "", false
	}
	for _, m := range escapeRE.FindAllStringSubmatchIndex(line, -1) {
		if text, bad := check(line[pos:m[0]]); bad {
			return text, true
		}
		pos = m[1]
		if m[2] < 0 {
			continue // OSC
		}
		params := strings.Split(line[m[2]:m[3]], ";")
		for i := 0; i < len(params); i++ {
			n, _ := strconv.Atoi(params[i])
			switch {
			case params[i] == "" || n == 0 || n == 49:
				hasBG = false
			case n == 48:
				hasBG = true
				if i+1 < len(params) && params[i+1] == "5" {
					i += 2
				} else {
					i += 4
				}
			case n == 38:
				if i+1 < len(params) && params[i+1] == "5" {
					i += 2
				} else {
					i += 4
				}
			case (n >= 40 && n <= 47) || (n >= 100 && n <= 107):
				hasBG = true
			}
		}
	}
	return check(line[pos:])
}

// TestEveryCellIsPainted: with any colored theme, no character of the page
// falls back to the terminal's own background, whatever reset form glamour
// writes after bold text, links, inline code, quotes, tables or code.
func TestEveryCellIsPainted(t *testing.T) {
	d := loadSource(t, "---\ntype: runbook\n---\n# Title\n\n"+
		"Intro with **bold**, *italic*, ~~strike~~, `code`, and a [link](https://example.com).\n\n"+
		"- one\n- [x] done\n\n> Quote with **bold**.\n\n| A | B |\n|---|--:|\n| a | 1 |\n\n---\n\n"+
		"```go\nfunc main() {}\n```\n\n```bash:exec\necho \"hi $X\"\n```\n")
	for _, name := range theme.Names() {
		th, err := theme.Resolve(name, true)
		if err != nil || th.Name == theme.Notty.Name {
			continue
		}
		out := Render(d, 70, th)
		painted := PaintCanvas(strings.Join(out.Lines, "\n"), 70, th)
		for i, line := range strings.Split(painted, "\n") {
			if text, bad := unpaintedCell(line); bad {
				t.Fatalf("%s: line %d draws %q on the terminal background:\n%q", name, i, text, line)
			}
		}
	}
}
