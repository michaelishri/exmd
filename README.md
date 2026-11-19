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

In the TUI, highlight a heading in the outline and press **`\``** to select the
code blocks in its section; **`r`** runs the highlighted block *in place*, with
its output streaming into the document beneath it. Arrows walk between blocks,
**`s`** stops a running block, and each executed block keeps its output and
status indicator as you move through the document.

Because the spec mandates no particular interface, nothing prevents other conforming
runners from executing runbooks headless (CI, pipelines) — the reference tool simply
optimizes for human use.

## Docs

- [`SPEC.md`](SPEC.md) — the specification (start here for format and semantics)
- [`docs/cli-design-principles.md`](docs/cli-design-principles.md) —
  CLI design principles shaping the tool's UX
