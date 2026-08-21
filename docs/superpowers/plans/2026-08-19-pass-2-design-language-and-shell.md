# Pass 2: Design Language and Shell

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development to implement this plan
> task-by-task. Tasks use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the design language real and boot the application
shell: the compiled theme, the capability resolver, the responsive
layout machinery, the screen registry with registry-derived footer
and help, the shell chrome (status line, footer, toasts, banners,
modal confirm), the pointer vocabulary's shell rows, and a
`cmd/poplar` that runs the TUI against the engines pass 1b wired.

**Architecture:** bubbletea v2's one root model (ADR-0011) over the
existing headless engines. `internal/theme` compiles every visual
decision; `internal/ui` owns the root model, registry, chrome, and
placeholder surfaces; `cmd/poplar` bridges engine state into the tea
program as messages, because `ui` may not import `sync`, `backend`,
or `outbox` (technical design section 2). Layout is pure formulas
computed once per `WindowSizeMsg` into one `LayoutMode` struct.

**Tech Stack:** Go 1.26, charm.land/bubbletea/v2 v2.0.9,
charm.land/bubbles/v2 v2.1.1, charm.land/lipgloss/v2 v2.0.6,
x/exp/golden as the golden library over one pure render seam,
teatest/v2 demoted to a small flow suite (both ride x/exp
pseudo-versions), charmbracelet/freeze as an on-demand CLI for
color-fidelity stills. Versions verified live 2026-08-19; bubbletea
v2.0.9 and lipgloss v2.0.6 are the pass-start patch bumps over the
build machine's pins (both patch-only: keyboard-stack restore on
exit, renderer artifact fix, width measurement improvements).
glamour v2 waits for pass 3 (the reader); gofrs/flock stays at
v0.12.1 (v0.13.0 is a minor bump nothing needs).

**Specs and inputs:**
- Pass cursor: `docs/superpowers/specs/poplar-refounding-STATUS.md`
  (pass 2 section; this plan implements it). The 1c gate rulings are
  recorded there; none touches this pass's scope.
- The design language,
  `docs/superpowers/specs/2026-07-27-poplar-design-language.md`
  (revision 2): the grammar, footer, undo presentation, component
  vocabulary, theme tokens, and section 9's responsive grammar. This
  plan's wireframes cite it; its amendments (decision 3) are task 1
  deliverables.
- ADR-0011 (root model, registry, LayoutMode rectangles), ADR-0012
  (input model), ADR-0017 (pointer vocabulary), ADR-0013 (error
  seam), requirements revision 5 (UX-1..7, UX-9, ST-2, SY-5, ER-1/3,
  QA-7, QA-10).
- Build machine sections 5 (pass structure), 6 (verification
  harness), 7 (pins), 8 (input machinery).
- The design-iteration survey
  (`docs/poplar/research/2026-07-28-tui-design-iteration-survey.md`),
  whose recommendation section addresses this plan by name. Its
  amendments A through G are folded here: one pure render seam with
  x/exp/golden on its output and teatest demoted to a small flow
  suite (A), the committed text gallery first with `cmd/sketch` as
  a thin wrapper over the same seam (B), three-tier size
  verification (C), fixtures as Go values with pinned
  clock/TZ/IDs (D), no VHS and nothing recorded this pass (E),
  freeze for color-fidelity stills (F), and the golden churn
  policy (G).
- Routed input: dispositions doc row 24
  (`docs/poplar/research/2026-08-18-pass-1-deferred-findings-dispositions.md`),
  uerr's stderr log fallback must be resolved before a full-screen
  TUI renders. Task 11 owns it.
- Routed input: ADR-0005's self-echo suppression waits for the first
  UI-driven mutation wiring. **No UI-driven mutation exists in this
  pass** (triage arrives with pass 3), so it stays routed, restated
  in this plan's non-goals so the omission is a decision.

## Scope ruling: the step-2 split (pass 2 / pass 2b)

The requirements' build-order step 2 assigns UX-1 through UX-5, UX-7
through UX-9, and ST-1 through ST-3 plus ST-5 to this pass. That is
two passes' work, and the sizing doctrine says to cut before the
burst rather than after. The cut, **ratified by Geoff at the
wireframe review (2026-08-19)**:

- **Pass 2 (this plan): the design language made real, and the
  shell.** Theme, resolver, layout, registry, root model, chrome,
  help, mouse, the TUI wired into `cmd/poplar`. Discharges UX-1
  through UX-5 and UX-7 (shell scope), UX-9's presentation
  machinery, ST-2, SY-5's rendering, QA-7's golden matrix.
- **Pass 2b (next plan): onboarding and config.** UX-8's text-entry
  model, the form component and focus helper, `internal/config` and
  the ST-3 surface, ST-1 first-run, ST-5 credential lifecycle.
  Pass 2b's screens get their wireframe review at its start.
  Pass 3 requires 2b (it needs a configured, authenticated client),
  so the spine's ordering intent survives the cut.

The cut point is clean: pass 2 ends with a bootable, themed,
mouse-aware shell whose text-entry surface count is zero, so UX-8
and everything downstream of a form lands together in 2b. Until 2b,
the live account keeps authenticating through the existing headless
config path (`~/.local/secrets` token), untouched by this pass.

## Global constraints

- One `poplar-implementer` per task; `poplar-reviewer` and
  `poplar-go-reviewer` in parallel on each diff. Reviewers attack
  each round's new guards first and prove revert-sensitivity by
  experiment. Gate evidence is a captured exit code, never a tail.
- `go-conventions` binds every Go file; `elm-conventions` and
  `bubbletea-design` bind every task touching `internal/ui` or
  `internal/theme`. `make check` is the floor for every task.
- The perf suite runs behind the `perf` build tag via `make perf`
  only, and never during implementer work.
- The UX-3 styling analyzer already gates: outside `internal/theme`
  and `internal/catkin`, no non-ASCII literal, no ANSI escape, no
  lipgloss call. Chrome and screens reach every glyph, color, and
  spacing value through `internal/theme`'s API. New
  `//poplar:allow-unicode` escapes are pass-end-reviewer reading.
- The import boundary holds: `internal/ui` imports `store` (read
  API), `theme`, `uerr`, and intent types only. Engine state reaches
  the UI as messages sent by `cmd/poplar`.
- Never set `run_in_background` on any Bash call. Unique names for
  every scratch file.
- Commits land on master: imperative mood, specific files,
  `Co-Authored-By: Claude <noreply@anthropic.com>`. Implementers do
  not push; the orchestrator pushes.
- Geoff is at the wireframe review (before any screen task
  dispatches) and the pass gate. The pass-gate demo runs on the gate
  terminal (kitty) and includes the manual pointer checklist:
  status-line digits, footer hints, banner dismiss, modal answers,
  wheel in help, exercised by hand (ADR-0017; no tooling injects
  terminal-level mouse).

## Design decisions this plan settles

Values the design language left to Phase 5, settled here so no
implementer invents taste. Each cites its constraint.

