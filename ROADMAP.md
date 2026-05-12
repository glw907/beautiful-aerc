# ROADMAP

> Strategic initiatives. Managed by `/log-project`. Issues tracked in `BACKLOG.md`.

## Active

### `internal/ui/` all-value path `ui-all-value`
Pass 27 (ADR-0212) converted `catkin.Model` to all-value and Pass
28 (ADR-0213) deleted the `mailcompose.Editor` adapter. Every
other UI subpackage still ships the pre-27 straddle: value
`Update`/`View` for the Elm contract, but state mutates through
pointer-receiver setters (`SetMessages`, `SetSize`, `SetLayout`,
`ToggleFold`, `dispatchTriage`, `layout()`, …) that the parent
calls directly. Surfaced by Audit B.1 (2026-05-12). The straddle
works — it forecloses the reasoning gains that motivated ADR-0212
(immutable snapshots, value diffs, no hidden aliasing) but it
isn't a bug.

Convert one subpackage per follow-up pass. Order, highest mutation
density first:

1. `internal/ui/messagelist/` — 22 pointer mutators on `*Model`,
   most-touched after compose.
2. `internal/ui/sidebar/` — 9 mutators on `*Model` plus `*Search`
   mutators.
3. `internal/ui/compose/` — entirely pointer-receivered, including
   `Init`/`Update`/`View`. Largest subpackage; the conversion will
   want its own ADR for the Msg vocabulary.
4. `internal/ui/account/` — 10 pointer dispatch helpers; value
   `Update`/`View`.
5. `internal/ui/reader/` — value almost-everywhere; one
   pointer-receiver `layout()`.
6. `internal/ui/contacts/` — `*Popover`, `*List`, `*Sidebar`
   mutators.
7. `internal/ui/outbox/` — `SetSize`, `SetRows`.
8. `internal/ui/wizard/` — every `*Section` type. Bootstrap-only
   surface; lowest priority.

Each conversion mirrors the catkin pattern: pointer setter →
`SetFooMsg{...}` handled in `Update`. Where the parent currently
calls `child.SetX(v)`, it returns a Cmd that emits the Msg, or
the parent threads `v` through the `Update` delegation. The cost
is parent-side refactoring more than child-side mechanical work
— the parent currently relies on synchronous `SetX` calls to set
state *before* the next render frame.

This is structural cleanup, not a bug. Sequence between feature
passes; not a gate on beta soak.

Related: (none yet — file with `/log-issue` as passes are scoped)

### AI-overengineering cleanup `overengineering-cleanup`
Three patterns the codebase explicitly forbids in `CLAUDE.md` /
`go-conventions` but ships anyway. All three are pre-beta-cheap to
fix; all three keep paying review tax until they go.

1. **Inline `mailcompose.Editor` + delete `CatkinEditor`.** 17-method
   single-impl interface with zero-line pass-throughs to `catkin.Model`,
   justified by a speculative v1.1 neovim adapter (ADR-0033). Pre-beta
   the rule is "strip the seam, the next consumer re-adds it with
   itself." Compose holds `catkin.Model` directly; v1.1 introduces
   the seam alongside the second implementation. **Subsumed by
   `catkin-all-value`** — the wrapper exists to babysit catkin's
   mixed receivers; once catkin is pure-value, the wrapper falls
   out for free. Land that project first.

2. **Collapse the triplicate `WithLogger` functional-options pattern.**
   `mailjmap.Option` / `mailimap.Option` / `cache.Option` each declare a
   variadic-options framing for one optional parameter. Replace with a
   plain `*slog.Logger` argument (nil → `slog.Default()`) or a
   `SetLogger` setter. Update ADR-0197 to record the simplification.

3. **Fold `internal/backoff/` and `internal/humanize/` into their
   callers.** Two micro-packages (12 and 18 source lines) plus test
   files plus package docs for helpers with three or four callers
   each. `backoff.Exponential` also carries defensive `<= 0` clamps
   on internal callers — directly forbidden by the no-defensive-
   checks-between-internal-callers rule. Move both into the package
   of their primary caller (`mailjmap` for backoff, `internal/ui` or
   `internal/cache` for `Bytes`), drop the clamps.

Three passes or one — small enough to land together if scheduled
adjacent to a quiet UI pass. None of the three is load-bearing on
behavior; all are net deletions.

Related: (none yet — file with `/log-issue` as passes are scoped)

### app.go decomposition `app-decomposition`
`internal/ui/app.go` is 1636 lines and `App.Update` is 874 of them
(line 243–1119), dispatching every screen, modal, key path, and
cross-cutting message from a single switch. `App.View` is another
118-line block. The seams already exist (`updateContactsKey` at
line 1541 is the proof) — they just haven't been pulled out.
Goal: split into per-screen controller methods (`updateAccount`,
`updateContacts`, `updateCompose`, `updateModals`, …) plus a thin
top-level `Update` that routes, and peel the file into
`app_update.go` / `app_view.go` / `app_chrome.go` so no single
file exceeds ~600 lines. Pure refactor — no behavior change, no
new features. Worth doing pre-beta while the `elm-conventions`
skill is still flexible about restructuring.

