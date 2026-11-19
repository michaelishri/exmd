package render

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/michaelishri/exmd/internal/doc"
)

// A heading that already begins with a number (1, 1., 5.1, 2), …) is left
// exactly as written. A single such heading suppresses numbering for the
// whole document, so mixed documents never gain a second numbering scheme.
var headingNumberPrefix = regexp.MustCompile(`^\s*\d+(?:[.)]\d+)*[.)]?(?:\s|$)`)

func headingHasNumber(text string) bool {
	return headingNumberPrefix.MatchString(text)
}

// counter is one active level in the outline stack: the heading level and
// the number assigned at that level.
type counter struct {
	level int
	n     int
}

// headingNumbers returns a display number for every heading in document
// order, or "" when the heading should not be numbered. The first h1 is the
// document title and is never numbered.
//
// Numbers are hierarchical with a trailing dot: 1., 1.1., 1.1.1. A stack of
// active levels lets skipped levels fall back to the nearest parent — an h4
// directly under an h2 becomes 1.1., never 1.0.1.
func headingNumbers(d *doc.Document) []string {
	numbers := make([]string, len(d.Headings))
	if len(d.Headings) == 0 {
		return numbers
	}
	for _, h := range d.Headings {
		if headingHasNumber(h.Text) {
			return numbers // all empty: the document numbers its own headings
		}
	}

	title := -1
	for i, h := range d.Headings {
		if h.Level == 1 {
			title = i
			break
		}
	}

	var stack []counter
	top := 0
	for i, h := range d.Headings {
		if i == title {
			continue
		}
		for len(stack) > 0 && stack[len(stack)-1].level > h.Level {
			stack = stack[:len(stack)-1]
		}
		switch {
		case len(stack) == 0:
			top++
			stack = append(stack, counter{level: h.Level, n: top})
		case stack[len(stack)-1].level == h.Level:
			stack[len(stack)-1].n++
		default:
			// A deeper level starts a fresh child counter. Its siblings
			// increment it until the parent changes and it is popped.
			stack = append(stack, counter{level: h.Level, n: 1})
		}
		parts := make([]string, 0, len(stack))
		for _, c := range stack {
			parts = append(parts, strconv.Itoa(c.n))
		}
		numbers[i] = strings.Join(parts, ".") + "."
	}
	return numbers
}

// atxHeading matches a rendered heading's source line, split into its prefix
// (hashes), text, and optional closing hashes.
var atxHeading = regexp.MustCompile(`^(\s*#{1,6}\s+)(.*?)(\s*#+\s*)?$`)

// prefixHeadingNumber inserts a generated number into an ATX heading's
// render chunk. The document source is never changed.
func prefixHeadingNumber(chunk, number string) string {
	if number == "" {
		return chunk
	}
	lines := strings.Split(chunk, "\n")
	m := atxHeading.FindStringSubmatch(lines[0])
	if m == nil {
		return chunk
	}
	lines[0] = m[1] + number + " " + m[2] + m[3]
	return strings.Join(lines, "\n")
}
