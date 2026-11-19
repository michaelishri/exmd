package exec

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/michaelishri/md-exec/internal/doc"
)

// runDoc loads a testdata document and runs it with buffers for output and
// a collected event log.
func runDoc(t *testing.T, name string, tune func(*Options)) (*RunResult, *bytes.Buffer, *bytes.Buffer, []Event) {
	return runDocCtx(t, context.Background(), name, tune)
}

func runDocCtx(t *testing.T, ctx context.Context, name string, tune func(*Options)) (*RunResult, *bytes.Buffer, *bytes.Buffer, []Event) {
	t.Helper()
	res, err := doc.Load(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	res.Preflight()
	if res.HasErrors() {
		t.Fatalf("fixture has errors:\n%s", diagText(res))
	}

	var outBuf, errBuf bytes.Buffer
	var events []Event
	opts := Options{
		Stdout: &outBuf,
		Stderr: &errBuf,
		Events: func(ev Event) { events = append(events, ev) },
	}
	if tune != nil {
		tune(&opts)
	}

	result, err := Run(ctx, res.Doc, opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return result, &outBuf, &errBuf, events
}

func diagText(res *doc.Result) string {
	var sb strings.Builder
	for _, d := range res.Diagnostics {
		fmt.Fprintln(&sb, d.Message)
	}
	return sb.String()
}

func TestBashExportFlow(t *testing.T) {
	result, out, _, events := runDoc(t, "flow.md", nil)

	if result.Failed != nil {
		t.Fatalf("unexpected failure: block %d status %d", result.Failed.Index, result.ExitStatus)
	}
	if result.BlocksRun != 4 {
		t.Fatalf("blocks run = %d, want 4", result.BlocksRun)
	}

	want := "hello from block 1\n[hello world]\n[absent]\n[absent]\n[absent]\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}

	foundWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "exported undeclared variable UNDECLARED") {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Fatalf("missing undeclared-export warning: %v", result.Warnings)
	}
	// UNDECLARED must not have leaked into the globals.
	if _, leaked := result.Globals["UNDECLARED"]; leaked {
		t.Fatal("undeclared export leaked into globals")
	}

	// Events: 4 starts, 4 ends with status 0, at least one warning.
	var starts, ends int
	for _, ev := range events {
		switch ev.Kind {
		case EventBlockStart:
			starts++
		case EventBlockEnd:
			ends++
			if ev.Status != 0 {
				t.Fatalf("block %d ended with status %d", ev.Block.Index, ev.Status)
			}
		}
	}
	if starts != 4 || ends != 4 {
		t.Fatalf("events: %d starts, %d ends", starts, ends)
	}

	// Output must be attributable to its block (§4.8): chunks are per-write,
	// so concatenate block 2's chunks before matching.
	var block2 strings.Builder
	for _, ev := range events {
		if ev.Kind == EventOutput && ev.Block.Index == 2 {
			block2.Write(ev.Data)
		}
	}
	if got := block2.String(); got != "[hello world]\n[absent]\n" {
		t.Fatalf("block 2 output = %q, want %q", got, "[hello world]\n[absent]\n")
	}
}

func TestMultilineHarvest(t *testing.T) {
	result, out, _, _ := runDoc(t, "multiline.md", nil)
	if result.Failed != nil {
		t.Fatalf("unexpected failure: %v", result.ExitStatus)
	}
	want := "[p1\np2]\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
}

func TestMasking(t *testing.T) {
	hostEnv := []string{"FROM_HOST=from-hostenv", "PATH=/usr/bin:/bin"}
	result, out, _, _ := runDoc(t, "masking.md", func(o *Options) { o.HostEnv = hostEnv })
	if result.Failed != nil {
		t.Fatalf("unexpected failure: %v", result.ExitStatus)
	}
	want := "from-default\nchanged-by-block\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q (masking or harvest broken)", out.String(), want)
	}
}

func TestIsolation(t *testing.T) {
	result, out, _, _ := runDoc(t, "isolation.md", nil)
	if result.Failed != nil {
		t.Fatalf("unexpected failure: %v", result.ExitStatus)
	}
	// Block 2's pwd must be docdir — the `cd /` did not leak.
	dir := filepath.Join(mustPwd(t), "internal", "exec", "testdata")
	if !strings.HasSuffix(strings.TrimSpace(out.String()), filepath.Base(dir)) {
		t.Fatalf("pwd output = %q, want it to end in %q", out.String(), dir)
	}
}

func mustPwd(t *testing.T) string {
	t.Helper()
	wd, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(filepath.Dir(wd)) // package dir's parent is internal/
}

func TestFailureStopsAndStillHarvests(t *testing.T) {
	result, out, _, _ := runDoc(t, "fail.md", nil)

	if result.Failed == nil {
		t.Fatal("expected a failing block")
	}
	if result.Failed.Index != 2 || result.ExitStatus != 3 {
		t.Fatalf("failed = block %d status %d, want block 2 status 3", result.Failed.Index, result.ExitStatus)
	}
	if result.BlocksRun != 2 {
		t.Fatalf("blocks run = %d, want 2", result.BlocksRun)
	}
	if !strings.Contains(out.String(), "about to fail") {
		t.Fatalf("stdout missing pre-failure output: %q", out.String())
	}
	if strings.Contains(out.String(), "never runs") {
		t.Fatal("blocks ran after the failure")
	}
	// Harvest applies even on failure (§4.3, §4.7).
	if result.Globals["BEFORE"] != "survives" {
		t.Fatalf("harvest from failing block not applied: %v", result.Globals)
	}
}