Related: (none yet — file with `/log-issue` as passes are scoped)

### Mouse support throughout `mouse-support`
Wire `v.MouseMode` + `v.OnMouse` across the UI. Scope: link clicking
in the reader (linkpicker becomes optional, not the only path),
attachment row clicks to open / save, wheel-scroll in messagelist
and reader, sidebar folder selection, and click-to-focus between
panes. Not a vim-replacement — keyboard remains primary; mouse is
additive for the surfaces where pointing genuinely beats a picker.
`OnMouse` is the v2 hit-test seam: each subpackage's `View()`
declares its clickable regions, bubbletea routes the click.
Aerc and Thunderbird both ship mouse on by default; a UI config
toggle defaults to on with an opt-out for keyboard purists.

Related: (none yet — file with `/log-issue` as passes are scoped)

### SPUA-aware width measurement for bubbletea `spua-width-upstream`
Collapse ADR-0084's architectural cost — `displayCells`, `displayTruncate`,
`padToVisibleWidth`, the `lipgloss.JoinHorizontal` ban on icon-bearing
rows, manual row-by-row joins in `AccountTab.View` / `App.renderFrame`,
the three-mode resolution at startup, and the CPR cell-width probe —
without giving up Nerd Font iconography.

**Status (Pass 9w, 2026-05-08):** upstream PR path closed; vendored
`internal/ansix/` shim queued as Pass 9w.1. The investigation found
that `charmbracelet/x/ansi` v0.11+ routes through
`clipperhouse/displaywidth`, which has no per-rune override. A PR
adding `Options.Overrides []OverrideRange` was implemented and
benchmarked on a local fork (parked at
`~/Projects/displaywidth/add-overrides`), but a maintainer comment
on PR #23 (2026-05-02) signaled a preference for wrapping the
library rather than accepting an in-library hook, and the repo had
zero external-contributor merges in the prior seven months. ADR-0180
codifies the decision; archived specs and PR draft live in
`docs/superpowers/archive/specs/`.

**Re-engagement trigger (future).** The fork prototype is preserved.
File the upstream PR — or, more likely, an issue linking the
maintainer's comment — once **two** conditions hold: (a) poplar
(or the eventual ansix package, if spun out) has demonstrable
adoption beyond a single project, *and* (b) the load-bearing
argument shifts from "we want a hook" to "the lipgloss-internal
callsites (`JoinHorizontal`, components like `bubbles/help`,
`bubble-table`) cannot be wrapped at the application layer because
they call `lipgloss.Width` inside their own render code, and the
library's 'just wrap it' answer therefore doesn't compose." That
argument is sharper than the original use-case framing and is what
would land this without burning review goodwill on a second cold
attempt. Until both hold, ansix is sufficient.

Related: BACKLOG #20 (closed by ADR-0084 — superseded if upstream
ever lands).

## Done

### Bubbletea v2 declarative View fields `v2-view-fields`
Pass 32 (ADR-0217) wired `ProgressBar` (priority ladder
attachment > outbox > sync), `ReportFocus` + an unfocused-only
new-mail toast (`[ui] new-mail-toast`, 1s coalesce), and
`KeyboardEnhancements.ReportEventTypes`. `uicore.GatedBinding`
tags catkin's six chord bindings; `catkin.ActiveChords(disambig)`
projects the renderable subset for helppopover Compose and the
compose body-focus footer hint. Deferred consumers: server-side
IDLE pause on blur and `IsRepeat` held-key acceleration — the
field plumbing is in place but no consumer is wired yet.

### Catkin Elm conformance (all-value path) `catkin-all-value`
Pass 27 (ADR-0212) landed the catkin all-value path: every pointer
mutator on `catkin.Model` and `catkin.Buffer` became a value-
returning `With*` setter, `Update` returns a fresh value, and the
wrapped `textarea.Model` is sealed behind value-typed accessors.
Pass 28 (ADR-0213) deleted the `mailcompose.Editor` interface and
`CatkinEditor` adapter — `compose.Model` now embeds `catkin.Model`
directly. Closed 2026-05-12 (Audit B.1 confirmed). The follow-on
work for the rest of `internal/ui/` lives in `ui-all-value`.

### Bubbletea v2 alignment + showcase positioning `v2-showcase`
Migrated poplar to charm.land/{bubbletea,lipgloss,bubbles}/v2 and
positioned it as a standout reference implementation for v0.9.0.
22 passes from 13.2 through Pass 22 covered substrate migration,
internal idiom alignment, UX completion, showcase features (inline
images, kitty keyboard text-entry parity, inline mode subcommands),
rendering polish, compatibility audit, and the v0.9.0 cut. Pass
22 added the wizard signature step and capped the initiative.
Completed: 2026-05-11 (v0.9.0 tagged, beta soak active).
Pass-by-pass detail: `docs/poplar/v2-roadmap.md`.
