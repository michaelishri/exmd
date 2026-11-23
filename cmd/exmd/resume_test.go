package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/michaelishri/exmd/internal/doc"
	"github.com/michaelishri/exmd/internal/session"
)

const resumeDocSource = `---
type: runbook
globals: {GREETING}
---

` + "```bash:exec\n" + `export GREETING="hello"
` + "```\n"

func setupResume(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "runbook.md")
	if err := os.WriteFile(path, []byte(resumeDocSource), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := doc.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := session.New(res.Doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Save(path, s); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResumeExitCodes(t *testing.T) {
	t.Run("help", func(t *testing.T) {
		setupResume(t)
		if got := run([]string{"--resume", "--help"}); got != 0 {
			t.Fatalf("--resume --help = %d, want 0", got)
		}
	})
	t.Run("list", func(t *testing.T) {
		setupResume(t)
		if got := run([]string{"--resume", "--list"}); got != 0 {
			t.Fatalf("--resume --list = %d, want 0", got)
		}
	})
	t.Run("list with document", func(t *testing.T) {
		path := setupResume(t)
		if got := run([]string{"--resume", "--list", path}); got != 2 {
			t.Fatalf("--resume --list DOC = %d, want 2", got)
		}
	})
	t.Run("list and clear conflict", func(t *testing.T) {
		path := setupResume(t)
		if got := run([]string{"--resume", "--list", "--clear", path}); got != 2 {
			t.Fatalf("--resume --list --clear = %d, want 2", got)
		}
	})
	t.Run("no document", func(t *testing.T) {
		setupResume(t)
		if got := run([]string{"--resume"}); got != 2 {
			t.Fatalf("--resume = %d, want 2", got)
		}
	})
	t.Run("missing flag value", func(t *testing.T) {
		setupResume(t)
		if got := run([]string{"--resume", "--set"}); got != 2 {
			t.Fatalf("--resume --set = %d, want 2", got)
		}
	})
	t.Run("valid snapshot needs a terminal", func(t *testing.T) {
		path := setupResume(t)
		if got := run([]string{path, "--resume"}); got != 2 {
			t.Fatalf("DOC --resume = %d, want 2 without a terminal", got)
		}
		if got := run([]string{"--resume", path}); got != 2 {
			t.Fatalf("--resume DOC = %d, want 2 without a terminal", got)
		}
		if got := run([]string{"--resume=" + path}); got != 2 {
			t.Fatalf("--resume=DOC = %d, want 2 without a terminal", got)
		}
	})
	t.Run("bad theme", func(t *testing.T) {
		path := setupResume(t)
		if got := run([]string{path, "--resume", "--theme=bogus"}); got != 2 {
			t.Fatalf("DOC --resume --theme=bogus = %d, want 2", got)
		}
	})
	t.Run("stale snapshot", func(t *testing.T) {
		path := setupResume(t)
		if err := os.WriteFile(path, []byte(resumeDocSource+"\nedited\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := run([]string{path, "--resume"}); got != 2 {
			t.Fatalf("stale DOC --resume = %d, want 2", got)
		}
	})
	t.Run("missing document", func(t *testing.T) {
		setupResume(t)
		if got := run([]string{"does-not-exist.md", "--resume"}); got != 2 {
			t.Fatalf("missing DOC --resume = %d, want 2", got)
		}
	})
	t.Run("clear then resume", func(t *testing.T) {
		path := setupResume(t)
		if got := run([]string{path, "--resume", "--clear"}); got != 0 {
			t.Fatalf("DOC --resume --clear = %d, want 0", got)
		}
		if got := run([]string{path, "--resume"}); got != 2 {
			t.Fatalf("resume after clear = %d, want 2", got)
		}
	})
}

func TestResumeErrorMentionsHowToClear(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "runbook.md")
	if err := os.WriteFile(path, []byte(resumeDocSource+"\nedited\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := doc.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := session.New(res.Doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Save(path, s); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(resumeDocSource+"\nmore edits\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := doc.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Validate(changed.Doc, nil)
	if err == nil || !strings.Contains(err.Error(), "--resume --clear") {
		t.Fatalf("stale snapshot error = %v, want a recovery hint", err)
	}
}

func TestExtractBoolAndResume(t *testing.T) {
	got, found := extractBool([]string{"DOC", "--clear", "--set", "A=B"}, "--clear")
	if !found {
		t.Fatal("--clear not found")
	}
	want := []string{"DOC", "--set", "A=B"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractBool = %v, want %v", got, want)
	}
	if _, found := extractBool([]string{"DOC"}, "--clear"); found {
		t.Fatal("found --clear where none exists")
	}

	for _, tc := range []struct {
		args  []string
		value string
	}{
		{[]string{"DOC", "--resume"}, ""},
		{[]string{"--resume", "DOC"}, ""},
		{[]string{"--resume=DOC"}, "DOC"},
		{[]string{"-resume", "DOC"}, ""},
	} {
		rest, value, found := extractResume(tc.args)
		if !found {
			t.Fatalf("%v: --resume not found", tc.args)
		}
		if value != tc.value {
			t.Fatalf("%v: value = %q, want %q", tc.args, value, tc.value)
		}
		if len(rest) != 1 || rest[0] != "DOC" {
			t.Fatalf("%v: rest = %v, want [DOC]", tc.args, rest)
		}
	}
}
