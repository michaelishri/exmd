package exec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"path/filepath"
	"sort"
	"syscall"
	"time"

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

	// Interrupted is true when the run was cancelled: the current block was
	// terminated (§4.7) and the run stopped there.
	Interrupted bool

	// Warnings collects closed-world notices (§4.4), one per name per block.
	Warnings []string

	// Globals is a snapshot of the global state where the run stopped.
	Globals map[string]string
}

// BlockResult reports one block's execution via Session.RunNext.
type BlockResult struct {
	Block       *doc.Block // nil when Done
	Status      int
	Done        bool // no more blocks to run
	Interrupted bool // this block was terminated by context cancellation
}

// Session executes a document block by block, carrying the global state
// (§4.3) between them. Interfaces drive it directly — RunNext for
// step-through, the Run convenience for whole-document runs.
//
// A Session is not safe for concurrent use; interfaces MUST only touch it
// while no RunNext is in flight.
type Session struct {
	d        *doc.Document
	opts     Options
	declared map[string]struct{}
	globals  map[string]string
	hostEnv  []string
	tmpDir   string

	next        int // index of the next block to run
	blocksRun   int
	warnings    []string
	lastFailed  *doc.Block
	lastStatus  int
	interrupted bool

	emittedInitial bool
	blockKilled    bool
	closed         bool
}

// NewSession resolves launch-time overrides (§2.2), stages the harnesses,
// and prepares to execute blocks in order.
func NewSession(d *doc.Document, opts Options) (*Session, error) {
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
	if err := writeHarnesses(tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}

	return &Session{
		d:        d,
		opts:     opts,
		declared: declared,
		globals:  present,
		hostEnv:  hostEnv,
		tmpDir:   tmpDir,
	}, nil
}

// Remaining reports how many blocks have not run yet.
func (s *Session) Remaining() int {
	return len(s.d.Blocks) - s.next
}

// Globals returns a snapshot of the current global state.
func (s *Session) Globals() map[string]string {
	out := make(map[string]string, len(s.globals))
	for name, val := range s.globals {
		out[name] = val
	}
	return out
}

// Warnings returns the closed-world notices accumulated so far (§4.4).
func (s *Session) Warnings() []string {
	return append([]string(nil), s.warnings...)
}

// Result snapshots the run's outcome so far.
func (s *Session) Result() *RunResult {
	res := &RunResult{
		BlocksRun:   s.blocksRun,
		Interrupted: s.interrupted,
		Warnings:    s.Warnings(),
		Globals:     s.Globals(),
	}
	if s.lastFailed != nil {
		res.Failed = s.lastFailed
		res.ExitStatus = s.lastStatus
	}
	return res
}

// Close releases the session's staging directory. RunNext on a closed
// session reports Done.
func (s *Session) Close() {
	if s.closed {
		return
	}
	s.closed = true
	os.RemoveAll(s.tmpDir)
}

// RunNext executes the next block in document order (§4.1). It emits the
// start/output/end observations plus the initial and post-harvest globals
// snapshots (§4.8), and applies the harvest delta before the block's status
// is evaluated (§4.3, §4.7).
//
// Cancelling ctx terminates the current process group — SIGTERM, then
// SIGKILL after a grace period (§4.7) — and the run stops there.
func (s *Session) RunNext(ctx context.Context, emit func(Event)) BlockResult {
	if s.closed || s.next >= len(s.d.Blocks) {
		return BlockResult{Done: true}
	}
	return s.runAt(ctx, s.next, emit)
}

// RunBlock executes the block at index (0-based) in isolation, regardless
// of the session's position: the state is the launch-time state (frontmatter
// defaults plus overrides, §2.2–§4.2) and the harvest applies to it (§4.3).
// Afterwards the session continues after the block.
func (s *Session) RunBlock(ctx context.Context, index int, emit func(Event)) BlockResult {
	if s.closed || index < 0 || index >= len(s.d.Blocks) {
		return BlockResult{Done: true}
	}
	return s.runAt(ctx, index, emit)
}

// runAt executes the block at index and advances the session past it.
func (s *Session) runAt(ctx context.Context, index int, emit func(Event)) BlockResult {
	if ctx == nil {
		ctx = context.Background()
	}
	if emit == nil {
		emit = func(Event) {}
	}
	if !s.emittedInitial {
		s.emittedInitial = true
		emit(Event{Kind: EventGlobals, Globals: s.Globals()})
	}

	s.blockKilled = false
	b := s.d.Blocks[index]
	s.next = index + 1
	s.blocksRun++

	emit(Event{Kind: EventBlockStart, Block: b})
	status := s.runBlock(ctx, b, emit)
	emit(Event{Kind: EventBlockEnd, Block: b, Status: status})

	if status != 0 {
		s.lastFailed = b
		s.lastStatus = status
		if s.blockKilled {
			s.interrupted = true
		}
	}
	return BlockResult{Block: b, Status: status, Interrupted: s.blockKilled}
}

// warn records a closed-world notice and emits it (§4.4).
func (s *Session) warn(emit func(Event), format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	s.warnings = append(s.warnings, msg)
	emit(Event{Kind: EventWarning, Message: msg})
}

