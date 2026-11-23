package theme

import "testing"

func TestResolve(t *testing.T) {
	tests := []struct {
		name           string
		colorSupported bool
		want           string
		wantErr        bool
	}{
		{"", true, "dark", false},
		{"", false, "notty", false},
		{"auto", true, "dark", false},
		{"auto", false, "notty", false},
		{"dark", true, "dark", false},
		{"dark", false, "dark", false},
		{"notty", true, "notty", false},
		{"notty", false, "notty", false},
		{"DARK", true, "dark", false},
		{"Notty", false, "notty", false},
		{"midnight", true, "", true},
		{"--help", true, "", true},
	}
	for _, tt := range tests {
		got, err := Resolve(tt.name, tt.colorSupported)
		if tt.wantErr {
			if err == nil {
				t.Errorf("Resolve(%q, %v): want error, got %q", tt.name, tt.colorSupported, got.Name)
			}
			continue
		}
		if err != nil {
			t.Errorf("Resolve(%q, %v): %v", tt.name, tt.colorSupported, err)
			continue
		}
		if got.Name != tt.want {
			t.Errorf("Resolve(%q, %v) = %q, want %q", tt.name, tt.colorSupported, got.Name, tt.want)
		}
	}
}

func TestDarkTokensComplete(t *testing.T) {
	// Every role must carry a color: an empty token would fall back to the
	// terminal's own palette and reintroduce the inconsistency this theme
	// is meant to remove.
	for name, c := range map[string]Color{
		"Background":        Dark.Background,
		"BackgroundPanel":   Dark.BackgroundPanel,
		"BackgroundElement": Dark.BackgroundElement,
		"Border":            Dark.Border,
		"Separator":         Dark.Separator,
		"Text":              Dark.Text,
		"TextMuted":         Dark.TextMuted,
		"Bright":            Dark.Bright,
		"Soft":              Dark.Soft,
		"Accent":            Dark.Accent,
		"Rule":              Dark.Rule,
		"CodePanel":         Dark.CodePanel,
		"ExecPanel":         Dark.ExecPanel,
		"Primary":           Dark.Primary,
		"Success":           Dark.Success,
		"Error":             Dark.Error,
		"Warning":           Dark.Warning,
		"Running":           Dark.Running,
		"SuccessBright":     Dark.SuccessBright,
		"ErrorBright":       Dark.ErrorBright,
		"RunningBright":     Dark.RunningBright,
	} {
		if c == "" {
			t.Errorf("Dark.%s is empty", name)
		}
	}
}

func TestResolveAllThemes(t *testing.T) {
	for _, name := range Names() {
		for _, supported := range []bool{true, false} {
			got, err := Resolve(name, supported)
			if err != nil {
				t.Fatalf("Resolve(%q, %v): %v", name, supported, err)
			}
			if got.Name != name {
				t.Errorf("Resolve(%q, %v) = %q", name, supported, got.Name)
			}
		}
	}
	for alias, want := range map[string]string{
		"catppuccin": "catppuccin-mocha",
		"atom":       "one-dark",
		"atom-light": "one-light",
		"tokyonight": "tokyo-night",
		"rosepine":   "rose-pine",
		"CATPPUCCIN": "catppuccin-mocha",
		"One-Dark":   "one-dark",
	} {
		got, err := Resolve(alias, true)
		if err != nil {
			t.Errorf("Resolve(%q): %v", alias, err)
			continue
		}
		if got.Name != want {
			t.Errorf("Resolve(%q) = %q, want %q", alias, got.Name, want)
		}
	}
}

func TestThemesComplete(t *testing.T) {
	for _, name := range Names() {
		if name == "notty" {
			continue
		}
		th, err := Resolve(name, true)
		if err != nil {
			t.Fatalf("resolve %s: %v", name, err)
		}
		tokens := map[string]Color{
			"Background": th.Background, "BackgroundPanel": th.BackgroundPanel,
			"BackgroundElement": th.BackgroundElement, "Border": th.Border,
			"Separator": th.Separator, "Text": th.Text, "TextMuted": th.TextMuted,
			"Bright": th.Bright, "Soft": th.Soft, "Accent": th.Accent,
			"Rule": th.Rule, "CodePanel": th.CodePanel, "ExecPanel": th.ExecPanel,
			"Primary": th.Primary, "Success": th.Success, "Error": th.Error,
			"Warning": th.Warning, "Running": th.Running,
			"SuccessBright": th.SuccessBright, "ErrorBright": th.ErrorBright,
			"RunningBright": th.RunningBright,
		}
		// dark derives its syntax colors from the general roles.
		if th.Syntax != (Syntax{}) {
			for role, c := range map[string]Color{
				"Syntax.Comment": th.Syntax.Comment, "Syntax.Keyword": th.Syntax.Keyword,
				"Syntax.String": th.Syntax.String, "Syntax.Function": th.Syntax.Function,
				"Syntax.Number": th.Syntax.Number, "Syntax.Type": th.Syntax.Type,
				"Syntax.Operator": th.Syntax.Operator, "Syntax.Punctuation": th.Syntax.Punctuation,
				"Syntax.Constant": th.Syntax.Constant,
			} {
				tokens[role] = c
			}
		}
		for role, c := range tokens {
			if c == "" {
				t.Errorf("%s.%s is empty", name, role)
			}
		}
	}
}

func TestNottyTokensEmpty(t *testing.T) {
	// The notty fallback must leave every token empty so styles render as
	// plain text.
	for _, th := range []Theme{Notty} {
		for name, c := range map[string]Color{
			"Background":         th.Background,
			"BackgroundPanel":    th.BackgroundPanel,
			"BackgroundElement":  th.BackgroundElement,
			"Border":             th.Border,
			"Separator":          th.Separator,
			"Text":               th.Text,
			"TextMuted":          th.TextMuted,
			"Bright":             th.Bright,
			"Soft":               th.Soft,
			"Accent":             th.Accent,
			"Rule":               th.Rule,
			"CodePanel":          th.CodePanel,
			"ExecPanel":          th.ExecPanel,
			"Primary":            th.Primary,
			"Success":            th.Success,
			"Error":              th.Error,
			"Warning":            th.Warning,
			"Running":            th.Running,
			"SuccessBright":      th.SuccessBright,
			"ErrorBright":        th.ErrorBright,
			"RunningBright":      th.RunningBright,
			"Syntax.Comment":     th.Syntax.Comment,
			"Syntax.Keyword":     th.Syntax.Keyword,
			"Syntax.String":      th.Syntax.String,
			"Syntax.Function":    th.Syntax.Function,
			"Syntax.Number":      th.Syntax.Number,
			"Syntax.Type":        th.Syntax.Type,
			"Syntax.Operator":    th.Syntax.Operator,
			"Syntax.Punctuation": th.Syntax.Punctuation,
			"Syntax.Constant":    th.Syntax.Constant,
		} {
			if c != "" {
				t.Errorf("Notty.%s = %q, want empty", name, c)
			}
		}
	}
}
