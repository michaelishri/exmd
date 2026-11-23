package theme

import (
	"fmt"
	"sort"
	"strings"
)

// The palettes below are adapted from their upstream projects. Where a
// palette's muted foreground was too dim for the 4.5:1 contrast rule, it has
// been lifted slightly; the identity colors are unchanged.

var (
	// Catppuccin Mocha.
	CatppuccinMocha = Theme{
		Name:              "catppuccin-mocha",
		Background:        "#1e1e2e",
		BackgroundPanel:   "#181825",
		BackgroundElement: "#313244",
		Border:            "#313244",
		Separator:         "#45475a",
		Text:              "#cdd6f4",
		TextMuted:         "#a6adc8",
		Bright:            "#f5e0dc",
		Soft:              "#bac2de",
		Accent:            "#cba6f7",
		Rule:              "#45475a",
		CodePanel:         "#181825",
		ExecPanel:         "#313244",
		Primary:           "#cba6f7",
		Success:           "#a6e3a1",
		Error:             "#f38ba8",
		Warning:           "#f9e2af",
		Running:           "#fab387",
		SuccessBright:     "#c3f4bd",
		ErrorBright:       "#f7b4c7",
		RunningBright:     "#ffd0ad",
		Syntax: Syntax{
			Comment: "#6c7086", Keyword: "#cba6f7", String: "#a6e3a1",
			Function: "#89b4fa", Number: "#fab387", Type: "#f9e2af",
			Operator: "#94e2d5", Punctuation: "#9399b2", Constant: "#f5e0dc",
		},
	}

	// Catppuccin Latte.
	CatppuccinLatte = Theme{
		Name:              "catppuccin-latte",
		Background:        "#eff1f5",
		BackgroundPanel:   "#e6e9ef",
		BackgroundElement: "#dce0e8",
		Border:            "#dce0e8",
		Separator:         "#b8bcc8",
		Text:              "#4c4f69",
		TextMuted:         "#5c5f77",
		Bright:            "#36394f",
		Soft:              "#6c6f85",
		Accent:            "#1e66f5",
		Rule:              "#dce0e8",
		CodePanel:         "#e6e9ef",
		ExecPanel:         "#dce0e8",
		Primary:           "#8839ef",
		Success:           "#3a9029",
		Error:             "#d20f39",
		Warning:           "#df8e1d",
		Running:           "#fe640b",
		SuccessBright:     "#2a701c",
		ErrorBright:       "#aa0b2e",
		RunningBright:     "#c74f08",
		Syntax: Syntax{
			Comment: "#6c6f85", Keyword: "#8839ef", String: "#40a02b",
			Function: "#1e66f5", Number: "#fe640b", Type: "#df8e1d",
			Operator: "#04a5e5", Punctuation: "#4c4f69", Constant: "#d20f39",
		},
	}

	// Atom One Dark.
	OneDark = Theme{
		Name:              "one-dark",
		Background:        "#282c34",
		BackgroundPanel:   "#21252b",
		BackgroundElement: "#3f4451",
		Border:            "#3f4451",
		Separator:         "#5c6370",
		Text:              "#abb2bf",
		TextMuted:         "#98a1af",
		Bright:            "#ffffff",
		Soft:              "#c8ccd4",
		Accent:            "#61afef",
		Rule:              "#3f4451",
		CodePanel:         "#21252b",
		ExecPanel:         "#31363f",
		Primary:           "#61afef",
		Success:           "#98c379",
		Error:             "#e06c75",
		Warning:           "#e5c07b",
		Running:           "#d19a66",
		SuccessBright:     "#bce3a1",
		ErrorBright:       "#ef9298",
		RunningBright:     "#e8b27e",
		Syntax: Syntax{
			Comment: "#8b929e", Keyword: "#c678dd", String: "#98c379",
			Function: "#61afef", Number: "#d19a66", Type: "#e5c07b",
			Operator: "#56b6c2", Punctuation: "#abb2bf", Constant: "#e5c07b",
		},
	}

	// Atom One Light.
	OneLight = Theme{
		Name:              "one-light",
		Background:        "#ffffff",
		BackgroundPanel:   "#fafafa",
		BackgroundElement: "#f0f0f1",
		Border:            "#e5e5e6",
		Separator:         "#d0d0d2",
		Text:              "#383a42",
		TextMuted:         "#6a6d78",
		Bright:            "#24252b",
		Soft:              "#494c56",
		Accent:            "#4078f2",
		Rule:              "#e5e5e6",
		CodePanel:         "#f5f5f6",
		ExecPanel:         "#f0f0f1",
		Primary:           "#a626a4",
		Success:           "#50a14f",
		Error:             "#e45649",
		Warning:           "#c18401",
		Running:           "#986801",
		SuccessBright:     "#3a7d39",
		ErrorBright:       "#b83d32",
		RunningBright:     "#7d5401",
		Syntax: Syntax{
			Comment: "#777a83", Keyword: "#a626a4", String: "#50a14f",
			Function: "#4078f2", Number: "#986801", Type: "#c18401",
			Operator: "#0184bc", Punctuation: "#383a42", Constant: "#986801",
		},
	}

	// Gruvbox Dark.
	GruvboxDark = Theme{
		Name:              "gruvbox-dark",
		Background:        "#282828",
		BackgroundPanel:   "#3c3836",
		BackgroundElement: "#504945",
		Border:            "#504945",
		Separator:         "#665c54",
		Text:              "#ebdbb2",
		TextMuted:         "#a89984",
		Bright:            "#fbf1c7",
		Soft:              "#d5c4a1",
		Accent:            "#83a598",
		Rule:              "#504945",
		CodePanel:         "#3c3836",
		ExecPanel:         "#44403c",
		Primary:           "#83a598",
		Success:           "#b8bb26",
		Error:             "#fb4934",
		Warning:           "#fabd2f",
		Running:           "#fe8019",
		SuccessBright:     "#d2d76a",
		ErrorBright:       "#ff927d",
		RunningBright:     "#ffb27a",
		Syntax: Syntax{
			Comment: "#928374", Keyword: "#fb4934", String: "#b8bb26",
			Function: "#83a598", Number: "#d3869b", Type: "#fabd2f",
			Operator: "#fe8019", Punctuation: "#ebdbb2", Constant: "#d3869b",
		},
	}

	// Gruvbox Light.
	GruvboxLight = Theme{
		Name:              "gruvbox-light",
		Background:        "#fbf1c7",
		BackgroundPanel:   "#f2e5bc",
		BackgroundElement: "#ebdbb2",
		Border:            "#d5c4a1",
		Separator:         "#bdae93",
		Text:              "#3c3836",
		TextMuted:         "#69605a",
		Bright:            "#282828",
		Soft:              "#504945",
		Accent:            "#076678",
		Rule:              "#d5c4a1",
		CodePanel:         "#f2e5bc",
		ExecPanel:         "#ebdbb2",
		Primary:           "#af3a03",
		Success:           "#79740e",
		Error:             "#9d0006",
		Warning:           "#af3a03",
		Running:           "#b57614",
		SuccessBright:     "#5a5a0a",
		ErrorBright:       "#800510",
		RunningBright:     "#8f5d11",
		Syntax: Syntax{
			Comment: "#7c6f64", Keyword: "#9d0006", String: "#79740e",
			Function: "#076678", Number: "#8f3f71", Type: "#b57614",
			Operator: "#af3a03", Punctuation: "#3c3836", Constant: "#8f3f71",
		},
	}

	// Dracula.
	Dracula = Theme{
		Name:              "dracula",
		Background:        "#282a36",
		BackgroundPanel:   "#21222c",
		BackgroundElement: "#343746",
		Border:            "#343746",
		Separator:         "#6272a4",
		Text:              "#f8f8f2",
		TextMuted:         "#a9a7bd",
		Bright:            "#ffffff",
		Soft:              "#e6e6f0",
		Accent:            "#bd93f9",
		Rule:              "#44475a",
		CodePanel:         "#21222c",
		ExecPanel:         "#343746",
		Primary:           "#bd93f9",
		Success:           "#50fa7b",
		Error:             "#ff5555",
		Warning:           "#f1fa8c",
		Running:           "#ffb86c",
		SuccessBright:     "#9dffb5",
		ErrorBright:       "#ff8a8a",
		RunningBright:     "#ffd29a",
		Syntax: Syntax{
			Comment: "#8292c4", Keyword: "#ff79c6", String: "#f1fa8c",
			Function: "#50fa7b", Number: "#bd93f9", Type: "#8be9fd",
			Operator: "#ff79c6", Punctuation: "#f8f8f2", Constant: "#bd93f9",
		},
	}

	// Tokyo Night.
	TokyoNight = Theme{
		Name:              "tokyo-night",
		Background:        "#1a1b26",
		BackgroundPanel:   "#24283b",
		BackgroundElement: "#292e42",
		Border:            "#292e42",
		Separator:         "#3b4261",
		Text:              "#c0caf5",
		TextMuted:         "#a0a9c8",
		Bright:            "#e2e8ff",
		Soft:              "#a9b4d8",
		Accent:            "#7aa2f7",
		Rule:              "#292e42",
		CodePanel:         "#1f2335",
		ExecPanel:         "#24283b",
		Primary:           "#7aa2f7",
		Success:           "#9ece6a",
		Error:             "#f7768e",
		Warning:           "#e0af68",
		Running:           "#ff9e64",
		SuccessBright:     "#c2e89a",
		ErrorBright:       "#f9a0b3",
		RunningBright:     "#ffc28c",
		Syntax: Syntax{
			Comment: "#7a82a8", Keyword: "#bb9af7", String: "#9ece6a",
			Function: "#7aa2f7", Number: "#ff9e64", Type: "#2ac3de",
			Operator: "#89ddff", Punctuation: "#c0caf5", Constant: "#ff9e64",
		},
	}

	// GitHub Dark.
	GitHubDark = Theme{
		Name:              "github-dark",
		Background:        "#0d1117",
		BackgroundPanel:   "#161b22",
		BackgroundElement: "#21262d",
		Border:            "#21262d",
		Separator:         "#30363d",
		Text:              "#e6edf3",
		TextMuted:         "#8b949e",
		Bright:            "#ffffff",
		Soft:              "#c9d1d9",
		Accent:            "#58a6ff",
		Rule:              "#30363d",
		CodePanel:         "#161b22",
		ExecPanel:         "#1c2128",
		Primary:           "#58a6ff",
		Success:           "#56d364",
		Error:             "#f85149",
		Warning:           "#d29922",
		Running:           "#e3b341",
		SuccessBright:     "#7ee787",
		ErrorBright:       "#ff7b72",
		RunningBright:     "#ffcb6b",
		Syntax: Syntax{
			Comment: "#8b949e", Keyword: "#ff7b72", String: "#a5d6ff",
			Function: "#d2a8ff", Number: "#79c0ff", Type: "#ffa657",
			Operator: "#ff7b72", Punctuation: "#c9d1d9", Constant: "#79c0ff",
		},
	}

	// GitHub Light.
	GitHubLight = Theme{
		Name:              "github-light",
		Background:        "#ffffff",
		BackgroundPanel:   "#f6f8fa",
		BackgroundElement: "#eaeef2",
		Border:            "#d0d7de",
		Separator:         "#c8d1da",
		Text:              "#1f2328",
		TextMuted:         "#656d76",
		Bright:            "#0d1117",
		Soft:              "#424a53",
		Accent:            "#0969da",
		Rule:              "#d0d7de",
		CodePanel:         "#f6f8fa",
		ExecPanel:         "#eaeef2",
		Primary:           "#8250df",
		Success:           "#1a7f37",
		Error:             "#cf222e",
		Warning:           "#9a6700",
		Running:           "#bc4c00",
		SuccessBright:     "#14632a",
		ErrorBright:       "#a3121d",
		RunningBright:     "#943c00",
		Syntax: Syntax{
			Comment: "#6e7781", Keyword: "#cf222e", String: "#0a3069",
			Function: "#8250df", Number: "#0550ae", Type: "#953800",
			Operator: "#cf222e", Punctuation: "#1f2328", Constant: "#0550ae",
		},
	}

	// Solarized Dark.
	SolarizedDark = Theme{
		Name:              "solarized-dark",
		Background:        "#002b36",
		BackgroundPanel:   "#073642",
		BackgroundElement: "#084152",
		Border:            "#073642",
		Separator:         "#586e75",
		Text:              "#eee8d5",
		TextMuted:         "#8a9fa4",
		Bright:            "#fdf6e3",
		Soft:              "#93a1a1",
		Accent:            "#6c99d3",
		Rule:              "#073642",
		CodePanel:         "#073642",
		ExecPanel:         "#0b3c4a",
		Primary:           "#6c99d3",
		Success:           "#99a900",
		Error:             "#dc322f",
		Warning:           "#b58900",
		Running:           "#cb4b16",
		SuccessBright:     "#bdc94a",
		ErrorBright:       "#ee6a66",
		RunningBright:     "#e87b4a",
		Syntax: Syntax{
			Comment: "#7f9a9f", Keyword: "#859900", String: "#2aa198",
			Function: "#268bd2", Number: "#d33682", Type: "#b58900",
			Operator: "#93a1a1", Punctuation: "#93a1a1", Constant: "#6c71c4",
		},
	}

	// Solarized Light.
	SolarizedLight = Theme{
		Name:              "solarized-light",
		Background:        "#fdf6e3",
		BackgroundPanel:   "#eee8d5",
		BackgroundElement: "#e5ddc8",
		Border:            "#d3c6aa",
		Separator:         "#c3b59a",
		Text:              "#2f4b54",
		TextMuted:         "#657b83",
		Bright:            "#073642",
		Soft:              "#586e75",
		Accent:            "#1a6da8",
		Rule:              "#d3c6aa",
		CodePanel:         "#eee8d5",
		ExecPanel:         "#e5ddc8",
		Primary:           "#1a6da8",
		Success:           "#5f7410",
		Error:             "#b32021",
		Warning:           "#8a6a00",
		Running:           "#a63c11",
		SuccessBright:     "#4d5f0d",
		ErrorBright:       "#90181a",
		RunningBright:     "#85300d",
		Syntax: Syntax{
			Comment: "#657b83", Keyword: "#859900", String: "#2aa198",
			Function: "#268bd2", Number: "#d33682", Type: "#b58900",
			Operator: "#657b83", Punctuation: "#586e75", Constant: "#6c71c4",
		},
	}

	// Nord.
	Nord = Theme{
		Name:              "nord",
		Background:        "#2e3440",
		BackgroundPanel:   "#3b4252",
		BackgroundElement: "#434c5e",
		Border:            "#434c5e",
		Separator:         "#4c566a",
		Text:              "#d8dee9",
		TextMuted:         "#9aa7b8",
		Bright:            "#eceff4",
		Soft:              "#e5e9f0",
		Accent:            "#88c0d0",
		Rule:              "#434c5e",
		CodePanel:         "#3b4252",
		ExecPanel:         "#434c5e",
		Primary:           "#88c0d0",
		Success:           "#a3be8c",
		Error:             "#bf616a",
		Warning:           "#ebcb8b",
		Running:           "#d08770",
		SuccessBright:     "#c5d8b3",
		ErrorBright:       "#df8a91",
		RunningBright:     "#e6a892",
		Syntax: Syntax{
			// Brightened in place to keep Nord's hues; desaturating toward
			// white made the small code text hard to distinguish.
			Comment: "#7b88a1", Keyword: "#81a1c1", String: "#a3be8c",
			Function: "#88c0d0", Number: "#b48ead", Type: "#8fbcbb",
			Operator: "#81a1c1", Punctuation: "#d8dee9", Constant: "#b48ead",
		},
	}

	// Nord Light.
	NordLight = Theme{
		Name:              "nord-light",
		Background:        "#eceff4",
		BackgroundPanel:   "#e5e9f0",
		BackgroundElement: "#d8dee9",
		Border:            "#d8dee9",
		Separator:         "#b8c0cc",
		Text:              "#2e3440",
		TextMuted:         "#616b7d",
		Bright:            "#1b1f27",
		Soft:              "#434c5e",
		Accent:            "#4c6f96",
		Rule:              "#d8dee9",
		CodePanel:         "#e5e9f0",
		ExecPanel:         "#d8dee9",
		Primary:           "#4c6f96",
		Success:           "#57745a",
		Error:             "#ab5360",
		Warning:           "#9a7a3e",
		Running:           "#a56d4f",
		SuccessBright:     "#456044",
		ErrorBright:       "#8d3f4a",
		RunningBright:     "#87583e",
		Syntax: Syntax{
			Comment: "#7b8799", Keyword: "#5e81ac", String: "#6d8f5b",
			Function: "#5e81ac", Number: "#8f5f80", Type: "#5d8380",
			Operator: "#5e81ac", Punctuation: "#2e3440", Constant: "#8f5f80",
		},
	}

	// Rose Pine.
	RosePine = Theme{
		Name:              "rose-pine",
		Background:        "#191724",
		BackgroundPanel:   "#1f1d2e",
		BackgroundElement: "#26233a",
		Border:            "#26233a",
		Separator:         "#403d52",
		Text:              "#e0def4",
		TextMuted:         "#a29dba",
		Bright:            "#f4f3f9",
		Soft:              "#c4c0da",
		Accent:            "#c4a7e7",
		Rule:              "#26233a",
		CodePanel:         "#1f1d2e",
		ExecPanel:         "#26233a",
		Primary:           "#c4a7e7",
		Success:           "#9ccfd8",
		Error:             "#eb6f92",
		Warning:           "#f6c177",
		Running:           "#ea9a97",
		SuccessBright:     "#b8e2e8",
		ErrorBright:       "#f294ad",
		RunningBright:     "#f5b5b2",
		Syntax: Syntax{
			Comment: "#7c7794", Keyword: "#c4a7e7", String: "#f6c177",
			Function: "#9ccfd8", Number: "#ebbcba", Type: "#ebbcba",
			Operator: "#9ccfd8", Punctuation: "#e0def4", Constant: "#f6c177",
		},
	}

	// Rose Pine Dawn.
	RosePineDawn = Theme{
		Name:              "rose-pine-dawn",
		Background:        "#faf4ed",
		BackgroundPanel:   "#fffaf3",
		BackgroundElement: "#f2e9e1",
		Border:            "#e6dfdb",
		Separator:         "#cec4bd",
		Text:              "#575279",
		TextMuted:         "#6f6a80",
		Bright:            "#3c3555",
		Soft:              "#6e6a86",
		Accent:            "#a04d66",
		Rule:              "#e6dfdb",
		CodePanel:         "#fffaf3",
		ExecPanel:         "#f2e9e1",
		Primary:           "#907aa9",
		Success:           "#286983",
		Error:             "#b85454",
		Warning:           "#a86c1b",
		Running:           "#b4637a",
		SuccessBright:     "#1f5368",
		ErrorBright:       "#993c3c",
		RunningBright:     "#94415a",
		Syntax: Syntax{
			Comment: "#797593", Keyword: "#907aa9", String: "#56949f",
			Function: "#286983", Number: "#d7827e", Type: "#a86c1b",
			Operator: "#56949f", Punctuation: "#575279", Constant: "#b4637a",
		},
	}

	// Everforest Light.
	EverforestLight = Theme{
		Name:              "everforest-light",
		Background:        "#fdf6e3",
		BackgroundPanel:   "#f2ecdc",
		BackgroundElement: "#e8e0c8",
		Border:            "#dcd3b6",
		Separator:         "#c8bda0",
		Text:              "#5c6a72",
		TextMuted:         "#66756e",
		Bright:            "#3d4b52",
		Soft:              "#708a7d",
		Accent:            "#3f7899",
		Rule:              "#dcd3b6",
		CodePanel:         "#f2ecdc",
		ExecPanel:         "#e8e0c8",
		Primary:           "#3f7899",
		Success:           "#5f7a2f",
		Error:             "#a84d4d",
		Warning:           "#b89038",
		Running:           "#c07a45",
		SuccessBright:     "#4d6527",
		ErrorBright:       "#8e3d3d",
		RunningBright:     "#9a6033",
		Syntax: Syntax{
			Comment: "#6b7d72", Keyword: "#a04d4d", String: "#5f7a2f",
			Function: "#3f7899", Number: "#b89038", Type: "#c07a45",
			Operator: "#6b7d72", Punctuation: "#5c6a72", Constant: "#b89038",
		},
	}

	// Ayu Dark, from the official ayu-theme/ayu-colors palette.
	AyuDark = Theme{
		Name:              "ayu-dark",
		Background:        "#0d1017",
		BackgroundPanel:   "#10141c",
		BackgroundElement: "#161a24",
		Border:            "#1b1f29",
		Separator:         "#2d3442",
		Text:              "#fcf8ec",
		TextMuted:         "#b9c2d9",
		Bright:            "#ffffff",
		Soft:              "#e6e5df",
		Accent:            "#e6b450",
		Rule:              "#2d3442",
		CodePanel:         "#10141c",
		ExecPanel:         "#161a24",
		Primary:           "#e6b450",
		Success:           "#aad94c",
		Error:             "#d95757",
		Warning:           "#ffb454",
		Running:           "#ff8f40",
		SuccessBright:     "#c4f06f",
		ErrorBright:       "#f08a8a",
		RunningBright:     "#ffb37a",
		Syntax: Syntax{
			Comment: "#5a6673", Keyword: "#ff8f40", String: "#aad94c",
			Function: "#ffb454", Number: "#d2a6ff", Type: "#59c2ff",
			Operator: "#f29668", Punctuation: "#fcf8ec", Constant: "#d2a6ff",
		},
	}

	// Ayu Light, from the official ayu-theme/ayu-colors palette.
	AyuLight = Theme{
		Name:              "ayu-light",
		Background:        "#fcfcfc",
		BackgroundPanel:   "#f8f9fa",
		BackgroundElement: "#ebeef0",
		Border:            "#d6dde4",
		Separator:         "#b9c2cb",
		Text:              "#020a12",
		TextMuted:         "#58687a",
		Bright:            "#021a3c",
		Soft:              "#414c5c",
		Accent:            "#b36b00",
		Rule:              "#d6dde4",
		CodePanel:         "#f8f9fa",
		ExecPanel:         "#ebeef0",
		Primary:           "#f29718",
		Success:           "#86b300",
		Error:             "#e65050",
		Warning:           "#eba400",
		Running:           "#fa8532",
		SuccessBright:     "#6b9000",
		ErrorBright:       "#c43c3c",
		RunningBright:     "#c4661f",
		Syntax: Syntax{
			Comment: "#adaeb1", Keyword: "#fa8532", String: "#86b300",
			Function: "#eba400", Number: "#a37acc", Type: "#22a4e6",
			Operator: "#f2a191", Punctuation: "#020a12", Constant: "#a37acc",
		},
	}
)

