# SPUA-aware width measurement — upstream-seam investigation

Status: investigation report. Implementation deferred to a
follow-up pass per user direction (investigate-only scope).
Date: 2026-05-08.

Companion to ROADMAP.md `spua-width-upstream`. Supersedes the
optimistic outcome path framed in that entry (one-line override
of `runewidth.DefaultCondition.RuneWidth`), which the survey
proves is no longer load-bearing in the current charm stack.

## Goal

Determine whether ADR-0084's architectural cost — the runtime
SPUA cell-width probe, the `uicore.DisplayCells` /
`DisplayTruncate` / `DisplayPadOrTruncate` family, the
`lipgloss.JoinHorizontal` ban for icon-bearing rows, and the
manual row-by-row joins in `AccountTab.View` / `App.renderFrame`
— can be collapsed by a single override at the lowest
width-measurement layer in the charm dependency chain, without
giving up Nerd Font iconography.

## Method

1. Walked the dependency chain `lipgloss.Width → x/ansi → ...` in
   the resolved versions from `go.mod`
   (lipgloss v1.1.1-pre.20250404, x/ansi v0.11.6, runewidth
   v0.0.23).
2. Identified what the GraphemeWidth path (the default lipgloss
   uses) actually calls.
3. Wrote a probe (`scripts/spuaprobe/main.go` — throwaway, will
   be deleted at pass-end) and ran it against poplar's resolved
   dependencies.

## Findings

### The current width chain

```
lipgloss.Width(s)
  → ansi.StringWidth(s)               // x/ansi/width.go
    → stringWidth(GraphemeWidth, s)
      → FirstGraphemeCluster(s, GraphemeWidth)
        → dwOptions.String(cluster)   // *displaywidth.Options
          → graphemeWidth(...)
            → propertyWidths[lookup(s)]   // generated trie, unexported
```

`dwOptions = &displaywidth.Options{EastAsianWidth: false}` lives
in `x/ansi@v0.11.6/method.go` and is the only place lipgloss's
default path consults. The trie data and `propertyWidths` array
are package-private to `clipperhouse/displaywidth`.

### `runewidth.DefaultCondition` is no longer load-bearing

`x/ansi@v0.11.6` keeps a `wcOptions = &runewidth.Condition{...}`
package var, but it is consulted **only** in `WcWidth` mode
(`ansi.StringWidthWc`, `Method(WcWidth).StringWidth`).
`lipgloss.Width` always uses `GraphemeWidth`, so a runewidth-side
override has no effect on lipgloss callsites. Mutating
`runewidth.DefaultCondition` does nothing for the path poplar
actually takes.

This is a recent change. `x/ansi@v0.8.0` *did* go through
`runewidth.StringWidth` for grapheme widths; the migration to
`clipperhouse/displaywidth` happened between v0.8 and v0.11. The
ROADMAP entry's hypothesis was based on the older shape.

### `clipperhouse/displaywidth` exposes no override hook

`Options` has one knob: `EastAsianWidth bool`. No per-rune
override, no callable hook, no Condition-style customization.
`propertyWidths` is unexported. The trie tables
(`stringWidthValues`, `stringWidthIndex`) are unexported. There
is no SetSPUA or similar API and none has been requested
upstream as of this writing.

### Empirical confirmation

`scripts/spuaprobe/main.go`, run against poplar's resolved deps:

```
ASCII A                       lipgloss=1  ansi.SW=1  ansi.SWWc=1  displaywidth=1  runewidth.SW=1
SPUA-A U+F01C (nf-fa-inbox)   lipgloss=1  ansi.SW=1  ansi.SWWc=1  displaywidth=1  runewidth.SW=1
SPUA-A U+E0A0 (powerline)     lipgloss=1  ansi.SW=1  ansi.SWWc=1  displaywidth=1  runewidth.SW=1
SPUA-B U+F01F0 (nf-md-email)  lipgloss=1  ansi.SW=1  ansi.SWWc=1  displaywidth=1  runewidth.SW=1
Wide CJK 你                    lipgloss=2  ansi.SW=2  ansi.SWWc=2  displaywidth=2  runewidth.SW=2
Emoji 😀                       lipgloss=2  ansi.SW=2  ansi.SWWc=2  displaywidth=2  runewidth.SW=2
```

Even `runewidth` reports SPUA at 1 — Unicode does not classify
Private Use Area as Wide; the rendered double-width comes from
the terminal's `symbol_map` configuration, which is runtime state
no static library can deduce.

## What this means for the ROADMAP outcome paths

**Path (a) — one-line override.** Not viable. The seam the
ROADMAP entry hypothesized does not exist in the current stack.

**Path (b) — vendor a shim + upstream PR.** The only path
forward. Two work products:

- *Vendored shim* (poplar-side): an `internal/ansix/` package
  whose `StringWidth` / `Truncate` add `(spuaCellWidth-1) *
  spuaCount(s)` to the result, then route the rest through
  `x/ansi`. Replaces the `uicore.DisplayCells` family with one
  layer below it. Deletion budget on the poplar side is modest
  (the four wrappers in `uicore/iconwidth.go` simplify; the
  manual row joins **stay** — see caveat below).

