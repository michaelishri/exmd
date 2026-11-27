package session

import (
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/michaelishri/exmd/internal/exec"
)

// compatSnapshot is the snapshot stored in testdata/snapshot-yaml-v3.yaml,
// which was written by the gopkg.in/yaml.v3 encoder exmd used before
// moving to go.yaml.in/yaml/v4. Its `fingerprint` key is from an older
// format and is ignored.
var compatSnapshot = Snapshot{
	Version: FormatVersion, DocPath: "/docs/runbook.md",
	Overrides: map[string]string{"TOKEN": "x: y\nmulti"},
	Globals:   map[string]string{"GREETING": "hello world", "EMPTY": "", "NUMLIKE": "0x1F", "YES": "yes"},
	Absent:    []string{"LATER"}, Next: 2, BlockSel: 1, BlockSelKey: "k2", Keys: []string{"k1", "k2"},
	Blocks: []BlockSnapshot{
		{Key: "k1", Index: 0, Lines: []string{"one", "  indented", "- dash", "# hash", "日本語"}, Done: true},
		{Key: "k2", Index: 1, Status: 3, Done: true, Err: "boom", Stale: true, Dropped: 7},
	},
	Runs: []RunSnapshot{{Key: "k1", After: exec.SessionState{
		Globals: map[string]string{"GREETING": "hello world"}, Absent: []string{"LATER"}, Next: 1}}},
	UpdatedAt: time.Date(2026, 11, 28, 11, 0, 0, 123456789, time.UTC),
}

// TestSnapshotFromOlderYAMLLibrary: sessions saved before the YAML library
// upgrade still load with every field intact.
func TestSnapshotFromOlderYAMLLibrary(t *testing.T) {
	data, err := os.ReadFile("testdata/snapshot-yaml-v3.yaml")
	if err != nil {
		t.Fatal(err)
	}
	got, err := decode(data, "snapshot-yaml-v3.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*got, compatSnapshot) {
		t.Fatalf("decoded snapshot differs:\n got %+v\nwant %+v", *got, compatSnapshot)
	}
}

// TestSnapshotRoundTrip: what Save writes, Load reads back unchanged.
func TestSnapshotRoundTrip(t *testing.T) {
	isolate(t)
	s := compatSnapshot
	if err := Save(s.DocPath, &s); err != nil {
		t.Fatal(err)
	}
	got, err := Load(s.DocPath)
	if err != nil || got == nil {
		t.Fatalf("Load: %v %v", got, err)
	}
	want := compatSnapshot
	want.UpdatedAt = s.UpdatedAt // Save stamps the time
	if !got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Fatalf("updated_at %v, want %v", got.UpdatedAt, want.UpdatedAt)
	}
	got.UpdatedAt = want.UpdatedAt
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("round trip differs:\n got %+v\nwant %+v", *got, want)
	}
}
