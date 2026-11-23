package render

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/michaelishri/exmd/internal/theme"
)

func TestPaintCanvasCoversEveryLine(t *testing.T) {
	canvas := backgroundEscape(theme.Dark.Background)
	got := PaintCanvas("a\n\n  b", 8, theme.Dark)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	for i, line := range lines {
		if !strings.HasPrefix(line, canvas) {
			t.Errorf("line %d has no canvas background: %q", i, line)
		}
		if !strings.HasSuffix(line, "\x1b[0m") {
			t.Errorf("line %d does not end with a reset: %q", i, line)
		}
		if w := ansi.StringWidth(line); w != 8 {
			t.Errorf("line %d width = %d, want 8", i, w)
		}
	}
}

func TestPaintCanvasRestoresCanvasAfterStyledSpan(t *testing.T) {
	canvas := backgroundEscape(theme.Dark.Background)

	// An inline code span ends with a reset; the canvas must come back so
	// its shaded background does not bleed into the following prose.
	prose := "before" + "\x1b[48;2;26;26;32m" + "code" + "\x1b[0m" + " after"
	got := paintLine(prose, 40, canvas)
	if !strings.Contains(got, "\x1b[0m"+canvas) {
		t.Fatalf("canvas not restored after inline code: %q", got)
	}

	// An executable line re-emits its own surface after each chroma reset;
	// that surface wins, while the row's final reset falls back to canvas.
	exec := backgroundEscape(theme.Dark.ExecPanel)
	code := exec + "code" + "\x1b[0m" + exec + "more\x1b[0m"
	got = paintLine(code, 40, canvas)
	if !strings.Contains(got, "\x1b[0m"+exec) {
		t.Fatalf("exec surface not preserved after chroma reset: %q", got)
	}
	if !strings.Contains(got, "more\x1b[0m"+canvas) {
		t.Fatalf("canvas not restored at the end of an exec row: %q", got)
	}
}

func TestPaintCanvasNottyIsPlain(t *testing.T) {
	view := "one\ntwo\n"
	if got := PaintCanvas(view, 80, theme.Notty); got != view {
		t.Fatalf("notty view changed: %q", got)
	}
}

// TestZeroChannelColorsAreNotResets checks that a token color with a zero
// channel (for example #7b5d00) is not mistaken for an ANSI reset, which
// would paint the canvas background behind that token.
func TestZeroChannelColorsAreNotResets(t *testing.T) {
	th, err := theme.Resolve("solarized-light", true)
	if err != nil {
		t.Fatal(err)
	}
	out := Render(loadDemo(t), 100, th)
	view := PaintCanvas(strings.Join(out.Lines, "\n"), 100, th)
	canvas := backgroundEscape(th.Background)

	var code string
	for _, line := range strings.Split(view, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ansi.Strip(line)), "cd ") {
			code = line
			break
		}
	}
	if code == "" {
		t.Fatal("no code line found")
	}

	syntax := theme.AdjustSyntax(th)
	for name, c := range map[string]theme.Color{
		"Type": syntax.Type, "Keyword": syntax.Keyword, "Function": syntax.Function,
	} {
		if c == "" || !strings.Contains(string(c), "00") {
			continue
		}
		fg := "\x1b[" + termenv.TrueColor.Color(string(c)).Sequence(false) + "m"
		if strings.Contains(code, fg+canvas) {
			t.Errorf("%s color %q was treated as a reset and painted with the canvas", name, c)
		}
	}
}

func TestBackgroundEscape(t *testing.T) {
	if got := backgroundEscape("#0e0e11"); got != "\x1b[48;2;14;14;17m" {
		t.Fatalf("escape = %q", got)
	}
	if got := backgroundEscape(""); got != "" {
		t.Fatalf("empty color should yield no escape, got %q", got)
	}
}

// TestThemeContrast checks that text and chrome colors have enough contrast
// against the canvas to be readable regardless of the terminal's palette.
func TestThemeContrast(t *testing.T) {
	bg := luminance(t, theme.Dark.Background)
	for name, c := range map[string]theme.Color{
		"Text":      theme.Dark.Text,
		"Bright":    theme.Dark.Bright,
		"Soft":      theme.Dark.Soft,
		"TextMuted": theme.Dark.TextMuted,
		"Accent":    theme.Dark.Accent,
	} {
		ratio := contrast(bg, luminance(t, c))
		if ratio < 4.5 {
			t.Errorf("contrast of %s against canvas = %.2f, want >= 4.5", name, ratio)
		}
	}
	for name, c := range map[string]theme.Color{
		"CodePanel": theme.Dark.CodePanel,
		"ExecPanel": theme.Dark.ExecPanel,
	} {
		if luminance(t, c) == bg {
			t.Errorf("%s is indistinguishable from the canvas", name)
		}
	}
}

func luminance(t *testing.T, c theme.Color) float64 {
	t.Helper()
	v := strings.TrimPrefix(string(c), "#")
	if len(v) != 6 {
		t.Fatalf("color %q is not a six-digit hex value", c)
	}
	ch := make([]float64, 3)
	for i := range ch {
		n, err := strconv.ParseUint(v[i*2:i*2+2], 16, 8)
		if err != nil {
			t.Fatalf("color %q: %v", c, err)
		}
		s := float64(n) / 255
		if s <= 0.03928 {
			ch[i] = s / 12.92
		} else {
			ch[i] = math.Pow((s+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*ch[0] + 0.7152*ch[1] + 0.0722*ch[2]
}

func contrast(a, b float64) float64 {
	if a < b {
		a, b = b, a
	}
	return (a + 0.05) / (b + 0.05)
}
