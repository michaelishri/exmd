---
name: theme-implementation
description: Add, change, or debug a color theme in exmd. Use when asked to add a palette, tweak theme colors, fix light/dark readability or contrast, or investigate why theme switching does not update code blocks.
---

# Theme implementation

Use this for any work on exmd palettes or theme plumbing. The recurring lesson
from past changes: **do not tune colors by eye in one theme at a time**. The
render pipeline has a few global behaviours; fix those once, then validate
every theme.

## Architecture you must respect

- `internal/theme/theme.go` — `Theme` struct and the default `dark` palette.
  The default `dark` theme has no `Syntax` block; its code colors come from
  role tokens through `legacyChroma` in `internal/render/style.go`.
- `internal/theme/themes.go` — one full `Theme` literal per preset, the
  `registry`, `aliases`, `Names()`, and `Resolve()`.
- `internal/theme/surface.go` — shared derived-surface helpers: `Lift`,
  `Contrast`, `ReadableOn`, `AdjustForSurface`, `AdjustSyntax`.
- `internal/render/style.go` — builds glamour styles and chroma entries.
- `internal/render/chroma.go` — registers one chroma style **per theme**.
- `internal/render/canvas.go` — paints the full frame background and repairs
  backgrounds after ANSI resets.
- `cmd/exmd` — resolves theme precedence: explicit `--theme` flag → saved
  config → default; diagnostics must use the resolved theme too.

## Adding a new theme

1. Add a complete `Theme` literal in `internal/theme/themes.go`. Fill **every**
   token: canvas/panel/element shades, borders, text roles, state colors, bright
   state variants, `CodePanel`, `ExecPanel`, and all nine `Syntax` fields.
   Leave nothing empty except on `Notty`.
2. Add it to `registry` (and `aliases` if useful). `Names()` drives `Resolve`
   errors and the help text, so no separate list is needed.
3. Update the theme table in `README.md`. `TestUsageListsThemes` will fail if
   `usage` in `cmd/exmd/main.go` does not list every name.
4. Run the full test suite (see below).

## Pain points and the rules that came from them

### 1. The terminal palette must never leak in

Every cell is painted by `PaintCanvas`. Do not introduce raw ANSI indices
(`"1"`, `"8"`, etc.) or terminal-default colors. `notty` is the only mode that
emits no color. Diagnostics and footers must use the resolved theme too.

### 2. Glamour's chroma registry is process-global

Glamour registers syntax styles under one fixed name (`"charm"`) and skips
registration once present. Without `internal/render/chroma.go`, the **first
theme rendered** owns code-block colors for the whole process — live switching
appears broken even though the rest of the UI changes. Never set
`CodeBlock.Chroma` directly on the final style; register a named style and set
`CodeBlock.Theme` instead. `TestThemeSwitchChangesCodeColors` guards this.

### 3. An SGR `0` inside a color is not a reset

`isReset` in `canvas.go` must only treat a **leading** `0`/empty parameter as a
reset. Colors such as `#7b5d00` emit `38;2;123;93;0`; matching any `0` paints
the canvas behind those tokens. `TestZeroChannelColorsAreNotResets` guards this.

### 4. Use hue-preserving adjustment, not hand-editing palettes

Light themes need darker code tokens; dark themes need brighter ones. Do **not**
blend toward white/black — that desaturates and makes text look washed out.
`AdjustForSurface` shifts HSL lightness only, preserving hue and saturation,
until the token clears 4.5:1 against that theme's executable surface.
`AdjustSyntax` applies it to all syntax tokens; `styleFor` applies it to code
text. Change this shared rule, not individual palettes, when readability is
wrong across themes.

### 5. Dark and light palettes are independent

Each theme owns its tokens. There is no shared palette between the two groups —
changing one cannot affect the other. The only shared logic is the render-time
contrast rule above. If fixing one group appears to break the other, the bug is
in shared plumbing (registry, reset handling, adjustment), not in the palettes.

### 6. Runtime switching updates every layer

`applyTheme` in `internal/tui/settings.go` must: call `SetTheme(th)` (rebuilds
global lipgloss styles), set `m.theme = th`, then `m.relayout()` (re-renders
content, inline boxes, outline and footer). Passing a `color bool` instead of a
theme anywhere will freeze part of the UI on the old palette.

### 7. Derived surfaces need measured foregrounds

Footer bands use `Lift` with direction based on surface luminance (lighter on
dark themes, darker on light themes). Their foreground comes from `ReadableOn`
against text candidates, not a fixed `TextMuted` token.

## Tests you must run / keep

- `go test ./...`, `go test -race ./...`, `go vet ./...`, `gofmt -l`,
  `git diff --check`.
- `TestResolveAllThemes`, `TestThemesComplete`, `TestResolveInvocationTheme` —
  registry, aliases, precedence, completeness.
- `TestThemeContrast` — default dark text vs canvas.
- `TestAdjustSyntaxContrast` — adjusted syntax tokens vs both code surfaces.
- `TestZeroChannelColorsAreNotResets` — zero-channel regression.
- `TestThemeSwitchChangesCodeColors` — chroma registry regression.
- `TestStyleUsesOnlyThemeColors` — no built-in palette leaks.
- Acceptance: capture the TUI in a pseudo-terminal with `--theme DARK` and
  `--theme LIGHT`; verify no terminal-indexed colors, full-width canvas, and
  readable code text. Restart the TUI after rebuilding — a running process
  keeps the palette it loaded at startup.

## Debugging checklist

1. Measure first. Capture emitted ANSI with a PTY/terminal script and compute
   contrast for the actual foreground/background pairs; do not guess.
2. Confirm the running binary is the one you just built (`./exmd`), and restart
   any open TUI session.
3. If only code blocks are wrong, suspect the chroma registry or `isReset`.
4. If only some themes are wrong, look for a token value shared by those
   themes (e.g. a trailing `00` channel, or a light surface).
5. If switching themes is wrong, suspect a cached style or a missing
   `applyTheme` step, not the palette values.
6. Keep changes minimal and add a regression test for every bug found.