- *Upstream PRs* (charm/clipperhouse-side): displaywidth grows an
  override hook (e.g. `Options.OverrideWidth func(grapheme
  string) (int, bool)`); `x/ansi` plumbs it through `dwOptions`;
  lipgloss exposes a setter. Three repositories, three
  reviewers, indeterminate timeline.

### Caveat: vendoring does NOT collapse the JoinHorizontal ban

`lipgloss.JoinHorizontal` calls `lipgloss.Width` internally — we
cannot reroute that without forking lipgloss. So even with the
vendored shim, the ADR-0083/0084 ban on `Join*` for icon-bearing
rows stays in effect when `spuaCellWidth != 1`. The manual
row-by-row joins in `AccountTab.View` and `App.renderFrame`
**stay**. That is the most invasive piece of the ADR-0084
machinery, and it only collapses if the upstream PR lands.

## ADR-0084 machinery: actual deletion budget

A vendored shim — without the upstream PR — would let us
collapse:

- `internal/ui/uicore/iconwidth.go` (133 LOC) — `DisplayCells`,
  `DisplayTruncate`, `DisplayTruncateEllipsis`,
  `DisplayPadOrTruncate`, `FillRowToWidth`. Replaced by direct
  `ansix.Width` / `ansix.Truncate` calls; the `spuaCellWidth`
  global moves into `internal/ansix/`.
- 23 callsite files that import `uicore.DisplayCells` /
  `DisplayTruncate` etc. would change their import to `ansix`
  (or to `lipgloss.Width` if the shim is set up so the
  override flows back through it). Mechanical sweep.

Would **not** collapse without the upstream PR:

- `lipgloss.JoinHorizontal` ban (the structural cost).
- Manual row-by-row joins in `AccountTab.View` /
  `App.renderFrame`.
- `internal/term/{cpr.go,probe.go,resolve.go,font.go}` — the
  Nerd Font detection machinery. `term.HasNerdFont` is needed
  regardless of width math (mode auto-detect).
  `term.MeasureSPUACells` is debatable: if we accept "fancy
  mode always assumes 2-cell SPUA" as the contract (defensible
  given the symbol_map convention is broadly adopted), we can
  delete the CPR probe (~157 LOC including tests). If we keep
  the per-machine empirical receipt, it stays.
- `cmd/poplar/diagnose.go` (63 LOC) — keep regardless; the
  receipt is a debugging asset.

Net: ~200–360 LOC plus 23 import sweeps. Real, not
revolutionary. The architectural cost the ROADMAP entry framed
("collapse the JoinHorizontal ban, the manual joins, the
machinery") is **mostly upstream-bound**.

## Recommended path forward

After pass-end discussion (2026-05-08) the chosen path is:

**File a PR at `clipperhouse/displaywidth` proposing
`Options.Overrides []OverrideRange`.** Direct PR, not issue-
first — pattern research at the repo found 3/3 recent feature
PRs came in cold, with no issue-first norm. Going straight to
PR matches observed culture. Solo maintainer who uses Copilot/
AI-assisted review per `AGENTS.md`; tests and GoDoc are
first-class deliverables. API shape has not been redirected
during review in any observed PR, so the shape must be right on
arrival.

Sequencing:

1. Next pass drafts the PR — forks displaywidth, implements
   `Options.Overrides`, writes table-driven tests + fuzz test +
   GoDoc, verifies the chain works through `lipgloss.Width`,
   files the PR.
2. While the PR sits, poplar code is unchanged. ADR-0084 stays
   in force.
3. On merge: file the corresponding plumbing PR at
   `charmbracelet/x/ansi` to expose the new field through
   `dwOptions`. Possibly also a small lipgloss-side change.
4. Once both upstream changes ship in a tagged release, queue a
   poplar-side cleanup pass: bump deps, set the SPUA override at
   startup, collapse `uicore/iconwidth.go`'s SPUA branches, drop
   ADR-0083/0084's `Join*` ban, delete manual row-by-row joins
   in `AccountTab.View` / `App.renderFrame`, sweep the 23 import
   sites.

If the displaywidth PR is rejected on scope, fall back to either
status quo (no poplar change, accept ADR-0084's architectural
cost) or a vendored `internal/ansix/` shim — the alternatives
documented in this spec's earlier draft. That decision waits for
maintainer signal.

## Risks and open questions

- The ROADMAP entry framed this as a "half-pass" investigation
  closing the question one way or the other. The honest answer
  is "the stack changed under the hypothesis; the cleanest
  outcome is to defer." That is a different shape than the
  entry implied. ROADMAP needs updating.
- `runewidth` v0.0.23 is still vendored transitively even if
  unused on the default path. No action needed; it's used by
  `WcWidth` callers (we have none).
- If charm reverts to runewidth in a future x/ansi version, the
  hypothesis revives. Worth re-checking when bumping `x/ansi`.

## ADR draft (deferred)

No ADR is written by this pass. ADR-0084 stays as-is. If the
user picks Option 3 or, later, upstream lands, that pass writes
an ADR superseding (or narrowing) ADR-0084.

## Throwaway artifacts

- `scripts/spuaprobe/main.go` — delete at pass-end. Reproducible
  from the table above plus the imports listed.
