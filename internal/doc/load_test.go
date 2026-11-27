package doc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadFixture(t *testing.T, name string, preflight bool) *Result {
	t.Helper()
	res, err := Load(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if preflight {
		res.Preflight()
	}
	return res
}

func allMessages(res *Result) string {
	var sb strings.Builder
	for _, d := range res.Diagnostics {
		sb.WriteString(d.Message)
		sb.WriteByte('\n')
	}
	return sb.String()
}

func TestLoadValid(t *testing.T) {
	tests := []struct {
		name        string
		file        string
		wantBlocks  int
		wantHeads   int
		wantGlobals []string // sorted names
	}{
		{"flow globals", "valid-flow.md", 2, 2, []string{"ENV", "JOB_ID", "NOTE"}},
		{"block globals", "valid-block.md", 1, 1, []string{"ENV", "JOB_ID"}},
		{"display only", "valid-display-only.md", 0, 0, nil},
		{"no globals key", "valid-no-globals.md", 1, 1, nil},
		{"list items", "valid-list-items.md", 2, 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := loadFixture(t, tt.file, false)
			if res.HasErrors() {
				t.Fatalf("unexpected errors:\n%s", allMessages(res))
			}
			if got := len(res.Doc.Blocks); got != tt.wantBlocks {
				t.Fatalf("blocks = %d, want %d", got, tt.wantBlocks)
			}
			if got := len(res.Doc.Headings); got != tt.wantHeads {
				t.Fatalf("headings = %d, want %d", got, tt.wantHeads)
			}
			var names []string
			for n := range res.Doc.Globals {
				names = append(names, n)
			}
			if tt.wantGlobals != nil && strings.Join(names, ",") != strings.Join(tt.wantGlobals, ",") {
				// order-insensitive compare
				m := map[string]bool{}
				for _, w := range tt.wantGlobals {
					m[w] = true
				}
				for _, n := range names {
					if !m[n] {
						t.Fatalf("unexpected global %q; declared = %v", n, names)
					}
					delete(m, n)
				}
				if len(m) != 0 {
					t.Fatalf("missing globals %v; declared = %v", m, names)
				}
			}
			if !res.Doc.IsRunbook {
				t.Fatal("expected IsRunbook")
			}
		})
	}
}

func TestLoadInvalid(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		preflight bool
		contains  []string // substrings that must appear across diagnostics
	}{
		{"kitchen sink", "invalid-many.md", true, []string{
			"no adapter for language `ruby`",
			"unknown block attribute `name`",
			"malformed block attribute `verbose`",
			"script file not found",
			"empty executable block",
			"empty script path",
		}},
		{"bad yaml", "invalid-bad-yaml.md", false, []string{"invalid frontmatter YAML"}},
		{"globals string form", "invalid-globals-string.md", false, []string{"`globals` must be a mapping"}},
		{"unknown attribute", "invalid-unknown-attribute.md", false, []string{"unknown block attribute `name`"}},
		{"malformed attribute", "invalid-malformed-attribute.md", false, []string{"malformed block attribute `verbose`"}},
		{"missing script", "invalid-missing-script.md", true, []string{"script file not found"}},
		{"empty body", "invalid-empty-body.md", false, []string{"empty executable block"}},
		{"globals names", "invalid-globals-names.md", false, []string{
			"invalid global name `bad-name`",
			"invalid global name `9lives`",
			"duplicate global `DUP`",
			"`globals` value for `LIST` must be a scalar",
		}},
		{"non-runbook", "invalid-non-runbook.md", false, []string{"not a runbook"}},
		{"spec version", "invalid-spec-2.md", false, []string{"unsupported spec version 2"}},
		{"unclosed frontmatter", "invalid-unclosed-frontmatter.md", false, []string{"frontmatter is not closed"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := loadFixture(t, tt.file, tt.preflight)
			if !res.HasErrors() {
				t.Fatalf("expected errors, got none")
			}
			msgs := allMessages(res)
			for _, want := range tt.contains {
				if !strings.Contains(msgs, want) {
					t.Errorf("diagnostics missing %q;\n%s", want, msgs)
				}
			}
		})
	}
}

