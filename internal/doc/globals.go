package doc

import (
	"fmt"
	"sort"
)

// ResolveGlobals computes the initial global state (SPEC.md §2.2):
// frontmatter defaults, overridden by launch-time overrides. Overrides for
// undeclared names are usage errors and are reported as diagnostics.
func (d *Document) ResolveGlobals(overrides map[string]string) (map[string]string, []Diagnostic) {
	out := make(map[string]string, len(d.Globals))
	for name, def := range d.Globals {
		if ov, ok := overrides[name]; ok {
			out[name] = ov
			continue
		}
		if def != nil {
			out[name] = *def
		}
	}

	var undeclared []string
	for name := range overrides {
		if _, declared := d.Globals[name]; !declared {
			undeclared = append(undeclared, name)
		}
	}
	sort.Strings(undeclared)
	var diags []Diagnostic
	for _, name := range undeclared {
		diags = append(diags, Diagnostic{
			Severity: SeverityError,
			Message:  fmt.Sprintf("unknown global `%s` (not declared in frontmatter `globals`)", name),
		})
	}
	return out, diags
}
