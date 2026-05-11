# ROADMAP

> Strategic initiatives. Managed by `/log-project`. Issues tracked in `BACKLOG.md`.

## Active

### Catkin Elm conformance (all-value path) `catkin-all-value`
`catkin.Model` (and `catkin.Buffer` underneath) ship with mixed
receivers: `Update`/`View`/`Value`/`Focused` are value receivers
(the Elm contract), but `SetStyles`/`SetSize`/`SetWidth`/`SetValue`/
`SetTidyHighlights`/`RegisterAnnotator`/`Focus`/`Blur`/`recordSnap`
mutate the receiver in place. The same instance is simultaneously
immutable-by-Update and mutable-by-setter. This violates
`elm-conventions` ("state in models, mutations only in Update, I/O
only in tea.Cmd, children signal parents via Msg types") at the
most-touched editor surface in the app, and the violation is
load-bearing — the 17-method `mailcompose.Editor` interface +
`CatkinEditor` adapter exists specifically to babysit the
straddle (compose holds a stable pointer that re-syncs the
value-Update result back into a field after every `Update`).

Convert every pointer-mutator on `catkin.Model` (and `catkin.Buffer`)
into a Msg type handled in `Update`. `SetStylesMsg`, `SetSizeMsg`,
`SetValueMsg`, `SetTidyHighlightsMsg`, `RegisterAnnotatorMsg`,
`FocusMsg`/`BlurMsg`, `SetUserWordlistPathMsg`. All-value
throughout. Bubbles/v2/textarea is the upstream contagion vector
— wrap it in a value-typed adapter inside catkin rather than
inheriting its mutation shape.

Knock-on cleanups: `mailcompose.Editor` + `CatkinEditor` collapse
(this subsumes item 1 of `overengineering-cleanup` — once catkin
is pure-value, compose holds `catkin.Model` directly and the
wrapper disappears). `compose.Model` and `wizard/section_signature`
become straight Elm children. Test fakes simplify — no more
stable-pointer dance.

This is the deepest structural fix on the roadmap; sequence it
before any further compose feature work so new code lands on
the conformant shape. ADR-sized — the Msg vocabulary and the
upstream-textarea-adapter shape are the load-bearing decisions.

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

### Bubbletea v2 declarative View fields `v2-view-fields`
Wire three `tea.View` fields poplar already has access to but doesn't
set: `ProgressBar` (OSC 9;4 terminal-native progress for sync, outbox
drain, attachment downloads), `ReportFocus` + `FocusMsg`/`BlurMsg`
(pause JMAP push / IMAP IDLE refresh on blur, kick a refresh on focus,
suppress bell), and `KeyboardEnhancements` (Kitty keyboard protocol —
disambiguates Ctrl+I/M/H from Tab/Enter/Backspace for catkin's chord
set, unlocks `IsRepeat` for held-key acceleration in messagelist /
reader, surfaces `ShiftedCode`/`BaseCode` for non-US keyboards).
Negotiation is graceful — terminals that don't speak the protocol
fall back to existing behavior. Mouse support is a separate
initiative.

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
