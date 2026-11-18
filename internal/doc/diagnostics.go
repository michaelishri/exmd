// Package doc implements the document load path of the md-exec specification
// (SPEC.md §2): frontmatter parsing and validation, executable-fence
// extraction (§3), the document outline, and collect-all load-time
// diagnostics (§2.3).
//
// Loading never aborts on the first problem: every diagnosable issue is
// collected so `md-exec check` can report them all at once. Execution is
// refused entirely when any error-severity diagnostic is present (§6.1).
package doc

import (
	"fmt"
	"sort"
)

// Severity classifies a diagnostic (SPEC.md §6.1).
type Severity int

const (
	// SeverityError marks a load-time problem that prevents execution.
	SeverityError Severity = iota
	// SeverityWarning marks a recoverable problem, such as unknown
	// frontmatter keys (SPEC.md §2.1).
	SeverityWarning
)

func (s Severity) String() string {
	if s == SeverityWarning {
		return "warning"
	}
	return "error"
}

// Diagnostic is a single load-time or pre-flight finding.
type Diagnostic struct {
	Severity Severity
	Line     int // 1-based source line; 0 when not line-specific
	Block    int // 1-based executable-block index; 0 when not block-specific
	Message  string
}

func (d Diagnostic) String() string {
	switch {
	case d.Block > 0 && d.Line > 0:
		return fmt.Sprintf("block %d (line %d): %s: %s", d.Block, d.Line, d.Severity, d.Message)
	case d.Block > 0:
		return fmt.Sprintf("block %d: %s: %s", d.Block, d.Severity, d.Message)
	case d.Line > 0:
		return fmt.Sprintf("line %d: %s: %s", d.Line, d.Severity, d.Message)
	default:
		return fmt.Sprintf("%s: %s", d.Severity, d.Message)
	}
}

// HasErrors reports whether any error-severity diagnostic is present.
func (r *Result) HasErrors() bool {
	for _, d := range r.Diagnostics {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Counts returns the number of errors and warnings.
func (r *Result) Counts() (errors, warnings int) {
	for _, d := range r.Diagnostics {
		if d.Severity == SeverityError {
			errors++
		} else {
			warnings++
		}
	}
	return errors, warnings
}

// Sort orders diagnostics by line, then errors before warnings, then block.
func (r *Result) Sort() {
	sort.SliceStable(r.Diagnostics, func(i, j int) bool {
		a, b := r.Diagnostics[i], r.Diagnostics[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Severity != b.Severity {
			return a.Severity < b.Severity
		}
		return a.Block < b.Block
	})
}
