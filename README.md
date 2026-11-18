# md-exec

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
| Lives in | `SPEC.md` | `md-exec` — this repo's Go implementation |
| Defines | The document format and execution semantics: frontmatter schema, `globals`, `lang:exec` fences, `@script` references, the env-harvest state rule, failure behavior | Everything about *using* the runner: the TUI, rendering, colors, progress views, keybindings |
| Normative? | Yes — a versioned contract (v1); any runner, in any language, can conform | No — one conforming implementation of the spec |
| Changes when | The format or semantics evolve (bump the `spec:` version) | Whenever the UX improves — no spec impact |

Rules of thumb:

- A change to what a **document** means, or how **blocks execute** → belongs in the spec.
- A change to how the **human** drives or watches the runner → tool only.
- The tool may add UX on top; it never weakens spec semantics (no interpolation, no
  persistent sessions — see SPEC.md §9).

## The tool

`md-exec` is the reference implementation of the spec, written in Go. **Execution is a
TUI experience.** Headless commands are utilities that read and validate documents —
they never execute blocks:

- `md-exec check DOC` — validate the document: frontmatter, fence grammar, adapters
  and interpreters, script-reference files; reports all diagnostics at once
- `md-exec print DOC` — render the document to the terminal
- `md-exec list DOC` — enumerate executable blocks: index, language, kind (embedded /
  `@script`), line number, referenced path
- `md-exec globals DOC [--set KEY=VALUE]...` — show the document's parameter surface:
  declared globals, defaults, overrides, and what starts present vs absent

In the TUI you watch blocks execute step-by-step, inspect the globals table as it
evolves, and review each block's source before and as it runs. Blocks always run
headless internally — fresh process, empty stdin — exactly as the spec requires; the
TUI is the layer the human sees, not an environment the blocks run in.

Because the spec mandates no particular interface, nothing prevents other conforming
runners from executing runbooks headless (CI, pipelines) — the reference tool simply
optimizes for human use.

## Status

- [x] Spec v1 draft (`SPEC.md`)
- [x] Document load path: frontmatter + block extraction (goldmark + yaml.v3)
- [x] Headless utilities: `check`, `list`, `globals`, `print`
- [x] TUI viewer: rendered content pane + outline/blocks sidebar (borderless, OpenCode-style)
- [x] Execution core: `bash` and `python3` adapters, env harvest
- [x] TUI execution UX: `r` runs the document with live block output and a
      live globals inspector; `n` steps one block at a time; `s` stops
      (the engine terminates the current process group, SPEC §4.7);
      `md-exec DOC --set K=V` overrides
- [x] `--theme auto|dark|notty` selects the palette for the TUI and `print`
      (auto: dark when the terminal supports color, notty otherwise)

## Docs

- [`SPEC.md`](SPEC.md) — the specification (start here for format and semantics)
- [`docs/10-design-principles-for-delightful-clis.md`](docs/10-design-principles-for-delightful-clis.md) —
  CLI design principles shaping the tool's UX