var registry = map[string]Theme{
	"dark":             Dark,
	"notty":            Notty,
	"catppuccin-mocha": CatppuccinMocha,
	"catppuccin-latte": CatppuccinLatte,
	"one-dark":         OneDark,
	"one-light":        OneLight,
	"gruvbox-dark":     GruvboxDark,
	"gruvbox-light":    GruvboxLight,
	"dracula":          Dracula,
	"tokyo-night":      TokyoNight,
	"github-dark":      GitHubDark,
	"github-light":     GitHubLight,
	"solarized-dark":   SolarizedDark,
	"solarized-light":  SolarizedLight,
	"nord":             Nord,
	"nord-light":       NordLight,
	"rose-pine":        RosePine,
	"rose-pine-dawn":   RosePineDawn,
	"everforest-light": EverforestLight,
	"ayu-dark":         AyuDark,
	"ayu-light":        AyuLight,
}

var aliases = map[string]string{
	"catppuccin": "catppuccin-mocha",
	"atom":       "one-dark",
	"atom-light": "one-light",
	"tokyonight": "tokyo-night",
	"rosepine":   "rose-pine",
}

// Names returns every canonical theme name, sorted. It drives both the CLI
// error message and the documentation tests.
func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Resolve maps a --theme flag value to a Theme. "auto" (the default, and the
// empty string) picks Dark when the terminal supports color and Notty
// otherwise. Names and aliases are case-insensitive.
func Resolve(name string, colorSupported bool) (Theme, error) {
	key := strings.ToLower(name)
	if key == "" || key == "auto" {
		if colorSupported {
			return Dark, nil
		}
		return Notty, nil
	}
	if canonical, ok := aliases[key]; ok {
		key = canonical
	}
	if th, ok := registry[key]; ok {
		return th, nil
	}
	return Theme{}, fmt.Errorf("unknown theme %q (valid: %s)",
		name, strings.Join(Names(), ", "))
}
