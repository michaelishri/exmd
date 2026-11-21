package render

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/michaelishri/exmd/internal/theme"
)

// sgr matches an ANSI Select Graphic Rendition sequence.
var sgr = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// backgroundEscape converts a theme color into a background SGR. It uses
// termenv's conversion so the hand-painted canvas matches the escapes
// lipgloss emits for the same token in the chrome. An empty token yields no
// escape, which is how "notty" renders plain text.
func backgroundEscape(c theme.Color) string {
	if c == "" {
		return ""
	}
	seq := termenv.TrueColor.Color(string(c)).Sequence(true)
	if seq == "" {
		return ""
	}
	return "\x1b[" + seq + "m"
}

// PaintCanvas paints the entire view with the theme's canvas background.
// Every written cell gets an explicit background: relying on the terminal's
// own would mix the theme with the terminal palette on light backgrounds.
//
// A reset inside a line is followed by the canvas again, unless the next
// escape explicitly sets a surface of its own (an executable block or an
// inline code span). That keeps each surface confined to the cells it was
// drawn on instead of bleeding to the end of the row.
func PaintCanvas(view string, width int, th theme.Theme) string {
	if th.Name == theme.Notty.Name {
		return view
	}
	canvas := backgroundEscape(th.Background)
	if canvas == "" {
		return view
	}
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		lines[i] = paintLine(line, width, canvas)
	}
	return strings.Join(lines, "\n")
}

func paintLine(line string, width int, canvas string) string {
	var sb strings.Builder
	sb.WriteString(canvas)

	last := 0
	seqs := sgr.FindAllStringIndex(line, -1)
	for i, loc := range seqs {
		sb.WriteString(line[last:loc[0]])
		seq := line[loc[0]:loc[1]]
		sb.WriteString(seq)
		if isReset(seq) && !nextSetsBackground(line, seqs, i) {
			sb.WriteString(canvas)
		}
		last = loc[1]
	}
	sb.WriteString(line[last:])

	if w := ansi.StringWidth(sb.String()); w < width {
		sb.WriteString(strings.Repeat(" ", width-w))
	}
	sb.WriteString("\x1b[0m")
	return sb.String()
}

// nextSetsBackground reports whether the escape immediately after the one
// at index i sets a background. Such an escape is the producer re-establishing
// its own surface; adding the canvas here would overwrite it.
func nextSetsBackground(line string, seqs [][]int, i int) bool {
	if i+1 >= len(seqs) || seqs[i+1][0] != seqs[i][1] {
		return false
	}
	return setsBackground(line[seqs[i+1][0]:seqs[i+1][1]])
}

func isReset(seq string) bool {
	params := paramsOf(seq)
	if params == nil {
		return false
	}
	for _, p := range params {
		if p == "" || p == "0" {
			return true
		}
	}
	return false
}

func setsBackground(seq string) bool {
	for _, p := range paramsOf(seq) {
		switch p {
		case "40", "41", "42", "43", "44", "45", "46", "47", "48":
			return true
		}
	}
	return false
}

func paramsOf(seq string) []string {
	if len(seq) < 3 || seq[0] != '\x1b' || seq[1] != '[' || seq[len(seq)-1] != 'm' {
		return nil
	}
	return strings.Split(strings.Trim(seq[2:len(seq)-1], ";"), ";")
}
