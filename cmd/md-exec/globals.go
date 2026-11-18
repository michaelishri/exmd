package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

type kv struct{ key, val string }

type setList []kv

func (s *setList) String() string {
	var sb strings.Builder
	for i, e := range *s {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "%s=%s", e.key, e.val)
	}
	return sb.String()
}

func (s *setList) Set(v string) error {
	key, val, found := strings.Cut(v, "=")
	if !found || key == "" {
		return fmt.Errorf("--set expects KEY=VALUE, got %q", v)
	}
	*s = append(*s, kv{key, val})
	return nil
}

// parseSetArgs accepts flags in any position relative to the document:
//
//	md-exec globals DOC --set KEY=VALUE
//	md-exec globals --set KEY=VALUE DOC --set KEY2=VALUE2
func parseSetArgs(args []string) (sets setList, positional []string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--set" || a == "-set":
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("--set requires a KEY=VALUE value")
			}
			i++
			if err := sets.Set(args[i]); err != nil {
				return nil, nil, err
			}
		case strings.HasPrefix(a, "--set="):
			if err := sets.Set(strings.TrimPrefix(a, "--set=")); err != nil {
				return nil, nil, err
			}
		case strings.HasPrefix(a, "-set="):
			if err := sets.Set(strings.TrimPrefix(a, "-set=")); err != nil {
				return nil, nil, err
			}
		case a == "-h" || a == "--help":
			return nil, nil, fmt.Errorf("%s", "help")
		default:
			positional = append(positional, a)
		}
	}
	return sets, positional, nil
}

// cmdGlobals shows the document's parameter surface (SPEC.md §2.2): declared
// globals, their defaults, launch-time overrides, and what starts present
// versus absent.
func cmdGlobals(args []string) int {
	sets, positional, err := parseSetArgs(args)
	if err != nil {
		if err.Error() == "help" {
			fmt.Println("usage: md-exec globals DOC [--set KEY=VALUE]...")
			return 0
		}
		fmt.Fprintf(os.Stderr, "md-exec globals: %v\n", err)
		return 2
	}
	if len(positional) != 1 {
		fmt.Fprintln(os.Stderr, "usage: md-exec globals DOC [--set KEY=VALUE]...")
		return 2
	}

	res, code := loadDoc(positional[0])
	if res == nil {
		return code
	}

	overrides := map[string]string{}
	for _, s := range sets {
		overrides[s.key] = s.val
	}
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

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSOURCE\tVALUE")
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
		fmt.Fprintf(w, "%s\t%s\t%s\n", name, src, val)
	}
	w.Flush()

	fmt.Println(styleDim.Render(fmt.Sprintf("%d declared global(s); absent ones start unset until overridden or exported by a block (SPEC.md §2.2)", len(names))))
	return 0
}
