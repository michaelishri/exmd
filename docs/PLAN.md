# md-exec — Implementation Plan

> **Status:** the approved pre-implementation plan, preserved verbatim in
> intent. Written before any Go code existed; the OpenCode-style UI amendment
> (§5) was folded in before implementation began. Section 8 records what
> changed between this plan and the shipped implementation.
>
> Companion documents: `SPEC.md` (the contract), `README.md` (the project),
> `docs/HANDOFF.md` (current state and continuation guide).

---

## 1. Library stack (versions verified against the Go module proxy)

| Package | Version | Role |
|---|---|---|
| `charmbracelet/bubbletea` | v1.3.10 | TUI runtime (Elm architecture) |
| `charmbracelet/lipgloss` | v1.1.0 | Panes, colors, layout |
| `charmbracelet/bubbles` | v1.0.0 | `viewport` (content scroll), `help` (status bar) |
| `charmbracelet/glamour` | v1.0.0 | Markdown → ANSI (left pane, and `print`) |
| `charmbracelet/x/ansi` | latest | ANSI-aware measuring/truncation for pane widths |
| `yuin/goldmark` | v1.8.7 | SPEC §2 load path: fence extraction + heading outline |
| `gopkg.in/yaml.v3` | v3.0.1 | Frontmatter (nilable globals map) |

No v2 tags existed on the stable Charm paths, so the mature v1 stack was
chosen.

## 2. Repository layout

```
md-exec/
├── cmd/md-exec/main.go        # entry: no subcommand + DOC.md → TUI; check/list/globals/print verbs
├── internal/
│   ├── doc/                   # SPEC §2: frontmatter, globals, blocks, outline, collect-all diagnostics
│   ├── render/                # doc + width → ANSI lines, heading offsets, block marks (shared TUI + print)
│   ├── exec/                  # SPEC §4–5: adapters, harness, harvest, runner
│   └── tui/                   # bubbletea models, panes, keymap, styles
├── examples/demo/
│   ├── demo.md
│   └── scripts/{prepare.sh, register.py}
└── testdata/                  # loader fixtures (valid + invalid docs)
```

## 3. Phases

1. **Load path** — `internal/doc`: goldmark AST walk (fence info strings,
   script refs, headings with line numbers), yaml.v3 frontmatter, SPEC §2.3
   collect-all validation. Falls out naturally: `check`, `list`, `globals`
   work headless. Golden tests against `testdata/`.
2. **TUI viewer** — panes, rendering, outline, scrolling (the core ask).
3. **Execution engine** — `internal/exec`: bash harness, python prelude,
   harvest, §4 semantics; engine tests.
4. **TUI execution UX** — run controls, globals inspector, step-through;
   `print` reuses `internal/render`.

## 4. Sample runbook — `examples/demo/`

A safe feature tour: writes only inside a scratch workspace it creates.
Covers every v1 spec feature a document *can* self-demonstrate (host-env
masking and spec-version rejection cannot be self-demoed — those go to
fixtures).

### 4.1 `demo.md` (as designed)

````markdown
---
type: runbook
title: md-exec feature tour
spec: 1
tags: demo,reference    # unknown frontmatter key → runner warns (spec §2.1)
globals: {AUTHOR: "Michael Ishri", NOTE: "hello from frontmatter", JOB_NAME, DEPLOY_ID, WORK, LOG_LINES}
---

# md-exec feature tour

This runbook exercises every v1 feature of the [spec](../../SPEC.md) safely:
it only writes inside a scratch workspace it creates for itself.
Provide `JOB_NAME` when running (launch-time override, e.g. `--set JOB_NAME=nightly`).

## 1. Preflight and point-of-use guards

```bash:exec
: "${JOB_NAME:?provide JOB_NAME via a launch-time override (e.g. --set JOB_NAME=nightly)}"
echo "author:  $AUTHOR"
echo "note:    $NOTE"
```

## 2. Isolation: `cd` does not leak

Blocks run in fresh processes (spec §4.5) — working-directory changes do not
survive a block:

```bash:exec
cd /tmp && echo "inside this block: $(pwd)"
```

```bash:exec
echo "in the next block:   $(pwd)"
```

## 3. Set a global from bash

`export` is the whole API (spec §4.3):

```bash:exec
WORK="$(mktemp -d)"
export WORK
echo "workspace: $WORK"
```

## 4. Set a global from Python (alias adapter)

`python` maps to the `python3` adapter (spec §5.2). `os.environ` writes are
harvested exactly like bash `export`:

```python:exec
import os
os.environ["DEPLOY_ID"] = "dpl-" + os.environ["JOB_NAME"]
print("registered:", os.environ["DEPLOY_ID"])
```

## 5. Script references

A body that is a single `@path` line executes a file relative to *this document's
directory* (spec §3.2). Shebangs are inert; input arrives via globals.

### 5.1 From bash

```bash:exec
@scripts/prepare.sh
```

### 5.2 From Python

```python3:exec
@scripts/register.py
```

## 6. Closed world and value fidelity

### 6.1 Only exported state is harvested

