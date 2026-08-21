# Poplar design language

**Date:** 2026-07-27
**Status:** Revision 2, for the Phase 4 gate (revision 1 was
adversarially reviewed; this revision folds the findings).
**Revision 3** (2026-08-20, pass 2 final fix round, spec m14): amends
the modal-confirm vocabulary entry (§6) with the full-shell wipe and
the `?`-help exemption pass 2's build actually shipped, and the
footer's testing clause (§4) with the mechanical no-advertised-no-op
claim guard-falsifiability finding F4 added. This is
the UX-3 artifact C5 requires before any screen is built. Every
screen derives from it; the `internal/theme` package (Phase 5,
build step 2) compiles its tokens as Go values; the UX-3 analyzer
enforces that no styling literal exists outside that package. Text
wireframes (Phase 5) precede every screen build and cite this
document.
**Exemplar:** the cairn-cms design system (its charter-adjective
method, grammar-token vocabulary, and grammar-versus-palette
boundary), translated to a terminal.

Behaviors the requirements spec defers to "the design-language
document" land here: the interaction grammar (UX-1), the footer
exception set (UX-2), the switch table and text-entry exception
list (UX-4), the text-entry model (UX-8), and the Catkin undo
boundary and command keymap (CO-12).

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
every surface where the verb exists; a verb bound on one surface
and absent on another is stated, never contradicted. Shifted
letters are distinct keys; Ctrl and Alt do not exist (C8); no
sequences, no chords.

