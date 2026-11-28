package tui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var tuiSGR = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

func controlIn(s string) (rune, bool) {
	for _, r := range tuiSGR.ReplaceAllString(s, "") {
		if (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return r, true
		}
	}
	return 0, false
}

// TestViewerNeutralizesControlCharacters opens a document whose title,
// headings, path and code carry escape sequences: the frame must contain
// only the viewer's own styling, and the window title must be plain.
func TestViewerNeutralizesControlCharacters(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "d\x1b[2Jir")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "doc.md")
	src := "---\ntype: runbook\ntitle: \"T\\e]0;pwned\\a\"\n---\n" +
		"# One \x1b]52;c;aGk=\x07\n\n## Two \x1b[8mhidden\n\n" +
		"```bash:exec\necho \x1b[2J hi\n```\n"
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, w := range []int{120, 60} {
		m := newViewerFromDoc(t, w, 30, path, nil)
		if r, ok := controlIn(m.View().Content); ok {
			t.Fatalf("width %d: control %U reached the frame", w, r)
		}
		m = pressKey(m, runeKey('t'))
		if r, ok := controlIn(m.View().Content); ok {
			t.Fatalf("width %d, blocks tab: control %U reached the frame", w, r)
		}
		title := m.View().WindowTitle
		if r, ok := controlIn(title); ok {
			t.Fatalf("window title carries control %U: %q", r, title)
		}
	}
}

// TestBlockOutputDropsControlCharacters feeds hostile output to an inline
// box: escape sequences and control characters never reach stored lines.
func TestBlockOutputDropsControlCharacters(t *testing.T) {
	m := newViewer(t, 120, 40)
	st := m.stateFor(0)
	st.write([]byte("a\x1b]52;c;aGk=\x07b\x1b[31mc\x1b[0m\x07\x9b\n"))
	st.write([]byte("tail\x1b[2J"))
	m.finalizeInline(0, nil)
	got := strings.Join(st.lines, "\n")
	if got != "abc\ntail" {
		t.Fatalf("lines = %q, want %q", got, "abc\ntail")
	}
}
