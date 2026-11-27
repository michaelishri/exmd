package exec

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/michaelishri/exmd/internal/doc"
)

// runIndexed runs one block and returns its captured stdout plus the
// session's state afterwards.
func runIndexed(t *testing.T, s *Session, index int) (string, SessionState) {
	t.Helper()
	var out strings.Builder
	res := s.RunBlock(context.Background(), index, func(ev Event) {
		if ev.Kind == EventOutput && ev.Stream == StreamStdout {
			out.Write(ev.Data)
		}
	})
	if res.Done {
		t.Fatalf("block %d did not run", index)
	}
	return out.String(), s.State()
}

func loadFlow(t *testing.T) *doc.Document {
	t.Helper()
	res, err := doc.Load(filepath.Join("testdata", "flow.md"))
	if err != nil {
		t.Fatal(err)
	}
	res.Preflight()
	if res.HasErrors() {
		t.Fatalf("fixture errors: %v", res.Diagnostics)
	}
	return res.Doc
}

func TestStateRoundTripCarriesGlobalsAndCursor(t *testing.T) {
	d := loadFlow(t)
	first, err := NewSession(d, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()

	out, state := runIndexed(t, first, 0)
	if !strings.Contains(out, "hello from block 1") {
		t.Fatalf("block 1 output: %q", out)
	}
	if state.Next != 1 {
		t.Fatalf("state after block 1: %+v", state)
	}
	if state.Globals["GREETING"] != "hello world" {
		t.Fatalf("harvested global missing: %+v", state.Globals)
	}

	restored, err := NewSessionFromState(d, Options{}, state)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	defer restored.Close()
	if got := restored.Globals()["GREETING"]; got != "hello world" {
		t.Fatalf("restored GREETING = %q", got)
	}
	if restored.Remaining() != len(d.Blocks)-1 {
		t.Fatalf("restored cursor lost: Remaining = %d", restored.Remaining())
	}
	out, _ = runIndexed(t, restored, state.Next)
	if !strings.Contains(out, "[hello world]") {
		t.Fatalf("restored session did not carry the global: %q", out)
	}
	if !strings.Contains(out, "[absent]") {
		t.Fatalf("unexported variable leaked across the restore: %q", out)
	}
}

func TestStateRoundTripPreservesAbsence(t *testing.T) {
	d := loadFlow(t)
	first, err := NewSession(d, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	for i := 0; i < 3; i++ {
		runIndexed(t, first, i)
	}
	state := first.State()
	if _, present := state.Globals["GREETING"]; present {
		t.Fatalf("GREETING should be absent after unset: %+v", state.Globals)
	}
	found := false
	for _, name := range state.Absent {
		if name == "GREETING" {
			found = true
		}
	}
	if !found {
		t.Fatalf("GREETING missing from absent list: %v", state.Absent)
	}

	restored, err := NewSessionFromState(d, Options{}, state)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if _, present := restored.Globals()["GREETING"]; present {
		t.Fatal("absent global reappeared after restore")
	}
	out, _ := runIndexed(t, restored, state.Next)
	if !strings.Contains(out, "[absent]") {
		t.Fatalf("absence not preserved: %q", out)
	}
}

func TestNewSessionFromStateRejectsInvalidState(t *testing.T) {
	d := loadFlow(t)
	base := SessionState{Next: 1}
	for _, tc := range []struct {
		name    string
		edit    func(*SessionState)
		wantErr string
	}{
		{"next past end", func(s *SessionState) { s.Next = 99 }, ""},
		{"negative next", func(s *SessionState) { s.Next = -1 }, ""},
		{"undeclared global", func(s *SessionState) { s.Globals = map[string]string{"NOPE": "x"} }, ""},
		{"undeclared absent", func(s *SessionState) { s.Absent = []string{"NOPE"} }, ""},
		{"both present and absent", func(s *SessionState) {
			s.Globals = map[string]string{"GREETING": "x"}
			s.Absent = []string{"GREETING"}
		}, "both present and absent"},
		{"absent twice", func(s *SessionState) {
			s.Absent = []string{"GREETING", "GREETING"}
		}, "listed as absent twice"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := base
			st.Globals = map[string]string{}
			tc.edit(&st)
			s, err := NewSessionFromState(d, Options{}, st)
			if err == nil {
				s.Close()
				t.Fatal("invalid state accepted")
			}
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestFreshStateKeepsDefaults(t *testing.T) {
	d := loadFlow(t)
	s, err := NewSession(d, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got := s.Globals()["DECLARED_CHECK"]; got != "declared default" {
		t.Fatalf("default global = %q", got)
	}
	if s.Remaining() != len(d.Blocks) {
		t.Fatal("fresh state changed the block cursor")
	}
}