1. **Status line at the top edge; footer at the bottom; the
   compact-cluster top bar** (ruled by Geoff at the wireframe
   review, 2026-08-19). The top row's origin holds the surface
   cluster: the active surface named in accent + bold (`1 Mail`),
   siblings as bare dim digits (`2 3 4`); the rest of the left
   segment belongs to the active surface's context (pine's
   title-bar instinct: 98% of time is spent in mail, and the row
   is high-value space); sync state right. The cluster's segment
   divider aligns exactly with the sidebar's divider column
   whenever a sidebar is present, and is dropped when none is.
   Sibling surface names live in the help overlay and appear on
   visit. Toasts render in the status line's right segment, so no
   layout shifts for a transient notice (charter: calm).
2. **Banners are a row directly under the status line.** Persistent,
   non-focus-stealing, push content down one row while present.
   `Esc` dismisses the topmost banner before acting as back (one
   instinct, two depths, matching the design language's Esc
   reasoning); the banner states its key inline. Pointer: click the
   banner's dismiss glyph.
3. **Design-language amendments** (the doc allows amendment
   through itself; task 1 commits all of them, each citing this
   plan and the adversarial-review fold):
   - Section 6 status-line wording moves to the compact cluster
     (decision 1); footer exception 2's reason restates (digits
     advertised by the cluster; names in help and on visit).
   - Color roles: `bg` and `bgPanel` join the roles (`bgDeep` was
     tried and rejected: dark steps below base are illegible);
     `selectedBg` is defined as accent-tinted; `focusedBorder`
     repurposes as the degrade profiles' focused-divider color.
   - Glyph tokens join with ASCII fallbacks: `dismiss ✕` (x),
     `edgeBarFocused ▌` U+258C (>), `edgeBarBlurred ▏` U+258F
     (|), `separator ·` U+00B7 (-), `scrollPos ≡` U+2261 (=),
     `treeBranch ├─` and `treeLast └─` (|- and `+-`).
   - Section 7 degrade tables amend: focused is carried by the
     edge bar's glyph weight (▌ vs ▏), which survives NO_COLOR;
     and the ANSI-16, NO_COLOR, and text-gallery profiles
     substitute single-line pane dividers for the ground steps
     (the ladder is the truecolor expression, never the only
     one).
   - Spacing roles gain `padBand 2` (chrome band inset) and
     `padCard 2` (card inset); markup literals stay banned.
