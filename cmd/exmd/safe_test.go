package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/michaelishri/exmd/internal/session"
)

// runCaptured runs the CLI with stdout and stderr captured.
func runCaptured(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	var outBuf, errBuf bytes.Buffer
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(&outBuf, outR); done <- struct{}{} }()
	go func() { _, _ = io.Copy(&errBuf, errR); done <- struct{}{} }()

	code = run(args)

	os.Stdout, os.Stderr = oldOut, oldErr
	outW.Close()
	errW.Close()
	<-done
	<-done
	return outBuf.String(), errBuf.String(), code
}

var cliSGR = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

// assertNoControl fails when anything but SGR styling, newlines and tabs
// reaches the terminal.
func assertNoControl(t *testing.T, label, s string) {
	t.Helper()
	for _, r := range cliSGR.ReplaceAllString(s, "") {
		if (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			t.Fatalf("%s: control %U reached the terminal:\n%q", label, r, s)
		}
	}
}

// TestCLINeutralizesDocumentControlCharacters checks every headless verb
// against a document whose metadata and content carry escape sequences.
func TestCLINeutralizesDocumentControlCharacters(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	src := "---\ntype: runbook\nglobals:\n  EVIL: \"x\\e]52;c;aGk=\\a\"\n\"bogus\\e[2J\": 1\n---\n" +
		"# Head \x1b]0;title\x07\n\n" +
		"```bash:exec\n@scr\x1b[1mipt.sh\n```\n\n" +
		"```bash:exec bad\x07attr\necho hi\n```\n"
	path := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"check", path},
		{"list", path},
		{"globals", path},
		{"globals", path, "--set", "EVIL=y\x1b]52;c;aGk=\x07"},
		{"print", path, "--width", "60"},
	} {
		out, errOut, _ := runCaptured(t, args...)
		label := strings.Join(args[:1], " ")
		assertNoControl(t, label+" stdout", out)
		assertNoControl(t, label+" stderr", errOut)
		if !strings.Contains(out+errOut, "^[") {
			t.Errorf("%s: expected a visible ^[ marker, got:\n%s%s", label, out, errOut)
		}
	}
}

// TestOpenAndResumeRunPreflight: the viewer and --resume refuse a document
// whose script or interpreter is missing before anything can run (SPEC.md
// §5.3), with the same diagnostics as `exmd check`.
func TestOpenAndResumeRunPreflight(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	src := "---\ntype: runbook\n---\n# T\n\n```bash:exec\n@missing.sh\n```\n"
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{path}, {path, "--resume"}} {
		if len(args) == 2 {
			// A saved session must exist for --resume to reach the document.
			snap := &session.Snapshot{Version: session.FormatVersion, DocPath: path, BlockSel: -1}
			if err := session.Save(path, snap); err != nil {
				t.Fatal(err)
			}
		}
		out, errOut, code := runCaptured(t, args...)
		if code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
		if !strings.Contains(out+errOut, "script file not found") {
			t.Errorf("%v: missing pre-flight diagnostic:\n%s%s", args, out, errOut)
		}
	}
}
