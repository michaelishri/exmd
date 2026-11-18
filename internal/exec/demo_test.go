package exec

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/michaelishri/md-exec/internal/doc"
)

// TestDemoRunbook is the end-to-end anchor: the feature-tour runbook from
// examples/demo must execute cleanly through the engine and demonstrate the
// spec's headline behaviors.
func TestDemoRunbook(t *testing.T) {
	res, err := doc.Load(filepath.Join("..", "..", "examples", "demo", "demo.md"))
	if err != nil {
		t.Fatalf("load demo: %v", err)
	}
	res.Preflight()
	if res.HasErrors() {
		t.Fatalf("demo has errors")
	}

	var out bytes.Buffer
	var warnings []string
	result, err := Run(context.Background(), res.Doc, Options{
		Overrides: map[string]string{"JOB_NAME": "nightly"},
		Stdout:    &out,
		Events: func(ev Event) {
			if ev.Kind == EventWarning {
				warnings = append(warnings, ev.Message)
			}
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Failed != nil {
		t.Fatalf("demo failed at block %d status %d", result.Failed.Index, result.ExitStatus)
	}
	if result.BlocksRun != len(res.Doc.Blocks) {
		t.Fatalf("blocks run = %d, want %d", result.BlocksRun, len(res.Doc.Blocks))
	}

	text := out.String()
	checks := []string{
		"in the next block:",          // §4.5 isolation (pwd back at docdir)
		"registered: dpl-",            // §5.2 python alias harvest (block 4)
		"prepared ",                   // §3.2 script reference (helper.sh ran)
		"first line\nsecond",          // §4.3 lossless multiline (LOG_LINES)
		"<absent — unexported",        // unexported state invisible
		"<absent — unset propagated>", // unset removal (§4.3)
		"deploy id: dpl-",             // final state readable
	}
	for _, want := range checks {
		if !strings.Contains(text, want) {
			t.Errorf("demo output missing %q", want)
		}
	}

	var warnedUndeclared bool
	for _, w := range warnings {
		if strings.Contains(w, "exported undeclared variable UNDECLARED_X") {
			warnedUndeclared = true
		}
	}
	if !warnedUndeclared {
		t.Errorf("demo should warn about UNDECLARED_X, got %v", warnings)
	}

	// Final global state.
	if result.Globals["DEPLOY_ID"] == "" {
		t.Errorf("DEPLOY_ID not set: %v", result.Globals)
	}
	if _, present := result.Globals["WORK"]; present {
		t.Errorf("WORK should have been unset by cleanup: %v", result.Globals)
	}
}
