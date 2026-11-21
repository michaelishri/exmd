package render

import (
	"reflect"
	"testing"

	gansi "github.com/charmbracelet/glamour/ansi"

	"github.com/michaelishri/exmd/internal/theme"
)

// TestStyleUsesOnlyThemeColors guards against glamour's built-in palettes
// leaking back in: every color emitted by the content renderer must be one
// of the active theme's tokens.
func TestStyleUsesOnlyThemeColors(t *testing.T) {
	c := styleFor(theme.Dark)
	allowed := map[string]bool{}
	for _, v := range []theme.Color{
		theme.Dark.Text, theme.Dark.TextMuted, theme.Dark.Bright,
		theme.Dark.Soft, theme.Dark.Accent, theme.Dark.Rule,
		theme.Dark.CodePanel, theme.Dark.ExecPanel, theme.Dark.Success,
		theme.Dark.Error, theme.Dark.Warning,
	} {
		allowed[string(v)] = true
	}

	check := func(where string, p gansi.StylePrimitive) {
		if p.Color != nil && !allowed[*p.Color] {
			t.Errorf("%s color %q is not a theme token", where, *p.Color)
		}
		if p.BackgroundColor != nil && !allowed[*p.BackgroundColor] &&
			*p.BackgroundColor != string(theme.Dark.CodePanel) &&
			*p.BackgroundColor != string(theme.Dark.ExecPanel) {
			t.Errorf("%s background %q is not a theme token", where, *p.BackgroundColor)
		}
	}

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

	if c.CodeBlock.Chroma == nil {
		t.Fatal("chroma palette is unset")
	}
	v := reflect.ValueOf(*c.CodeBlock.Chroma)
	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		p := v.Field(i).Interface().(gansi.StylePrimitive)
		if typ.Field(i).Name == "Background" {
			if p.BackgroundColor != nil || p.Color != nil {
				t.Error("chroma background should be unpainted")
			}
			continue
		}
		if p.Color == nil {
			t.Errorf("chroma.%s has no theme color", typ.Field(i).Name)
			continue
		}
		check("chroma."+typ.Field(i).Name, p)
	}
}
