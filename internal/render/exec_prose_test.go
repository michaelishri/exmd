package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/michaelishri/exmd/internal/theme"
)

// TestProseAfterExecBlockIsOutsideSurface covers the reported bug: text
// following an executable fence must render on the page, not inside the
// executable block's shaded surface.
func TestProseAfterExecBlockIsOutsideSurface(t *testing.T) {
	d := loadSource(t, "---\ntype: runbook\n---\n\n"+
		"```bash:exec\n@scripts/myscript.sh\n```\n\nMy text.\n")
	out := Render(d, 80, theme.Dark)
	if len(out.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(out.Blocks))
	}
	bm := out.Blocks[0]
	execEscape := backgroundEscape(theme.Dark.ExecPanel)
	if strings.Contains(out.Lines[bm.Line], execEscape) {
		t.Fatalf("badge sits on the exec surface: %q", out.Lines[bm.Line])
	}
	if bm.End >= len(out.Lines) {
		t.Fatalf("block extent %d past output (%d lines)", bm.End, len(out.Lines))
	}
	foundCode := false
	for i := bm.Line + 1; i < bm.End; i++ {
		if !strings.Contains(out.Lines[i], execEscape) {
			t.Fatalf("line %d in block has no exec surface: %q", i, out.Lines[i])
		}
		if strings.Contains(ansi.Strip(out.Lines[i]), "@scripts/myscript.sh") {
			foundCode = true
		}
	}
	if !foundCode {
		t.Fatal("script reference not rendered inside the block")
	}
	if strings.Contains(out.Lines[bm.End], execEscape) ||
		strings.TrimSpace(ansi.Strip(out.Lines[bm.End])) != "" {
		t.Fatalf("no plain spacing row below the block: %q", out.Lines[bm.End])
	}
	prose := out.Lines[bm.End+1:]
	joined := strings.Join(prose, "\n")
	if !strings.Contains(ansi.Strip(joined), "My text.") {
		t.Fatalf("prose after the block is missing: %q", ansi.Strip(joined))
	}
	for i := bm.End + 1; i < len(out.Lines); i++ {
		if strings.Contains(out.Lines[i], execEscape) {
			t.Fatalf("prose line %d painted with the exec surface: %q", i, out.Lines[i])
		}
	}
}

// TestDisplayFenceAfterExecBlockIsNotShaded checks that a display-only fence
// following an executable block is its own chunk and keeps the page surface.
func TestDisplayFenceAfterExecBlockIsNotShaded(t *testing.T) {
	d := loadSource(t, "---\ntype: runbook\n---\n\n"+
		"```bash:exec\necho hello\n```\n\n```bash\nplain fence\n```\n\nAfter.\n")
	out := Render(d, 80, theme.Dark)
	if len(out.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(out.Blocks))
	}
	bm := out.Blocks[0]
	execEscape := backgroundEscape(theme.Dark.ExecPanel)
	for i := bm.End + 1; i < len(out.Lines); i++ {
		if strings.Contains(out.Lines[i], execEscape) {
			t.Fatalf("line %d after the block has the exec surface: %q", i, out.Lines[i])
		}
	}
	joined := ansi.Strip(strings.Join(out.Lines[bm.End+1:], "\n"))
	for _, want := range []string{"plain fence", "After."} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q after the block: %q", want, joined)
		}
	}
}

// TestHeadingAfterBlockProse checks that prose does not hide a following
// heading's mark or number, even with no blank line before the heading.
func TestHeadingAfterBlockProse(t *testing.T) {
	d := loadSource(t, "---\ntype: runbook\n---\n\n"+
		"```bash:exec\necho hello\n```\n\nMy text.\n## Step\n\nEnd.\n")
	out := Render(d, 80, theme.Dark)
	if len(out.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(out.Blocks))
	}
	if len(out.Headings) != 1 {
		t.Fatalf("headings = %d, want 1", len(out.Headings))
	}
	bm, hm := out.Blocks[0], out.Headings[0]
	if hm.Line <= bm.End {
		t.Fatalf("heading mark %d not after block extent %d", hm.Line, bm.End)
	}
	execEscape := backgroundEscape(theme.Dark.ExecPanel)
	if strings.Contains(out.Lines[hm.Line], execEscape) {
		t.Fatalf("heading painted with exec surface: %q", out.Lines[hm.Line])
	}
	joined := ansi.Strip(strings.Join(out.Lines[bm.End+1:hm.Line], "\n"))
	if !strings.Contains(joined, "My text.") {
		t.Fatalf("prose before heading missing: %q", joined)
	}
	if hm.Number == "" || !strings.Contains(ansi.Strip(out.Lines[hm.Line]), "Step") {
		t.Fatalf("heading mark wrong: %+v / %q", hm, out.Lines[hm.Line])
	}
}

// TestAdjacentExecBlocks checks that a block ending immediately before the
// next one still renders both blocks with their own surfaces.
func TestAdjacentExecBlocks(t *testing.T) {
	d := loadSource(t, "---\ntype: runbook\n---\n\n"+
		"```bash:exec\necho one\n```\n```bash:exec\necho two\n```\n\nMy text.\n")
	out := Render(d, 80, theme.Dark)
	if len(out.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2", len(out.Blocks))
	}
	execEscape := backgroundEscape(theme.Dark.ExecPanel)
	for i, bm := range out.Blocks {
		if bm.End >= len(out.Lines) {
			t.Fatalf("block %d extent past output", i)
		}
		code := ""
		for l := bm.Line + 1; l < bm.End; l++ {
			if strings.Contains(ansi.Strip(out.Lines[l]), "echo") {
				code = out.Lines[l]
			}
		}
		if !strings.Contains(code, execEscape) {
			t.Fatalf("block %d code row has no exec surface: %q", i, code)
		}
	}
	last := out.Blocks[1].End
	joined := ansi.Strip(strings.Join(out.Lines[last+1:], "\n"))
	if !strings.Contains(joined, "My text.") {
		t.Fatalf("prose after adjacent blocks missing: %q", joined)
	}
	for i := last + 1; i < len(out.Lines); i++ {
		if strings.Contains(out.Lines[i], execEscape) {
			t.Fatalf("prose line %d painted with exec surface: %q", i, out.Lines[i])
		}
	}
}

// TestProseAfterExecBlockNotty checks the unstyled pipeline still separates
// the block from following prose.
func TestProseAfterExecBlockNotty(t *testing.T) {
	d := loadSource(t, "---\ntype: runbook\n---\n\n"+
		"```bash:exec\necho hello\n```\n\nMy text.\n")
	out := Render(d, 80, theme.Notty)
	if len(out.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(out.Blocks))
	}
	bm := out.Blocks[0]
	if bm.End >= len(out.Lines) {
		t.Fatalf("block extent %d past output (%d lines)", bm.End, len(out.Lines))
	}
	if !strings.Contains(ansi.Strip(strings.Join(out.Lines[bm.End+1:], "\n")), "My text.") {
		t.Fatalf("prose after block missing: %q", out.Lines)
	}
	for _, line := range out.Lines {
		if strings.Contains(line, "\x1b[") {
			t.Fatalf("notty output contains escapes: %q", line)
		}
	}
}