```bash:exec
INVISIBLE="set but not exported"   # invisible to later blocks (spec §4.5)
export UNDECLARED_X="undeclared"   # harvested → warning: ignored (spec §4.4)
false || echo "survived: || true is the failure escape hatch (spec §4.7)"
```

### 6.2 What you should see next

```bash:exec
echo "INVISIBLE is: ${INVISIBLE:-<absent — unexported state did not survive>}"
echo "--- LOG_LINES (a multiline global, harvested losslessly) ---"
echo "$LOG_LINES"
echo "--- end ---"
```

## 7. Cleanup: unsetting a global

The harvest delta includes removals (spec §4.3):

```bash:exec
unset WORK
echo "WORK unset"
```

```bash:exec
echo "WORK is now: ${WORK:-<absent — unset propagated>}"
```

## 8. Display-only fences

Fences without `:exec` never execute (spec §3.3):

```bash
# This block is inert — it renders, it does not run.
echo "just showing code"
```

## 9. Where we ended up

```bash:exec
echo "run by $AUTHOR for job $JOB_NAME"
echo "deploy id: $DEPLOY_ID"
echo "workspace: ${WORK:-<unset>}"
```
````

### 4.2 `scripts/prepare.sh` (referenced by `@scripts/prepare.sh`)

```bash
#!/usr/bin/env bash
# Referenced via @scripts/prepare.sh — sourced by the runner's bash harness.
# A shebang line is inert (spec §3.2); input arrives via globals.

mkdir -p "$WORK/staging"
printf 'artifact\n' > "$WORK/staging/artifact.txt"

# Multiline value: exercises lossless harvesting (spec §4.3)
export LOG_LINES=$'first line\nsecond line\nthird line'
echo "prepared $WORK/staging"
```

### 4.3 `scripts/register.py` (referenced by `@scripts/register.py`)

```python
"""Referenced via @scripts/register.py — executed by the runner's python3 adapter.

os.environ writes are harvested exactly like bash `export` (spec §4.3).
"""
import json, os

manifest = {
    "job": os.environ["JOB_NAME"],
    "workspace": os.environ["WORK"],
    "lines": os.environ.get("LOG_LINES", "").splitlines(),
}
with open(os.path.join(os.environ["WORK"], "manifest.json"), "w") as f:
    json.dump(manifest, f, indent=2)

os.environ["DEPLOY_ID"] = f"dpl-{len(manifest['lines'])}x-{os.environ['JOB_NAME']}"
print("registered:", os.environ["DEPLOY_ID"])
```

### 4.4 Feature-coverage map

| Spec feature | Exercised by |
|---|---|
| `type`/`title`/`spec` frontmatter | frontmatter |
| Unknown-key warning | `tags:` |
| Flow-form globals · defaults · bare keys | `AUTHOR`/`NOTE` · `JOB_NAME`/`DEPLOY_ID`/`WORK`/`LOG_LINES` |
| Point-of-use guard (absent global) | §1 |
| Fresh-process isolation (`cd`) | §2 |
| bash export harvest | §3 |
| `python` alias + `os.environ` harvest | §4 |
| `@script` refs, docdir-relative (bash + python3) | §5.1, §5.2 |
| Multiline value harvested losslessly | `LOG_LINES` |
| Harvest of *changed* value (overwrite) | §5.2 re-sets `DEPLOY_ID` |
| Unexported state invisible | §6.1 → §6.2 |
| Closed-world warning (undeclared export) | §6.1 `UNDECLARED_X` |
| `|| true` failure escape hatch | §6.1 |
| Unset propagates as removal | §7 |
| Display fence inert | §8 |
| h1/h2/h3 outline depth | throughout |

## 5. UI design (final approved form: OpenCode-style, borderless)

The first draft used bordered lipgloss panes; the author replaced it with the
OpenCode TUI look — essentially borderless, separation by background shades,
color reserved for state, unobtrusive text.

### 5.1 Design tokens (`internal/theme`)

