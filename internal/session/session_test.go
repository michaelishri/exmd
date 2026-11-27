package session

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/michaelishri/exmd/internal/doc"
)

const flowSource = `---
type: runbook
globals: {GREETING, TOKEN: "default"}
---

## one

` + "```bash:exec\n" + `export GREETING="hello world"
echo one
` + "```\n" + `
## two

` + "```bash:exec\n" + `echo "[$GREETING] [$TOKEN]"
` + "```\n"

func writeDoc(t *testing.T, src string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runbook.md")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func load(t *testing.T, path string) *doc.Document {
	t.Helper()
	res, err := doc.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if res.HasErrors() {
		t.Fatalf("fixture errors: %v", res.Diagnostics)
	}
	return res.Doc
}

func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func TestPathStablePerDocument(t *testing.T) {
	isolate(t)
	a, err := Path("/tmp/one.md")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Path("/tmp/one.md")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Path("/tmp/two.md")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("path is not stable: %s != %s", a, b)
	}
	if a == c {
		t.Fatal("two documents share a snapshot path")
	}
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(a, dir+string(os.PathSeparator)) {
		t.Fatalf("snapshot %s is outside %s", a, dir)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	isolate(t)
	d := load(t, writeDoc(t, flowSource))
	s, err := New(d, map[string]string{"TOKEN": "override"})
	if err != nil {
		t.Fatal(err)
	}
	keys, err := BlockKeys(d)
	if err != nil {
		t.Fatal(err)
	}
	s.Next = 1
	s.BlockSel = 0
	s.BlockSelKey = keys[0]
	s.Globals["GREETING"] = "hello world"
	s.Blocks = []BlockSnapshot{{
		Key: keys[0], Index: 0, Lines: []string{"one"}, Status: 0, Done: true,
	}}
	if err := Save(d.Path, s); err != nil {
		t.Fatal(err)
	}
	path, err := Path(d.Path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("snapshot mode = %o, want 600", info.Mode().Perm())
	}
	got, err := Load(d.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("snapshot not loaded")
	}
	if got.Version != FormatVersion || got.Next != 1 || got.BlockSel != 0 {
		t.Fatalf("wrong header: %+v", got)
	}
	if got.Globals["GREETING"] != "hello world" || got.Overrides["TOKEN"] != "override" {
		t.Fatalf("wrong state: %+v", got)
	}
	if len(got.Blocks) != 1 || !reflect.DeepEqual(got.Blocks[0].Lines, []string{"one"}) {
		t.Fatalf("wrong blocks: %+v", got.Blocks)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("updated_at not set")
	}
}

func TestLoadMissing(t *testing.T) {
	isolate(t)
	got, err := Load(writeDoc(t, flowSource))
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("missing snapshot = %+v, want nil", got)
	}
}

