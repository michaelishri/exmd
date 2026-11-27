// Package session stores resumable tool-level snapshots of a runbook viewer.
//
// A snapshot is not a persistent shell: it records only legal §4 state (the
// declared globals and the block cursor) plus the TUI's own output boxes and
// selection. Resuming replays that state into a fresh exec.Session, so no live
// process, working directory, function, or import survives (SPEC.md §4.5, §9).
package session

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/michaelishri/exmd/internal/config"
	"github.com/michaelishri/exmd/internal/doc"
	"github.com/michaelishri/exmd/internal/exec"
)

// FormatVersion is the on-disk snapshot format. Unknown versions are refused
// rather than interpreted.
const FormatVersion = 2

// Snapshot is one saved viewer session. Files are keyed by the absolute
// document path, so there is a single latest snapshot per document.
type Snapshot struct {
	Version     int               `yaml:"version"`
	DocPath     string            `yaml:"doc_path"`
	Fingerprint string            `yaml:"fingerprint,omitempty"` // informational only in v2
	Overrides   map[string]string `yaml:"overrides,omitempty"`

	Globals map[string]string `yaml:"globals,omitempty"`
	Absent  []string          `yaml:"absent,omitempty"`

	Next        int             `yaml:"next"`
	BlockSel    int             `yaml:"block_sel"`
	BlockSelKey string          `yaml:"block_sel_key,omitempty"`
	Keys        []string        `yaml:"keys,omitempty"`
	Blocks      []BlockSnapshot `yaml:"blocks,omitempty"`
	Runs        []RunSnapshot   `yaml:"runs,omitempty"`

	UpdatedAt time.Time `yaml:"updated_at"`
}

// BlockSnapshot is one block's visible output and status. Key identifies the
// executable block by content, so it survives insertions and renumbering.
type BlockSnapshot struct {
	Key     string   `yaml:"key"`
	Index   int      `yaml:"index"`
	Lines   []string `yaml:"lines,omitempty"`
	Dropped int      `yaml:"dropped,omitempty"` // earlier lines not kept
	Status  int      `yaml:"status,omitempty"`
	Done    bool     `yaml:"done,omitempty"`
	Active  bool     `yaml:"active,omitempty"`
	Err     string   `yaml:"err,omitempty"`
	Stale   bool     `yaml:"stale,omitempty"`
}

// RunSnapshot is one completed execution, recorded in execution order. After
// holds the §4 state observed immediately after that run, so a modified
// document can roll back to the last still-valid checkpoint.
type RunSnapshot struct {
	Key   string            `yaml:"key"`
	After exec.SessionState `yaml:"after"`
}

// RestorePlan is the result of matching a snapshot against a possibly edited
// document. It is built by Snapshot.Plan and applied by the TUI.
type RestorePlan struct {
	State    exec.SessionState
	Blocks   map[int]BlockRestore
	BlockSel int
	Changed  []string
	Stale    []string
}

// BlockRestore is one block's restored presentation state.
type BlockRestore struct {
	Lines   []string
	Dropped int
	Status  int
	Done    bool
	Active  bool
	Err     string
	Stale   bool
}

// Dir is the directory holding session snapshots, alongside the tool's
// other on-disk state (config.Dir).
func Dir() (string, error) {
	base, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "sessions"), nil
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
	binary.LittleEndian.PutUint64(n[:], uint64(len(data)))
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
	keys, err := BlockKeys(d)
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
		BlockSel:    -1,
		Keys:        keys,
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
		return nil, fmt.Errorf("parse %s: %w; clear the snapshot with `exmd DOC --resume --clear` and start a new session", path, err)
	}
	if s.Version != FormatVersion {
		return nil, fmt.Errorf("parse %s: unsupported snapshot version %d (expected %d); clear the snapshot with `exmd DOC --resume --clear` and start a new session", path, s.Version, FormatVersion)
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
	if err := ensureDir(filepath.Dir(path)); err != nil {
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

// ensureDir creates the snapshot directory private to the user. Snapshots
// hold globals and block output, which can include secrets, so an existing
// directory with wider permissions is tightened too.
func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}

// Entry is one snapshot file found by List. Err is set, and Snapshot nil,
// when the file cannot be read or parsed; one bad file never hides the rest.
type Entry struct {
	File     string // absolute path of the snapshot file
	Snapshot *Snapshot
	Err      error
}

// List returns every saved snapshot file, readable ones newest first, then
// unreadable ones by file name.
func List() ([]Entry, error) {
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
	var out []Entry
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		file := filepath.Join(dir, e.Name())
		ent := Entry{File: file}
		data, err := os.ReadFile(file)
		if err == nil {
			ent.Snapshot, err = decode(data, file)
		}
		ent.Err = err
		out = append(out, ent)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Snapshot, out[j].Snapshot
		switch {
		case a == nil || b == nil:
			if (a == nil) != (b == nil) {
				return a != nil
			}
			return out[i].File < out[j].File
		case a.UpdatedAt.Equal(b.UpdatedAt):
			return a.DocPath < b.DocPath
		default:
			return a.UpdatedAt.After(b.UpdatedAt)
		}
	})
	return out, nil
}

// Prune removes snapshot files that can no longer be resumed: unreadable or
// unsupported ones, and those whose document no longer exists. It returns
// the entries it removed.
func Prune() ([]Entry, error) {
	entries, err := List()
	if err != nil {
		return nil, err
	}
	var removed []Entry
	for _, e := range entries {
		if e.Err == nil {
			if _, statErr := os.Stat(e.Snapshot.DocPath); statErr == nil {
				continue
			}
		}
		if err := os.Remove(e.File); err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, err
		}
		removed = append(removed, e)
	}
	return removed, nil
}

// Validate checks that a snapshot can be safely replayed against the current
// document. Errors include the action the user can take to recover.
func (s *Snapshot) Validate(d *doc.Document, overrides map[string]string) error {
	hint := "clear it with `exmd DOC --resume --clear` and start a new session"
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
	// The document-wide fingerprint is informational in v2. Edited
	// executable blocks are matched by key and rolled back by Plan instead
	// of rejecting the whole snapshot.
	if !sameMap(s.Overrides, overrides) {
		return fmt.Errorf("--set overrides differ from the saved session; replay them or clear the snapshot with `exmd DOC --resume --clear`")
	}
	if err := exec.ValidateState(d, filterState(d, s.State())); err != nil {
		return fmt.Errorf("saved state is invalid: %w; %s", err, hint)
	}
	// Block indices and selection are informational in v2: documents can
	// shrink or be reordered, and Plan remaps selection by content key.
	return nil
}

// State converts the saved globals and cursor into exec state.
func (s *Snapshot) State() exec.SessionState {
	return exec.SessionState{
		Globals: cloneMap(s.Globals),
		Absent:  append([]string(nil), s.Absent...),
		Next:    s.Next,
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
