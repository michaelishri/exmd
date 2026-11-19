package doc

import "path/filepath"

// BlockKind distinguishes embedded code from script references (SPEC.md §3.2).
type BlockKind int

const (
	// KindEmbedded is code written directly inside the fence.
	KindEmbedded BlockKind = iota
	// KindScript is a single-line `@path` script reference.
	KindScript
)

func (k BlockKind) String() string {
	if k == KindScript {
		return "script"
	}
	return "embedded"
}

// BlockAttribute is an attribute from a fence info string. v1 defines none
// (SPEC.md §3.1); attributes are recorded so they can be diagnosed.
type BlockAttribute struct {
	Key       string
	Value     string
	Malformed bool // token did not have a key=value shape
}

// Block is one executable fenced code block (SPEC.md §3).
type Block struct {
	Index      int // 1-based among executable blocks, in document order
	Lang       string
	Kind       BlockKind
	Body       string // embedded source with normalized newlines; "" for script references
	ScriptPath string // referenced path exactly as written (after `@`)
	ScriptAbs  string // ScriptPath resolved against the document directory
	Line       int    // 1-based line of the fence info string
	BodyLine   int    // 1-based line of the first body line
}

// Heading is a Markdown heading, used for the document outline.
type Heading struct {
	Level int // 1..6
	Text  string
	Line  int // 1-based source line
}

// Document is a parsed, validated document (SPEC.md §2).
type Document struct {
	Path      string
	Dir       string
	Title     string
	IsRunbook bool
	TypeValue string // `type` exactly as written ("" when absent)

	// Source is the BOM-stripped, newline-normalized source text; line
	// numbers are unchanged. BodyLine is the 1-based first line after the
	// frontmatter (1 when there is none).
	Source   []byte
	BodyLine int

	// Globals holds the declared globals (SPEC.md §2.2). A nil pointer is a
	// bare key: declared, starts absent.
	Globals map[string]*string

	Blocks   []*Block
	Headings []*Heading
}

// Result pairs the parsed document with collect-all diagnostics (SPEC.md §2.3).
type Result struct {
	Doc         *Document
	Diagnostics []Diagnostic
}

// HeadingText returns the document title for display, falling back to the
// file base name.
func (d *Document) HeadingText() string {
	if d.Title != "" {
		return d.Title
	}
	return filepath.Base(d.Path)
}