func TestLoadRejectsMalformedAndOldVersions(t *testing.T) {
	isolate(t)
	path := writeDoc(t, flowSource)
	snapPath, err := Path(path)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, body, want string
	}{
		{"garbage", "not: [valid", "parse"},
		{"old version", "version: 0\ndoc_path: x\n", "unsupported snapshot version"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(snapPath, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Load error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestFingerprintTracksDocumentAndScripts(t *testing.T) {
	isolate(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "helper.sh")
	if err := os.WriteFile(script, []byte("echo one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	docPath := filepath.Join(dir, "runbook.md")
	src := "---\ntype: runbook\n---\n\n" +
		"```bash:exec\n@helper.sh\n```\n"
	if err := os.WriteFile(docPath, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	d := load(t, docPath)
	first, err := Fingerprint(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("echo two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := Fingerprint(d)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("editing a referenced script did not change the fingerprint")
	}
	if err := os.WriteFile(docPath, []byte(src+"\nmore text\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	thirdDoc := load(t, docPath)
	third, err := Fingerprint(thirdDoc)
	if err != nil {
		t.Fatal(err)
	}
	if second == third {
		t.Fatal("editing the document did not change the fingerprint")
	}
}

func TestValidate(t *testing.T) {
	isolate(t)
	path := writeDoc(t, flowSource)
	d := load(t, path)
	overrides := map[string]string{"TOKEN": "nightly"}
	s, err := New(d, overrides)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(d, overrides); err != nil {
		t.Fatalf("valid snapshot rejected: %v", err)
	}

	if err := s.Validate(d, map[string]string{"TOKEN": "other"}); err == nil || !strings.Contains(err.Error(), "overrides") {
		t.Fatalf("different overrides: %v", err)
	}
	bad := *s
	bad.Globals = map[string]string{"NOPE": "x"}
	if err := bad.Validate(d, overrides); err != nil {
		t.Fatalf("undeclared globals are filtered in v2: %v", err)
	}
	bad = *s
	bad.Next = 99
	if err := bad.Validate(d, overrides); err != nil {
		t.Fatalf("out-of-range cursor should be clamped: %v", err)
	}
	bad = *s
	bad.Blocks = []BlockSnapshot{{Index: 42}}
	if err := bad.Validate(d, overrides); err != nil {
		t.Fatalf("block indices are informational in v2: %v", err)
	}

	if err := os.WriteFile(path, []byte(flowSource+"\nedited\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	changed := load(t, path)
	if err := s.Validate(changed, overrides); err != nil {
		t.Fatalf("edited document should be accepted: %v", err)
	}

	old := *s
	old.Version = 1
	if err := old.Validate(d, overrides); err == nil || !strings.Contains(err.Error(), "unsupported snapshot version") {
		t.Fatalf("v1 snapshot: %v", err)
	}
}

func TestDeleteAndList(t *testing.T) {
	isolate(t)
	paths := []string{writeDoc(t, flowSource), writeDoc(t, flowSource)}
	for _, p := range paths {
		s, err := New(load(t, p), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := Save(p, s); err != nil {
			t.Fatal(err)
		}
	}
	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Err != nil || list[1].Err != nil {
		t.Fatalf("List returned %+v, want 2 readable snapshots", list)
	}
	if err := Delete(paths[0]); err != nil {
		t.Fatal(err)
	}
	list, err = List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("List after delete = %d, want 1", len(list))
	}
	if err := Delete(paths[0]); err != nil {
		t.Fatalf("deleting a missing snapshot: %v", err)
	}
}

func TestStateConversion(t *testing.T) {
	isolate(t)
	d := load(t, writeDoc(t, flowSource))
	s, err := New(d, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Next = 1
	s.Globals["GREETING"] = "hello"
	st := s.State()
	if st.Next != 1 || st.Globals["GREETING"] != "hello" {
		t.Fatalf("State() = %+v", st)
	}
	if st.Globals["TOKEN"] != "default" {
		t.Fatalf("default global lost: %+v", st.Globals)
	}
}

// TestListToleratesBadFiles: one corrupt or old snapshot must not hide the
// others; it is listed last with its error and file path.
func TestListToleratesBadFiles(t *testing.T) {
	isolate(t)
	good := writeDoc(t, flowSource)
	s, err := New(load(t, good), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(good, s); err != nil {
		t.Fatal(err)
	}
	dir, _ := Dir()
	for name, body := range map[string]string{
		"aaa-corrupt.yaml": "version: [",
		"bbb-v1.yaml":      "version: 1\ndoc_path: /x.md\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	list, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("List returned %d entries, want 3", len(list))
	}
	if list[0].Err != nil || list[0].Snapshot == nil {
		t.Fatalf("readable snapshot should come first: %+v", list[0])
	}
	for _, e := range list[1:] {
		if e.Err == nil || e.Snapshot != nil || !strings.HasPrefix(e.File, dir) {
			t.Fatalf("bad file entry = %+v", e)
		}
	}
}

// TestPruneRemovesUnresumableSnapshots keeps snapshots of documents that
// still exist and removes broken ones and orphans.
func TestPruneRemovesUnresumableSnapshots(t *testing.T) {
	isolate(t)
	keep, gone := writeDoc(t, flowSource), writeDoc(t, flowSource)
	for _, p := range []string{keep, gone} {
		s, err := New(load(t, p), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := Save(p, s); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	dir, _ := Dir()
	if err := os.WriteFile(filepath.Join(dir, "corrupt.yaml"), []byte("version: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	removed, err := Prune()
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 2 {
		t.Fatalf("pruned %d, want 2", len(removed))
	}
	list, _ := List()
	if len(list) != 1 || list[0].Snapshot == nil || list[0].Snapshot.DocPath != keep {
		t.Fatalf("after prune: %+v", list)
	}
}

// TestSnapshotDirIsPrivate: snapshots can hold secrets, so the directory is
// created (or tightened) to 0700 and files are 0600.
func TestSnapshotDirIsPrivate(t *testing.T) {
	isolate(t)
	dir, _ := Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeDoc(t, flowSource)
	s, err := New(load(t, p), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(p, s); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("dir mode = %o, want 700", perm)
	}
	file, _ := Path(p)
	if info, err = os.Stat(file); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %v (%v), want 600", info.Mode().Perm(), err)
	}
}