func TestLoadCRLEndings(t *testing.T) {
	res := loadFixture(t, "valid-crlf.md", false)
	if res.HasErrors() {
		t.Fatalf("unexpected errors:\n%s", allMessages(res))
	}
	if got := len(res.Doc.Blocks); got != 1 {
		t.Fatalf("blocks = %d, want 1", got)
	}
	b := res.Doc.Blocks[0]
	if strings.Contains(b.Body, "\r") {
		t.Fatalf("body not normalized: %q", b.Body)
	}
}

func TestBlockEndLine(t *testing.T) {
	tests := []struct {
		name  string
		file  string
		want  []int // expected EndLine per block, in document order
		lines []int // expected Line per block
	}{
		{"embedded", "valid-block.md", []int{13}, []int{11}},
		{"list items", "valid-list-items.md", []int{9, 15}, []int{7, 13}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := loadFixture(t, tt.file, false)
			if res.HasErrors() {
				t.Fatalf("unexpected errors:\n%s", allMessages(res))
			}
			if len(res.Doc.Blocks) != len(tt.want) {
				t.Fatalf("blocks = %d, want %d", len(res.Doc.Blocks), len(tt.want))
			}
			for i, b := range res.Doc.Blocks {
				if b.Line != tt.lines[i] || b.EndLine != tt.want[i] {
					t.Errorf("block %d: line/end = %d/%d, want %d/%d", i, b.Line, b.EndLine, tt.lines[i], tt.want[i])
				}
			}
		})
	}
}

func TestBlockEndLineScriptRef(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	src := "---\ntype: runbook\n---\n\n```bash:exec\n@helper.sh\n```\n\nMy text.\n"
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	res, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(res.Doc.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(res.Doc.Blocks))
	}
	if b := res.Doc.Blocks[0]; b.Kind != KindScript || b.Line != 5 || b.EndLine != 7 {
		t.Fatalf("block kind/line/end = %v/%d/%d, want script/5/7", b.Kind, b.Line, b.EndLine)
	}
}

func TestBlockEndLineWithTrailingBlankBody(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	src := "---\ntype: runbook\n---\n\n```bash:exec\necho hi\n\n```\n\nMy text.\n"
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	res, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(res.Doc.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(res.Doc.Blocks))
	}
	if b := res.Doc.Blocks[0]; b.Line != 5 || b.BodyLine != 6 || b.EndLine != 8 {
		t.Fatalf("block line/body/end = %d/%d/%d, want 5/6/8", b.Line, b.BodyLine, b.EndLine)
	}
}

func TestValidListItemsOrder(t *testing.T) {
	res := loadFixture(t, "valid-list-items.md", false)
	if len(res.Doc.Blocks) != 2 {
		t.Fatalf("blocks = %d", len(res.Doc.Blocks))
	}
	if res.Doc.Blocks[0].Index != 1 || res.Doc.Blocks[1].Index != 2 {
		t.Fatalf("indices not assigned: %d, %d", res.Doc.Blocks[0].Index, res.Doc.Blocks[1].Index)
	}
	if res.Doc.Blocks[0].Line >= res.Doc.Blocks[1].Line {
		t.Fatalf("blocks out of document order: %d then %d", res.Doc.Blocks[0].Line, res.Doc.Blocks[1].Line)
	}
}

func TestResolveGlobals(t *testing.T) {
	res := loadFixture(t, "valid-flow.md", false)
	d := res.Doc

	got, diags := d.ResolveGlobals(nil)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got["ENV"] != "staging" {
		t.Fatalf("ENV = %q", got["ENV"])
	}
	if got["NOTE"] != "hello from frontmatter" {
		t.Fatalf("NOTE = %q", got["NOTE"])
	}
	if _, present := got["JOB_ID"]; present {
		t.Fatal("bare JOB_ID should start absent")
	}

	got, _ = d.ResolveGlobals(map[string]string{"ENV": "prod", "TYPO": "x"})
	if got["ENV"] != "prod" {
		t.Fatalf("ENV override = %q", got["ENV"])
	}
	var errs []Diagnostic
	_, diags = d.ResolveGlobals(map[string]string{"TYPO": "x"})
	for _, dg := range diags {
		if dg.Severity == SeverityError {
			errs = append(errs, dg)
		}
	}
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "unknown global `TYPO`") {
		t.Fatalf("expected unknown-global error, got %v", diags)
	}
}

