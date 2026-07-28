# Poplar design language

**Date:** 2026-07-27
**Status:** Draft for the Phase 4 gate. This is the UX-3 artifact
C5 requires before any screen is built. Every screen derives from
it; the `internal/theme` package (Phase 5, build step 2) compiles
its tokens as Go values; the UX-3 analyzer enforces that no
styling literal exists outside that package. Text wireframes
(Phase 5) precede every screen build and cite this document.
**Exemplar:** the cairn-cms design system (its charter-adjective
method, grammar-token vocabulary, and grammar-versus-palette
boundary), translated to a terminal.

Behaviors the requirements spec defers to "the design-language
document" land here: the interaction grammar (UX-1), the footer
exception set (UX-2), the text-entry model (UX-8), and the
Catkin undo boundary (CO-12).

## 1. The design charter

Poplar aims to be **calm, crisp, instant, opinionated,
coder-native, and teachable**. It must not be busy, decorative,
sluggish, configurable-instead-of-designed, or dependent on prior
mutt-family knowledge. The adjectives grade every design decision
the way cairn's charter grades the admin:

- **Calm**: the screen at rest is quiet. Color marks state, never
  decoration; the count of attention-demanding moments on one
  screen stays low; empty states are composed, not apologetic.
  The anchor spirit is iA Writer by way of cairn: chrome defers
  to content.
- **Crisp**: alignment is exact, spacing follows the roles below,
  truncation is deliberate and marked. Polish is the absence of
  small defects.
- **Instant**: the design never buys looks with latency. Any
  decoration that costs a frame loses.
- **Opinionated**: one layout, one sort order, one idiom. Where
  the field offers options, poplar ships a decision.
- **Coder-native**: monospace is the medium, not a constraint.
  Code, diffs, and URLs are first-class citizens of the reader.
- **Teachable**: the footer teaches the grammar; a first-day user
  discovers every verb without documentation (the alpine
  inheritance). Nothing advertised is illegal; nothing legal is
  hidden.

## 2. The interaction grammar (UX-1)

One vocabulary of global verbs, each bound to the same key on
every surface. Shifted letters are distinct keys; Ctrl and Alt do
not exist (C8); no sequences, no chords.

