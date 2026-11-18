package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOutlinePaddingAndSpacing checks the sidebar geometry: the strip is
// full height, its first row is the document's first h1 as the title, then
// a blank pad row, the packed entries between the pads (one column of base
// left padding, no blank lines between entries), blank shaded rows filling
// the rest, and the outer margin rows outside the strip.
func TestOutlinePaddingAndSpacing(t *testing.T) {
	m := newViewer(t, 120, 40)
	rows := m.content.Height
	v := m.outline.view(rows)
	lines := strings.Split(v, "\n")
	if len(lines) != rows {
		t.Fatalf("sidebar rows = %d, want %d", len(lines), rows)
	}

	// Title row: the document's first h1.
	if !strings.Contains(lines[0], "md-exec feature tour") {
		t.Fatalf("title row missing the first h1: %q", lines[0])
	}
	// Blank internal pad row under the title.
	if lines[1] == "" || strings.TrimSpace(lines[1]) != "" {
		t.Fatalf("internal pad row missing under the title: %q", lines[1])
	}

	// The entries sit between the pads: the first entry (an h1, selected
	// by the scroll-spy) with its marker, one column in.
	n := len(m.outline.entries)
	if n < 2 {
		t.Fatalf("demo should have several headings, got %d", n)
	}
	if n > rows-3 {
		t.Fatalf("demo headings exceed the sidebar: %d", n)
	}
	if !strings.HasPrefix(lines[2], " ▸ md-exec feature tour") {
		t.Fatalf("first entry wrong: %q", lines[2])
	}
	for r := 3; r <= n+1; r++ {
		if strings.TrimSpace(lines[r]) == "" {
			t.Fatalf("row %d is blank: blank lines were removed", r)
		}
		if !strings.HasPrefix(lines[r], " ") {
			t.Fatalf("row %d has no base padding: %q", r, lines[r])
		}
	}
	// Blank shaded rows fill the rest of the column — full height, with
	// the outer margins sitting outside the strip.
	if lines[len(lines)-1] == "" {
		t.Fatal("strip should be painted to the bottom of the body")
	}
}

// TestOutlineTitleIsFirstH1 checks that the sidebar title is the document's
// first level-1 heading even when it is not the first body line.
func TestOutlineTitleIsFirstH1(t *testing.T) {
	src := `---
type: runbook
title: Fixture
spec: 1
---

Intro prose before any heading.

## An h2 comes first

# The Real Title

More text after the h1.
`
	path := filepath.Join(t.TempDir(), "late-h1.md")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newViewerFromDoc(t, 120, 40, path, nil)
	if m.outline.title != "The Real Title" {
		t.Fatalf("sidebar title = %q, want the first h1 %q", m.outline.title, "The Real Title")
	}
	v := m.outline.view(m.content.Height)
	if !strings.Contains(strings.Split(v, "\n")[0], "The Real Title") {
		t.Fatalf("title row wrong: %q", strings.Split(v, "\n")[0])
	}
}

// TestOutlineTitleFallback checks documents without an h1: the title falls
// back to the document title (frontmatter, else file name).
func TestOutlineTitleFallback(t *testing.T) {
	m := newViewerFromDoc(t, 120, 40, "../../internal/doc/testdata/valid-flow.md", nil)
	if m.outline.title != "Flow globals" {
		t.Fatalf("sidebar title = %q, want the frontmatter title %q", m.outline.title, "Flow globals")
	}
}
