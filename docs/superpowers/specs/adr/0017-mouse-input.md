# ADR-0017: mouse as an accelerator over the keyboard-complete grammar

Date 2026-07-27. Status: proposed (Phase 5 gate; revision 2
after the machine-design adversarial review). Directive: solid
mouse support from day one (Geoff, 2026-07-27, at Phase 5 open).
Implements and amends UX-6; ADR-0012's keyboard decisions stand
unchanged. Evidence: the Phase 5 tooling audit
(`docs/poplar/research/2026-07-27-phase5-tooling-audit.md`).

## Context

UX-6 ("Mouse basics", SHOULD) already binds wheel scroll and
click-to-select where the terminal reports them, with the
registry test asserting every mouse-reachable action has a
keymap entry, and RD-16 requires that mouse reporting never
remove a copy path. The day-one directive raises the ambition
past a SHOULD's scope, so this record proposes amending UX-6 to
MUST with the vocabulary below as its scope (requirements
revision 4, ruled at the machine gate). bubbletea v2 makes mouse
support a per-frame declaration: `View.MouseMode` selects none,
cell-motion (click, release, wheel, drag-while-pressed), or
all-motion (hover), with four typed messages and automatic SGR
negotiation. The field's v2 exemplar (crush) ships click, wheel,
drag-select, and hand-rolled multi-click; no library provides
double-click or general hit-testing (bubblezone v2 documents a
caveat against the lipgloss v2 compositor).

## Decision