| Token | Role | Dark default (calibrate later) |
|---|---|---|
| `background` | app canvas (terminal's own bg) | near-black |
| `backgroundPanel` | raised surfaces: outline column, code blocks | one shade up |
| `backgroundElement` | emphasis: selected outline row, chips | two shades up |
| `borderSubtle` | the one hairline separator (above status bar) | dark gray |
| `text` / `textMuted` | content / all chrome and meta | light gray / mid gray |
| `primary` | state only: current section, badges, markers | violet-blue accent |
| `success` `error` `warning` | reserved for run state | — |

Themes: `opencode-dark` (default) and `notty` fallback for color-less
terminals; exact hexes calibrated at implementation time.

### 5.2 Layout — separation by shade, never by border

```
 md-exec feature tour                                examples/demo/demo.md
 ────────────────────────────────────────────────────────────────────────────

   # md-exec feature tour
   This runbook exercises every v1 feature…

   ▸ bash — line 12                      ← badge: accent glyph + muted meta
     WORK="$(mktemp -d)"                 ← code block shaded backgroundPanel
     export WORK

                                    │      1. Preflight…           ← outline column:
                                    │      2. Isolation…              backgroundPanel shade,
                                    │    ▸ 3. Set a global from bash  current row = element shade
                                    │      4. Set a global from Py…   + primary marker
                                    │
 j/k scroll · tab focus · t blocks · ? help · q quit          L 128/642 · spec v1 · 7 blocks
 ────────────────────────────────────────────────────────────────────────────
```

1. **No boxes anywhere.** The ¼ outline pane is a painted column: whole right
   strip in `backgroundPanel`, entries in `textMuted`, current section shaded
   `backgroundElement` with a `▸` marker in `primary`. One empty gutter column
   separates content from sidebar; the only line in the UI is the hairline
   above the status bar.
2. **Header row (1 line):** doc title bold, path muted, right-aligned chips
   (`spec v1` · `N globals` · `N blocks`). Degrades to bare title on narrow
   terminals.
3. **Exec-block badges:** `▸ bash — line 12` — accent glyph, muted meta.
4. **Status bar (1 line):** muted key hints left; position + doc stats right,
   current section in `primary` (scroll-spy echo).
5. **Scrollbar:** optional 1-column slim scrollbar, off in `notty`.
6. **glamour retuning:** derive from the dark style but override block
   backgrounds to the panel token; clean headings without inverted banners;
   accent used sparingly.
7. **Color discipline:** bright `text` for prose, `textMuted` for chrome,
   `primary` strictly for state and interactive markers.

### 5.3 Behavior

- **Left pane:** glamour render, per-chunk so each heading has a rendered-line
  offset; `bubbles/viewport` scrolls; mouse wheel enabled.
- **Right pane:** outline from `internal/doc` (h1–h3 indented). **Scroll-spy**
  auto-highlights the current section. `t` toggles a **Blocks tab** listing
  executable blocks (index, lang, kind, line). Hides below ~90 columns.
- **Focus model:** `tab`/`shift+tab` moves focus content ↔ outline; `enter`
  jumps to the selected heading; `j/k/↑/↓`, `ctrl+u/d`, `g/G` scroll; `?`
  help; `q` quit.
- **Resize:** `WindowSizeMsg` re-runs the pure render; scroll position
  re-anchors to the topmost visible heading.

## 6. Test fixtures (`testdata/`)

Valid: flow globals · block globals · CRLF document · blocks inside list
items · display-only (zero blocks). Invalid — one diagnostic each to prove
collect-all reporting: bad YAML · string-form `globals` · unknown block
attribute · missing script file · empty `:exec` body · invalid global name ·
non-scalar global value · duplicate global key · non-runbook doc · `spec: 2` ·
unclosed frontmatter · display-only doc · unknown language.

## 7. Assumed defaults (as approved)

- Module path `github.com/michaelishri/md-exec`
- Bare `md-exec DOC.md` opens the TUI — tool is TUI-first
- Outline gets the Blocks tab on `t`; scroll-spy always on
- Outline hidden below ~90 cols
- Bubbletea v1 stack (v2 had no stable tags)
- Glamour `notty` when color is unavailable, auto dark otherwise

---

## 8. Outcomes and deviations from this plan

Recorded after implementation; the plan survived largely intact.

**Kept as designed:** the stack, repo layout, phase order, the full demo
runbook (shipped nearly verbatim), the feature-coverage map, the borderless
token system, scroll-spy, badges, chips, status bar, hide-below-90-cols, and
all assumed defaults.

**Changed during implementation:**

- **Fixtures moved** from a root `testdata/` into package-local
  `internal/doc/testdata/` (idiomatic for fixture-relative tests); likewise
  `internal/exec/testdata/` for engine docs.
- **Golden-file render tests** became structural tests (mark ordering, width
  budget, determinism) — the theme/ANSI surface was judged too volatile for
  goldens; `internal/render` stays theme-independent as planned.
- **glamour styling** uses `glamour.WithStyles(<derived StyleConfig>)` built
  from `styles.DarkStyleConfig` in code rather than JSON files.
- **Engine grew a `Session`** (`NewSession`/`RunNext`/`Close`) beyond the
  planned one-shot runner, to power TUI step-through (`n`), continue (`r`),
  and cancellation (`s`) — with process-group kill semantics per §4.7 and a
  context-carrying `Run(ctx, …)`.
- **`EventGlobals` snapshots** were added to the event stream so the sidebar
  can show the globals table evolving live (§4.8 observation).
- **Spec amendments made during implementation:** §5.1.6 (adapters suppress
  interpreter-managed names — bash rewrites `PWD`/`SHLVL`/`OLDPWD`/`_` every
  block) and §4.2 gained the missing "blocks run in docdir" rule.
- **Harness choice:** the plan's "python prelude" became a standalone wrapper
  script (`runpy` + `atexit`); the bash harness dumps `compgen -e` from an
  `EXIT` trap — `env -0` was rejected because a child process would inject
  `_=/usr/bin/env` into the dump.
- **Doc struct additions** the renderer needed: `Document.Source` (normalized)
  and `Document.BodyLine` (first line after frontmatter) as chunk origin.
- **Still open:** `--theme` flag; design feedback from the author (pending by
  choice, see `docs/HANDOFF.md` §10).
