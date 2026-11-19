// Package exec implements the execution engine of the exmd specification
// (SPEC.md §4–5): blocks run in fresh processes with the composed
// environment, state flows forward through the environment-delta harvest,
// and the world is closed over the frontmatter's declared globals.
//
// Events make execution observable (SPEC.md §4.8) so interfaces — the TUI
// today, stream runners later — can attribute every line of output to its
// block and show state as it evolves.
package exec

import (
	"fmt"

	"github.com/michaelishri/exmd/internal/doc"
)

// Stream identifies a block's output streams.
type Stream int

const (
	// StreamStdout is the block's standard output.
	StreamStdout Stream = iota
	// StreamStderr is the block's standard error.
	StreamStderr
)

// EventKind classifies engine events.
type EventKind int

const (
	// EventBlockStart precedes a block's execution.
	EventBlockStart EventKind = iota
	// EventOutput carries a chunk of a block's output. Data is copied.
	EventOutput
	// EventBlockEnd follows a block's execution; Status is its exit status.
	// The harvest delta has already been applied (SPEC.md §4.3).
	EventBlockEnd
	// EventWarning reports a closed-world violation (SPEC.md §4.4): an
	// undeclared export was ignored.
	EventWarning
	// EventGlobals carries a Globals snapshot: emitted once before the
	// first block and after each block's harvest, so interfaces can show
	// state evolving (§4.8).
	EventGlobals
)

func (k EventKind) String() string {
	switch k {
	case EventBlockStart:
		return "start"
	case EventOutput:
		return "output"
	case EventBlockEnd:
		return "end"
	case EventWarning:
		return "warning"
	case EventGlobals:
		return "globals"
	default:
		return fmt.Sprintf("EventKind(%d)", k)
	}
}

// Event is one observation from a run.
type Event struct {
	Kind    EventKind
	Block   *doc.Block
	Stream  Stream
	Data    []byte
	Status  int
	Message string
	Globals map[string]string // EventGlobals snapshot; treat as read-only
}