**The grammar rule.** The mouse is an accelerator, never a
capability: every mouse action maps to an existing grammar verb
legal in the active state, and every verb keeps its key. The
keyboard remains complete on every terminal, including those
with no mouse reporting (UX-6's acceptance test, kept). This is
the ladder principle applied to input: pointing changes how a
verb is reached, never what the verbs are. The design language
gains a pointer table in its section 2; the registry carries it,
and the grammar test asserts that no pointer binding names a
verb absent or illegal in the state it fires in.

**The v1 pointer vocabulary**, one row per verb-mapping:

| Pointer action | Verb it accelerates |
|---|---|
| click on a list, picker, or agenda row | move cursor to it |
| double-click on a row | open (`Enter`) |
| click on a sidebar folder or calendar | goto (`g` target) |
| click on a pane (wide rung's split) | focus that pane |
| click on a surface digit in the status line | surface-switch — only in states where the digits switch (the UX-4 table); a no-op in entry states and modals, exactly as the keys are |
| click on a footer hint | that hint's verb, same state rule |
| click on a banner's dismiss | dismiss |
| wheel in lists, reader, and help overlay | navigate / line scroll |
| click in a focused text-entry field, entry state only | move the in-field cursor; in command states a click is a no-op |
| click on a modal answer | that answer (`y`/`n`) |
| drag in the reader | select for yank (`y`'s copy mode) |

Drag-select in the reader is the one SHOULD in the set (it rides
copy mode, which owns the selection model); the rest is the
proposed UX-6 MUST scope, discharged screen by screen with each
screen's pass. Hover states, drag-and-drop, and context menus
are ruled out for v1: each would be a pointer-only capability,
which the grammar rule forbids.

**Mechanics.**

- `MouseMode` is cell-motion on every screen; all-motion is
  reserved (crush's pattern: upgrade per frame only where a
  hover state exists, which v1 does not).
- Hit-testing is two-grained, and priced honestly (the review
  killed revision 1's "bounds fall out for free"). Pane and row
  grain: `LayoutMode` carries per-pane rectangles (the ADR-0011
  revision block) and the windowed list resolves row hits by
  walking its visible slice only — both crush's production
  patterns. Character grain (footer hints, status-line digits,
  modal answers, in-field cursor) cannot be derived from pane
  rectangles: those chrome components register hit spans at
  render time from the same styled strings they emit. The
  components are few, they already own their layout math, and
  the spans are testable data; this is the work the day-one
  directive buys.
- Single click acts immediately, always. Poplar's click pair is
  hierarchical (click selects, double-click opens the selected),
  so no action is deferred waiting for a possible second click —
  revision 1 copied crush's delayed-click pattern without its
  reason (crush defers only where click-1 and click-2 semantics
  conflict). Double-click is a 400 ms window over the same row;
  the threshold is a compiled constant, not config.
- Wheel input is coalesced before Update by the
  program-construction filter (machine design section 8):
  ~16 ms sample window, signed accumulation, direction reset —
  an unfiltered burst is one store round-trip per tick against
  QA-2's budget, and terminals disagree about ticks per detent.
- Terminal-native selection: capturing the mouse takes the
  terminal's drag-to-copy behind its bypass modifier, and
  the modifier is not universal — Shift on kitty, foot,
  alacritty, VTE, and Windows Terminal; Option on iTerm2; none
  on macOS Terminal.app short of disabling mouse reporting
  (review finding; C10 keeps macOS in scope). The help overlay
  therefore says "your terminal's mouse-reporting bypass"
  without naming a key, and poplar's yank path (RD-16,
  OSC 52) stays the first-class copy story.

**Testing.** Mouse coverage lives at the message layer: grammar
tests assert the pointer table against the registry per state;
scripted Update tests and teatest `Send` inject typed mouse
messages per screen (click-to-cursor, double-click timing
windows, wheel coalescing, drag). Goldens are unaffected
(pointer input changes state, not rendering rules). The tmux
verification layer stays keyboard-only — terminal-level mouse
injection has no usable tooling (audit finding) — so each
screen pass's close checklist carries the manual real-terminal
pointer item (machine design section 5).

## Alternatives considered

- **bubblezone for hit-testing**: maintained and v2-tagged, but
  its README flags the lipgloss v2 compositor caveat, and
  crush (the largest v2 app) hand-tracks bounds instead. Poplar
  still hand-rolls the character-grain spans bubblezone would
  have provided; the trade is a small amount of owned, testable
  code against a dependency with a documented conflict at the
  compositor layer this stack sits on.
- **All-motion everywhere**: pays a continuous event-volume tax
  for hover states v1 does not render.
- **Deferred single-click** (revision 1): adds 400 ms of latency
  to the product's most common pointer action and to rows with
  no double-click semantics at all; unnecessary once the click
  pair is hierarchical.
- **Mouse-optional posture** (defer to post-v1, UX-6 as
  ratified): declined by directive; retrofitting hit-testing
  after screens exist is the expensive order, and day-one
  support keeps the pointer table growing with each screen's
  pass instead of as a migration.

## Consequences

Every screen pass's wireframe names its pointer targets alongside
its keys, and the screen's registry entry binds both. The
optimistic-paint contract (LT-2) applies to pointer-initiated
triage identically. Chrome components own hit-span registration
as part of their render contract, which the component vocabulary
(design language section 6) absorbs. The one honest cost:
capturing the mouse moves the terminal's native drag-to-copy
behind a per-terminal bypass, a trade every mouse-enabled TUI
makes; poplar's answer is its copy mode plus the help note.
If a future feature needs hover (all-motion), it must pass the
grammar rule's accelerator test or supersede this record.

## Revision 3 (2026-08-19, pass 2 task 5a round 2)

**Wheel coalescing moves out of the program-construction filter and
into the root model.** The filter design this record originally
specified strands input: `tea.WithFilter` can only suppress or
transform a message as it arrives, so a coalesced message can be
emitted only in reaction to a *later* tick. A gesture's tail, the
ticks accumulated since the last emitted message, sits pending
forever once scrolling stops with no further tick to trigger a
flush; a single, isolated detent never scrolls at all, since nothing
ever arrives after it to close its sum. This was found during
task 5a's spec review over the implemented filter, not predicted at
authoring time.

The fix: the coalescing decision, the running signed sum, the
gesture's opening tick, and the direction it opened in, moves onto
the root model, and a `tea.Tick(wheelWindow)` flush timer arms itself
the moment a gesture opens (one tick, not renewed per further tick in
the same gesture). A gesture flushes into one `WheelMsg`, carrying
the coordinates of its first tick, on whichever comes first: that
timer firing, or a later tick reversing the running sum's direction.
Because the timer is armed unconditionally at open time and fires
regardless of what else happens, every gesture flushes eventually:
a single detent flushes after one `wheelWindow`; a continuous scroll
still emits at most one message per `wheelWindow` (the budget this
record priced against QA-2 is unchanged); no gesture's tail is ever
stranded.

This keeps the coalescing state Rule-1-conformant (elm-conventions:
all state in models) rather than Rule-2-exempt: the mutable window
state that previously lived in the filter closure, the one recorded
exception to "mutation only in Update", now lives in the root
model's fields and mutates only inside its Update, the ordinary
case every other piece of App state already follows. The
program-construction filter is no longer necessary for wheel input at
all; `tea.MouseWheelMsg` reaches `Update` through the normal message
path like everything else, and `Update`'s dispatch is what "tags"
it into the gesture. A future coalescing need (motion sampling, most
plausibly) may still want a `tea.WithFilter` seam, evaluated on its
evidence rather than by wheel's example.
