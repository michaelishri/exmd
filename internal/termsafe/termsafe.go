// Package termsafe neutralizes terminal control characters in untrusted
// text before it reaches the terminal.
//
// A document (or a block's output) can carry raw escape sequences: OSC 52
// writes the clipboard, OSC 8 hides link targets, cursor movement and
// concealed text can make code look different from what will run. Text from
// a document is therefore cleaned before it is drawn:
//
//   - Visible replaces each control character with a printable marker (ESC
//     becomes ^[), so a reviewer can see that something is there.
//   - Strip drops control characters, for prose where markers would only
//     add noise.
//
// Newlines and tabs are layout, not control, and are kept by both.
package termsafe

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// isControl reports whether r can steer the terminal: C0 controls other than
// tab and newline, DEL, and the C1 range (U+0080–U+009F), whose 8-bit forms
// some terminals interpret as CSI, OSC and friends.
func isControl(r rune) bool {
	switch {
	case r == '\t' || r == '\n':
		return false
	case r < 0x20, r == 0x7f:
		return true
	case r >= 0x80 && r <= 0x9f:
		return true
	}
	return false
}

// needsWork reports whether s contains anything Visible or Strip would change.
func needsWork(s string) bool {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if (r == utf8.RuneError && size == 1) || isControl(r) {
			return true
		}
		i += size
	}
	return false
}

// Visible replaces control characters with printable markers: caret notation
// for C0 and DEL (ESC → ^[, BEL → ^G, DEL → ^?), <U+XXXX> for C1, and U+FFFD
// for bytes that are not valid UTF-8.
func Visible(s string) string {
	if !needsWork(s) {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s) + 8)
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		switch {
		case r == utf8.RuneError && size == 1:
			sb.WriteRune(utf8.RuneError)
		case r == 0x7f:
			sb.WriteString("^?")
		case r < 0x20 && isControl(r):
			sb.WriteByte('^')
			sb.WriteByte(byte(r) + 0x40)
		case isControl(r):
			fmt.Fprintf(&sb, "<U+%04X>", r)
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// Strip removes escape sequences whole, then any remaining control
// characters. Bytes that are not valid UTF-8 are dropped.
func Strip(s string) string {
	if !needsWork(s) {
		return s
	}
	s = ansi.Strip(s)
	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if (r == utf8.RuneError && size == 1) || isControl(r) {
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}
