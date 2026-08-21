# Pass 2c: real-terminal verification and the help screen

> **For agentic workers:** REQUIRED SUB-SKILL: use
> superpowers:subagent-driven-development to implement this plan task
> by task, one poplar-implementer per task, poplar-reviewer and
> poplar-go-reviewer in parallel per diff. Steps use checkbox syntax.

**Goal:** Close the pass 2 gate on evidence a terminal painted: land
the real-kitty capture harness as gate tooling, fix the defects it
found, recompose the help screen, and re-run the gate with Geoff.

**Architecture:** No new layers. `cmd/sketch` stops owning a second
composition path and calls the one `App.View` uses; the theme's
degrade profiles get two falsifiable fixes; the help overlay is
recomposed from a revised wireframe Geoff ratifies first; the harness
(`scripts/kitty-shot`, `scripts/gate-captures`) and its grading
fan-out become the standing tier-4 gate medium above goldens,
teatest, and tmux-check.

**Tech stack:** Go 1.26, bubbletea v2, lipgloss v2, kitty 0.48
remote control, xdotool, ImageMagick `import`.

**Specs:** the pass 2 plan
(`docs/superpowers/archive/plans/2026-08-19-pass-2-design-language-and-shell.md`,
wireframes F1-F8 and decisions 1-13), the design language revision 3,
the pinned exemplar (`docs/poplar/design/2026-08-19-shell-exemplar/`),
the build machine design (`2026-07-27-poplar-build-machine.md`), and
the gate evidence this plan answers:
`docs/poplar/design/2026-08-21-pass-2-gate-captures/` (the auditor's
report `audit.md`, the 66 graded findings `findings.json`, the
manifest, and 14 decisive PNGs).

## Why this pass exists

Pass 2 closed with every mechanical gate green (159 gallery goldens,
the teatest flow suite, tmux-check) and its first minute in Geoff's
own kitty found a help screen he called a mess. None of the green
media had looked at a painted frame. The 2026-08-21 spike drove a
real kitty window through 108 captures (both themes, every rung
boundary, the pointer vocabulary, all fourteen sketch fixtures in
four profiles, the exemplar as reference) and a fresh-context
grading fan-out (7 Opus vision graders, 19 adversarial refuters, one
is-it-right auditor, 2.4M subagent tokens). The verdict: the live
shell is right (grounds byte-exact, placeholders centered, floor
clean, pointer working), the help screen is uncomposed, two degrade
defects sit in shipped code, and `cmd/sketch`, the sole source of
painted evidence for banner, toast, modal, and every degrade
profile, mis-renders composition on every frame.

This pass is sized at six tasks. A second task split proposes a pass
split per the standing rule.

## Global constraints

- `go-conventions`, `elm-conventions`, `bubbletea-design` before any
  Go; `tui-visual-verify` before any claim that a screen works.
- Every new or changed guard is proven falsifiable by experiment
  (revert the fix, observe red). Reviewers attack guards first.
- Gate evidence is a captured exit code from `make check`, never a
  tail.
- Captures that enter the design record are PNGs from the real kitty
  plus the `.ansi` dump beside each, labeled from the screen's own
  state, never from a key count.
- Single-key modifier-free bindings (ADR-0012). No new public surface
  beyond what a task names.
- Design choices Geoff has not ruled on are not made by an
  implementer; the rulings section below carries them, with a
  proposed default each so one sitting settles them.

## Rulings this pass needs from Geoff (one sitting)

Each carries a proposed default; a ruling is a number and a word.
Items 1-12 are the pass 2 gate items restated with what the captures
show (audit.md section 3 has the long form).

1. Kitty demo: ruled from the banked PNGs plus the re-shot set task 6
   produces. Default: accept the live shell as captured.
2. Modal wipe: unrulable until task 2 makes sketch faithful; ruled
   from task 6's capture. Default: the wipe stands (design language
   revision 3).
3. Chrome bands with no non-color channel in ANSI-16/NO_COLOR:
   confirmed flat. Default: position-only stands for pass 2; revisit
   when a banner meets ANSI-16 in pass 3.
4. `NO_COLOR=` empty enables color: default stands (no-color.org).
5. codeBg ground step: no pass-2 surface renders code; default
   deferred to pass 3's reader.
6. Profile conflates depth with Unicode: evidenced (`+---+` frame,
   `x` for `✕`, `-` for `·` colliding with `1-4`). Default: split
   Unicode detection from color depth, routed to BACKLOG for pass 3.
7. `readerContentCap=100`: no evidence possible; default stands.
8. Sync copy sentence case: default stands.
9. Help ground merges with the chrome bands: confirmed, both themes.
   Default: the overlay's content paints on base, chrome bands stay
   panel, like every other screen (task 4).
