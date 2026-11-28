package render

import (
	"testing"

	gansi "charm.land/glamour/v2/ansi"
	"github.com/alecthomas/chroma/v2"
	chromastyles "github.com/alecthomas/chroma/v2/styles"

	"github.com/michaelishri/exmd/internal/theme"
)

// TestStyleUsesOnlyThemeColors guards against glamour's built-in palettes
// leaking back in: every color emitted by the content renderer must be one
// of the active theme's tokens.
func TestStyleUsesOnlyThemeColors(t *testing.T) {
	for _, name := range theme.Names() {
		if name == "notty" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			th, err := theme.Resolve(name, true)
			if err != nil {
				t.Fatalf("resolve %s: %v", name, err)
			}
			allowed := map[string]bool{}
			adj := func(c theme.Color) theme.Color {
				return theme.AdjustForSurface(c, th.ExecPanel, 4.5)
			}
			syntax := theme.AdjustSyntax(th)
			for _, c := range []theme.Color{
				th.Text, th.TextMuted, th.Bright, th.Soft, th.Accent, th.Rule,
				th.CodePanel, th.ExecPanel, th.Success, th.Error, th.Warning,
				adj(th.Text), adj(th.TextMuted), adj(th.Bright), adj(th.Soft),
				adj(th.Accent), adj(th.Success), adj(th.Error), adj(th.Warning),
				syntax.Comment, syntax.Keyword, syntax.String, syntax.Function,
				syntax.Number, syntax.Type, syntax.Operator, syntax.Punctuation,
				syntax.Constant,
			} {
				if c != "" {
					allowed[string(c)] = true
				}
			}

			check := func(where string, p gansi.StylePrimitive) {
				if p.Color != nil && !allowed[*p.Color] {
					t.Errorf("%s color %q is not a theme token", where, *p.Color)
				}
				if p.BackgroundColor != nil && !allowed[*p.BackgroundColor] {
					t.Errorf("%s background %q is not a theme token", where, *p.BackgroundColor)
				}
			}

			c := styleFor(th)
			check("document", c.Document.StylePrimitive)
			check("blockquote", c.BlockQuote.StylePrimitive)
			check("heading", c.Heading.StylePrimitive)
			check("h1", c.H1.StylePrimitive)
			check("h2", c.H2.StylePrimitive)
			check("h3", c.H3.StylePrimitive)
			check("h4", c.H4.StylePrimitive)
			check("h5", c.H5.StylePrimitive)
			check("h6", c.H6.StylePrimitive)
			check("link", c.Link)
			check("linkText", c.LinkText)
			check("image", c.Image)
			check("imageText", c.ImageText)
			check("rule", c.HorizontalRule)
			check("code", c.Code.StylePrimitive)
			check("codeBlock", c.CodeBlock.StylePrimitive)

			if c.CodeBlock.Theme == "" {
				t.Fatal("chroma theme name is unset")
			}
			registered := chromastyles.Get(c.CodeBlock.Theme)
			if registered == nil || registered.Name != c.CodeBlock.Theme {
				t.Fatalf("chroma style %q is not registered", c.CodeBlock.Theme)
			}
			for _, token := range chromaTokens {
				entry := registered.Get(token)
				if !entry.Colour.IsSet() {
					t.Errorf("chroma.%s has no theme color", token)
					continue
				}
				colour := entry.Colour.String()
				if !allowed[colour] {
					t.Errorf("chroma.%s color %q is not a theme token", token, colour)
				}
			}
		})
	}
}

// TestThemeSyntaxIsUsed checks that a theme's own syntax palette reaches
// chroma, rather than being derived from the generic roles.
func TestThemeSyntaxIsUsed(t *testing.T) {
	th := theme.CatppuccinMocha
	syntax := theme.AdjustSyntax(th)
	c := styleFor(th)
	registered := chromastyles.Get(c.CodeBlock.Theme)
	if got := registered.Get(chroma.Keyword).Colour.String(); got != string(syntax.Keyword) {
		t.Fatalf("chroma keyword = %q, want %q", got, syntax.Keyword)
	}
	if got := registered.Get(chroma.NameFunction).Colour.String(); got != string(syntax.Function) {
		t.Fatalf("chroma function = %q, want %q", got, syntax.Function)
	}

	want := theme.AdjustForSurface(theme.Dark.Accent, theme.Dark.ExecPanel, 4.5)
	darkStyle := styleFor(theme.Dark)
	darkRegistered := chromastyles.Get(darkStyle.CodeBlock.Theme)
	if got := darkRegistered.Get(chroma.Keyword).Colour.String(); got != string(want) {
		t.Fatalf("legacy chroma keyword = %q, want %q", got, want)
	}
}
