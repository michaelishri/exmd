package doc

import "testing"

func TestParseFenceInfo(t *testing.T) {
	tests := []struct {
		info    string
		lang    string
		matched bool
		nAttrs  int
	}{
		{"bash:exec", "bash", true, 0},
		{"python3:exec", "python3", true, 0},
		{"python:exec", "python", true, 0},
		{"bash:exec ", "bash", true, 0},
		{" bash:exec", "bash", true, 0},
		{"bash:exec name=deploy", "bash", true, 1},
		{"bash:exec a=1 b=2", "bash", true, 2},
		{"node-js:exec", "node-js", true, 0},
		{"bash", "", false, 0},
		{"exec", "", false, 0},
		{":exec", "", false, 0},
		{"bash:execute", "", false, 0},
		{"bash: exec", "", false, 0},
		{"bash :", "", false, 0},
		{"9lang:exec", "", false, 0},
		{"", "", false, 0},
	}
	for _, tt := range tests {
		lang, attrs, matched := parseFenceInfo(tt.info)
		if matched != tt.matched || lang != tt.lang || len(attrs) != tt.nAttrs {
			t.Errorf("parseFenceInfo(%q) = (%q, %d attrs, %v), want (%q, %d attrs, %v)",
				tt.info, lang, len(attrs), matched, tt.lang, tt.nAttrs, tt.matched)
		}
	}
}
