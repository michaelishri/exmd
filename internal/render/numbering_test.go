package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/michaelishri/exmd/internal/doc"
)

func loadUnnumbered(t *testing.T) *doc.Document {
	t.Helper()
	res, err := doc.Load("testdata/unnumbered.md")
	if err != nil {
		t.Fatalf("load unnumbered fixture: %v", err)
	}
	if res.HasErrors() {
		t.Fatal("unnumbered fixture has errors")
	}
	return res.Doc
}

func TestHeadingNumbers(t *testing.T) {
	d := loadUnnumbered(t)
	nums := headingNumbers(d)
	want := map[string]string{
		"Numbered sections": "", // the title h1
		"Preflight":         "1.",
		"Checks":            "1.1.",
		"Deep":              "1.1.1.",
		"More checks":       "1.2.",
		"Build":             "2.",
		"Package":           "2.1.",
	}
	if len(nums) != len(d.Headings) {
		t.Fatalf("numbers = %d, want %d", len(nums), len(d.Headings))
	}
	for i, h := range d.Headings {
		expected, ok := want[h.Text]
		if !ok {
			t.Fatalf("unexpected heading %q", h.Text)
		}
		if nums[i] != expected {
			t.Errorf("heading %q = %q, want %q", h.Text, nums[i], expected)
		}
	}
}

func TestRenderedHeadingNumbers(t *testing.T) {
	d := loadUnnumbered(t)
	out := Render(d, 80, false)
	joined := strings.Join(out.Lines, "\n")
	for _, heading := range []string{
		"1. Preflight", "1.1. Checks", "1.1.1. Deep",
		"1.2. More checks", "2. Build", "2.1. Package",
	} {
		if !strings.Contains(joined, heading) {
			t.Fatalf("rendered output missing %q:\n%s", heading, joined)
		}
	}
	nums := headingNumbers(d)
	for i, mark := range out.Headings {
		if mark.Number != nums[i] {
			t.Errorf("mark %d number = %q, want %q", i, mark.Number, nums[i])
		}
	}
}

func TestDocumentSourceIsUnchanged(t *testing.T) {
	d := loadUnnumbered(t)
	before := string(d.Source)
	_ = Render(d, 80, false)
	if string(d.Source) != before {
		t.Fatal("rendering modified the document source")
	}
}

// TestDemoKeepsItsOwnNumbers checks the suppression rule against the real
// demo document, whose headings already carry 1., 5.1, etc.
func TestDemoKeepsItsOwnNumbers(t *testing.T) {
	d := loadDemo(t)
	for i, n := range headingNumbers(d) {
		if n != "" {
			t.Fatalf("demo heading %d got generated number %q", i, n)
		}
	}
	out := Render(d, 80, false)
	joined := strings.Join(out.Lines, "\n")
	if !strings.Contains(joined, "1. Preflight and point-of-use guards") {
		t.Fatal("demo's own heading number disappeared")
	}
	if strings.Contains(joined, "1. 1.") {
		t.Fatal("demo's numbered heading was numbered twice")
	}
}

func TestHeadingNumberDetection(t *testing.T) {
	for _, tt := range []struct {
		text string
		want bool
	}{
		{"1 Preflight", true},
		{"1. Preflight", true},
		{"5.1 From bash", true},
		{"1.2.3 Detail", true},
		{"2) Notes", true},
		{"Preflight", false},
		{"Chapter 1", false},
		{"v2 API", false},
	} {
		if got := headingHasNumber(tt.text); got != tt.want {
			t.Errorf("headingHasNumber(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestOneEmbeddedNumberSuppressesWholeDocument(t *testing.T) {
	d := &doc.Document{Headings: []*doc.Heading{
		{Level: 1, Text: "Title"},
		{Level: 2, Text: "Preflight"},
		{Level: 2, Text: "2) Notes"},
	}}
	for i, n := range headingNumbers(d) {
		if n != "" {
			t.Fatalf("heading %d = %q, want no generated numbers", i, n)
		}
	}
}

func TestOnlyFirstH1IsSkipped(t *testing.T) {
	d := &doc.Document{Headings: []*doc.Heading{
		{Level: 1, Text: "Title"},
		{Level: 2, Text: "One"},
		{Level: 1, Text: "Another"},
		{Level: 2, Text: "Sub"},
	}}
	got := headingNumbers(d)
	want := []string{"", "1.", "2.", "2.1."}
	if len(got) != len(want) {
		t.Fatalf("numbers = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("numbers = %v, want %v", got, want)
		}
	}
}

func TestPrefixHeadingNumber(t *testing.T) {
	for _, tt := range []struct{ in, number, want string }{
		{"## Preflight", "1.", "## 1. Preflight"},
		{"  ### Checks  ", "1.1.", "  ### 1.1. Checks  "},
		{"## Title ##", "2.", "## 2. Title ##"},
	} {
		if got := prefixHeadingNumber(tt.in, tt.number); got != tt.want {
			t.Errorf("prefixHeadingNumber(%q, %q) = %q, want %q",
				tt.in, tt.number, got, tt.want)
		}
	}
}

func TestRenderWidthWithNumbers(t *testing.T) {
	d := loadUnnumbered(t)
	out := Render(d, 40, false)
	for i, l := range out.Lines {
		if w := ansi.StringWidth(l); w > 40 {
			t.Fatalf("line %d width %d exceeds 40: %q", i, w, l)
		}
	}
}
