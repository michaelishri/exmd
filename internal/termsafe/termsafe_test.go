package termsafe

import "testing"

func TestVisible(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain text", "plain text"},
		{"tab\tand\nnewline", "tab\tand\nnewline"},
		{"\x1b[31mred\x1b[0m", "^[[31mred^[[0m"},
		{"\x1b]52;c;aGk=\x07", "^[]52;c;aGk=^G"},
		{"cr\rbs\bdel\x7f", "cr^Mbs^Hdel^?"},
		{"c1 \u009b2J", "c1 <U+009B>2J"},
		{"bad \xff byte", "bad \uFFFD byte"},
		{"日本語 ✓", "日本語 ✓"},
	}
	for _, c := range cases {
		if got := Visible(c.in); got != c.want {
			t.Errorf("Visible(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStrip(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain text", "plain text"},
		{"tab\tand\nnewline", "tab\tand\nnewline"},
		{"\x1b[31mred\x1b[0m", "red"},
		{"\x1b]8;;http://evil\x07link\x1b]8;;\x07", "link"},
		{"bell\x07 c1\u009b", "bell c1"},
		{"bad \xff byte", "bad  byte"},
	}
	for _, c := range cases {
		if got := Strip(c.in); got != c.want {
			t.Errorf("Strip(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestNoControlSurvives checks every rune below U+00A0: nothing that can
// steer a terminal is left in either form.
func TestNoControlSurvives(t *testing.T) {
	for r := rune(0); r < 0xa0; r++ {
		s := string(r)
		for name, out := range map[string]string{"Visible": Visible(s), "Strip": Strip(s)} {
			for _, c := range out {
				if isControl(c) {
					t.Errorf("%s(%U) left control %U", name, r, c)
				}
			}
		}
	}
}
