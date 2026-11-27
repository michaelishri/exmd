package exec

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/michaelishri/exmd/internal/doc"
)

// runSource writes a runbook (plus extra files beside it) and runs it.
func runSource(t *testing.T, src string, files map[string]string) (*RunResult, string, string) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := doc.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	res.Preflight()
	if res.HasErrors() {
		t.Fatalf("fixture has errors:\n%s", diagText(res))
	}
	var out, errOut bytes.Buffer
	result, err := Run(context.Background(), res.Doc, Options{Stdout: &out, Stderr: &errOut})
	if err != nil {
		t.Fatal(err)
	}
	return result, out.String(), errOut.String()
}

func bashDoc(blocks ...string) string {
	var sb strings.Builder
	sb.WriteString("---\ntype: runbook\nglobals: {A, B}\n---\n")
	for _, b := range blocks {
		sb.WriteString("\n```bash:exec\n" + b + "\n```\n")
	}
	return sb.String()
}

func hasWarning(r *RunResult, part string) bool {
	for _, w := range r.Warnings {
		if strings.Contains(w, part) {
			return true
		}
	}
	return false
}

// TestBashHarnessUnderBlockTraps covers the ways a block's own EXIT trap
// and shell options used to defeat the harvest. Each case must keep its
// exports, its exit status, and the observable trap behaviour (§5.1.4).
func TestBashHarnessUnderBlockTraps(t *testing.T) {
	cases := []struct {
		name       string
		block      string
		wantOut    []string
		wantStatus int
		wantA      string
		wantB      string
	}{
		{name: "block EXIT trap runs and exports survive",
			block:   "trap 'echo cleanup' EXIT\nexport A=1",
			wantOut: []string{"cleanup"}, wantA: "1"},
		{name: "handler sees the exit status",
			block:   "trap 'echo \"rc=$?\"' EXIT\nexport A=1\nexit 3",
			wantOut: []string{"rc=3"}, wantStatus: 3, wantA: "1"},
		{name: "exports made in the handler are harvested",
			block: "trap 'export B=2' EXIT\nexport A=1", wantA: "1", wantB: "2"},
		{name: "handler calling exit keeps earlier exports",
			block: "trap 'exit 5' EXIT\nexport A=1", wantStatus: 5, wantA: "1"},
		{name: "lowercase and numeric EXIT specs",
			block:   "trap 'echo zero' 0\ntrap 'echo low' exit\nexport A=1",
			wantOut: []string{"low"}, wantA: "1"},
		{name: "removing the handler",
			block: "trap 'echo nope' EXIT\ntrap - EXIT\nexport A=1", wantA: "1"},
		{name: "trap -p reports the block's handler",
			block:   "trap \"echo 'q'\" EXIT\ntrap -p EXIT\ntrap -p | head -1\ntrap - EXIT",
			wantOut: []string{`trap -- 'echo '\''q'\''' EXIT` + "\n" + `trap -- 'echo '\''q'\''' EXIT`}},
		{name: "saved handler restored with eval",
			block:   "trap 'echo restored' EXIT\ns=$(trap -p EXIT)\ntrap - EXIT\neval \"$s\"\nexport A=1",
			wantOut: []string{"restored"}, wantA: "1"},
		{name: "quotes survive a trap -p round trip",
			block:   `trap "echo 'it''s' \"q\" back\\\\slash" EXIT` + "\ns=$(trap -p EXIT)\ntrap - EXIT\neval \"$s\"\nexport A=1",
			wantOut: []string{`its q back\slash`}, wantA: "1"},
		{name: "other signals pass through",
			block:   "trap 'echo int' INT\ntrap '' TERM\ntrap -p INT TERM\nexport A=1",
			wantOut: []string{"trap -- 'echo int' SIGINT", "trap -- '' SIGTERM"}, wantA: "1"},
		{name: "subshell traps stay the subshell's",
			block:   "( trap 'echo sub' EXIT; export B=9 )\nexport A=1",
			wantOut: []string{"sub"}, wantA: "1"},
		{name: "IFS, noclobber and nounset",
			block: "IFS=''\nset -Cu\nexport A=1", wantA: "1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, out, errOut := runSource(t, bashDoc(c.block), nil)
			if r.ExitStatus != c.wantStatus {
				t.Errorf("status = %d, want %d (stderr %q)", r.ExitStatus, c.wantStatus, errOut)
			}
			if errOut != "" {
				t.Errorf("unexpected stderr: %q", errOut)
			}
			for _, w := range c.wantOut {
				if !strings.Contains(out, w) {
					t.Errorf("stdout %q missing %q", out, w)
				}
			}
			if strings.Contains(out, "nope") || strings.Contains(out, "zero") {
				t.Errorf("a replaced or removed handler ran: %q", out)
			}
			if got := r.Globals["A"]; got != c.wantA {
				t.Errorf("A = %q, want %q", got, c.wantA)
			}
			if got := r.Globals["B"]; got != c.wantB {
				t.Errorf("B = %q, want %q", got, c.wantB)
			}
			if hasWarning(r, "no environment dump") {
				t.Errorf("unexpected missing-dump warning: %v", r.Warnings)
			}
		})
	}
}