4. **Border sets:** single-line for pane dividers, rounded for
   modals and cards, heavy for the focused pane's border (the
   degrade tables already assign heavy to focus). Spinner frames:
   braille `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`, ASCII fallback `|/-\`.
5. **Contrast-role assignment:** `fg`, `fgMuted`, `unread`, `error`,
   `warn`, `success`, `link`, `quote`, `accent` are text roles
   (4.5:1); `fgSubtle`, `focusedBorder`, `flag`, `diffAdd`,
   `diffDel`, and `calendarSlot[*]` are indicator roles (3:1).
6. **The palette: cool slate, v3 values** (ruled by Geoff; the
   adversarial review's contrast findings folded). Grounds share a
   grammar: base carries content, panel elevates
   chrome and cards, selection is accent-tinted, code insets on
   its ratified value. Every role below is verified at 4.5:1
   (text) / 3:1 (indicator) against ALL FOUR grounds, both
   themes; the UX-7 test asserts exactly that matrix, and the
   theme task may nudge lightness only.

   | Role | Dark | Light |
   |---|---|---|
   | bg | `#16181D` | `#E4E7EC` |
   | bgPanel | `#262B36` | `#FFFFFF` |
   | selectedBg | `#2A3441` | `#C4CDDB` |
   | codeBg | `#1D2026` | `#D9DDE4` |
   | border | `#3A414D` (structural, deliberately sub-contrast, exempt from the role floors) | `#B4BCC9` |
   | fg | `#D4D8DF` | `#262B33` |
   | fgMuted | `#969DA9` | `#4C545F` |
   | fgSubtle | `#7A8290` | `#646D79` |
   | accent | `#85B3D1` | `#285370` |
   | unread | `#ECEEF2` (with bold) | `#12161C` (with bold) |
   | error | `#DF8484` | `#90342E` |
   | warn | `#D4B36A` | `#6A4E0A` |
   | success | `#97BE8C` | `#3A5A32` |
   | link | accent alias, underlined (both themes) | accent alias, underlined |
   | quote | `#A0A5AE` (italic, with the `│` bar) | `#4A525D` |
   | flag | warn alias | warn alias |
   | diffAdd / diffDel | success / error values | success / error values |
   | calendarSlot[8] | muted 8-hue cycle in the same saturation band: accent blue, success, warn, `#C99BC0`, `#7FBFB2`, error, `#B0A1E0`, `#C0A98F` | derived per-role at light lightness, same hue order |

   The v4 light grounds are deepened from the first light draft so
   the card's panel step survives without lines (the polish review
   measured the original at 1.10:1, near the perceptual floor).
      fgSubtle is an indicator role and never carries reading text:
   dates and tree connectors render in fgMuted; snippets are the
   one fgSubtle text use, deliberately sub-reading-weight
   (review finding 1's resolution).

7. **SY-5's four states, rendered:** `synced` (dim, fgMuted),
   `⠹ syncing 4,312/36,102` (spinner + progress where a total is
   known), `offline` (warn), `backing off · retry 12s` (warn).
   Backfill progress rides the same segment
   (`⠹ bodies 18,204/36,102`). A nonzero outbox shows `2 queued`
   beside the sync state. All through theme tokens.
8. **The footer is one prioritized row** (Geoff, 2026-08-19,
   superseding the same-day distribute-across-width ruling and the
   two-row form): each screen's registry entry carries a committed
   hint priority order; the footer renders the width-maximal
   prefix of that order with a three-cell gap, and `? help` is
   pinned right at every width as the pointer to the complete
   keymap. The help overlay is the completeness surface
   (requirements revision 6 amends UX-2 accordingly; the
   five-entry exception list retires). Width changes what is
   shown, never what is legal, matching the ladder principle.
9. **Placeholder surfaces state store facts, not apologies or
   roadmap talk.** Surface name plus live store counts, centered,
   composed (charter: empty states are composed). They are
   throwaway; pass 3 replaces the mail one.
10. **The prototyping and review medium is the terminal itself,
   never an HTML mock.** The design-iteration survey's field
   finding: no surveyed project prototypes TUI design outside the
   terminal; the loop is run-and-look plus committed text renders.
   Poplar's mechanism, from the survey: every screen renders
   through one pure seam, a gallery target sweeps fixtures ×
   profiles × sizes through that seam into committed text files
   (the artifact both Geoff and an agent can read), `cmd/sketch`
   wraps the same seam interactively for feeling a screen in
   kitty, and freeze turns gallery text into color-accurate stills
   where color is the subject (theme review). Because the styling
   analyzer bans styling outside `internal/theme`, the gallery
   cannot drift from the product: whatever it renders is what the
   app renders. Pre-build mockups need no tool, only ordering:
   theme, LayoutMode, and chrome land first, then approved
   wireframes are composed from real primitives with fixture
   content, and that composed render becomes the screen's first
   golden.

11. **The ground grammar** (v3, post-review): grounds step upward
   only. Base carries every content pane including the rail; the
   panel ground elevates chrome bands, the reader card, the help
   overlay, and modals; selection is the accent-tinted ground plus
   the edge bar. Pane separation is whitespace gutters and
   alignment, plus exactly one structural line (the rail/list
   divider in fgSubtle). Focus is carried by edge-bar glyph
   weight (`▌` focused, `▏` unfocused), never by border color
   alone, and never two active-state signals of equal weight at
   once. In degrade profiles the ground steps substitute to
   single-line dividers (decision 3's amendment).
12. **Mail list row anatomy** (binds pass 3's wireframes; folded
   from the review's GUI-switcher findings): edge bar, unread `●`,
   attribute column (`⚑` flag in the warm accent, attachment
   glyph dim), sender 17 cells, thread-count column (`▸ N`,
   threads collapsed by default per TH-3), subject in fg, then the
   `·` separator token and a snippet in fgSubtle italic (the
   emHint type role; Geoff, 2026-08-19: color alone left subject
   and snippet reading as one line) filling remaining width
   (Gmail's row anatomy; the review measured 32-42 dead cells
   without it), a guaranteed three-cell gap, then the date
   right-aligned in fgMuted (time today, `Mon D` otherwise), all
   truncation marked with the ellipsis token. The attachment
   glyph is ruled: `⊕` stays (Geoff, 2026-08-19), the design
   language's existing token, chosen over a letter or a count with
   the field-precedent finding on record.

13. **The pinned exemplar** (Geoff, 2026-08-19): composition v4
   is ratified and committed at
   `docs/poplar/design/2026-08-19-shell-exemplar/` (generator,
   ANSI render, stripped render, README). **It is the design
   language's visual exemplar: help, config, calendar, contacts,
   and compose all design from it** — its ground grammar, hint
   atom, edge-bar vocabulary, card anatomy, and copy register are
   the reference a new screen imitates before consulting rules.
   Task 1's design-language amendment adds the exemplar pointer to
   the design language's section 10. Both pin-time items are ruled
   (Geoff, 2026-08-19): the toast keeps the dim `9s` countdown,
   which satisfies UX-9's visible-countdown MUST as written, and
   the attachment glyph stays `⊕`.

## Wireframes (the design ritual)

One frame per screen per responsive class that changes its layout
(design language section 9's rule). **The review lens is the
standard rung first** (100-139 columns, 120 representative), per
Geoff's ruling and the design language's polish-center clause;
spartan 80 is a completeness check, not the design center. Frames
F9 and F10 carry representative pass-3 mail content, seeded from
the legacy client's refined layout (its cursor gutter bar, dim-read
rows, sidebar counts, thread connectors), because the shell's
chrome cannot be judged against empty placeholders; they bind the
shell's composition only, never pass 3's tasks. Pointer targets are
annotated per frame (ADR-0017). Glyphs shown are the truecolor
profile; degrade profiles substitute per the theme's tables.

Rulings folded from the wireframe review (Geoff, 2026-08-19): the
slate palette (decision 6); the compact-cluster top bar (decision
1); the wide rung stays list-beside-reader, with the line-length
evidence recorded at
`docs/poplar/research/2026-08-19-reading-pane-line-length-research.md`;
marker columns render with a separating cell (`● ⚑`, never `●⚑`);
the top bar's segment divider aligns with the sidebar's divider
column whenever a sidebar is present.

### F9. Standard rung 120, mail surface (pass-3 content preview)

```
  1 Mail  2 3 4          Inbox · 12,406 messages · 14 unread                                 ⠹ Syncing 4,312 of 36,102
                      │
▏ Inbox           14  │    ●    Ada Lovelace            Analytical engine notes · The engine weaves algebrai…    09:41
  Drafts              │  ▌      Charles Babbage         Re: difference engine budget · The Treasury insists …    09:12
  Sent                │    ● ⚑  Grace Hopper            COBOL committee minutes · Attached are the minutes f…    08:55
  Archive             │      ⊕  Katherine Johnson       Trajectory review, Friday · Can you check my numbers…   Aug 18
  Junk             1  │         Fastmail                Your receipt for August · Thanks for your payment. T…   Aug 17
  Trash               │    ●    Frank Lee           ▸ 4 Server migration plan · I propose we move the primar…   Aug 16
  Lists           98  │      ⊕  GitHub                  Code review: sync engine PR #42 · poplar/sync: 3 com…   Aug 14
                      │         Donald Knuth            TAOCP errata, volume 4 · A reader reports an off-by-…   Aug 13
                      │
                      │
                      │
```

- Ground grammar (v3, adversarial review folded, plus two Geoff
  iterations): chrome bands (top, footer), the reader card, and
  modals sit on the elevated panel ground; every content pane,
  the rail included, sits on base (dark ground steps below a
  near-black base are physically illegible, and a panel-ground
  rail spotlit the center pane). The rail/list boundary is one
  quiet vertical line in fgSubtle, the composition's single
  structural line (aerc's pattern), which is also the degrade
  profiles' pane channel, so every profile shares one bone
  structure.
- List rows: cursor = thick accent edge bar `▌` (thin `▏` when the
  pane is unfocused) plus the accent-tinted selection ground;
  unread = `●` plus bold bright; read rows dim; thread-collapsed
  by default with a `▸ N` count column (TH-3, LT-1); subject then
  a fgSubtle snippet fills the row (the GUI-mail anatomy); marked
  `…` truncation everywhere; dates right-aligned in fgMuted, time
  today and `Mon D` otherwise.
- Pointer targets: cluster digits, rail rows (goto), list rows
  (cursor; double-click opens), footer hints, wheel.

### F10. Wide rung 150, split (pass-3 content preview)

```
  1 Mail  2 3 4          Inbox · 12,406 messages · 14 unread                                                               ⠹ Syncing 4,312 of 36,102
                      │
▏ Inbox           14  │    ●    Ada Lovelace            Analytical engine n…    09:41
  Drafts              │  ▌      Charles Babbage         Re: difference engi…    09:12      Re: difference engine budget
  Sent                │    ● ⚑  Grace Hopper            COBOL committee min…    08:55
  Archive             │      ⊕  Katherine Johnson       Trajectory review, …   Aug 18      From   Charles Babbage <charles@difference.example>
  Junk             1  │         Fastmail                Your receipt for Au…   Aug 17      To     geoff@907.life
  Trash               │    ●    Frank Lee           ▸ 4 Server migration pl…   Aug 16      Date   Tue 19 Aug 2026, 09:12
  Lists           98  │      ⊕  GitHub                  Code review: sync e…   Aug 14      ───────────────────────────────────────────────────────
                      │         Donald Knuth            TAOCP errata, volum…   Aug 13
                      │                                                                    The Treasury insists on itemized brasswork
                      │                                                                    before any further disbursement. I enclose
                      │                                                                    the schedule of parts.
                      │
                      │                                                                    │ your estimate of the mill assembly
                      │
                      │                                                                    https://example.org/engine-budget
                      │
                      │                                                                      mill:   4,102 gears
                      │                                                                      store:  1,000 columns of 40 wheels
                      │
                      │                                                                    ⊕ parts-schedule.pdf · 96 KB
                      │                                                                                                                        34%
                      │
                      │
```

- List beside reader (the ratified wide rung, evidence note in
  the line-length research doc). The reader is a floating card on
  the panel ground with base-ground margins above and below and a
  base gutter each side (Geoff, 2026-08-19: the margins are
  deliberate and symmetric); the header block ends in a light
  fgSubtle rule inside the card (ruled over a second header
  ground, which would stack three surfaces in ten cells); a Date
  header row, a quote bar `▏`, underlined links, code inset on
  codeBg, and a `≡ 34%` scroll readout (the field's converged
  alternative to a drawn scrollbar).
- Pointer targets add: click a pane to focus it, drag in the
  reader (SHOULD, copy mode).

### F11. Spartan 80, mail list single pane (pass-3 content preview)

```
  1 Mail  2 3 4                                                  ⠹ Syncing 12%

  ●    Ada Lovelace            Analytical engine notes · The engine …    09:41
▌      Charles Babbage         Re: difference engine budget · The Tr…    09:12
  ● ⚑  Grace Hopper            COBOL committee minutes · Attached ar…    08:55
    ⊕  Katherine Johnson       Trajectory review, Friday · Can you c…   Aug 18
       Fastmail                Your receipt for August · Thanks for …   Aug 17
  ●    Frank Lee           ▸ 4 Server migration plan · I propose we …   Aug 16
    ⊕  GitHub                  Code review: sync engine PR #42 · pop…   Aug 14
       Donald Knuth            TAOCP errata, volume 4 · A reader rep…   Aug 13
```

- The composition must stand without the rail: chrome bands,
  selection ground, edge bar, and marked truncation carry it
  (review finding: the 80-column case was previously untested).
  Footer rows compress to key-only before dropping anything.

### F1. Shell frame, mail placeholder, spartan 80×24 (completeness check)

```
 1 Mail  2 3 4                                           ⠹ syncing 4,312/36,102
────────────────────────────────────────────────────────────────────────────────


                                     Mail

                        36,102 messages in 14 folders

                               synced just now


────────────────────────────────────────────────────────────────────────────────
 q quit   ? help
```

- With no sidebar present (placeholder, and any spartan screen),
  the top bar drops the segment divider; the cluster keeps its
  origin position.
- The footer advertises exactly the placeholder's legal keys: `q`
  (surface root) and `?`. Digits are exception 2; `j/k` and
  friends are not legal here (nothing scrolls).
- Pointer targets: the cluster's digit cells, footer hints.

### F2. Status line with an undo toast, plus a banner, standard 120

```
  1 Mail  2 3 4          Inbox · 12,406 messages                                      3 messages archived   u undo  9s
```

- The toast occupies the status line's right segment for its 10 s
  window, countdown visible (UX-9); the sync state compresses to
  its bare word beside it and yields entirely below 100 columns.
  Newest toast wins; every toast also logs through the ER-1 seam.
- The banner row (warn `!` gutter, message, key hint, dismiss
  glyph) pushes content down one row. `Esc` dismisses it first;
  click on `✕` is the pointer path. It never steals focus.

### F3. Short-height compression, 120×18

```
 1 Mail  2 3 4      │ Inbox · 3 of 12,406 · 14 unread                                                    ⠹ syncing 4,312
────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
 (content)
────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
 q quit   ? help
```

- Under 20 rows: the footer holds one row (screens with more verbs
  compress descriptions before dropping any hint), banners demote
  to toasts, the sync segment drops its total. Chrome compresses
  before content (composition rule 1).

### F4. Floor state, 48×12

```

          poplar needs at least 60×15
             this window is 48×12

```

- Centered, nothing else, never garbled chrome. Below 60 columns
  or below 15 rows. Its own golden.

### F5. Help overlay, spartan 80×24 (one column)

```
 1 Mail  2 3 4                                                            synced
────────────────────────────────────────────────────────────────────────────────
 Help · Mail

 Global
   j / k          down / up
   Space / b      page forward / back
   Home / End     first / last (G works too)
   Enter          open
   Esc            back · dismiss banner
   1 2 3 4        mail · calendar · people · config
   u              undo
   ?              help
   q              quit (at a surface root)

 This screen
   q              quit
────────────────────────────────────────────────────────────────────────────────
 j/k move   Space/b page   Esc close
```

- A full-content-region overlay on the screen stack (Esc pops),
  registry-derived (UX-5); the surface-digit names live here (the
  cluster shows bare digits, so help is where siblings are named,
  along with the folder-jump capitals from pass 3 on).
- Scrollable when taller than the region; wheel scrolls.

### F6. Help overlay, standard-and-up (two columns)

As F5 with the Global and This-screen sections side by side at 100
columns and up (the one layout boundary any pass-2 screen
consumes). Within a column, rows never wrap.

### F7. Modal confirm (quit with pending outbox), natural size, centered

```
                                     ╭────────────────────────────────────────────╮
                                     │                                            │
                                     │  Quit with 2 unsent messages?              │
                                     │  They'll send the next time you open poplar.│
                                     │                                            │
                                     │            y quit     n stay               │
                                     │                                            │
```

- One question, named consequence, `y`/`n`/`Esc` (grammar-exempt
  context 1). Rounded border set, padModal 2/1, clamped and
  centered. The dimmed screen behind it stays rendered.
- Pointer targets: the answer cells. Everything else is a no-op,
  exactly as keys.

### F8. Calendar / People / Config placeholders

Same composition as F1's content block, per surface:
`Calendar — 5,112 events, synced` / `People — contacts sync lands
with pass 5` (contacts count once the store row exists) /
`Config — the config surface lands with pass 2b`. Each is a
registered screen so UX-4's round trip and the grammar test cover
all four surfaces from this pass on. Layout identical at every
rung; floor and short-height behavior inherited from the shell.


## File structure

```
internal/theme/            theme.go (roles, profiles), glyphs.go,
                           spacing.go, contrast_test.go, golden
                           degrade tables
internal/ui/               app.go (root model), layout.go
                           (LayoutMode), registry.go (Screen,
                           entries, tests), render.go (the pure
                           seam), statusline.go, footer.go,
                           banner.go, toast.go, confirm.go,
                           help.go, placeholder.go, mouse.go
                           (dispatch, hit spans), messages.go
                           (engine-state msg types), wheel.go
                           (the WithFilter coalescer)
internal/ui/fixtures/      named fixture states as exported Go
                           values, clock/TZ/IDs pinned in-package
internal/ui/testdata/      x/exp/golden files and the committed
                           gallery renders
cmd/sketch/                dev-only interactive gallery viewer, a
                           thin bubbletea wrapper over the seam;
                           never in a release artifact
cmd/poplar/                main.go grows the TUI default path and
                           the message bridge; headless mode stays
                           behind --headless for the harnesses
internal/uerr/             log fallback rework (task 11)
```

---

## Task 1: Theme package and the token amendments

**Requirement IDs:** UX-3 (theme half), UX-7 (contrast criteria).
**Deliverables:** 4 (theme package, contrast test, two
design-language amendments).

**Outcome:** `internal/theme` compiles every design-language token
as Go values: color roles as functions of `isDark` and profile
(truecolor / ANSI-16 / NO_COLOR), glyph tokens with per-profile
fallbacks, spacing roles, type roles, border sets, spinner frames,
using the palette table above. The design language gains the `bg`
color role and the `dismiss ✕` glyph token via committed amendment
edits citing this plan.

**Acceptance criteria:**
- A contrast test computes WCAG ratios over the compiled values
  for both themes: every text role at or above 4.5:1 and every
  indicator role at or above 3:1 against ALL FOUR grounds (bg,
  bgPanel, selectedBg, codeBg), per decisions 5 and 6. Fails on
  any miss.
- The style API is ground-parameterized (crush's Focused/Blurred
  pairing generalized): a role resolves against the ground it
  renders on, so no caller can pair a role with an unverified
  ground. The degrade substitution tables (decision 3) compile
  here: ANSI-16 and NO_COLOR profiles carry the divider set and
  the edge-bar weight channel.
- Degrade-table test: under ANSI-16 and NO_COLOR profiles, unread,
  selected, focused, and error resolve to the distinct non-color
  channels the design language names (marker, reverse, heavy
  border, `!` gutter + reverse); no two share a channel.
- Every glyph token has an ASCII fallback and the NO_COLOR/ANSI-16
  profiles serve it; a test walks the glyph set.
- `make check` green; the styling analyzer stays silent because the
  package is its exemption.

**Boundaries:** `internal/theme` only, plus the two design-language
amendment edits. No lipgloss `Style` construction outside what the
role API exposes; the package exposes styles and values, never raw
hex to callers. Contract: theme API consumed by tasks 4-10 as
`theme.New(isDark bool, p Profile) Theme` with role accessors and
`theme.Glyphs`, `theme.Spacing` value sets (exact shape is the
implementer's, names below are the produced interface).

**Produces:** `theme.Profile` (`ProfileTrueColor`, `ProfileANSI16`,
`ProfileNoColor`), `theme.New(isDark, profile) Theme`, `Theme`
role-accessor methods returning lipgloss styles/colors, `GlyphSet`
via `Theme.Glyphs()`, spacing constants (`GapLabel=1`,
`GapControl=2`, `Gutter=1`, `PadPane=1`, `PadModalX=2`,
`PadModalY=1`, `GapSection=1`).

**Salvage:** crush's three-layer token architecture, verified from
source 2026-08-19 (semantic token opts a la quickstyle, theme
functions, one central Styles struct; padding never margin; paired
Focused/Blurred styles) is the shape exemplar, per the
pane-background research doc part 2. The legacy theme predates the
token grammar; the palette is new.

- [ ] Task 1 complete

## Task 2: Capability profile resolver

**Requirement IDs:** UX-3 (runtime resolution), QA-7 (profile as
input). **Deliverables:** 2 (resolver, first-frame policy).

**Outcome:** The runtime profile resolves from `NO_COLOR`, `TERM`,
`COLORTERM`, and bubbletea's background-color query, with a config
override seam (the override value arrives with 2b; the seam takes a
functional option now). The first frame never waits on the
terminal: a 100 ms bounded wait for `tea.BackgroundColorMsg`, then
default dark renders and a later answer repaints.

**Acceptance criteria:**
- Table test over env combinations: `NO_COLOR=1` wins; `TERM=dumb`
  degrades; `COLORTERM=truecolor` upgrades; defaults are recorded
  in the table, not sniffed in tests (QA-7's rule).
- The repaint path is a golden input: a golden pair (pre-answer
  dark, post-answer light) proves the repaint is deterministic,
  not a flash bug.
- The 100 ms bound is a compiled constant with a test that a
  never-answering terminal still renders (teatest, no background
  message sent).

**Boundaries:** `internal/ui` (resolver consumed at program
construction in `cmd/poplar`, task 11). Must not block `Init`.

**Produces:** `ui.ResolveProfile(env, override) (theme.Profile,
isDark bool)` shape plus the `tea.BackgroundColorMsg` handling in
the root model (delivered as part of app.go's message routing,
consumed by task 5).

**Salvage:** none.

- [ ] Task 2 complete

## Task 3: LayoutMode

**Requirement IDs:** design language section 9 (the responsive
grammar), ADR-0011 revision 3 (pane rectangles). **Deliverables:**
1 (the layout package surface with its tests).

**Outcome:** `ui.ComputeLayout(width, height int) LayoutMode`: pure
formulas mapping a terminal size to width class, gutter and
divider columns (decision 11: the rail/list divider column, the
2-cell card gutters), and a ground per pane rectangle (every cell
carries a ground; unpainted cells are a defect), (floor / spartan /
standard / wide at <60 / 60-99 / 100-139 / 140+), height class
(floor <15 / short 15-19 / full 20+), chrome allocation (status
row, banner row when present, footer rows per height class), the
content rectangle, and per-pane `image.Rectangle`s for the pane set
a screen declares (this pass: the single content pane; the sidebar
and split slots exist in the type with the drop-priority order
encoded, unconsumed until pass 3).

**Acceptance criteria:**
- Boundary tests at every threshold ±1 (59/60, 99/100, 139/140
  columns; 14/15, 19/20 rows), asserting class and the one thing
  each boundary changes (section 9's one-change rule).
- Pane-drop test: given panes with declared minimums, the drop
  order is split first, then sidebar; no pane is ever returned
  below its minimum (composition rule 1).
- Pure function: no I/O, no globals; property test that the same
  input always yields the same struct.

**Boundaries:** `internal/ui/layout.go` + tests only.

**Produces:** `type LayoutMode struct` with `Width, Height int`,
`Class WidthClass`, `HeightClass HeightClass`,
`Content image.Rectangle`, `Panes map[PaneID]image.Rectangle`,
`FooterRows int`, `BannerRow bool` (exact field names may be
refined; the consuming tasks 5-10 read class, rectangles, and
footer rows).

**Salvage:** the legacy responsive model
(`docs/poplar/research/2026-07-27-poplar-responsive-design.md`) is
reference for formula shapes; the classes and thresholds come from
the design language, not from it.

- [ ] Task 3 complete

## Task 4: Screen registry and the conformance tests

**Requirement IDs:** UX-1, UX-2 (test machinery), UX-4 (test), C4
(AccountScoped). **Deliverables:** 4 (registry, grammar test,
switch-table test, AccountScoped).

**Outcome:** The `Screen` interface and package-level registry
(ADR-0011): a screen registers at init with its keymap
(`bubbles/key` bindings implementing `help.KeyMap`), its pointer
targets (verb + target kind, per ADR-0017), and its UX-4 state
class (digits-switch, printable-entry, or modal). The footer, help
overlay, grammar test, and switch-table test all derive from the
same entries. `AccountScoped[T]` wraps account-keyed UI state.

**Acceptance criteria:**
- Reflection test: any screen type in `internal/ui/...` not
  registered fails the build's test step (UX-1 acceptance).
- Grammar test: iterates the registry, fails on any binding that
  contradicts the design language's verb tables; knows exactly the
  two named exemptions (modal confirm, Catkin command state) and
  fails on a third.
- Switch-table test: every registered screen resolves to a UX-4
  state; the digits-switch set and the printable-entry set match
  the design language's lists; the entry set equals the set of
  screens accepting printable input (empty this pass, asserted
  empty, the assertion goes live with 2b's forms).
- Pointer-grammar test: no registered pointer target names a verb
  absent or illegal in its state (ADR-0017's registry clause).
- UX-2 machinery: a registry-driven check that a screen's rendered
  footer hint set equals its advertised keymap minus the five
  documented exceptions; fails on an undocumented exception.
  Consumed by task 7's footer and every screen task from pass 3 on.

**Boundaries:** `internal/ui/registry.go` + tests. The keymap data
model is `bubbles/key`; no string-switch key dispatch anywhere
(elm-conventions rule 7).

**Produces:** `type Screen interface` (tea model composition +
`Entry() ScreenEntry`), `ui.Register(ScreenEntry)`,
`type ScreenEntry struct` carrying `Keys` (a `help.KeyMap`
implementer), `Pointer []PointerBinding`, `SwitchState StateClass`,
`type AccountScoped[T]`. Tasks 5-10 register everything they build.

**Salvage:** crush's registry-derived help pattern (catalogued in
the tooling audit) as the shape exemplar; no code copies.

- [ ] Task 4 complete

## Task 5: Root model, the render seam, placeholders, the gallery

**Requirement IDs:** UX-1 (shared list-nav test seeds), UX-4
(round trip), QA-7 (seam and gallery). **Deliverables:** 6 (root
model, pure render seam, fixtures package, gallery target, four
placeholder screens, wheel filter).

**Outcome:** `internal/ui.App`: the root model owning the active
surface, the screen stack (help and modals push onto it),
account-keyed UI state via `AccountScoped`, `WindowSizeMsg` →
`ComputeLayout` → one `LayoutMode` every child consumes, and digit
surface switching with state preserved per surface. Four
placeholder surface screens per wireframe F1/F8, reading live
counts from the store's read pool as commands (elm-conventions
rule 3). The wheel-coalescing `tea.WithFilter` (16 ms window,
signed accumulation, direction reset) at program construction.
The render seam (survey amendment A): every static render flows
through one pure function of the shape
`Render(screen, state, LayoutMode, theme) string`, no program, no
I/O; static goldens call `x/exp/golden.RequireEqual` on its
output, profile and size always explicit inputs. The fixtures
package (amendment D): named screen states as exported Go values,
clock, timezone, and IDs pinned in-package. The gallery target
(amendment B): `make gallery` sweeps fixtures × profiles × rungs
and boundary sizes through the seam into committed text files
under `internal/ui/testdata/`, regenerated by the same flag
convention as the goldens; where the gallery covers a case, no
separate golden duplicates it.

**Acceptance criteria:**
- UX-4 round trip: switch 1→3→1 preserves surface state
  byte-for-byte (cursor/scroll once those exist; this pass asserts
  the placeholder's scroll-free state and the mechanism), scripted
  keystroke test.
- Digits switch only in digits-switch states: a modal on the stack
  eats digits as answers/no-ops, asserted.
- `Esc` pops the stack; at a surface root it is a no-op this pass
  (quit is `q`; banner-dismiss precedence lands in task 8's test).
- Wheel filter: a synthetic 30-tick burst inside the window reaches
  `Update` as one message with the summed delta; a direction flip
  resets; recorded as the elm-conventions exception it is (pure,
  stateless).
- Seam purity: two calls with the same fixture, LayoutMode, and
  theme return byte-identical strings (QA-7's core assertion, in
  one test); gallery renders of F1 at 80×24 and 100×30 in
  truecolor dark are committed and stable across regeneration.
- Resize preserves state: a 100×30 → 80×24 → 100×30 round trip
  leaves placeholder state intact (composition rule 4's mechanism,
  full coverage as content arrives).

**Boundaries:** `internal/ui` (app.go, render.go, placeholder.go,
wheel.go), `internal/ui/fixtures`, the gallery make target. Store
access through the read pool only; no engine imports. Placeholder
store reads go through one `tea.Cmd` per refresh, triggered by
store-changed messages (task 6's bridge defines the message; this
task stubs the trigger with Init-time load). Fixture content never
reaches the seam from a data file (Go values only), and the
styling analyzer covers the fixtures package like any other.

**Produces:** `ui.NewApp(deps) App` (deps: read pool handle, theme,
profile), `App` as the program's root model; `PlaceholderScreen`
registered per surface; `ui.Render(screen, state, LayoutMode,
theme) string` as the one static-render seam; `fixtures.<Name>`
values; `make gallery`. Consumed by every later task: chrome tasks
6 through 9 add their fixtures and gallery entries in the same
commit as their component.

**Salvage:** legacy `internal/ui/app.go` root-model shape is
copy-with-rewrite reference only; the registry and LayoutMode
mechanics are new.

- [ ] Task 5 complete

## Task 6: Status line and the engine-state bridge

**Requirement IDs:** SY-5 (rendering), C6 (indicator as chrome),
ER-3 (stale-sync honesty). **Deliverables:** 3 (status line
component, message types, cmd bridge).

**Outcome:** The status line per wireframes F1-F3: surface
indicator left (active in accent+bold), sync segment right per
decision 7, toast-yield behavior per F2. `internal/ui/messages.go`
defines the engine-state message vocabulary (`SyncStateMsg`,
`BackfillProgressMsg`, `OutboxCountMsg`, `StoreChangedMsg`);
`cmd/poplar` grows the bridge goroutine translating the sync
engine's and outbox's existing state surfaces into `p.Send` calls
(the ui package never imports the engines).

**Acceptance criteria:**
- All four SY-5 states render distinctly, golden per state
  (truecolor + NO_COLOR profiles; the degrade channel assertion
  rides task 12's matrix).
- Progress renders count/total when known, count alone otherwise;
  a rate-limited backfill shows the warn state, never a silent
  stall (SY-5's named criterion, driven by the message the bridge
  emits for the engine's existing backoff state).
- The status line never scrolls or wraps: property test over long
  inputs at 60 columns, truncation is deliberate and marked
  (ellipsis token).
- Bridge test: a scripted engine-state transition sequence in the
  headless runner produces exactly the message sequence, no
  polling (a transition emits, steady state does not).
- Hit spans: the indicator registers one span per surface digit
  cell at render time; spans are data a test asserts against the
  rendered string (ADR-0017 character grain).

**Boundaries:** `internal/ui/statusline.go`, `messages.go`;
`cmd/poplar` bridge wiring. The bridge consumes engine surfaces
that exist (sync's health/state, outbox counts); if a needed
surface is missing, the task adds a minimal exported observer to
the engine package with its test, never a UI import into the
engine.

**Produces:** `StatusLine` component consumed by `App.View`;
message types every later pass reuses; `HitSpan` registration
convention (`type HitSpan struct{ Verb key.Binding; Rect
image.Rectangle }`, registered per render, consumed by task 10's
dispatcher).

**Salvage:** legacy `status_bar.go` is the anti-exemplar (BACKLOG
#61/#62 are its open defects: hardcoded hint, overflow past
width); this component's tests must cover both defect shapes
(hint text derives from the binding; long text truncates inside
width). Close #61 and #62 as superseded-by-rewrite in the BACKLOG
entry, citing the two tests, once they pass.

- [ ] Task 6 complete

## Task 7: Footer

**Requirement IDs:** UX-2. **Deliverables:** 2 (component,
per-screen conformance run).

**Outcome:** The registry-derived footer per F1/F9 and decision 8:
one row, the width-maximal prefix of the screen's committed
priority list, three-cell gaps, `? help` pinned right at every
width; text-entry state display deferred to 2b.

**Acceptance criteria:**
- UX-2 conformance (task 4's machinery) passes for every
  registered screen: footer set equals advertised keymap minus
  documented exceptions; asserted in one table test over the
  registry.
- Hint text derives from `key.Binding.Help()`, never a literal
  (BACKLOG #62's defect class, pinned by test).
- Priority-prefix property test: at every width the rendered set
  is exactly the longest prefix of the priority list that fits
  with the help hint; every rendered hint is legal in the active
  state; widening never removes a hint (monotone growth).
- Help completeness: the UX-2 revision-6 test asserts the help
  overlay equals the full keymap for every registered screen.
- Goldens: the footer at 80, 120, and 150 columns showing the
  prefix growing.
- Hit spans registered per hint; span text equals the rendered
  hint (character grain).

**Boundaries:** `internal/ui/footer.go` + tests.

**Produces:** `Footer` component consumed by `App.View`, driven
entirely by the active screen's `ScreenEntry`.

**Salvage:** none beyond the registry pattern.

- [ ] Task 7 complete

## Task 8: Toast, banner, modal confirm, undo presentation

**Requirement IDs:** UX-9 (presentation machinery), ER-1 (toast
logging), ER-3 (named states), FO-4/LT-4 seam (confirm shape).
**Deliverables:** 4 (toast, banner, confirm, undo countdown).

**Outcome:** Three chrome components per wireframes F2/F7: the
toast region (status-line right segment, one at a time, newest
wins, each logged through uerr's seam), the banner row (persistent,
Esc-dismiss-first precedence, dismiss glyph, never steals focus),
and the modal confirm (one question, named consequence, y/n/Esc,
rounded border, padModal, clamped centered, digits are no-ops).
The UX-9 undo presentation rides the toast: action name, visible
10 s countdown, `u` binding, single level, driven by a
`ui.UndoOffer` value so pass 3's triage plugs in without reshaping.

**Acceptance criteria:**
- Toast: newest-wins under two rapid offers; every toast write
  produces exactly one ER-1 log line (asserted against a captured
  log); countdown ticks render 9→0 deterministically as
  message-level tests with an injected fake clock (survey
  amendment E: timing behavior asserts at the message layer,
  never through a recording or a real timer).
- Undo: `u` inside the window emits the offer's undo message; `u`
  after expiry is a no-op; the window does not survive quit
  (asserted: quit during countdown discards the offer and the
  confirm, if any, names it). Exercised with a fake action; the
  real mutations arrive with pass 3.
- Banner: Esc with a banner visible dismisses it and does not pop
  the stack; second Esc behaves normally; a focused text-entry
  context consumes no banner keypress (asserted with a stubbed
  entry-state flag, the real contexts arrive in 2b).
- Confirm: y/n/Esc each answer; every other key including digits
  is a no-op; golden at natural size over a dimmed 100×30 backdrop.
- Hit spans: banner dismiss glyph, confirm answer cells.

**Boundaries:** `internal/ui/toast.go`, `banner.go`, `confirm.go`.
uerr is imported for the seam, not modified (task 11 owns uerr).

**Produces:** `Toast`, `Banner`, `Confirm` components and
`type UndoOffer struct{ Label string; Undo tea.Cmd }`; the quit
path consumes `Confirm` in task 11's wiring.

**Salvage:** none.

- [ ] Task 8 complete

## Task 9: Help overlay

**Requirement IDs:** UX-5. **Deliverables:** 1 (the overlay with
its goldens).

**Outcome:** The registry-derived help overlay per F5/F6: full
content-region takeover on the screen stack, Global section from
the grammar registry, screen section from the active screen's
entry, one column under 100 columns and two at 100+, scrollable
with the navigation family and wheel.

**Acceptance criteria:**
- Opens on `?` from every registered screen (table test over the
  registry, UX-5's acceptance).
- Content derives from the registry: a test mutates a fake
  screen's entry and asserts the rendered help follows (drift is
  impossible by construction, the test proves the construction).
- Goldens: F5 at 80×24, F6 at 100×30, boundary 99/100.
- Wheel scrolls it (typed mouse message injection); `Esc` pops.

**Boundaries:** `internal/ui/help.go` + tests.

**Produces:** `HelpScreen` registered like any screen (it is in
the UX-4 digits-switch list: switching surfaces from help is
legal and pops it).

**Salvage:** crush's help pattern as shape reference.

- [ ] Task 9 complete

## Task 10: Pointer dispatch

**Requirement IDs:** UX-6 (shell rows of ADR-0017's vocabulary).
**Deliverables:** 3 (dispatcher, double-click state, scripted
mouse tests).

**Outcome:** Mouse dispatch per ADR-0017's mechanics: cell-motion
mode everywhere; pane-grain resolution against `LayoutMode`
rectangles; character-grain resolution against the chrome's
registered hit spans (status digits, footer hints, banner dismiss,
confirm answers); the 400 ms double-click window as a compiled
constant with hierarchical click semantics (single click acts
immediately, always); wheel routed to the focused scrollable
(help, this pass). Every pointer action maps to a registered verb
legal in the active state; anything else is a no-op.

**Acceptance criteria:**
- Scripted `Update` tests injecting typed mouse messages: click a
  status digit switches surfaces in digits-switch states and
  no-ops over a modal (the state rule, both directions); click a
  footer hint runs its verb; click banner dismiss dismisses;
  click a confirm answer answers; wheel in help scrolls by the
  coalesced delta.
- Double-click state machine: two clicks inside 400 ms on the same
  target emit the open path, outside the window two selects;
  tested at window±1 ms with virtual time. (No rows exist yet;
  the test drives a synthetic target and pass 3 inherits the
  machine.)
- Grammar: the pointer-target test (task 4) passes over the full
  registry; goldens are untouched by mouse (pointer changes
  state, not rendering rules; asserted by a golden before/after a
  no-op hover-free click sequence).

**Boundaries:** `internal/ui/mouse.go` + tests. No zone library
(ADR-0011 revision 3).

**Produces:** the dispatch entry `App` calls for every
`tea.MouseMsg`; the `HitSpan` convention finalized.

**Salvage:** crush's multi-click state and bounds-walking patterns
(audit catalogue) as shape reference.

- [ ] Task 10 complete

## Task 11: cmd/poplar wiring and the uerr fallback

**Requirement IDs:** ST-2, SY-7 (render path), ER-1/ER-4 (fallback
rework), QA-1 (trace preserved). **Deliverables:** 4 (TUI default
path, headless flag, uerr rework, quit path).

**Outcome:** `cmd/poplar` runs the TUI by default: open store,
migrations, sweep, construct the program (profile resolver, wheel
filter, mouse cell-motion), start engines, bridge state, run. The
existing headless engine runner stays behind `--headless` (the
perf and QA harnesses and the live suites keep their subject).
`--startup-trace` keeps its current headless semantics unchanged
this pass (QA-1's first-screenful-of-rows form arrives with pass
3's list). uerr's log fallback reworked per dispositions row 24:
when the state-dir file cannot open, the fallback is a temp-dir
file, never stderr while a TUI may be rendering; if that also
fails, writes are counted through the existing LogHealth path and
stderr is used only after the tea program exits (the
reportLogHealth call sites already exist). Quit: `q` at a surface
root quits, confirmed through task 8's modal when the outbox
holds work.

**Acceptance criteria:**
- ST-2: with the network down (backend connector refused), a warm
  start reaches the interactive placeholder with store counts;
  asserted via teatest against the real store fixture, and the
  status line shows offline.
- SY-7: a second instance against a live store refuses with the
  actionable pid message before any TUI init (existing behavior,
  re-asserted at the new entry path).
- uerr: with `XDG_STATE_HOME` unwritable, logs land in the
  temp-dir fallback and a banner names the degradation (ER-3);
  with both unwritable, LogHealth reports the drop count at exit
  and nothing ever wrote to the TUI's terminal mid-run (asserted:
  captured stderr is empty until program exit).
- Quit path: `q` with empty outbox exits clean and marks clean
  shutdown; `q` with queued intents shows F7's confirm, `y` quits,
  `n` stays; scripted.
- The full existing gate stays green: `make check` including the
  headless harness tests, captured exit code.

**Boundaries:** `cmd/poplar/main.go`, `internal/uerr` (fallback
only), no engine behavior changes. The bridge from task 6 is
consumed, not reshaped.

**Produces:** the shipping entry point; `--headless` contract for
the harnesses (documented in the flag's help text).

**Salvage:** none; the legacy main predates the architecture.

- [ ] Task 11 complete

## Task 12: The gallery matrix, cmd/sketch, the flow suite, QA-7

**Requirement IDs:** QA-7, UX-7 (degrade renders), design language
section 9 (testing clause). **Deliverables:** 5 (gallery matrix
completion, cmd/sketch, teatest flow suite, churn policy, tmux
smoke).

**Outcome:** The three-tier size verification completed (survey
amendment C): tier 1 is task 3's LayoutMode boundary table (no
rendering; named cases at 59/60, 79/80, 99/100, 139/140 columns
and 14/15, 19/20 rows); tier 2 is the gallery completed to every
registered screen at each rung it renders distinctly at, boundary
sizes it consumes, floor state, short height, 80×24 explicitly,
per capability profile (truecolor dark, truecolor light, ANSI-16,
NO_COLOR); tier 3 is real resize through `scripts/tmux-check` on
the gate platform plus the manual pointer checklist. `cmd/sketch`
lands as the thin interactive wrapper over the seam (keys cycle
fixture, profile, and rung; its help text states it does not
verify pointer coordinates or glyph widths). The teatest flow
suite stays deliberately small: the quit path, the surface-switch
round trip, task 2's never-answering-terminal case, and task 11's
ST-2 startup, and nothing the seam or gallery already covers.
The UX-7 degrade renders prove focused and error carry distinct
non-color channels in ANSI-16 and NO_COLOR (unread and selected
join with pass 3's list; task 1's token-level test already covers
all four). freeze stills of the F1 and F2 gallery renders in both
themes are committed for the design record (survey amendment F).

The teatest swap path (survey amendment A's closing requirement,
landed with this task): `charmbracelet/x/exp/teatest/v2` lives under
the experimental `x/exp` path (ADR-0014 revision 2's named
risk), so a bubbletea bump it has not caught up with swaps for a
hand-rolled harness over `tea.WithInput`/`tea.WithOutput` piping
keystrokes in and reading rendered frames back, the same
virtual-terminal shape teatest itself wraps. Only the five functions
in `cmd/poplar/flow_test.go` move: the ST-2 offline-start test, both
quit-path subtests, the surface-switch round trip, and the
never-answering-terminal case. Nothing else in the module imports
teatest, and every committed gallery file is plain text `ui.Render`
itself produces, so a swap touches one file and no golden.

QA-7's TZ-and-locale clause is discharged by construction, not by a
subprocess-TZ test (review round 1, finding 2: `t.Setenv("TZ", ...)`
cannot move an already-initialized `time.Local`, and no source in
`internal/ui` or `internal/theme` called it anyway): `internal/ui`'s
`TestNoWallClockReferences` fails any non-test source in those two
packages, `internal/ui/fixtures` exempted, that references
`time.Now`, `time.Local`, or `time.Since`, so the by-construction
claim stays enforced when pass 3 adds date-rendering pressure.

Churn policy (survey amendment G): `make gallery` is the only way a
committed render or its ground-map sidecar changes; the diff is read
before committing, the same as any other golden; a chrome task's
churn stays inside the screens it touched; the pass-end reviewer
reads gallery churn explicitly.

**Acceptance criteria:**
- The gallery check runs in `make check`'s test step and CI
  verbatim: a stray diff between committed renders and a fresh
  sweep fails; regeneration is a named make target.
- Each gallery render commits with a ground-map sidecar (one
  character per cell naming its ground), because the ANSI-
  stripped text of a ground-structured design is whitespace and
  a text-only lane would be blind to pane regressions (review
  finding). The spinner frame index is a fixture input, never
  wall-clock.
- Two full in-process sweeps are byte-identical (QA-7), locale
  and TZ pinned by the fixtures package, asserted by a test that
  flips TZ and expects pinned output.
- The teatest swap path is written into this plan's record as
  one paragraph when the suite lands: what replaces teatest if it
  lags a bubbletea bump, and exactly which tests are affected
  (survey amendment A's closing requirement).
- Churn policy (survey amendment G), stated in the Makefile
  target's help and this plan: the regeneration command, the rule
  that a regenerated diff is read before commit rather than waved
  through, and the expectation that a chrome task's churn stays
  inside the screens it touched; the pass-end reviewer reads
  gallery churn explicitly.
- `scripts/tmux-check` drives the built binary through the smoke
  flow (launch, switch all four surfaces, open help, quit) on the
  gate platform, keyboard-only, captured exit code.

**Boundaries:** gallery files, `cmd/sketch`, the flow-suite test
file, `scripts/`, Makefile targets. No component changes except
defects the matrix exposes (each lands as its fix commit).
`cmd/sketch` is excluded from release artifacts and `make install`.

**Produces:** the committed render baseline every later screen
pass extends; the composition path pass 3 uses to turn its
approved wireframes into first goldens.

**Salvage:** the QA-7 profile-matrix concept from the build
machine; the survey's gallery and sketch shape; no code exists
yet.

- [ ] Task 12 complete

---

## Pass-end consolidation (standing, not a task)

Simplify over the pass diff, reviewer fan-out (three lenses
minimum; attack the pass's new guards first: the grammar test, the
UX-2 conformance check, the contrast test, the gallery check, and
the gallery churn of the round under review),
deadcode run with justifications, STATUS outcomes block, plan
archival, the manual pointer checklist on kitty, and the pass gate
with Geoff: the shell demoed live on the gate terminal, both
themes, resize through every rung, mouse vocabulary by hand.

## Non-goals, restated as decisions

- No mail list, reader, or any store-content screen beyond
  placeholder counts (pass 3).
- No text-entry context, form, config surface, first-run, or
  re-auth flow (pass 2b); UX-8 machinery deferred with them.
- ADR-0005 self-echo suppression stays routed to the first
  UI-driven mutation (pass 3, with triage).
- No `bg`-painting debate: the terminal never shows through;
  poplar paints its token background (decision 3).
- No hover, drag-and-drop, context menus (ADR-0017 rules them
  out); no drag-select (it rides copy mode, pass 3).
- The contacts placeholder shows no count if the store lacks a
  contacts read surface this pass; it says so plainly rather than
  growing one (F8's note).

## Self-review record

Checked against the spec set before presentation: every pass-2
requirement ID maps to a task (UX-1: 4/5; UX-2: 4/7; UX-3: 1/2;
UX-4: 4/5; UX-5: 9; UX-6 shell rows: 6/7/8/10; UX-7: 1/12; UX-9
machinery: 8; ST-2: 11; SY-5 render: 6; ER-1/3/4 touchpoints:
8/11; QA-7: 5/12; QA-10's conventions gates already run). UX-8,
ST-1, ST-3, ST-5 are the 2b cut. Type names cross-referenced
between Produces/Consumes blocks. No placeholder text remains.