10. Copy: ASCII `x` in the floor notice (default: keep `×` per F4,
    with the ASCII fallback only under NO_COLOR's glyph table);
    `quit (at a surface root)` restored in help (default: restore);
    footer verbs `navigate`/`back` versus F5's `move`/`close`
    (default: F5's).
11. Vale possessive-own rule: default adopt, as a warning.
12. Ratification list: default ratify as recorded, with the
    exception that `? help` on the help screen reads `? close`
    (audit item 12; default: change it).

Auditor questions, new:

13. Help two-column trigger. Width alone strands a one-row section
    at 150 columns. Default: two columns when width is 100+ AND both
    sections carry more than one row; otherwise one column.
14. Help duplicate `q quit` under This screen for placeholders.
    Default: omit This screen when its entries are all already in
    Global; show it only when a screen adds a key.
15. Key-name case in help. Default: the exemplar's (`Enter`, `Esc`,
    `Space/b`, `Home/End`, `1-4`).
16. Help hints that are no-ops when the content fits (`j/k`,
    `Space/b` in footer and body). Default: advertise the navigation
    family only when the help is taller than the region (UX-2's
    no-advertised-no-op, already a mechanical guard).
17. Cluster spacing when the active surface sits mid-run or last
    (`1 2 Calendar  3 4`, `1 2 3 4 Config`). Default: symmetric
    two-cell gaps on both sides of the named surface.
18. `0 events` on Calendar states a fact about a store that does not
    exist. Default: `Calendar sync is not available yet.`, the
    People/Config register.
19. Launch and quit scrollback residue (`checking store
    integrity...` lines printed before the alt screen). Default: the
    lines go to the log, and a startup that takes longer than one
    second paints them inside the alt screen instead.
20. Sync signal: the live shell shows only `Syncing` with a braille
    spinner that is a two-pixel speck at 11pt, no progress, which is
    indistinguishable from a hang. Default: the status shows `Syncing
    4,312 of 36,102` whenever the engine reports a total (decision
    7), and the spinner glyph set moves to a larger-ink set; the
    engine side is pass 3's if the total is not already on the
    bridge.
21. fgSubtle carries the sibling digits (the exemplar) while decision
    6 says it never carries reading text. Default: amend decision 6
    to name the cluster digits as the second sanctioned fgSubtle
    use; the exemplar stands.
22. ANSI-16 muted/subtle render faint on top of bright black
    (1.29:1, invisible). Default: drop `Faint` at ANSI-16 where the
    slot already dims; keep it at NO_COLOR where it is the only
    channel (task 3).

## File structure

- `scripts/kitty-shot`, `scripts/gate-captures`: the harness and the
  matrix (new, landed by task 1).
- `docs/poplar/design/2026-08-21-pass-2-gate-captures/`: the design
  record (exists; task 6 adds the re-shot set).
- `internal/ui/app.go`: `App.View` delegates composition to a new
  `composeView` that `cmd/sketch` also calls (task 2).
- `cmd/sketch/main.go`: loses its private `ui.Render` call and its
  stale-frame residue (task 2).
- `internal/theme/theme.go`, `internal/ui/confirm.go`: the two
  degrade fixes (task 3).
- `internal/ui/help.go`, `internal/ui/statusline.go`,
  `internal/ui/placeholder.go`, `cmd/poplar/`: the help
  recomposition and the ruling-driven fixes (tasks 4, 5).
- `internal/ui/testdata/gallery/`: regenerated where a task changes a
  frame, in the task's own commit.

## Task 1: The harness as gate tooling

**Deliverables:** 3 (the two scripts committed with their fixes, the
design-record README, the build-machine amendment).

**Outcome:** `scripts/kitty-shot` and `scripts/gate-captures` are
the repo's tier-4 gate medium, documented where the other tiers
are, and their two known distortions are gone.

**Acceptance criteria:**
- `kitty-shot` launches kitty with `background_opacity=1.0` as well
  as `window_padding_width=0`, so the light base ground composites
  at its ratified value (audit item H measured 217,219,224 against
  228,231,236 under the owner's 0.95 opacity).
- `gate-captures` labels every capture from the screen (the app's
  status row or the floor notice), fails the run on a mismatch with
  the manifest, and names rung pairs only where design language
  section 9 has a boundary (the `80/79` label is removed; the pairs
  are 60/59, 100/99, 140/139, 15/14, 20/19).
- The matrix adds a help capture in a window short enough that help
  exceeds the region, then one wheel notch, so the wheel claim can
  fail; and a light-ground ANSI-16 and NO_COLOR set (kitty light,
  sketch profiles), which the pack lacked entirely.
- The design record directory gets a README naming the method, the
  matrix, and how to re-shoot; the build machine design gains a
  tier-4 paragraph pointing at it and the `tui-visual-verify` skill.
- `make check` is unaffected (the harness needs a display and an
  account; it runs on the gate platform by hand, like tmux-check).

**Boundaries:** the two scripts, the README, one paragraph in the
build machine doc. No Go.

- [ ] Task 1 complete

## Task 2: One composition path for App.View and cmd/sketch

**Deliverables:** 3 (`composeView`, sketch on it, the equality
guard).

**Outcome:** Whatever sketch renders is what the app renders, by
construction, so sketch is valid painted evidence for the states the
live app cannot reach on demand (banner, toast, modal, offline,
backing-off, every degrade profile).

**Acceptance criteria:**
- `App.View`'s composition (floor notice, placeholder FullRegion via
  `isPlaceholderScreen`, the stack-top FullRegion, the modal's
  full-shell wipe) moves into one function taking the layout, theme,
  status line, banner, active screen, and stack, returning the
  `Frame`; `App.View` only adds `MouseMode`, `AltScreen`, and the
  cursor. `cmd/sketch` calls the same function with the fixture's
  state and no other render call exists in `cmd/sketch`.
- A guard proves equality: for every fixture in `sketchFixtures`, an
  `App` seeded with that fixture's state renders a frame
  byte-identical to sketch's; the test is shown red by reverting
  sketch to a direct `ui.Render` call.
- Sketch clears the whole window on every frame (no stale larger
  frame behind a smaller rung; SK-3's residue), verified by a
  capture at rung 100x30 after rung 150x26 through `kitty-shot`.
- The four sketch captures the auditor cited (`mail`, `help`,
  `modal-confirm`, `mail-nocolor`) re-shot through `kitty-shot` show
  no rail, no divider, a centered placeholder, and the modal's
  full-shell wipe; the PNGs join the design record.
- Gallery goldens do not change (the seam moved, the frames did not);
  the diff proves it.

**Boundaries:** `internal/ui/app.go` (extract only), a new
`internal/ui/compose.go` if the function earns its own file,
`cmd/sketch/main.go`, tests. Docs: the sketch package comment loses
its "modal and help are out of scope" waiver.

- [ ] Task 2 complete

## Task 3: The two degrade defects

**Deliverables:** 2.

**Outcome:** ANSI-16 keeps muted text legible and renders a selected
pill as one plate.

**Acceptance criteria:**
- `theme.Style` no longer applies `Faint` at `ProfileANSI16` for
  `RoleFgMuted`/`RoleFgSubtle` (slot 8 already dims); NO_COLOR keeps
  `Faint` as its only channel. A test asserts the SGR of a muted
  style per profile (faint present at NO_COLOR, absent at ANSI-16
  and truecolor), red on revert.
- `confirmAnswerRow` renders the default-answer pill as one styled
  run (key and label together on `GroundSelected`, a single
  foreground), so reverse video at ANSI-16 cannot split it; a test
  asserts the rendered pill carries exactly one SGR open and one
  close around the whole `  n stay  ` run, red on revert. The
  truecolor pill is visually unchanged (gallery goldens for
  `modal-confirm-*-truecolor-*` unchanged, `-ansi16` regenerated and
  read).
- Both fixes captured through `kitty-shot` at ANSI-16 and read by a
  reviewer who did not write them.

**Boundaries:** `internal/theme/theme.go`, `internal/ui/confirm.go`,
tests, two regenerated goldens.

- [ ] Task 3 complete

## Task 4: The help screen, recomposed

**Deliverables:** 4 (revised wireframe ratified, the overlay, its
goldens and captures, the registry hint gating). This is the pass's
design ritual: the wireframe is reviewed by Geoff before any code.

**Outcome:** Help reads as a composed screen at 80, 120, and 150
columns, per the revised F5/F6 Geoff ratifies, against rulings 9,
10, 12, 13, 14, 15, 16, 21.

**Acceptance criteria:**
- Wireframe first: revised F5 (80x24) and F6 (120x36 and 150x45)
  text frames, drawn in the terminal medium from the exemplar,
  showing the ground step (content on base, bands on panel), the
  header gutter, the two-column trigger, the shared key column,
  restored copy, key-name case, and the legal-hint footer; reviewed
  by Geoff (one interaction point) before dispatch.
- The overlay paints its content region on `GroundBase` with the
  status line and footer on `GroundPanel`, measured in a kitty
  capture at both themes (pixels sampled, not the `.ansi`).
- Two columns only under ruling 13's condition; This screen omitted
  under ruling 14's; one key-column width shared across sections;
  key names per ruling 15; `quit (at a surface root)` restored; the
  footer pin reads `? close` while help is front (ruling 12).
- Navigation hints (footer and body) appear only when the help is
  taller than its region; the existing no-advertised-no-op guard
  covers it and a new test shows a short window advertising `j/k`
  and a tall one not, red on revert.
- Goldens for help at 80x24, 99x30, 100x30, 120x36, 150x45 in all
  four profiles regenerated and read; the `kitty-shot` captures at
  80x24 and 150x45, both themes, join the design record and are
  graded by a fresh reviewer against the ratified frames.

**Boundaries:** `internal/ui/help.go`, `internal/ui/footer.go` only
for the pin label, tests, goldens, the plan's wireframe section
(appended to this plan as "F5 revision 2" and "F6 revision 2").

- [ ] Task 4 complete

## Task 5: Ruling-driven shell fixes

**Deliverables:** 4 (cluster gaps, placeholder copy, scrollback
residue, the sync signal's status half). Each lands only as Geoff
ruled; a default he overrules is dropped from this task and noted in
the ledger.

**Outcome:** The small things the auditor said his eye would catch
are gone before the gate re-run.

**Acceptance criteria:**
- Cluster gaps symmetric around the named surface at every position
  (ruling 17); the status-line goldens for calendar, contacts, and
  config regenerated; a test asserts equal gaps either side of the
  active name for all four surfaces.
- Calendar placeholder copy per ruling 18; goldens regenerated.
- No line reaches stdout before the alt screen (ruling 19): the
  recovery `report` lines log instead, and a `kitty-shot` capture
  after `q` shows only the shell prompt.
- Status line renders `Syncing N of M` whenever the bridge carries a
  total (ruling 20); if the pass-1b engine does not expose the total,
  the task records that in the ledger and routes the engine half to
  pass 3 rather than inventing it. The spinner glyph set changes
  only if ruling 20 says so.
- Design language amendment per ruling 21 (decision 6's fgSubtle
  sentence) and the ASCII-`x` outcome of ruling 10, both in the doc
  with revision 4 noted.

**Boundaries:** `internal/ui/statusline.go`, `internal/ui/placeholder.go`,
`internal/store/recovery.go`'s report path (or `cmd/poplar`'s
caller), the design language doc, tests, goldens.

- [ ] Task 5 complete

## Task 6: The gate, re-run

**Deliverables:** 2 (the re-shot matrix with its grading, the gate
sitting).

**Outcome:** The pass 2 gate is ruled on painted evidence.

**Acceptance criteria:**
- `scripts/gate-captures` re-run at HEAD on the gate platform; the
  grading fan-out re-run as a workflow (fresh graders, refuters, one
  auditor; the 2026-08-21 script is the template) with the rule that
  a STRUCTURAL finding against shipped code blocks the sitting.
- The auditor's report and the decisive PNGs replace the
  2026-08-21 set in the design record (the old set stays in git
  history; the README names both runs).
- The main loop reads the help, modal, toast, banner, and floor PNGs
  itself before the sitting (the one-check rule).
- The sitting with Geoff: the 22 rulings above that are still open,
  the kitty demo by his own hand if he wants it, then the pass 2 and
  2c gates close together; the STATUS records the rulings, the
  ledgers are deleted per the standing instruction, and pass 2b's
  starter prompt stands as the next action.

**Boundaries:** no code; captures, the workflow, docs, STATUS.

- [ ] Task 6 complete

## Pass-end consolidation (standing)

Simplify sweep over the Go this pass touched, the reviewer fan-out
(poplar-reviewer, poplar-go-reviewer, prose-voice-reviewer on the
docs), `make check` with a captured exit code, `deadcode ./...`
recorded, STATUS outcomes block, plan archival, budget and
interaction-point tally (expected interaction points: the ruling
sitting, the help wireframe review, the gate sitting).

## Non-goals, restated as decisions

- No live banner/toast/modal triggers are added to the app for the
  harness's sake; task 2 makes sketch faithful instead, which is the
  cheaper honest path and keeps dev-only surface out of the binary.
- No VHS pipeline this pass; kitty is the gate platform and the
  truthful medium. VHS stays the documented CI option for docs
  stills.
- Unicode-versus-color-depth profile split (ruling 6) is pass 3's.
- The sync engine's progress total, if absent, is pass 3's.

## Self-review record

Spec coverage: audit.md sections 2A through 2J each map to a task or
a ruling (A: task 4; B: task 2; C: noted for the sitting, pass 3
fills the bands; D: ruling 17/task 5; E: ruling 20/task 5; F: no
action, the counts work; G: ruling 18/task 5; H: task 1; I: task 1;
J: ruling 19/task 5). The six surviving structural findings map to
tasks 2 (SK-1, L1, L2, DG-9) and 3 (DG-3, DG-4). Placeholder scan:
no TBDs; every acceptance criterion names its observable. Type
consistency: `composeView` is the only new name and appears in tasks
2 and the file structure only.