func TestNonRunbookInert(t *testing.T) {
	// A non-runbook document must collect no blocks (SPEC.md §2.1).
	res := loadFixture(t, "invalid-non-runbook.md", false)
	if len(res.Doc.Blocks) != 0 {
		t.Fatalf("non-runbook collected %d blocks", len(res.Doc.Blocks))
	}
}

func TestScriptResolution(t *testing.T) {
	res := loadFixture(t, "invalid-missing-script.md", false)
	b := res.Doc.Blocks[0]
	if b.Kind != KindScript {
		t.Fatalf("kind = %v", b.Kind)
	}
	if !filepath.IsAbs(b.ScriptAbs) {
		t.Fatalf("ScriptAbs not absolute: %q", b.ScriptAbs)
	}
	if b.ScriptPath != "does/not/exist.sh" {
		t.Fatalf("ScriptPath = %q", b.ScriptPath)
	}
	if _, err := os.Stat(b.ScriptAbs); !os.IsNotExist(err) {
		t.Fatalf("expected missing file")
	}
}

// TestHeadingLines covers the source span of each heading form, and keeps
// headings inside containers out of the document outline.
func TestHeadingLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	src := "---\ntype: runbook\n---\n# ATX\n\nSetext\n======\n\nTwo\nlines\n---\n\n#\n\n> ## Quoted\n\n- ## Listed\n"
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []Heading{
		{Level: 1, Text: "ATX", Line: 4, EndLine: 4},
		{Level: 1, Text: "Setext", Line: 6, EndLine: 7},
		{Level: 2, Text: "Two lines", Line: 9, EndLine: 11},
		{Level: 1, Text: "", Line: 13, EndLine: 13},
	}
	if len(res.Doc.Headings) != len(want) {
		t.Fatalf("headings = %d, want %d", len(res.Doc.Headings), len(want))
	}
	for i, h := range res.Doc.Headings {
		if *h != want[i] {
			t.Errorf("heading %d = %+v, want %+v", i, *h, want[i])
		}
	}
}

// TestFrontmatterYAMLErrors pins how YAML problems are reported: on the file
// line the parser points at, without its frontmatter-relative "line N:".
func TestFrontmatterYAMLErrors(t *testing.T) {
	for _, tc := range []struct {
		name, yaml string
		wantLine   int
		wantMsg    string
	}{
		{"unclosed flow sequence", "type: [runbook", 3, "did not find expected ',' or ']'"},
		{"tab indentation", "type: runbook\n\tbad: tab", 3, "found a tab character that violates indentation"},
		{"two values on one key", "type: runbook\nkey: value: other", 3, "mapping values are not allowed in this context"},
		{"float spec", "type: runbook\nspec: 1.5", 3, "frontmatter `spec` must be an integer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "doc.md")
			if err := os.WriteFile(path, []byte("---\n"+tc.yaml+"\n---\n# T\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			res, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			var found bool
			for _, d := range res.Diagnostics {
				if strings.Contains(d.Message, tc.wantMsg) {
					found = true
					if d.Line != tc.wantLine {
						t.Errorf("line = %d, want %d (%s)", d.Line, tc.wantLine, d.Message)
					}
					if yamlLineRE.MatchString(d.Message) {
						t.Errorf("message keeps the parser's relative line: %q", d.Message)
					}
				}
			}
			if !found {
				t.Fatalf("no diagnostic containing %q: %+v", tc.wantMsg, res.Diagnostics)
			}
		})
	}
}