| Verb | Key | Meaning |
|---|---|---|
| navigate | `j` / `k` | down / up one row in lists; down / up one line in the reader (arrows are synonyms) |
| page | `Space` / `b` | forward / back one page, in lists and reading panes alike |
| extremes | `Home` / `End` | first / last item (advertised); `G` is a vim synonym of `End` |
| open | `Enter` | open the thing under the cursor |
| back | `Esc` | close, dismiss, or step out; never destructive |
| message-step | `n` / `p` | next / previous message from within the reader (alpine's idiom); after a reader triage action, the reader advances to `n`'s target (LT-2) |
| search | `/` | the surface's search (one grammar, SR-2) |
| goto | `g` | the surface's jump: type-ahead folder switcher in mail, natural-language date jump in calendar, letter jump in contacts, section jump in config |
| next-unread | `Tab` | next unread message (mail surfaces; unbound elsewhere) |
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
| attachments (reader) | `v` (mutt's view) |

Mail-surface folder jumps (FO-2), capitals mirroring their role
names: `I` Inbox, `D` Drafts, `S` Sent, `A` Archive, `J` Junk,
`T` Trash. Calendar-surface verbs: `t` jumps to today (CA-2);
`t` is unbound on other surfaces, and the mail capitals are
unbound on calendar, which the same-key-same-verb rule permits
(absence is not contradiction; the grammar test enforces exactly
that).

Rulings the tables encode: `g` is the unified goto verb, so first
position belongs to `Home` (vim's `gg` is a sequence and is out)
while `G` keeps vim's last-position instinct as a synonym; `Esc`
carries both back and leave-field because they are the same
instinct at two depths; surface keys are direct digits because a
cycle key hides the destination and letters would collide with
triage verbs; thread fold and unfold ride `h` / `l` in thread
views (the vim tree instinct), stated here so the wireframes
inherit rather than invent them.

**Scope rule.** The grammar tables bind browse and command
states. Text-entry states are governed entirely by section 3's
model (printable keys are input there; `Tab` is next-field in
multi-field forms; the global verbs are reached through
leave-field first), and the grammar test checks non-contradiction
over browse and command states only. This is the rule, not an
exemption list entry: entry states are a different mode by
design, and the footer always shows which mode is active.

Remaining per-screen verbs (link mode, the RSVP card's answer
keys, the fallback-stack cycle) are Phase 5 wireframe decisions;
they bind inside their screen's registry entry, may not contradict
these tables, and the grammar test enforces non-contradiction
mechanically. Two named contexts are exempt from the
non-contradiction check, each recorded here: the **modal confirm**
(`y` / `n` / `Esc` answer a named question; `y` is "yes" there,
not yank) and **Catkin's command state** (section 3), whose
buffer-editing keymap deliberately reuses global keys for
buffer-scoped verbs. The exemption list is closed; the grammar
test knows exactly these two.

**The switch table (UX-4).** Both lists enumerate states, and the
UX-4 test resolves each screen to its active state before
checking. States where the digit keys switch surfaces (state
preserved per UX-4): mail list, thread view, reader, calendar
agenda, calendar grid views (when built), event detail, contact
list, contact card, config sections, help overlay, and compose's
message-level command state (digits are commands there, not
input). States where digits are input, reached and left through
`Esc` first, which the test asserts equals the set of states
accepting printable input: compose headers, Catkin's entry state,
Catkin's command state (unlisted printables insert, so it accepts
printable input by construction), the search bar, form fields
(config, event create/edit, first-run token form), and picker
filter fields. Modal confirms are neither: they answer `y`/`n`/
`Esc` and everything else is a no-op, so they are left before
switching.

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

**Catkin's command state** is the second named grammar exemption:
typing inserts (the modeless iA Writer surface of CO-1); `Esc`
enters the command state, whose buffer keymap is vim-idiom bent
to single presses, no operator-motion sequences (C8):

| Key | Buffer verb |
|---|---|
| `h` `j` `k` `l` | character and line motion |
| `w` / `b` | word forward / back |
| `0` / `$` | line start / end |
| `g` / `G` | buffer top / bottom |
| `x` | delete character |
| `c` | change word: delete the word at the cursor, return to entry |
| `D` | delete line |
| `J` | join lines |
| `p` | paste the last deleted text |
| `o` / `O` | open line below / above, returning to entry |
| `i` / `a` / `I` / `A` | return to entry at cursor / after / line start / line end |
| `u` / `U` | buffer undo / redo |

Any printable key not in the table returns to entry by inserting.
Message-level verbs (send, postpone, attach, identity switch)
live in compose's command state per UX-8, reached the same
way; the footer distinguishes the two command scopes. **The undo
boundary (CO-12)**: `u` in Catkin's command state is the buffer's
undo, scoped to the compose session and independent of the
global UX-9 undo; the global undo never reaches into buffer
edits, and the buffer undo never reverses a message-level action.
External mutations through Catkin's buffer-mutation API
(signature materialization now, AI tidy later) land as exactly
one buffer-undo entry each.

## 4. The footer (UX-2)

**One prioritized row** (decision 8, Geoff 2026-08-19, superseding
the same-day distribute-across-width ruling and the earlier
two-row, advertise-everything form; requirements revision 6 amends
UX-2 to match). Each screen's registry entry carries a committed
hint priority order, derived from the screen's keymap so it
cannot drift; the footer renders the width-maximal prefix of that
order, each hint as `key description` in two or three words, joined
by a three-cell gap (`gapHint`, section 7). `? help` is pinned right
at every width, held off the last shown hint by its six-cell
reserve (`gapPin`, section 7, the pinned exemplar's value), the
constant pointer to the help overlay, which is
the completeness surface: it lists every legal key regardless of
what the footer's width-limited prefix shows. Width changes what
the footer shows, never what is legal—the ladder principle applied
to the footer's row. The footer renders the current text-entry
state when one is active, leading with the leave-field verb (`Esc
commands`, section 3).

A screen's committed priority order naturally omits some legal keys
at every width a real terminal reaches, and that is by design, not
an exception list to close: the active surface digit is advertised
permanently by the status line's compact cluster (section 6, amended
by pass 2 decision 1), so a screen's priority list has no reason
to spend a slot on `1`-`4`; `q` retains its root semantics and a
non-root screen's priority list leads with `Esc` instead; the
folder-jump capitals `I D S A J T` are taught by the goto picker
(`g`) and the help overlay rather than by six footer slots; a
capability-gated key (the remote-image load key when the terminal
lacks graphics support, RD-6) is simply absent from the priority
list on that terminal, since decision 8 already promises an absent
key is never legal either. None of this needs its registry rule:
the UX-2 test's mechanical claims are that the footer is always a
legal prefix of the committed order, that the help overlay's content
is always the complete keymap, and, added by the pass 2 final fix
round (revision 3, guard-falsifiability finding F4, RULING: this
exceeds rather than contradicts the narrowed claim above), that
every hint the footer actually offers fires its verb (a non-nil Cmd
or a changed View) when driven through `App.Update` the same way a
click's `fireVerb` does, over every registered surface-root entry.

## 5. Undo presentation (UX-9)

One presentation on every surface that offers undo: a toast in
the status region naming the action ("Archived 3 messages"), a
visible 10-second countdown, `u` as the key, single-level depth.
The countdown window matches the outbox hold (technical design
section 7), so undo during it is a queue annihilation; undo after
dispatch issues the compensating mutation and the toast says
"Undoing…" until the store reflects it. The window does not
survive quit, and the countdown makes that legible. Permanent
deletions never show the toast; they show the FO-4 confirmation
instead. RSVP answers show no undo (re-answer is the correction).

## 6. Component vocabulary

Every screen composes from this vocabulary; a screen that needs a
component not listed here adds it to this document first (the
cairn rule: a gap that forces improvisation is a defect).

- **List**: the one-line-row scrolling list (mail list, thread
  list, agenda, contacts, config sections). Owns cursor,
  selection marks, unread/flag markers, truncation. One row is
  always exactly one line (LT-1); the implementation is poplar's
  windowed list model reading store pages (technical design
  section 12), never an all-in-memory component.
- **Sidebar**: the folder/calendar rail with counts and
  visibility toggles. Collapsible; never focused by default.
- **Status line**: one row at the top edge (amended by the pass 2
  plan's decision 1,
  `docs/superpowers/plans/2026-08-19-pass-2-design-language-and-shell.md`,
  ruled by Geoff at the wireframe review: pine's title-bar instinct,
  since 98% of time is spent in mail and the row is high-value
  space). The origin holds the compact surface cluster: the active
  surface named in accent and bold (`1 Mail`), siblings as bare dim
  digits (`2 3 4`); sibling names live in the help overlay and
  appear on visit, never spelled out in the status line itself. The
  rest of the left segment carries the active surface's context;
  sync state (SY-5) and transient counts sit right, and toasts
  render in that same right segment so a transient notice never
  shifts the layout (charter: calm). The cluster's segment divider
  aligns with the sidebar's divider column when a sidebar is
  present, and drops when none is. Never scrolls, never wraps. The
  footer (section 4) is the bottom edge; banners are the row
  directly under the status line (decision 2), pushing content down
  one row while present.
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
  question, named consequence, `y`/`n`/`Esc`; a grammar-exempt
  context (section 2). Amended by the pass 2 final fix round
  (2026-08-20, revision 3, exemplar authority per decision 13): the
  screen behind it wipes to the base ground rather than rendering
  dimmed, since no dimmed-backdrop rendering exists in the shipped
  theme to compare against. That wipe is a standing gate item for
  Geoff's eye, the same open question the code half of spec M4
  left. And `?` never opens help while a modal is showing (the
  modal owns its fixed y/n/Esc grammar instead of the global
  toggle).
- **Picker**: the type-ahead chooser (move-to-folder, goto,
  attach path, identity switch, date jump). One input line plus
  a filtered list; `Enter` accepts, `Esc` leaves; can
  create-in-place where the flow allows (LT-5).
- **Form**: labeled fields with inline help (config surface,
  event create/edit, first-run flow). Follows UX-8; `Tab` moves
  fields; validation errors are named inline. Built on the
  shared focus-management helper, not a third-party form engine
  (ADR-0011).
- **Card**: the framed detail block (event card in the reader,
  contact card, event detail). Fields as label/value rows;
  actions in the footer, not in the card.
- **Reader**: the message pane: header block (RD-9), rendered
  body, attachment list, inline cards. Owns paging, line scroll,
  message stepping, and copy mode.
- **Help overlay**: the full keymap view, registry-derived
  (UX-5); also the advertised home of the folder-jump capitals.
- **Progress**: the in-place progress state for the C1 exceptions
  and initial sync (spinner plus label plus count where known; a
  progress state always names what it waits on).

## 7. Theme tokens

The `internal/theme` package compiles every visual decision as Go
values (C5). The grammar-versus-palette boundary follows cairn:
roles and relationships are the grammar and never change per
theme; the palette values fill the roles. lipgloss v2 is pure, so
every color role is a function of `isDark bool`, resolved by the
runtime capability resolver (technical design section 12).

**Color roles** (the inventory; values land with the Phase 5
theme build, amended by pass 2 decision 6,
`docs/superpowers/plans/2026-08-19-pass-2-design-language-and-shell.md`):
`bg`, `bgPanel`, `fg`, `fgMuted`, `fgSubtle`, `accent`, `unread`,
`selectedBg`, `border`, `focusedBorder`, `error`, `warn`,
`success`, `link`, `codeBg`, `quote`, `diffAdd`, `diffDel`,
`flag`, and `calendarSlot[8]`. `bg` and `bgPanel` join the
inventory as the two grounds content and chrome step between
(`bgDeep`, a third dark step below `bg`, was tried and rejected:
the review found the extra step illegible against poplar's dark
palette); `selectedBg` is accent-tinted, never a plain gray step;
`border` joins as a structural role, deliberately sub-contrast
and exempt from the floors below, carrying pane dividers, the
reader's header rule, and modal frames; `focusedBorder` repurposes
as the degrade profiles' focused-divider color rather than a
truecolor border tint. Rules: text roles hold 4.5:1 against all
four grounds (`bg`, `bgPanel`, `selectedBg`, `codeBg`), indicator
roles 3:1 against the same four (UX-7, asserted by a test over
the values, `border` exempted); the accent is reserved for the
focused/active moment, never decoration; calendar colors are
theme-assigned (CA-8), slots assigned in calendar sort order and
cycling past eight, with the cycle documented in the calendar
sidebar so a collision is legible rather than mysterious.

**Degrade tables**: the theme ships an ANSI-16 profile and a
NO_COLOR profile in which unread, selected, focused, and error
each map to a distinct non-color channel: unread = the `●`
marker, selected = reverse video, focused = the edge bar's glyph
weight (`▌` focused, `▏` unfocused: a channel that survives
NO_COLOR because the two states are different glyphs, not
different colors, unlike a border color), error = the `!` gutter
marker alone. The UX-7 golden asserts no two states share a
marker. Amended by the pass 2 review's ruling I1
(`docs/superpowers/plans/2026-08-19-pass-2-design-language-and-shell.md`):
reverse video is selection's exclusive channel; the original
wording also assigned it to error, an internal contradiction the
review caught (a row that is both selected and in error would
have carried the same reverse cue for two different reasons, and
error alone would have been indistinguishable from selected alone
at the reverse channel). Error's channel is the gutter marker by
itself. Amended by pass 2 decision 3: the ANSI-16, NO_COLOR, and
text-gallery profiles additionally substitute a drawn single-line
divider, the `divider │` glyph token, for the ground-color steps
that separate panes at full color, since neither profile can
render those steps legibly; the ladder is the truecolor expression
of pane separation, never the only one. At those two profiles
every border weight (single-line, rounded, heavy) also collapses
to the same plain ASCII frame, since box-drawing weight is not a
channel either profile carries reliably; the focused state's
channel stays the edge bar's glyph weight, never the border.

**Glyph tokens**: every non-ASCII glyph the UI renders is a named
token with an ASCII fallback in the degrade profiles: `unread ●`,
`flagged ⚑`, `attachment ⊕`, `collapsed ▸`, `expanded ▾`,
`selected ✓`, `ellipsis …`, the border sets, and the spinner
frames. Pass 2 decision 3 adds: `dismiss ✕` (`x`), `edgeBarFocused
▌` U+258C (`>`), `edgeBarBlurred ▏` U+258F (`|`), `separator ·`
U+00B7 (`-`), `scrollPos ≡` U+2261 (`=`), `treeBranch ├─` (`|-`),
and `treeLast └─` (`+-`). The pass 2 review's ruling C3c adds one
more: `divider │` U+2502 (`|`), the drawn structural line the
degrade profiles substitute for a ground step (above); the same
token also serves the reader's quote-bar prefix, one glyph and one
`border`-role styling for both uses rather than two tokens that
would otherwise differ only in name. Task 8's fix round (findings
r1) adds `warnMarker ▲` U+25B2 (`^`), the banner's leading
glyph; its first fallback, `!`, collided with `errorGutter`'s
fallback exactly where color cannot tell the two apart, so the
fallback is `^` instead. Every fallback occupies the
same terminal-cell width as its full glyph (`ellipsis …`'s
fallback is `~`, not `...`, for exactly this reason), so a degrade
substitution never shifts a column budget decision 12 fixed at
full color. Plain Unicode only, no private-use-area or
patched-font glyphs. The analyzer rule is stated stronger than
UX-3's block list, which misses several of these blocks, and at
UX-3's repo-wide scope: outside `internal/theme`, anywhere in
the repo, **no rune or string literal containing any non-ASCII
code point** that reaches rendered output, along with UX-3's
lipgloss-constructor, ANSI-literal, and numeric-spacing rules. `internal/catkin` is the one recorded
exemption: it defines its style-parameter struct and poplar
injects theme-derived values (it cannot import the theme package
by the spinoff rule), so the analyzer skips it and the injection
contract is its gate. The gate is asked to ratify the stricter
rule (technical design section 18).

**Spacing roles** (cells and rows, the cairn gap grammar in
terminal units): `gapLabel 1` (label to its value),
`gapControl 2` (control to neighbor), `gutter 1` (marker column
to text), `padPane 1` (pane edge to content), `padModal 2/1`
(modal horizontal/vertical), `gapSection 1 row`. Pass 2 decision 3
adds `padBand 2` (chrome band inset) and `padCard 2` (card inset).
Task 3's fix round 1 adds `gapPane 2` (pane to neighboring pane),
the same plan-defect class as the divider token (ruling M3,
`docs/superpowers/plans/2026-08-19-pass-2-design-language-and-shell.md`):
LayoutMode's rail-clearance and list/reader gutter are both this
role, not a locally reinvented literal. Task 4's fix round 1 adds
`gapHint 3` (footer hint to its neighbor, and to the pinned help
hint), the same plan-defect class again (ruling CR5, same plan
doc): the footer's hint-spacing arithmetic is a spacing role, not a
locally reinvented literal. Task 7's fix round 1 adds `gapPin 6`
(the footer's reserve before the pinned help hint) and narrows
`gapHint`'s scope to the inter-hint gap alone: the pinned exemplar's
`PIN_GAP` won the conflict between the two values (ruling F1,
`docs/superpowers/plans/2026-08-19-pass-2-design-language-and-shell.md`),
so `gapHint` no longer also governs the pinned hint's gap.
Markup-level literals are forbidden by the analyzer; a screen
reaches spacing only through roles.

**Type roles**: `emTitle` (bold), `emLabel` (dim), `emValue`
(normal), `emHint` (dim italic where supported, dim otherwise).
Roles, not per-screen choices.

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

## 9. Responsive layout grammar

(Added at the gate by Geoff's directive, 2026-07-27, and refined
the same day: fully functional at traditional sizes, taking real
advantage of large windows, grounded first in research on the
sizes people actually use. Evidence:
`docs/poplar/research/2026-07-27-terminal-size-survey.md`; the
legacy model at
`docs/poplar/research/2026-07-27-poplar-responsive-design.md` is
salvaged reference.) The method: layout is formulas plus thresholds in
pure Go, computed once per `WindowSizeMsg` into one `LayoutMode`
struct every component consumes; no renderer hardcodes a width;
no CSS-style query system; no user-configurable breakpoints.

**The ladder principle.** Size changes what is shown, never what
the keys mean (C6 holds at every size). The product is a
strictly-necessary core plus an additive ladder: the core is
complete at small sizes, and each rung up adds capability the
extra space genuinely earns. Two rules keep the ladder honest.
A rung exists only where *capability* changes (a pane appears);
density, column widths, and truncation flex continuously on the
formulas inside a rung and are never rungs themselves. And each
boundary changes exactly one obvious thing, so a resize reads as
a single legible transition, never a reshuffle.

**Three rungs, plus the floor.** The evidence sized the ladder:
80×24 is still the universal default (the core must be complete
there), the usage cluster is 100-140 columns (the polish
center), splits make 60-100 columns routine, and the ultrawide
tail is 2-3% of desktops, too small to justify a fourth
capability rung, so capped reading measures are the wide rung's
behavior rather than a separate class.

| Class | Columns | What the space buys |
|---|---|---|
| floor | under 60 | the too-small state: a centered notice naming the required minimum, nothing else; never garbled chrome, never a hard block above it |
| spartan | 60-99 | the strictly-necessary core, complete: one content pane at a time, every switch-bar verb, compact columns via the formulas. Fully functional at 80×24 by test, graceful in a split at 66 |
| standard | 100-139 | + the sidebar (the one capability this boundary adds); columns relax toward full. The modal cluster lives here; this rung is polished hardest |
| wide | 140+ | + the split: list beside reader, reading while triaging, each pane at or above its minimum. Within the rung, reader content caps at ~100 cells and Catkin's writing measure at ~80; surplus becomes calm centered margin, the iA instinct |

Further capability at extreme width (a docked calendar peek, a
thread-context pane beside compose) is deliberately not a v1
rung: the pane-priority model admits a fourth pane later without
restructuring, and any such extra must individually pass C11's
lean test when proposed. The rung count may shrink but not grow
without amending this document.

**Height classes.** Short windows are a first-class condition,
not an edge case (the largest real cohort is likely embedded
editor terminals, short and width-constrained): under 20 rows,
chrome compresses first (footer to one row, banners demote to
toasts) before any content degrades; 20 rows and up carries full
chrome; grid calendar views additionally require the rows they
declare and degrade by name to the agenda below that (CA-3's
views are never squeezed into illegibility). Below 15 rows is
the floor state.

**Composition rules.**

1. Panes appear whole or not at all. Every pane declares a
   minimum usable width; the layout drops panes in priority
   order (split reader first, then sidebar) rather than
   squeezing any pane below its minimum. Content outranks
   chrome; the footer and status line survive to the floor.
2. Columns follow the coverage-cliff method: continuous slopes
   for smoothly scaling values (sender, sidebar), discrete
   thresholds for enums (date format, icon visibility),
   thresholds at round numbers, boundary-tested at threshold
   plus and minus one. The legacy formulas are the starting
   values; Phase 5 re-derives the cliffs from the measurement
   spike's 36k-message harvest, which is a better corpus than
   the legacy sample.
3. List columns degrade by priority: flags and date yield before
   sender, sender truncates before subject, and subject never
   drops below the ~30-cell preview-readable floor while the
   pane meets its minimum.
4. Resize preserves state: cursor, scroll, selection, and every
   text-entry buffer survive relayout byte-for-byte; Catkin
   recomputes soft wrap without touching content; relayout
   completes within one frame (QA-2's budget applies to
   `WindowSizeMsg` like any message).
5. Modals and pickers clamp against the terminal and center;
   they have natural sizes, not slopes.

**Testing.** The named classes are a golden-matrix dimension:
each screen's goldens render at one representative size per
class whose layout differs, plus boundary sizes at each
threshold the screen consumes, folded into the QA-7 profile
matrix. Phase 5 wireframes are drawn per class that changes the
screen's layout (spartan, standard, and wide at minimum for mail
screens), and the too-small floor state is itself a golden.

## 10. What Phase 5 fills in

This document pins the contracts; the build fills the values
inside them, and only inside them:

- Palette values for the color roles, both themes, passing the
  contrast tests. The palette is poplar's; cairn's Warm
  Stone is the craft exemplar, not the hue source.
- Exact glyph code points and border sets.
- Per-screen keymaps and layouts, via text wireframes citing
  this grammar, one wireframe per screen before its build pass.
- The remaining screen-verb assignments (link mode, RSVP answer
  keys, fallback-stack cycle), bound by the grammar tables, the
  two named exemptions, and the registry test.

**The visual exemplar.** Pass 2 ratifies and pins the shell
composition at
`docs/poplar/design/2026-08-19-shell-exemplar/` (generator, ANSI
render, stripped render, README): the design language's visual
reference for every later screen. Help, config, calendar,
contacts, and compose design from its ground grammar, hint atom,
edge-bar vocabulary, card anatomy, and copy register before
consulting the rules above, the way a cairn site designs from the
cairn system (`docs/superpowers/plans/2026-08-19-pass-2-design-language-and-shell.md`,
decision 13).

Amendments to the grammar, the exemption list, the exception
list, or the component vocabulary go through this document first;
the analyzer and registry tests make silent divergence a build
failure.
