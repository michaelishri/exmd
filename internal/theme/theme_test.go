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

func TestNottyTokensEmpty(t *testing.T) {
	// The notty fallback must leave every token empty so styles render as
	// plain text.
	for _, th := range []Theme{Notty} {
		for name, c := range map[string]Color{
			"Background":        th.Background,
			"BackgroundPanel":   th.BackgroundPanel,
			"BackgroundElement": th.BackgroundElement,
			"Border":            th.Border,
			"Text":              th.Text,
			"TextMuted":         th.TextMuted,
			"Primary":           th.Primary,
			"Success":           th.Success,
			"Error":             th.Error,
			"Warning":           th.Warning,
		} {
			if c != "" {
				t.Errorf("Notty.%s = %q, want empty", name, c)
			}
		}
	}
}