// runBlock executes one block (§4.2–§4.6) and applies its harvest delta
// before its status is evaluated (§4.3, §4.7).
func (s *Session) runBlock(ctx context.Context, b *doc.Block, emit func(Event)) int {
	adapter, ok := Lookup(b.Lang)
	if !ok {
		// Pre-flight (§5.3) prevents this; fail loudly if a caller skipped it.
		s.warn(emit, "no adapter for language `%s`", b.Lang)
		return 127
	}

	envIn := compose(s.hostEnv, s.declared, s.globals)

	var srcPath string
	if b.Kind == doc.KindScript {
		srcPath = b.ScriptAbs
	} else {
		p := filepath.Join(s.tmpDir, fmt.Sprintf("block-%03d.%s", b.Index, adapter.Ext))
		if err := os.WriteFile(p, []byte(b.Body), 0o600); err != nil {
			s.warn(emit, "cannot stage block source: %v", err)
			return 126
		}
		srcPath = p
	}

	envOut := filepath.Join(s.tmpDir, fmt.Sprintf("env-%03d.txt", b.Index))
	if f, err := os.Create(envOut); err != nil {
		s.warn(emit, "cannot stage env dump: %v", err)
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

	cmd := adapter.command(s.tmpDir)
	cmd.Dir = s.d.Dir // blocks run in docdir (§4.2)
	cmd.Env = childEnv
	cmd.Stdin = nil // empty stream: runbooks are non-interactive (§4.6)
	cmd.Stdout = s.writer(b, StreamStdout, emit)
	cmd.Stderr = s.writer(b, StreamStderr, emit)
	// Own process group so cancellation can terminate the harness and
	// everything it spawned as a unit (§4.7). Unix only.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		s.warn(emit, "cannot start `%s`: %v", adapter.Bin, err)
		return 126
	}

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	status := 0
	select {
	case err := <-waitCh:
		if err != nil {
			var ee *osexec.ExitError
			if errors.As(err, &ee) {
				status = ee.ExitCode() // -1 when signal-killed
			} else {
				status = -1
			}
		}
	case <-ctx.Done():
		// §4.7: terminate the current block — SIGTERM the group, then
		// SIGKILL after a grace period. The EXIT trap / atexit hook will
		// not run; a missing dump simply means no harvest.
		s.blockKilled = true
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		select {
		case <-waitCh:
		case <-time.After(2 * time.Second):
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-waitCh
		}
		if cmd.ProcessState != nil {
			status = cmd.ProcessState.ExitCode()
		} else {
			status = -1
		}
	}

	s.harvest(b, childEnv, envOut, emit)
	emit(Event{Kind: EventGlobals, Globals: s.Globals()})
	return status
}

// harvest applies the block's environment delta to the globals (§4.3): the
// closed world keeps undeclared names out (§4.4), removals propagate, and
// the delta applies even when the block failed (§4.7).
func (s *Session) harvest(b *doc.Block, envIn []string, envOut string, emit func(Event)) {
	data, err := os.ReadFile(envOut)
	if err != nil || len(data) == 0 {
		// No usable dump (e.g. the block was killed or replaced its
		// process): no delta.
		return
	}
	set, removed := diffEnv(envIn, parseEnv(data))

	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, managed := shellManaged[name]; managed {
			continue // interpreter infrastructure, never document state (§5.1.6)
		}
		if _, declared := s.declared[name]; declared {
			s.globals[name] = set[name]
			continue
		}
		s.warn(emit, "exported undeclared variable %s (ignored)", name)
	}
	for _, name := range removed {
		if _, managed := shellManaged[name]; managed {
			continue
		}
		if _, declared := s.declared[name]; declared {
			delete(s.globals, name)
		}
	}
}

// writer returns the destination for a block's stream: the caller's writer
// plus, when events are enabled, an observation tap (§4.8).
func (s *Session) writer(b *doc.Block, st Stream, emit func(Event)) io.Writer {
	var out io.Writer = io.Discard
	if st == StreamStdout && s.opts.Stdout != nil {
		out = s.opts.Stdout
	}
	if st == StreamStderr && s.opts.Stderr != nil {
		out = s.opts.Stderr
	}
	if emit == nil {
		return out
	}
	return io.MultiWriter(out, &eventWriter{emit: emit, b: b, st: st})
}

type eventWriter struct {
	emit func(Event)
	b    *doc.Block
	st   Stream
}

func (w *eventWriter) Write(p []byte) (int, error) {
	w.emit(Event{
		Kind:   EventOutput,
		Block:  w.b,
		Stream: w.st,
		Data:   append([]byte(nil), p...),
	})
	return len(p), nil
}

// Run executes the document's blocks in order (§4.1) and returns how it
// ended. I/O errors — and refusal via an undeclared override — return an
// error; block failures are reported in the result. Cancelling ctx stops
// the run at the current block (§4.7).
func Run(ctx context.Context, d *doc.Document, opts Options) (*RunResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	s, err := NewSession(d, opts)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	emit := func(ev Event) {
		if opts.Events != nil {
			opts.Events(ev)
		}
	}
	for {
		if s.RunNext(ctx, emit).Done {
			break
		}
		if s.lastFailed != nil {
			break // §4.7: stop at the first failure
		}
	}
	return s.Result(), nil
}
