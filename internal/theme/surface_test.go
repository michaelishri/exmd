package theme

import (
	"strconv"
	"testing"
)

func TestLift(t *testing.T) {
	if got := Lift("#18181e", 0.12); got == "#18181e" {
		t.Fatal("Lift did not change the color")
	}
	if got := Lift("", 0.12); got != "" {
		t.Fatalf("Lift of an empty token = %q, want empty", got)
	}
	if got := Lift("not-a-color", 0.12); got != "not-a-color" {
		t.Fatalf("Lift of a malformed token = %q", got)
	}

	// Lifting toward white must increase every channel.
	got := string(Lift("#18181e", 0.12))
	for i, base := range []string{"18", "18", "1e"} {
		n, _ := strconv.ParseUint(got[1+i*2:3+i*2], 16, 8)
		want, _ := strconv.ParseUint(base, 16, 8)
		if n <= want {
			t.Errorf("channel %d = %02x, want > %s", i, n, base)
		}
	}
}

func TestLiftDarkensLightColors(t *testing.T) {
	got := string(Lift("#e6e9ef", 0.12))
	for i, base := range []string{"e6", "e9", "ef"} {
		n, _ := strconv.ParseUint(got[1+i*2:3+i*2], 16, 8)
		want, _ := strconv.ParseUint(base, 16, 8)
		if n >= want {
			t.Errorf("channel %d = %02x, want < %s", i, n, base)
		}
	}
}

func TestReadableFooterForeground(t *testing.T) {
	for _, name := range Names() {
		if name == "notty" {
			continue
		}
		th, err := Resolve(name, true)
		if err != nil {
			t.Fatalf("resolve %s: %v", name, err)
		}
		for label, surface := range map[string]Color{
			"left":     Lift(th.BackgroundElement, 0.12),
			"settings": Lift(th.BackgroundPanel, 0.10),
		} {
			fg := ReadableOn(surface, th.Bright, th.Text, th.Soft, th.TextMuted)
			if ratio := Contrast(surface, fg); ratio < 4.5 {
				t.Errorf("%s %s footer text %.2f:1 against %s", name, label, ratio, surface)
			}
		}
	}
}

func TestAdjustSyntaxContrast(t *testing.T) {
	for _, name := range Names() {
		if name == "notty" {
			continue
		}
		th, err := Resolve(name, true)
		if err != nil {
			t.Fatalf("resolve %s: %v", name, err)
		}
		s := AdjustSyntax(th)
		for role, c := range map[string]Color{
			"Comment": s.Comment, "Keyword": s.Keyword, "String": s.String,
			"Function": s.Function, "Number": s.Number, "Type": s.Type,
			"Operator": s.Operator, "Punctuation": s.Punctuation,
			"Constant": s.Constant,
		} {
			if c == "" {
				continue
			}
			if ratio := Contrast(th.ExecPanel, c); ratio < 4.5 {
				t.Errorf("%s %s on exec surface = %.2f, want >= 4.5", name, role, ratio)
			}
			if ratio := Contrast(th.CodePanel, c); ratio < 4.5 {
				t.Errorf("%s %s on code surface = %.2f, want >= 4.5", name, role, ratio)
			}
		}
	}
}

func TestFooterSurfaceIsAboveSidebar(t *testing.T) {
	for _, name := range Names() {
		if name == "notty" {
			continue
		}
		th, err := Resolve(name, true)
		if err != nil {
			t.Fatalf("resolve %s: %v", name, err)
		}
		footer := Lift(th.BackgroundElement, 0.12)
		if footer == th.BackgroundPanel || footer == th.BackgroundElement {
			t.Errorf("%s footer surface %q is not lifted above the sidebar", name, footer)
		}
	}
}
