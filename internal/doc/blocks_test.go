package doc

import (
	"reflect"
	"testing"
)

func TestParseFenceInfo(t *testing.T) {
	tests := []struct {
		info      string
		lang      string
		matched   bool
		wantAttrs []BlockAttribute
	}{
		{"bash:exec", "bash", true, nil},
		{"python3:exec", "python3", true, nil},
		{"python:exec", "python", true, nil},
		{"bash:exec ", "bash", true, nil},
		{" bash:exec", "bash", true, nil},
		{"bash:exec name=deploy", "bash", true, []BlockAttribute{{Key: "name", Value: "deploy"}}},
		{"bash:exec a=1 b=2", "bash", true, []BlockAttribute{{Key: "a", Value: "1"}, {Key: "b", Value: "2"}}},
		{"bash:exec loose", "bash", true, []BlockAttribute{{Key: "loose", Malformed: true}}},
		{"node-js:exec", "node-js", true, nil},
		{"bash", "", false, nil},
		{"exec", "", false, nil},
		{":exec", "", false, nil},
		{"bash:execute", "", false, nil},
		{"bash: exec", "", false, nil},
		{"bash :", "", false, nil},
		{"9lang:exec", "", false, nil},
		{"", "", false, nil},
	}
	for _, tt := range tests {
		lang, attrs, matched := parseFenceInfo(tt.info)
		if matched != tt.matched || lang != tt.lang || !reflect.DeepEqual(attrs, tt.wantAttrs) {
			t.Errorf("parseFenceInfo(%q) = (%q, %v, %v), want (%q, %v, %v)",
				tt.info, lang, attrs, matched, tt.lang, tt.wantAttrs, tt.matched)
		}
	}
}
