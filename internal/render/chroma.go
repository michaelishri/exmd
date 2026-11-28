package render

import (
	"fmt"
	"hash/fnv"
	"strings"
	"sync"

	gansi "charm.land/glamour/v2/ansi"
	"github.com/alecthomas/chroma/v2"
	chromastyles "github.com/alecthomas/chroma/v2/styles"

	"github.com/michaelishri/exmd/internal/theme"
)

// Glamour registers a code style under one fixed name ("charm") and skips
// registration once that name exists. In a long-lived TUI that means the
// first theme rendered keeps owning the syntax colors forever. We register
// a separate style per palette and point CodeBlock.Theme at it instead, so
// switching themes at runtime actually changes the code colors.
var chromaMu sync.Mutex

var chromaTokens = []chroma.TokenType{
	chroma.Text, chroma.Error, chroma.Comment, chroma.CommentPreproc,
	chroma.Keyword, chroma.KeywordReserved, chroma.KeywordNamespace,
	chroma.KeywordType, chroma.Operator, chroma.Punctuation,
	chroma.Name, chroma.NameBuiltin, chroma.NameTag, chroma.NameAttribute,
	chroma.NameClass, chroma.NameConstant, chroma.NameDecorator,
	chroma.NameException, chroma.NameFunction, chroma.NameOther,
	chroma.Literal, chroma.LiteralNumber, chroma.LiteralDate,
	chroma.LiteralString, chroma.LiteralStringEscape,
	chroma.GenericDeleted, chroma.GenericEmph, chroma.GenericInserted,
	chroma.GenericStrong, chroma.GenericSubheading, chroma.Background,
}

// registerChromaStyle returns the name of a globally registered chroma style
// for this theme's code colors.
func registerChromaStyle(th theme.Theme, c *gansi.Chroma) string {
	entries := chromaEntries(c)
	hash := fnv.New32a()
	for _, token := range chromaTokens {
		hash.Write([]byte(entries[token]))
	}
	name := fmt.Sprintf("exmd-%s-%x", th.Name, hash.Sum32())

	chromaMu.Lock()
	defer chromaMu.Unlock()
	if _, ok := chromastyles.Registry[name]; !ok {
		chromastyles.Register(chroma.MustNewStyle(name, entries))
	}
	return name
}

func chromaEntries(c *gansi.Chroma) chroma.StyleEntries {
	primitives := []gansi.StylePrimitive{
		c.Text, c.Error, c.Comment, c.CommentPreproc,
		c.Keyword, c.KeywordReserved, c.KeywordNamespace, c.KeywordType,
		c.Operator, c.Punctuation, c.Name, c.NameBuiltin, c.NameTag,
		c.NameAttribute, c.NameClass, c.NameConstant, c.NameDecorator,
		c.NameException, c.NameFunction, c.NameOther, c.Literal,
		c.LiteralNumber, c.LiteralDate, c.LiteralString, c.LiteralStringEscape,
		c.GenericDeleted, c.GenericEmph, c.GenericInserted, c.GenericStrong,
		c.GenericSubheading, c.Background,
	}
	entries := make(chroma.StyleEntries, len(chromaTokens))
	for i, token := range chromaTokens {
		entries[token] = primitiveStyle(primitives[i])
	}
	return entries
}

func primitiveStyle(p gansi.StylePrimitive) string {
	var parts []string
	if p.Color != nil && *p.Color != "" {
		parts = append(parts, *p.Color)
	}
	if p.BackgroundColor != nil && *p.BackgroundColor != "" {
		parts = append(parts, "bg:"+*p.BackgroundColor)
	}
	if p.Italic != nil && *p.Italic {
		parts = append(parts, "italic")
	}
	if p.Bold != nil && *p.Bold {
		parts = append(parts, "bold")
	}
	if p.Underline != nil && *p.Underline {
		parts = append(parts, "underline")
	}
	return strings.Join(parts, " ")
}
