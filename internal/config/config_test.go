package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Theme != "" {
		t.Fatalf("Theme = %q, want empty", c.Theme)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := Save(Config{Theme: "catppuccin-mocha"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Theme != "catppuccin-mocha" {
		t.Fatalf("Theme = %q", c.Theme)
	}
	info, err := os.Stat(filepath.Join(dir, "exmd", "config.yaml"))
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestLoadMalformed(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "exmd", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("theme: [not, a, string]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("Load accepted malformed YAML")
	}
}

func TestSessionsEnabled(t *testing.T) {
	off, on := false, true
	for _, tc := range []struct {
		yaml string
		want bool
	}{
		{"", true},
		{"theme: dark\n", true},
		{"save_sessions: true\n", true},
		{"save_sessions: false\n", false},
	} {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)
		path := filepath.Join(dir, "exmd", "config.yaml")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(tc.yaml), 0o600); err != nil {
			t.Fatal(err)
		}
		c, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if got := c.SessionsEnabled(); got != tc.want {
			t.Errorf("%q: SessionsEnabled = %v, want %v", tc.yaml, got, tc.want)
		}
	}
	if !(Config{SaveSessions: &on}).SessionsEnabled() || (Config{SaveSessions: &off}).SessionsEnabled() {
		t.Error("explicit values not honoured")
	}
}