| Verb | Key | Meaning everywhere |
|---|---|---|
| navigate | `j` / `k` | down / up one row (arrows are synonyms) |
| page | `Space` / `b` | forward / back one page in a reading pane |
| extremes | `Home` / `End` | first / last (also `G` for last, vim) |
| open | `Enter` | open the thing under the cursor |
| back | `Esc` | close, dismiss, or step out; never destructive |
| search | `/` | the surface's search (one grammar, SR-2) |
| goto | `g` | the surface's jump: folder switcher in mail, date jump in calendar, letter jump in contacts, section jump in config |
| next-unread | `Tab` | next unread message (mail surfaces; alpine) |
| select | `x` | toggle mark on the cursor row |
| select-by | `;` | select by criteria (alpine's Select model) |
| undo | `u` | the one undo (UX-9) |
| help | `?` | the help overlay (UX-5) |
| quit | `q` | leave poplar from a surface root, confirmed if the outbox holds work |
| surface-switch | `1` `2` `3` `4` | mail, calendar, contacts, config |
| leave-field | `Esc` | exit a text-entry field to its command state (UX-8) |

The triage verb set, identical from list, thread view, and reader
(LT-2):

| Verb | Key |
|---|---|
| archive | `a` |
| delete (to Trash) | `d` |
| flag toggle | `*` |
| read/unread toggle | `e` |
| move (picker) | `s` (pine's save) |
| junk/not-junk | `!` |
| reply / reply-all | `r` / `R` |
| forward | `f` |
| compose new | `m` |
| yank (copy mode) | `y` |
| today (calendar surfaces) | `t` |

Rulings the table encodes: `g` is the unified goto verb, so
"top of list" belongs to `Home` (vim's `gg` is a sequence and is
out); `Esc` carries both back and leave-field because they are the
same instinct at two depths; the surface keys are direct digits
because a cycle key hides the destination and a letter key would
collide with triage verbs. Per-screen keymaps (link mode, the
RSVP card's answer keys, the fallback-stack cycle, fold verbs)
are Phase 5 wireframe decisions; they bind inside their screen's
registry entry, may not contradict this table, and the grammar
test enforces the non-contradiction mechanically.

## 3. The text-entry model (UX-8)

One model for every context that accepts printable input (compose
body, compose headers, search bar, forms, pickers):

- **Entry state**: printable keys are input. The only command keys
  are `Esc` (leave-field), `Enter` (the field's accept, where the
  field has one), `Tab` (next field in multi-field forms), and
  the arrows/Home/End family for in-field movement.
- **Command state**: `Esc` exits the field into the context's
  command state, where every context verb is a modifier-free
  single key (send, postpone, attach, identity switch, scope
  toggle, discard). A second `Esc` steps out of the context
  entirely, through the same confirmation rules as any back.
- The footer always shows which state the user is in and the
  state's verbs (UX-2); the entry-state footer leads with
  `Esc commands`.

Catkin follows the same model with a richer command state: typing
inserts (the modeless iA Writer surface of CO-1); `Esc` enters
Catkin's command state, where the vim-idiom motions and word/line
operations of CO-12 live as single keys; any printable key that
is not a command returns to entry by inserting. **The undo
boundary (CO-12)**: inside Catkin's command state, `u` is the
buffer's own vim-style undo, scoped to the compose session and
independent of the global UX-9 undo; the global undo never
reaches into buffer edits, and the buffer undo never reverses a
message-level action. The boundary is this sentence.

## 4. The footer (UX-2)

Alpine's mode-scoped key-hint footer, derived from the screen's
registry entry so it cannot drift. At most two rows; hints are
ordered grammar verbs first, then screen verbs, each as
`key description` in two or three words. The footer renders the
current text-entry state when one is active.

The advertised-set exception list (UX-2 caps it at five, each
with a committed reason):

1. **Arrow keys, `Home`/`End`, `PgUp`/`PgDn`**: synonyms of
   advertised vim-family keys; advertising both spellings would
   double the footer for zero information.
2. **`G`**: synonym of `End`, kept for vim hands; same reason.
3. **Digit surface keys `1`-`4`**: advertised in the status
   line's surface indicator (which shows the four surfaces and
   highlights the active one) rather than the footer; repeating
   them in the footer would spend a row on chrome navigation.
4. **`q` on non-root screens**: quit is advertised only at
   surface roots; elsewhere `Esc` is the way out and `q` still
   works as quit-from-root semantics require.

The list is closed; adding an entry requires amending this
document (the registry test fails on an undocumented exception).

## 5. Undo presentation (UX-9)

One presentation on every surface that offers undo: a toast in
the status region naming the action ("Archived 3 messages"), a
visible 10-second countdown, `u` as the key, single-level depth.
Undo after outbox dispatch issues the compensating mutation and
the toast says "Undoing…" until the store reflects it. The window
does not survive quit, and the toast's countdown makes that
legible. Permanent deletions never show the toast; they show the
FO-4 confirmation instead. RSVP answers show no undo (re-answer
is the correction, section 14 of the requirements).

## 6. Component vocabulary

Every screen composes from this vocabulary; a screen that needs a
component not listed here adds it to this document first (the
cairn rule: a gap that forces improvisation is a defect).

- **List**: the one-line-row scrolling list (mail list, thread
  list, agenda, contacts, config sections). Owns cursor,
  selection marks, unread/flag markers, truncation. One row is
  always exactly one line (LT-1's density ruling).
- **Sidebar**: the folder/calendar rail with counts and
  visibility toggles. Collapsible; never focused by default.
- **Status line**: one row at the top or bottom edge carrying
  surface indicator (`1 Mail  2 Cal  3 People  4 Config`), sync
  state (SY-5's synced/syncing/offline/backing-off), and
  transient counts. Never scrolls, never wraps.
- **Footer**: section 4.
- **Toast**: the transient notice region (undo, background
  outcomes). One toast at a time; newest wins; each logged
  through the ER-1 seam.
- **Banner**: the persistent non-focus-stealing notice row
  (reminder due, degraded render, partial search coverage,
  keyring fallback). Dismissable; consumes no keypress while a
  text-entry context is focused (CA-7).
- **Modal confirm**: the y/n question box (permanent delete,
  folder delete with count, quit with pending outbox). One
  question, named consequence, `y`/`n`/`Esc`.
- **Picker**: the type-ahead chooser (move-to-folder, goto
  folder, attach path, identity switch, date jump). One input
  line plus a filtered list; `Enter` accepts, `Esc` leaves;
  can create-in-place where the flow allows (LT-5).
- **Form**: labeled fields with inline help (config surface,
  event create/edit, first-run token flow). Follows UX-8;
  `Tab` moves fields; validation errors are named inline.
- **Card**: the framed detail block (event card in the reader,
  contact card, event detail). Fields as label/value rows;
  actions in the footer, not in the card.
- **Reader**: the message pane: header block (RD-9), rendered
  body, attachment list, inline cards. Owns paging and copy
  mode.
- **Help overlay**: the full keymap view, registry-derived
  (UX-5).
- **Progress**: the in-place progress state for the C1
  exceptions and initial sync (spinner plus label plus count
  where known; a progress state always names what it waits on).

## 7. Theme tokens

The `internal/theme` package compiles every visual decision as Go
values (C5). The grammar-versus-palette boundary follows cairn:
roles and relationships are the grammar and never change per
theme; the palette values fill the roles. lipgloss v2 is pure, so
every color role is a function of `isDark bool`, resolved once at
startup from the terminal background query.

**Color roles** (the inventory; values land with the Phase 5
theme build): `fg`, `fgMuted`, `fgSubtle`, `accent`, `unread`,
`selectedBg`, `focusedBorder`, `error`, `warn`, `success`,
`link`, `codeBg`, `quote`, `diffAdd`, `diffDel`, `flag`, and
`calendarSlot[8]`. Rules: text roles hold 4.5:1 against their
background, indicator roles 3:1 (UX-7, asserted by a test over
the values); the accent is reserved for the focused/active
moment, never for decoration (cairn's accent reservation);
`calendarSlot` colors are assigned by the theme, never by the
server (CA-8).

**Degrade tables**: the theme ships an ANSI-16 profile and a
NO_COLOR profile in which unread, selected, focused, and error
each map to a distinct non-color channel: unread = the `●`
marker, selected = reverse video, focused = the heavy border
glyph set, error = the `!` gutter marker plus reverse. The UX-7
golden asserts no two states share a marker.

**Glyph tokens**: every non-ASCII glyph the UI renders is a
named token with an ASCII fallback in the degrade profiles:
`unread ●`, `flagged ⚑`, `attachment ⊕`, `collapsed ▸`,
`expanded ▾`, `selected ✓`, `ellipsis …`, the border sets (light
for panes, heavy for focus), and the spinner frames. Plain
Unicode only, no private-use-area or patched-font glyphs; the
showcase must render on a stock terminal font. Exact code points
are theme values; the UX-3 analyzer forbids glyph literals
outside the package.

**Spacing roles** (cells and rows, the cairn gap grammar in
terminal units): `gapLabel 1` (label to its value),
`gapControl 2` (control to neighbor), `gutter 1` (marker column
to text), `padPane 1` (pane edge to content), `padModal 2/1`
(modal horizontal/vertical), `gapSection 1 row`. Markup-level
literals are forbidden by the analyzer; a screen reaches spacing
only through roles.

**Type roles** are trivial in a terminal (one size) but weight
and emphasis are not: `emTitle` (bold), `emLabel` (dim),
`emValue` (normal), `emHint` (dim italic where supported, dim
otherwise). Roles, not per-screen choices.

## 8. Surface unification (C6)

The same key means the same verb on all four surfaces (section
2); the same components render all four; one search grammar
(SR-2), one undo model (section 5), one time parser
(`internal/when`) behind every date field, jump, and query
operator. Config is a surface like the others: the same list,
form, and picker components, navigated with the same keys
(ST-3's alpine-style setup screens). The status line's surface
indicator is the C6 unification made visible: four surfaces, one
grammar, one chrome.

## 9. What Phase 5 fills in

This document pins the contracts; the build fills the values
inside them, and only inside them:

- Palette values for the color roles, both themes, passing the
  contrast tests. The palette is poplar's own; cairn's Warm
  Stone is the craft exemplar, not the hue source.
- Exact glyph code points and border sets.
- Per-screen keymaps and layouts, via text wireframes citing
  this grammar, one wireframe per screen before its build pass.
- The remaining screen-verb assignments (link mode, RSVP answer
  keys, fallback-stack cycle, fold verbs), bound by the grammar
  table and the registry test.

Amendments to the grammar, the exception list, or the component
vocabulary go through this document first; the analyzer and
registry tests make silent divergence a build failure.