func TestPythonExitCode(t *testing.T) {
	result, _, _, _ := runDoc(t, "pyexit.md", nil)
	if result.Failed == nil || result.Failed.Index != 1 || result.ExitStatus != 4 {
		t.Fatalf("failed = %+v status %d, want block 1 status 4", result.Failed, result.ExitStatus)
	}
	if result.BlocksRun != 1 {
		t.Fatalf("blocks run = %d, want 1", result.BlocksRun)
	}
}

func TestPythonAliasAndTraceback(t *testing.T) {
	result, _, errOut, _ := runDoc(t, "pyboom.md", nil)
	if result.Failed == nil || result.ExitStatus != 1 {
		t.Fatalf("failed = %+v status %d, want block 1 status 1", result.Failed, result.ExitStatus)
	}
	if !strings.Contains(errOut.String(), "RuntimeError: boom") {
		t.Fatalf("stderr missing traceback: %q", errOut.String())
	}
}

func TestScriptRefs(t *testing.T) {
	result, out, _, _ := runDoc(t, "scripts.md", nil)
	if result.Failed != nil {
		t.Fatalf("unexpected failure: %v", result.ExitStatus)
	}
	want := "[from-script] [pydefault]\n[py-changed]\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
}

func TestStdinIsEmpty(t *testing.T) {
	result, out, _, _ := runDoc(t, "stdin.md", nil)
	if result.Failed != nil {
		t.Fatalf("unexpected failure: %v", result.ExitStatus)
	}
	if !strings.Contains(out.String(), "stdin was empty") {
		t.Fatalf("cat did not see an empty stdin: %q", out.String())
	}
}

func TestUndeclaredOverrideRefused(t *testing.T) {
	res, err := doc.Load(filepath.Join("testdata", "flow.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Run(context.Background(), res.Doc, Options{Overrides: map[string]string{"NOT_DECLARED": "x"}})
	if err == nil || !strings.Contains(err.Error(), "unknown global `NOT_DECLARED`") {
		t.Fatalf("expected refusal, got %v", err)
	}
}

func TestCancelInterruptsBlock(t *testing.T) {
	// The block sleeps for 5s; cancelling after 200ms must terminate the
	// process group quickly (§4.7) and stop the run there.
	ctx, cancel := context.WithCancel(context.Background())
	timer := time.AfterFunc(200*time.Millisecond, cancel)
	defer timer.Stop()

	start := time.Now()
	result, _, _, _ := runDocCtx(t, ctx, "sleep.md", nil)
	elapsed := time.Since(start)

	if result.Failed == nil || result.Failed.Index != 1 {
		t.Fatalf("failed = %+v, want block 1", result.Failed)
	}
	if !result.Interrupted {
		t.Fatal("expected Interrupted")
	}
	if result.BlocksRun != 1 {
		t.Fatalf("blocks run = %d, want 1", result.BlocksRun)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("cancellation took %v — the process group was not killed", elapsed)
	}
}

// TestRunBlockIsolation checks RunBlock: the block at the given index runs
// in isolation on the launch-time state, the harvest applies to it, and the
// session continues after that block.
func TestRunBlockIsolation(t *testing.T) {
	res, err := doc.Load(filepath.Join("testdata", "flow.md"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSession(res.Doc, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Block 2 depends on the global block 1 exports — running it in
	// isolation means that global is absent (the launch-time state).
	var events []Event
	emit := func(ev Event) { events = append(events, ev) }

	br := s.RunBlock(context.Background(), 1, emit)
	if br.Done || br.Block.Index != 2 {
		t.Fatalf("run block = %+v", br)
	}
	if br.Status != 0 {
		t.Fatalf("status = %d", br.Status)
	}
	// The session continues after the block.
	if s.Remaining() != len(res.Doc.Blocks)-2 {
		t.Fatalf("remaining = %d", s.Remaining())
	}

	// The events cover exactly the one block plus the initial snapshot.
	var starts, ends int
	for _, ev := range events {
		switch ev.Kind {
		case EventBlockStart:
			starts++
		case EventBlockEnd:
			ends++
		}
	}
	if starts != 1 || ends != 1 {
		t.Fatalf("events: %d starts, %d ends (want 1/1)", starts, ends)
	}
}

func TestSessionStepping(t *testing.T) {
	res, err := doc.Load(filepath.Join("testdata", "flow.md"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSession(res.Doc, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Step through blocks one at a time; state carries between steps (§4.3).
	var lastOut bytes.Buffer
	opts := s.opts // no output writers; capture via engine events instead
	_ = opts
	events := []Event{}
	emit := func(ev Event) { events = append(events, ev) }

	br := s.RunNext(context.Background(), emit)
	if br.Done || br.Block.Index != 1 {
		t.Fatalf("first step = %+v", br)
	}
	if s.Remaining() != 3 {
		t.Fatalf("remaining = %d, want 3", s.Remaining())
	}

	// After block 1 the exported global is visible to the session state.
	if s.Globals()["GREETING"] != "hello world" {
		t.Fatalf("GREETING = %q", s.Globals()["GREETING"])
	}

	br = s.RunNext(context.Background(), emit)
	if br.Block.Index != 2 {
		t.Fatalf("second step = %+v", br)
	}

	// Run the rest.
	for {
		br = s.RunNext(context.Background(), emit)
		if br.Done {
			break
		}
	}
	if s.Result().BlocksRun != 4 {
		t.Fatalf("blocks run = %d", s.Result().BlocksRun)
	}

	// Events must include the initial globals snapshot and one per harvest.
	var snapshots int
	for _, ev := range events {
		if ev.Kind == EventGlobals {
			snapshots++
		}
	}
	if snapshots != 5 { // initial + one per block
		t.Fatalf("globals snapshots = %d, want 5", snapshots)
	}
	_ = lastOut
}
