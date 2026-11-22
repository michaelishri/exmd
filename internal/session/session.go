// Package session stores resumable tool-level snapshots of a runbook viewer.
//
// A snapshot is not a persistent shell: it records only legal §4 state (the
// declared globals and the block cursor) plus the TUI's own output boxes and
// selection. Resuming replays that state into a fresh exec.Session, so no live
// process, working directory, function, or import survives (SPEC.md §4.5, §9).
package session

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/michaelishri/exmd/internal/doc"
	"github.com/michaelishri/exmd/internal/exec"
)

// FormatVersion is the on-disk snapshot format. Unknown versions are refused
// rather than interpreted.
const FormatVersion = 1

// Snapshot is one saved viewer session. Files are keyed by the absolute
// document path, so there is a single latest snapshot per document.
type Snapshot struct {
	Version     int               `yaml:"version"`
	DocPath     string            `yaml:"doc_path"`
	Fingerprint string            `yaml:"fingerprint"`
	Overrides   map[string]string `yaml:"overrides,omitempty"`

	Globals map[string]string `yaml:"globals,omitempty"`
	Absent  []string          `yaml:"absent,omitempty"`

	Next        int             `yaml:"next"`
	BlocksRun   int             `yaml:"blocks_run"`
	Warnings    []string        `yaml:"warnings,omitempty"`
	Interrupted bool            `yaml:"interrupted,omitempty"`
	FailedIndex int             `yaml:"failed_index"`
	ExitStatus  int             `yaml:"exit_status,omitempty"`
	BlockSel    int             `yaml:"block_sel"`
	Blocks      []BlockSnapshot `yaml:"blocks,omitempty"`

	UpdatedAt time.Time `yaml:"updated_at"`
}

// BlockSnapshot is one block's visible output and status. Its index is into
// doc.Blocks (0-based).
type BlockSnapshot struct {
	Index  int      `yaml:"index"`
	Lines  []string `yaml:"lines,omitempty"`
	Status int      `yaml:"status,omitempty"`
	Done   bool     `yaml:"done,omitempty"`
	Active bool     `yaml:"active,omitempty"`
	Err    string   `yaml:"err,omitempty"`
}

// Dir is the directory holding session snapshots. It lives alongside the
// tool's other on-disk state and honours XDG_CONFIG_HOME.
func Dir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(base, "exmd", "sessions"), nil
}

// Path returns the snapshot file for a document.
func Path(docPath string) (string, error) {
	abs, err := filepath.Abs(docPath)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(abs))
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, hex.EncodeToString(sum[:])+".yaml"), nil
}

