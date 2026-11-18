package exec

import (
	"testing"
)

func TestComposeMasksDeclaredAndAppendsGlobals(t *testing.T) {
	host := []string{
		"PATH=/bin",
		"SHADOWED=from-host",
		"OTHER=kept",
		"OTHER=dup", // first occurrence wins
	}
	declared := map[string]struct{}{"SHADOWED": {}, "G1": {}}
	globals := map[string]string{"SHADOWED": "from-global", "G1": "v1"}

	got := compose(host, declared, globals)
	want := []string{"PATH=/bin", "OTHER=kept", "G1=v1", "SHADOWED=from-global"}
	if len(got) != len(want) {
		t.Fatalf("compose = %v, want %v", got, want)
	}
	seen := map[string]bool{}
	for _, kv := range got {
		seen[kv] = true
	}
	for _, kv := range want {
		if !seen[kv] {
			t.Fatalf("compose missing %q; got %v", kv, got)
		}
	}
	// Globals must come after host entries so they win.
	if got[len(got)-1] != "SHADOWED=from-global" {
		t.Fatalf("globals not appended last: %v", got)
	}
}

func TestDiffEnv(t *testing.T) {
	in := []string{"A=1", "B=2", "C=3", "__MDEXEC_BLOCK=/x"}
	out := []string{"A=1", "B=changed", "D=new", "__MDEXEC_BLOCK=/x"}
	set, removed := diffEnv(in, out)
	if set["B"] != "changed" || set["D"] != "new" || len(set) != 2 {
		t.Fatalf("set = %v", set)
	}
	if len(removed) != 1 || removed[0] != "C" {
		t.Fatalf("removed = %v", removed)
	}
}

func TestParseEnvLossless(t *testing.T) {
	data := []byte("A=plain\x00B=line1\nline2\x00C=\x00")
	got := parseEnv(data)
	if len(got) != 3 {
		t.Fatalf("parseEnv = %v", got)
	}
	if got[1] != "B=line1\nline2" {
		t.Fatalf("multiline value lost: %q", got[1])
	}
	if got[2] != "C=" {
		t.Fatalf("empty value lost: %q", got[2])
	}
}
