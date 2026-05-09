# Bubble consolidation verdict (Pass 9y)

The bubble-evaluation roadmap closed in Pass 9y. Two formal evals
(A — five strong matches; B — four medium matches) and three
harvest passes (9x.1 list family, 9x.2 chrome family, 9x.3
table/form family) ran the same rubric over nine community
libraries against poplar's hand-rolled equivalents.

Every verdict was **Keep + harvest**. The cumulative adoption
count across all five exercises:

- One inline adoption (movepicker filter-match rune highlight,
  9x.1 §1, landed alongside the catalog).
- One adjacent cleanup landed during 9x.3 (`contacts.Styles.Border`
  removed; not a harvest).
- One "candidate for later" worth keeping in mind: `bubbles/help`
  `shouldAddItem` progressive-truncation loop (9x.2 §3) — useful
  if helppopover ever needs to drop rightmost groups before
  falling back to a whole-box `tooNarrow` string. No current
  trigger.

Everything else is catalog-only. The catalogs at
`docs/poplar/research/2026-05-08-{bubbles-list,chrome-family,table-form}-patterns.md`
are the durable record.

## Per-library Keep + flip condition

Each line names the specific upstream change that would have to
land for a future pass to revisit the verdict. Absent that change,
the eval doesn't need re-running.

### `rmhubbert/bubbletea-overlay`

Same Superfile-derived cell-compositing algorithm as poplar's
`PlaceOverlay`. The library has no cascade primitive; poplar's
nine-level `if IsOpen()` chain in `App.View` stays in App
regardless. Adoption trades a hand-roll for a dependency with no
behavior change.

**Flip if** the library grows a cascade-ordering primitive with
mutual exclusion and priority. Today it composites two `Viewable`
values; that's not the gap.

Source: Eval A (`bubbletea-overlay` section).

### `bubbles/help`

Spatial named-group grid (three columns, 3×2 Go To tile, footer
hint outside any group) cannot be expressed through `KeyMap`'s
`ShortHelp() []key.Binding` / `FullHelp() [][]key.Binding`
interface. `FullHelpView` calls `lipgloss.JoinHorizontal`
internally — banned under SPUA-A icon mode (ADR-0084). The library
also drops the `wired` dim affordance (planned-binding visibility
is a first-class poplar feature, not a help-bar nicety).

**Flip if** helppopover drops the named-group grid contract *and*
abandons the `wired` dim affordance *and* ADR-0084 narrows. Three
preconditions; none on the table.

Source: Eval A + 9x.2 §1–§6.

### `daltonsw/bubbleup`

Domain mismatch. The library renders categorized status alerts
with `go-colorful` Lab-space lerp animation against a hardcoded
`#000000` background. Poplar's toast carries a `triageOp` payload,
a `tea.Cmd` inverse for undo, and a monotonic deadline — none of
which the library exposes. Adoption adds two deps and a 10 Hz tick
for zero LOC reduction; the entire `renderToast` function is the
domain content the library cannot host.

**Flip if** poplar adds a separate animated-notification surface
alongside the triage banner (post-1.0 only). The triage banner
itself is the wrong consumer.

Source: Eval A + 9x.2 §1–§5 (bubbleup).

### `charmbracelet/huh`

Two structural blockers. `huh.Group` accepts fields at
construction and never mutates the field list — poplar's contacts
form rebuilds `focusList()` on every email/phone add/remove.
`huh.Form.View()` has no body-only render mode — Contacts mode
mounts the form as a right-pane column where the frame supplies
borders, and stripping huh's chrome means forking `form.go` +
`group.go`.

**Flip on the first-run wizard (Pass 14)** if the wizard field set
stays static (account name, provider, host, port, credentials).
That decision belongs to the Pass 14 plan; this eval does not
foreclose it. Contacts/form remains hand-rolled regardless.

Source: Eval A + 9x.3 §1–§11 (huh).

### `evertras/bubble-table`

`renderRowData` and `renderHeaders` both call
`lipgloss.JoinHorizontal` to assemble column cells — banned under
SPUA-A icon mode (ADR-0084), and the SPUA-A flag-cell compensation
poplar performs manually cannot be expressed through the column-
width API. Threading depth-prefix walks (`├─ │ └─`),
`ActionTargets()` thread-aware expansion, fold state, and the
UID-keyed `marked` map all live outside the rectangular `RowData`
shape.

**Flip if** the library removes `JoinHorizontal` from row + header
render *and* messagelist drops threading. Threading is the reason
messagelist exists; the second condition won't hold.

Source: Eval A + 9x.3 §1–§11 (bubble-table).

### `bubbles/list` — three consumers

Eval B compared `bubbles/list` against three distinct surfaces.
All three landed Keep for overlapping reasons.

**Pickers** (movepicker / linkpicker / attachpicker). Chrome-on-by-
default is wrong for inline-splice modal lists; `SetShowFilter(false)`
etc. leave layout residue. The two-phase `FilterState` machine
(Filtering / FilterApplied / Unfiltered) buys nothing for poplar's
per-keystroke `recompute`. Paginator math is dead weight. The 13
chrome slots in `Styles` would import dead.

**Compose dropdown.** A 7-row positional splice below the focused
header row is structurally different from a self-contained list
component. Both `bubbles/list` and `treilik/bubblelister` carry
chrome and lifecycle that the dropdown doesn't need.

**Sidebar folder column.** Three permanent groups (Primary,
Disposal, Custom) with blank-line separation, nested-flat display
of `Folder/Sub/Path`, and no internal scroll — the column is a
fixed-tall composite, not a list. A `list.Model` per group adds
three lifecycles for one synthetic visual.

**Flip if** the library grows a chrome-off-by-default mode *and*
drops the two-phase `FilterState` *and* exposes positional splice
as a primary shape. Three preconditions across two unrelated
subsystems; not realistic.

Sources: Eval B (three sections), 9x.1 §1–§9.

### `mistakenelf/teacup` statusbar

Domain mismatch. Teacup's four positional slots
(First/Second/Third/Fourth) don't fit poplar's six semantic
segments (fill, counts, outbox glyph, connection icon, connection
text, terminator) with a left-side fill that absorbs width
variance. The library's final assembly calls
`lipgloss.JoinHorizontal` (ADR-0084 anti-pattern, named here as
the canonical call site).

**Flip if** the library grows a fill-absorbs-variance segment
shape. The four-slot positional contract is load-bearing for
teacup's file-manager domain; no upstream signal of change.

Source: Eval B + 9x.2 §1–§5 (teacup).

## Cross-cutting outcomes

The harvest produced no cross-package extractions. The
`uicore.ListBodyRows` candidate from 9x.1 (movepicker
`visibleRows` + reader `visibleLinkRows` + compose/AttachPicker
`viewportRows`) sits at three sites with three different reserve
constants (7, 7, 5); 9x.3 confirmed messagelist and contacts/list
are full-pane lists with a different layout path, so they don't
add a fourth consumer. Re-evaluate when a fourth modal-list
surface appears.

## Future-bubble policy

A future evaluation of any of these nine libraries (or a sibling
in the same family) does not need a fresh rubric pass. It needs:

1. The named flip condition above.
2. Evidence that the upstream change has landed.
3. Evidence that poplar's consumer-side contract (named in the
   blocker) is unchanged or has been brought into the new shape
   deliberately.

Without all three, the verdict stands. New libraries (not surveyed
in Eval A or Eval B) get a fresh rubric run; the bar is the same
core question: *does adopting the bubble make poplar better?*
