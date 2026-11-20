package tui

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// clipPayloads decodes each OSC 52 copy sequence in a raw clipboard write.
func clipPayloads(t *testing.T, seq string) []string {
	t.Helper()
	const prefix = "\x1b]52;c;"
	parts := strings.Split(seq, prefix)
	if len(parts) < 2 {
		t.Fatalf("no OSC 52 copy sequences in %q", seq)
	}
	var out []string
	for _, part := range parts[1:] {
		end := strings.IndexByte(part, '\a')
		if end < 0 {
			t.Fatalf("unterminated OSC 52 sequence: %q", part)
		}
		payload, err := base64.StdEncoding.DecodeString(part[:end])
		if err != nil {
			t.Fatalf("decode clipboard payload: %v", err)
		}
		out = append(out, string(payload))
	}
	return out
}

func pressU(m Model) Model {
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	return mm.(Model)
}

func TestOutputURLs(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   string
		want []string
	}{
		{"single", "go to https://example.com/x now", []string{"https://example.com/x"}},
		{
			"punctuation",
			"see https://a.test/x, https://b.test/y. done!",
			[]string{"https://a.test/x", "https://b.test/y"},
		},
		{
			"duplicates",
			"https://x.test/a https://x.test/a https://x.test/b",
			[]string{"https://x.test/a", "https://x.test/b"},
		},
		{
			"query",
			"https://x.test/s?a=1&b=2",
			[]string{"https://x.test/s?a=1&b=2"},
		},
		{
			"parentheses",
			"(https://x.test/a_(b)) and https://x.test/c)",
			[]string{"https://x.test/a_(b)", "https://x.test/c"},
		},
		{
			"multiline",
			"first https://a.test\nsecond https://b.test",
			[]string{"https://a.test", "https://b.test"},
		},
		{"no scheme", "example.com and www.foo.test", nil},
		{"case", "HTTPS://EXAMPLE.COM/X", []string{"HTTPS://EXAMPLE.COM/X"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := outputURLs(tt.in); !slices.Equal(got, tt.want) {
				t.Fatalf("outputURLs(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestCopyURLCycles checks that repeated u presses copy each distinct URL in
// order and wrap back to the first.
func TestCopyURLCycles(t *testing.T) {
	m := runSelectedBlock(t, enterBlockModeFromFirstHeading(t,
		newViewerFromDoc(t, 120, 40, "testdata/urls.md", nil)))

	seq := captureClipboard(t, func() {
		for range 4 {
			m = pressU(m)
		}
	})
	want := []string{
		"https://example.com/one",
		"https://example.org/two",
		"https://example.com/a_(b)",
		"https://example.com/one",
	}
	if got := clipPayloads(t, seq); !slices.Equal(got, want) {
		t.Fatalf("copied URLs = %v, want %v", got, want)
	}
	if !strings.Contains(m.flash, "copied URL 1/3:") {
		t.Fatalf("footer reaction = %q, want URL 1 of 3", m.flash)
	}
}

func TestCopyURLSingleFlash(t *testing.T) {
	src := `---
type: runbook
---

## Link

` + "```bash:exec\necho https://only.test/x\n```\n"
	path := filepath.Join(t.TempDir(), "one-url.md")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	m := runSelectedBlock(t, enterBlockModeFromFirstHeading(t,
		newViewerFromDoc(t, 120, 40, path, nil)))
	seq := captureClipboard(t, func() { m = pressU(m) })
	if got := clipPayloads(t, seq); !slices.Equal(got, []string{"https://only.test/x"}) {
		t.Fatalf("copied URLs = %v", got)
	}
	if !strings.Contains(m.flash, "copied URL: https://only.test/x") {
		t.Fatalf("footer reaction = %q, want a single-URL confirmation", m.flash)
	}
}

func TestCopyURLResetsOnRerun(t *testing.T) {
	m := runSelectedBlock(t, enterBlockModeFromFirstHeading(t,
		newViewerFromDoc(t, 120, 40, "testdata/urls.md", nil)))
	first := captureClipboard(t, func() {
		m = pressU(m)
		m = pressU(m)
	})
	if got := clipPayloads(t, first); got[len(got)-1] != "https://example.org/two" {
		t.Fatalf("copied URLs = %v, want to end at the second URL", got)
	}

	m = runSelectedBlock(t, m) // re-run the same block
	again := captureClipboard(t, func() { m = pressU(m) })
	if got := clipPayloads(t, again); !slices.Equal(got, []string{"https://example.com/one"}) {
		t.Fatalf("copied URL after re-run = %v, want the first URL", got)
	}
}

func TestCopyURLWithoutURL(t *testing.T) {
	m := runSelectedBlock(t, enterBlockModeFromFirstHeading(t, newViewer(t, 120, 40)))
	seq := captureClipboard(t, func() { m = pressU(m) })
	if seq != "" {
		t.Fatalf("clipboard written with no URLs: %q", seq)
	}
	if !strings.Contains(m.flash, "no URL in this block's output") {
		t.Fatalf("footer reaction = %q, want a no-URL message", m.flash)
	}
}

func TestCopyURLWithoutBlock(t *testing.T) {
	m := newViewer(t, 120, 40)
	seq := captureClipboard(t, func() { m = pressU(m) })
	if seq != "" {
		t.Fatalf("clipboard written with no block: %q", seq)
	}
	if !strings.Contains(m.flash, "select a block") {
		t.Fatalf("footer reaction = %q, want a hint to select a block", m.flash)
	}
}

func TestURLHintAppearsOnlyWithURLs(t *testing.T) {
	m := enterBlockModeFromFirstHeading(t, newViewerFromDoc(t, 120, 40, "testdata/urls.md", nil))
	footer := func(m Model) string {
		lines := strings.Split(m.View(), "\n")
		return ansi.Strip(lines[len(lines)-1])
	}
	if strings.Contains(footer(m), "u url") {
		t.Fatal("u hint shown before the block has output")
	}
	m = runSelectedBlock(t, m)
	if !strings.Contains(footer(m), "u url") {
		t.Fatalf("u hint missing with URLs in output: %q", footer(m))
	}
}

func TestCopyURLFailureKeepsCursor(t *testing.T) {
	m := runSelectedBlock(t, enterBlockModeFromFirstHeading(t,
		newViewerFromDoc(t, 120, 40, "testdata/urls.md", nil)))

	prev := clipboardOut
	clipboardOut = errWriter{err: errors.New("clipboard unavailable")}
	t.Setenv("TERM", "xterm-256color")
	m = pressU(m)
	if !strings.Contains(m.flash, "copy failed: clipboard unavailable") {
		t.Fatalf("footer reaction = %q, want the write error", m.flash)
	}
	clipboardOut = prev

	seq := captureClipboard(t, func() { m = pressU(m) })
	if got := clipPayloads(t, seq); !slices.Equal(got, []string{"https://example.com/one"}) {
		t.Fatalf("copied URL after failure = %v, want to retry the first URL", got)
	}
}