// Fingerprint hashes the document source together with every script it
// references. Changing either invalidates saved sessions for that document.
func Fingerprint(d *doc.Document) (string, error) {
	h := sha256.New()
	writePart(h, d.Source)
	paths := make([]string, 0, len(d.Blocks))
	for _, b := range d.Blocks {
		if b.Kind == doc.KindScript && b.ScriptAbs != "" {
			paths = append(paths, b.ScriptAbs)
		}
	}
	sort.Strings(paths)
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return "", fmt.Errorf("script %s: %w", p, err)
		}
		writePart(h, []byte(p))
		writePart(h, data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// writePart length-prefixes a chunk so distinct inputs cannot collide.
func writePart(w io.Writer, data []byte) {
	var n [8]byte
	lenBytes := uint64(len(data))
	for i := 0; i < 8; i++ {
		n[i] = byte(lenBytes >> (8 * i))
	}
	_, _ = w.Write(n[:])
	_, _ = w.Write(data)
}

// New builds an initial snapshot for a document and its launch overrides.
func New(d *doc.Document, overrides map[string]string) (*Snapshot, error) {
	fp, err := Fingerprint(d)
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(d.Path)
	if err != nil {
		return nil, err
	}
	present, diags := d.ResolveGlobals(overrides)
	for _, dg := range diags {
		if dg.Severity == doc.SeverityError {
			return nil, fmt.Errorf("launch-time overrides: %s", dg.Message)
		}
	}
	s := &Snapshot{
		Version:     FormatVersion,
		DocPath:     abs,
		Fingerprint: fp,
		Overrides:   cloneMap(overrides),
		Globals:     present,
		FailedIndex: -1,
		BlockSel:    -1,
		UpdatedAt:   time.Now().UTC(),
	}
	for name := range d.Globals {
		if _, ok := present[name]; !ok {
			s.Absent = append(s.Absent, name)
		}
	}
	sort.Strings(s.Absent)
	return s, nil
}

// Load reads the snapshot for a document. A missing file is not an error.
func Load(docPath string) (*Snapshot, error) {
	path, err := Path(docPath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return decode(data, path)
}

func decode(data []byte, path string) (*Snapshot, error) {
	var s Snapshot
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if s.Version != FormatVersion {
		return nil, fmt.Errorf("parse %s: unsupported snapshot version %d (expected %d)", path, s.Version, FormatVersion)
	}
	return &s, nil
}

// Save writes a snapshot atomically: a temporary file in the same directory
// is renamed into place, so a crash cannot leave a half-written snapshot.
func Save(docPath string, s *Snapshot) error {
	path, err := Path(docPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	s.UpdatedAt = time.Now().UTC()
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".snapshot-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// Delete removes a document's snapshot. Deleting a missing one is a no-op.
func Delete(docPath string) error {
	path, err := Path(docPath)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// List returns all saved snapshots, newest first.
func List() ([]*Snapshot, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*Snapshot
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		s, err := decode(data, e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].DocPath < out[j].DocPath
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

// Validate checks that a snapshot can be safely replayed against the current
// document. Errors include the action the user can take to recover.
func (s *Snapshot) Validate(d *doc.Document, overrides map[string]string) error {
	hint := "clear it with `exmd resume DOC --clear` and start a new session"
	if s.Version != FormatVersion {
		return fmt.Errorf("unsupported snapshot version %d (expected %d); %s", s.Version, FormatVersion, hint)
	}
	abs, err := filepath.Abs(d.Path)
	if err != nil {
		return err
	}
	if s.DocPath != abs {
		return fmt.Errorf("snapshot belongs to another document (%s); %s", s.DocPath, hint)
	}
	fp, err := Fingerprint(d)
	if err != nil {
		return err
	}
	if s.Fingerprint != fp {
		return fmt.Errorf("the document or one of its scripts changed since the snapshot was saved; %s", hint)
	}
	if !sameMap(s.Overrides, overrides) {
		return fmt.Errorf("--set overrides differ from the saved session; replay them or clear the snapshot with `exmd resume DOC --clear`")
	}
	if s.Next < 0 || s.Next > len(d.Blocks) {
		return fmt.Errorf("saved block cursor %d is outside this document; %s", s.Next, hint)
	}
	if s.BlockSel < -1 || s.BlockSel >= len(d.Blocks) {
		return fmt.Errorf("saved block selection %d is outside this document; %s", s.BlockSel, hint)
	}
	if s.FailedIndex < -1 || s.FailedIndex >= len(d.Blocks) {
		return fmt.Errorf("saved failure index %d is outside this document; %s", s.FailedIndex, hint)
	}
	for name := range s.Globals {
		if _, ok := d.Globals[name]; !ok {
			return fmt.Errorf("saved global %q is no longer declared; %s", name, hint)
		}
	}
	seen := map[string]bool{}
	for _, name := range s.Absent {
		if _, ok := d.Globals[name]; !ok {
			return fmt.Errorf("saved global %q is no longer declared; %s", name, hint)
		}
		if seen[name] {
			return fmt.Errorf("saved global %q is duplicated; %s", name, hint)
		}
		seen[name] = true
	}
	for name := range s.Globals {
		if seen[name] {
			return fmt.Errorf("saved global %q is both present and absent; %s", name, hint)
		}
	}
	for _, b := range s.Blocks {
		if b.Index < 0 || b.Index >= len(d.Blocks) {
			return fmt.Errorf("saved output for block %d is outside this document; %s", b.Index, hint)
		}
	}
	return nil
}

// State converts the saved globals and cursor into exec state.
func (s *Snapshot) State() exec.SessionState {
	return exec.SessionState{
		Restored:        true,
		Globals:         cloneMap(s.Globals),
		Absent:          append([]string(nil), s.Absent...),
		Next:            s.Next,
		BlocksRun:       s.BlocksRun,
		Warnings:        append([]string(nil), s.Warnings...),
		LastFailedIndex: s.FailedIndex,
		LastStatus:      s.ExitStatus,
		Interrupted:     s.Interrupted,
	}
}

func cloneMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func sameMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if other, ok := b[k]; !ok || other != v {
			return false
		}
	}
	return true
}