// TestReplacedProcessWarns: a block that execs away leaves no dump; the
// lost exports are reported instead of silently dropped.
func TestReplacedProcessWarns(t *testing.T) {
	r, _, _ := runSource(t, bashDoc("export A=1\nexec true"), nil)
	if _, ok := r.Globals["A"]; ok {
		t.Fatalf("A harvested from a block with no dump")
	}
	if !hasWarning(r, "block 1 left no environment dump") {
		t.Fatalf("warnings = %v, want a missing-dump warning", r.Warnings)
	}
}

// TestTruncatedDumpIgnored: a dump cut short (no end record) is not applied,
// so a half-written value never becomes a global.
func TestTruncatedDumpIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, []byte(bashDoc("true")), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := doc.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSession(res.Doc, Options{HostEnv: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	envOut := filepath.Join(t.TempDir(), "env")
	for _, tc := range []struct {
		data string
		want bool
	}{
		{"A=trunc", false},
		{"A=full\x00" + dumpEnd + "\x00", true},
	} {
		if err := os.WriteFile(envOut, []byte(tc.data), 0o600); err != nil {
			t.Fatal(err)
		}
		delete(s.globals, "A")
		s.harvest(res.Doc.Blocks[0], nil, envOut, nil)
		if _, ok := s.globals["A"]; ok != tc.want {
			t.Errorf("dump %q: A harvested = %v, want %v", tc.data, ok, tc.want)
		}
	}
}

// TestPythonImportsBesideSource: blocks run as if started with
// `python3 FILE`, so modules beside the script — or beside the document for
// an embedded block — import, and sys.argv[0] names the file.
func TestPythonImportsBesideSource(t *testing.T) {
	src := "---\ntype: runbook\n---\n\n" +
		"```python:exec\n@sub/main.py\n```\n\n" +
		"```python:exec\nimport top\nprint(top.X)\n```\n"
	files := map[string]string{
		"sub/main.py":   "import sys, helper\nprint(helper.X, sys.argv[0].endswith('sub/main.py'))\n",
		"sub/helper.py": "X = 'helper'\n",
		"top.py":        "X = 'top'\n",
	}
	r, out, errOut := runSource(t, src, files)
	if r.Failed != nil {
		t.Fatalf("block %d failed: %s", r.Failed.Index, errOut)
	}
	if out != "helper True\ntop\n" {
		t.Fatalf("stdout = %q", out)
	}
}

// TestEveryLanguageHasAnAdapter keeps the language list and the adapters in
// step: every language pre-flight accepts can run.
func TestEveryLanguageHasAnAdapter(t *testing.T) {
	for _, lang := range []string{"bash", "python3", "python"} {
		bin, ok := doc.Interpreter(lang)
		if !ok {
			t.Fatalf("%s: not a known language", lang)
		}
		a, ok := Lookup(lang)
		if !ok || a.Bin != bin {
			t.Fatalf("%s: adapter %+v, want interpreter %s", lang, a, bin)
		}
	}
	if _, ok := Lookup("ruby"); ok {
		t.Fatal("ruby has an adapter")
	}
}
