# exmd

> Executable Markdown: ordinary `.md` files that run.

A **runbook** is a Markdown document whose fenced code blocks execute in order,
sharing state through environment variables. Documents stay fully valid Markdown —
readable, renderable, and diffable everywhere.

````markdown
---
type: runbook
title: Deploy the API
globals: {ENV: staging, IMAGE_TAG: latest, DEPLOY_ID}
---

## Build

```bash:exec
docker build -t "ghcr.io/acme/api:$IMAGE_TAG" .
```

## Register

```python3:exec
os.environ["DEPLOY_ID"] = register_deployment()
```
````

## The spec and the tool are different things

This repo deliberately separates the two. Don't confuse them:

| | **The specification** | **The tool** |
|---|---|---|
| Lives in | `SPEC.md` | `exmd` — this repo's Go implementation |
| Defines | The document format and execution semantics: frontmatter schema, `globals`, `lang:exec` fences, `@script` references, the env-harvest state rule, failure behavior | Everything about *using* the runner: the TUI, rendering, colors, progress views, keybindings |
| Normative? | Yes — a versioned contract (v1); any runner, in any language, can conform | No — one conforming implementation of the spec |
| Changes when | The format or semantics evolve (bump the `spec:` version) | Whenever the UX improves — no spec impact |

Rules of thumb:

- A change to what a **document** means, or how **blocks execute** → belongs in the spec.
- A change to how the **human** drives or watches the runner → tool only.
- The tool may add UX on top; it never weakens spec semantics (no interpolation, no
  persistent sessions — see SPEC.md §9).

## Install with Homebrew

On macOS or Linux with an ARM64 or x86-64 processor, install from the
[Homebrew tap](https://github.com/michaelishri/homebrew-tap).

```sh
brew install michaelishri/tap/exmd
exmd --version
```

## The tool

`exmd` is the reference implementation of the spec, written in Go. **Execution is a
TUI experience.** Headless commands are utilities that read and validate documents —
they never execute blocks:

- `exmd check DOC` — validate the document: frontmatter, fence grammar, adapters
  and interpreters, script-reference files; reports all diagnostics at once
- `exmd print DOC [--width N] [--theme auto|dark|notty]` — render the
  document to the terminal
- `exmd list DOC` — enumerate executable blocks: index, language, kind (embedded /
  `@script`), line number, referenced path
- `exmd globals DOC [--set KEY=VALUE]...` — show the document's parameter surface:
  declared globals, defaults, overrides, and what starts present vs absent

Themes are owned by the tool, not mixed with the terminal palette. `auto`
(the default) and `dark` use exmd's dark palette and paint the whole canvas,
so the reader looks the same in a light terminal and a dark one.
`--theme notty` emits no color at all, leaving the terminal's own palette in
charge.

In the TUI, highlight a heading in the outline and press **`\``** to select the
code blocks in its section; **`r`** runs the highlighted block *in place*, with
its output streaming into the document beneath it. Arrows walk between blocks,
**`c`** copies the focused block's output to the clipboard as plain text (via
OSC 52), **`s`** stops a running block, and each executed block keeps its output
and status indicator as you move through the document. Drag with the mouse in
the text pane to select any on-screen text; releasing copies the selection to
the clipboard as plain text. When the focused block's output contains `http://`
or `https://` links, **`u`** copies a URL — pressing it again cycles through
the URLs in that output.

### Sessions

The TUI saves one session per document as you work: when a block starts, while
its output streams, when it finishes, and when you quit. Resume it with:

```sh
exmd DOC --resume
```

`exmd --resume --list` shows saved sessions; `exmd DOC --resume --clear`
discards one. A resumed session restores the harvested globals, each block's
output and status, the next block, and the highlighted block. It does **not** restore a
live process, the host environment, or the scroll position: a run that was
still active comes back stopped, with its partial output kept and ready to
re-run.

Snapshots live in `$XDG_CONFIG_HOME/exmd/sessions/` (or
`~/.config/exmd/sessions/`). They are refused when the document or any script
it references has changed since the save, when `--set` overrides differ, or
when the snapshot is malformed — in each case the error says how to recover.

This is tool-level UX, not a new execution semantic: saved globals are replayed
into a fresh run through the §4 state rules, so no shell, working directory,
function, or import survives a resume (SPEC.md §4.5, §9).

Headings that don't already start with a number are numbered automatically in
both the viewer and `exmd print` — `1.`, `1.1.`, and so on. A document whose
headings already carry numbers (like `demo.md`) is displayed exactly as
written.

Because the spec mandates no particular interface, nothing prevents other conforming
runners from executing runbooks headless (CI, pipelines) — the reference tool simply
optimizes for human use.

## Docs

- [`SPEC.md`](SPEC.md) — the specification (start here for format and semantics)
- [`docs/cli-design-principles.md`](docs/cli-design-principles.md) —
  CLI design principles shaping the tool's UX
- [`docs/releasing.md`](docs/releasing.md) — how to build and publish a GitHub release
