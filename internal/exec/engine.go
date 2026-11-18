package exec

import (
	"errors"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/michaelishri/md-exec/internal/doc"
)

// Options configures a run.
type Options struct {
	// Overrides are launch-time overrides for declared globals (§2.2);
	// overriding an undeclared name is refused.
	Overrides map[string]string

	// HostEnv is the host environment snapshot (§4.2); nil → os.Environ().
	HostEnv []string

	// Stdout and Stderr receive block output live (§4.8); nil → discarded.
	Stdout io.Writer
	Stderr io.Writer

	// Events receives execution observations; nil → none.
	Events func(Event)
}

// RunResult reports how a run ended (§4.7).
type RunResult struct {
	// BlocksRun counts attempted blocks, including a failing one.
	BlocksRun int

	// Failed is the first block that exited non-zero, or nil when the whole
	// document succeeded.
	Failed     *doc.Block
	ExitStatus int // status of the failing block (-1 when signal-killed)

	// Warnings collects closed-world notices (§4.4), one per name per block.
	Warnings []string

	// Globals is a snapshot of the global state where the run stopped.
	Globals map[string]string
}

// Run executes the document's blocks in order (§4.1) and returns how it
// ended. I/O errors — and refusal via an undeclared override — return an
// error; block failures are reported in the result.
func Run(d *doc.Document, opts Options) (*RunResult, error) {
	overrides := opts.Overrides
	if overrides == nil {
		overrides = map[string]string{}
	}
	present, diags := d.ResolveGlobals(overrides)
	for _, dg := range diags {
		if dg.Severity == doc.SeverityError {
			return nil, fmt.Errorf("launch-time overrides: %s", dg.Message)
		}
	}

	hostEnv := opts.HostEnv
	if hostEnv == nil {
		hostEnv = os.Environ()
	}
	declared := make(map[string]struct{}, len(d.Globals))
	for name := range d.Globals {
		declared[name] = struct{}{}
	}

	tmpDir, err := os.MkdirTemp("", "mdexec-")
	if err != nil {
		return nil, fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	if err := writeHarnesses(tmpDir); err != nil {
		return nil, err
	}

	e := &engine{
		d:        d,
		opts:     opts,
		declared: declared,
		globals:  present,
		hostEnv:  hostEnv,
		tmpDir:   tmpDir,
	}
	return e.run(), nil
}

type engine struct {
	d        *doc.Document
	opts     Options
	declared map[string]struct{}
	globals  map[string]string
	hostEnv  []string
	tmpDir   string
}

func (e *engine) emit(ev Event) {
	if e.opts.Events != nil {
		e.opts.Events(ev)
	}
}

func (e *engine) warn(res *RunResult, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	res.Warnings = append(res.Warnings, msg)
	e.emit(Event{Kind: EventWarning, Message: msg})
}

func (e *engine) run() *RunResult {
	res := &RunResult{}
	defer func() {
		res.Globals = make(map[string]string, len(e.globals))
		for name, val := range e.globals {
			res.Globals[name] = val
		}
	}()

	for _, b := range e.d.Blocks {
		res.BlocksRun++
		e.emit(Event{Kind: EventBlockStart, Block: b})
		status := e.runBlock(b, res)
		e.emit(Event{Kind: EventBlockEnd, Block: b, Status: status})
		if status != 0 {
			res.Failed = b
			res.ExitStatus = status
			return res
		}
	}
	return res
}

// runBlock executes one block (§4.2–§4.6) and applies its harvest delta
// before its status is evaluated (§4.3, §4.7).
func (e *engine) runBlock(b *doc.Block, res *RunResult) int {
	adapter, ok := Lookup(b.Lang)
	if !ok {
		// Pre-flight (§5.3) prevents this; fail loudly if a caller skipped it.
		e.warn(res, "no adapter for language `%s`", b.Lang)
		return 127
	}

	envIn := compose(e.hostEnv, e.declared, e.globals)

	var srcPath string
	if b.Kind == doc.KindScript {
		srcPath = b.ScriptAbs
	} else {
		p := filepath.Join(e.tmpDir, fmt.Sprintf("block-%03d.%s", b.Index, adapter.Ext))
		if err := os.WriteFile(p, []byte(b.Body), 0o600); err != nil {
			e.warn(res, "cannot stage block source: %v", err)
			return 126
		}
		srcPath = p
	}

	envOut := filepath.Join(e.tmpDir, fmt.Sprintf("env-%03d.txt", b.Index))
	if f, err := os.Create(envOut); err != nil {
		e.warn(res, "cannot stage env dump: %v", err)
		return 126
	} else {
		f.Close()
	}

	childEnv := make([]string, 0, len(envIn)+2)
	childEnv = append(childEnv, envIn...)
	childEnv = append(childEnv,
		"__MDEXEC_BLOCK="+srcPath,
		"__MDEXEC_ENVOUT="+envOut,
	)

	cmd := adapter.command(e.tmpDir)
	cmd.Dir = e.d.Dir // blocks run in docdir (§4.2)
	cmd.Env = childEnv
	cmd.Stdin = nil // empty stream: runbooks are non-interactive (§4.6)
	cmd.Stdout = e.writer(b, StreamStdout)
	cmd.Stderr = e.writer(b, StreamStderr)

	status := 0
	if err := cmd.Run(); err != nil {
		var ee *osexec.ExitError
		if errors.As(err, &ee) {
			status = ee.ExitCode() // -1 when signal-killed
		} else {
			e.warn(res, "cannot start `%s`: %v", adapter.Bin, err)
			return 126
		}
	}

	e.harvest(b, childEnv, envOut, res)
	return status
}

// harvest applies the block's environment delta to the globals (§4.3): the
// closed world keeps undeclared names out (§4.4), removals propagate, and
// the delta applies even when the block failed (§4.7).
func (e *engine) harvest(b *doc.Block, envIn []string, envOut string, res *RunResult) {
	data, err := os.ReadFile(envOut)
	if err != nil || len(data) == 0 {
		// No usable dump (e.g. the block replaced its process): no delta.
		return
	}
	set, removed := diffEnv(envIn, parseEnv(data))

	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, declared := e.declared[name]; declared {
			e.globals[name] = set[name]
			continue
		}
		e.warn(res, "exported undeclared variable %s (ignored)", name)
	}
	for _, name := range removed {
		if _, declared := e.declared[name]; declared {
			delete(e.globals, name)
		}
	}
}

// writer returns the destination for a block's stream: the caller's writer
// plus, when events are enabled, an observation tap (§4.8).
func (e *engine) writer(b *doc.Block, s Stream) io.Writer {
	var out io.Writer = io.Discard
	if s == StreamStdout && e.opts.Stdout != nil {
		out = e.opts.Stdout
	}
	if s == StreamStderr && e.opts.Stderr != nil {
		out = e.opts.Stderr
	}
	if e.opts.Events == nil {
		return out
	}
	return io.MultiWriter(out, &eventWriter{e: e, b: b, s: s})
}

type eventWriter struct {
	e *engine
	b *doc.Block
	s Stream
}

func (w *eventWriter) Write(p []byte) (int, error) {
	w.e.emit(Event{
		Kind:   EventOutput,
		Block:  w.b,
		Stream: w.s,
		Data:   append([]byte(nil), p...),
	})
	return len(p), nil
}

// GlobalsSnapshot returns a copy of the engine's current global state.
func (e *engine) GlobalsSnapshot() map[string]string {
	out := make(map[string]string, len(e.globals))
	for name, val := range e.globals {
		out[name] = val
	}
	return out
}

var _ = strings.TrimSpace
