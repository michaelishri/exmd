package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

type kv struct{ key, val string }

type setList []kv

func (s *setList) Set(v string) error {
	key, val, found := strings.Cut(v, "=")
	if !found || key == "" {
		return fmt.Errorf("--set expects KEY=VALUE, got %q", v)
	}
	*s = append(*s, kv{key, val})
	return nil
}

// toMap turns parsed overrides into the map the document API expects.
func (s setList) toMap() map[string]string {
	out := make(map[string]string, len(s))
	for _, e := range s {
		out[e.key] = e.val
	}
	return out
}

// parseSetArgs accepts --set in any position relative to the document:
//
//	exmd globals DOC --set KEY=VALUE
//	exmd globals --set KEY=VALUE DOC --set KEY2=VALUE2
func parseSetArgs(args []string) (setList, []string, error) {
	args, help := extractHelp(args)
	if help {
		return nil, nil, errHelp
	}
	values, positional, err := parseFlag(args, "set", "set")
	if err != nil {
		return nil, nil, err
	}
	var sets setList
	for _, v := range values {
		if err := sets.Set(v); err != nil {
			return nil, nil, err
		}
	}
	return sets, positional, nil
}

// cmdGlobals shows the document's parameter surface (SPEC.md §2.2): declared
// globals, their defaults, launch-time overrides, and what starts present
// versus absent.
func cmdGlobals(args []string) int {
	sets, positional, err := parseSetArgs(args)
	if errors.Is(err, errHelp) {
		fmt.Println("usage: exmd globals DOC [--set KEY=VALUE]...")
		return 0
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "exmd globals: %v\n", err)
		return 2
	}
	if len(positional) != 1 {
		fmt.Fprintln(os.Stderr, "usage: exmd globals DOC [--set KEY=VALUE]...")
		return 2
	}

	res, code := loadDoc(positional[0])
	if res == nil {
		return code
	}

	overrides := sets.toMap()
	_, diags := res.Doc.ResolveGlobals(overrides)
	res.Diagnostics = append(res.Diagnostics, diags...)
	res.Sort()
	printDiagnostics(res)
	if res.HasErrors() {
		return 2
	}

	names := make([]string, 0, len(res.Doc.Globals))
	for name := range res.Doc.Globals {
		names = append(names, name)
	}
	sort.Strings(names)

	rows := make([][]string, 0, len(names))
	for _, name := range names {
		src, val := "absent", "—"
		if ov, ok := overrides[name]; ok {
			src, val = "override", ov
		} else if def := res.Doc.Globals[name]; def != nil {
			src, val = "default", *def
		}
		if val == "—" {
			val = styleDim.Render("—")
		}
		rows = append(rows, []string{name, src, val})
	}
	printTable([]string{"NAME", "SOURCE", "VALUE"}, rows)

	printThemeLine(styleDim.Render(fmt.Sprintf("%d declared global(s); absent ones start unset until overridden or exported by a block (SPEC.md §2.2)", len(names))))
	return 0
}
