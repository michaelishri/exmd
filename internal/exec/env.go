package exec

import (
	"bytes"
	"sort"
	"strings"
)

// shellManaged names are maintained by the interpreter itself (bash sets
// PWD/SHLVL/OLDPWD on startup and _ per external command). They appear in
// every block's environment delta but are infrastructure, not document
// state — adapters MUST suppress them (SPEC.md §5.1.6).
var shellManaged = map[string]struct{}{
	"_": {}, "OLDPWD": {}, "PWD": {}, "SHLVL": {},
}

// compose builds a block's process environment (SPEC.md §4.2):
//
//	block_env := (host_env − declared_names) ∪ current_globals
//
// Declared names are document-controlled for the whole run (§2.2 masking),
// so host entries of those names are dropped; globals are appended after.
func compose(hostEnv []string, declared map[string]struct{}, globals map[string]string) []string {
	out := make([]string, 0, len(hostEnv)+len(globals))
	seen := make(map[string]struct{}, len(hostEnv))
	for _, kv := range hostEnv {
		name, _, _ := strings.Cut(kv, "=")
		if _, isDeclared := declared[name]; isDeclared {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, kv)
	}
	names := make([]string, 0, len(globals))
	for name := range globals {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out = append(out, name+"="+globals[name])
	}
	return out
}

// parseEnv parses a NUL-delimited name=value dump into entries.
func parseEnv(data []byte) []string {
	var out []string
	for _, kv := range bytes.Split(data, []byte{0}) {
		if len(kv) == 0 {
			continue
		}
		out = append(out, string(kv))
	}
	return out
}

// envMap indexes an environment by name (last entry wins).
func envMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, kv := range env {
		name, val, _ := strings.Cut(kv, "=")
		m[name] = val
	}
	return m
}

// diffEnv compares the environment passed to a block (E_in) with the final
// environment the block would hand to its children (E_out, SPEC.md §4.3).
// set carries added and changed names; removed carries unsets.
func diffEnv(in, out []string) (set map[string]string, removed []string) {
	inMap := envMap(in)
	outMap := envMap(out)

	set = make(map[string]string)
	for name, val := range outMap {
		if old, ok := inMap[name]; !ok || old != val {
			set[name] = val
		}
	}
	for name := range inMap {
		if _, ok := outMap[name]; !ok {
			removed = append(removed, name)
		}
	}
	sort.Strings(removed)
	return set, removed
}
