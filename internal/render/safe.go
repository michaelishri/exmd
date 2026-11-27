package render

import (
	"strings"

	"github.com/michaelishri/exmd/internal/termsafe"
)

// safeLines neutralizes terminal control characters in the document source
// before it is rendered. Inside fenced code — the text a reader reviews
// before running it — every control character is shown as a marker (ESC
// becomes ^[), so code cannot hide what it does. In prose they are removed.
// Line count and order are unchanged, so source line numbers still hold.
func safeLines(lines []string) []string {
	out := make([]string, len(lines))
	var fenceChar byte
	fenceLen := 0
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t>")
		if fenceLen == 0 {
			if c, n := fenceRun(trimmed); n >= 3 && (c != '`' || !strings.ContainsRune(trimmed[n:], '`')) {
				fenceChar, fenceLen = c, n
				out[i] = termsafe.Visible(line)
				continue
			}
			out[i] = termsafe.Strip(line)
			continue
		}
		out[i] = termsafe.Visible(line)
		if c, n := fenceRun(trimmed); c == fenceChar && n >= fenceLen && strings.TrimSpace(trimmed[n:]) == "" {
			fenceLen = 0
		}
	}
	return out
}

// fenceRun returns the fence character a line starts with and how many
// times it repeats; n is 0 when the line does not start with ` or ~.
func fenceRun(s string) (byte, int) {
	if s == "" || (s[0] != '`' && s[0] != '~') {
		return 0, 0
	}
	n := 0
	for n < len(s) && s[n] == s[0] {
		n++
	}
	return s[0], n
}
