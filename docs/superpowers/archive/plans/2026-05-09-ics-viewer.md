---
title: Pass 12 — `.ics` calendar invite viewer
status: in-progress
date: 2026-05-09
spec: docs/superpowers/specs/2026-05-09-ics-viewer-design.md
---

## Goal

Display-only viewer for `.ics` calendar invites — first VEVENT
rendered as an inline block between header panel and chip row.
Spec settles every open question; this plan enumerates the
implementation tasks.

## Settled (from spec)

- Library: `github.com/arran4/golang-ical` (MIT, go.mod dep, no
  vendoring).
- Package: new `internal/icalendar/` exporting `Invite`,
  `ParseInvite`, `ErrNoEvent`.
- First VEVENT only; multi-event files surface only the first.
- All times rendered in `time.Local`.
- Parse failure → hide the block (and log nothing — same branch
  as ErrNoEvent).
- Surface is an inline block, not a chip + popover. The
  underlying `.ics` part still appears in the attachment chip row
  for save-to-disk.
- Recurrence humanization is FREQ + INTERVAL only; anything
  fancier yields `""` and the row is omitted.
- `[CANCELLED]` row above Summary on `Method == "CANCEL"`,
  styled `ColorWarning`.
- Icon: `📅` fancy / `[i]` simple via new
  `uicore.IconSet.Calendar` field.

## Tasks

The pass fits within the 8–12 task budget. Tasks 1–2 are
independent (domain package vs UI plumbing seam) and ship in
parallel; tasks 3–6 sequence after.

### Task 1 — `internal/icalendar/` domain package

**Files:** `internal/icalendar/invite.go`,
`internal/icalendar/invite_test.go`,
`internal/icalendar/testdata/*.ics`

- Add `github.com/arran4/golang-ical` to `go.mod` via
  `go get`.
- Implement value type:
  ```go
  type Invite struct {
      Summary       string
      Start, End    time.Time
      Location      string
      Organizer     string
      AttendeeCount int
      Method        string
      Recurrence    string
  }
  ```
- `func ParseInvite(b []byte) (Invite, error)`:
  - Empty input → `ErrNoEvent`.
  - Parser error → `ErrNoEvent` (callers don't distinguish).
  - No VEVENT in calendar → `ErrNoEvent`.
  - First VEVENT only (document in godoc).
  - Empty Summary → `"(no title)"`.
  - Method comes from VCALENDAR-level `METHOD` property,
    uppercased.
  - Organizer comes from `ORGANIZER` CN parameter, falling back
    to the mailto address.
  - AttendeeCount = number of `ATTENDEE` properties.
  - Start/End from `DTSTART`/`DTEND`; respect TZID where the
    library exposes it, else interpret as local.
- `func humanizeRRULE(rule string) string`:
  - `FREQ=DAILY`, `INTERVAL` → `"Every day"` /
    `"Every N days"`.
  - `FREQ=WEEKLY` → `"Every week"` / `"Every N weeks"`.
  - `FREQ=MONTHLY` → `"Every month"` / `"Every N months"`.
  - `FREQ=YEARLY` → `"Every year"` / `"Every N years"`.
  - Anything containing BYDAY / BYMONTHDAY / COUNT / UNTIL /
    EXDATE / non-trivial keys → `""`.
- `var ErrNoEvent = errors.New("icalendar: no VEVENT in part")`.
- Table-driven tests with real-world fixtures in `testdata/`:
  Google REQUEST, Outlook REQUEST with TZID, recurring weekly,
  all-day event, CANCEL, multi-event PUBLISH (assert first
  event), malformed input (`ErrNoEvent`).
- `humanizeRRULE` table tests for each FREQ + interval pair and
  the drop-cases.

### Task 2 — Body-fetch wiring & `BodyLoadedMsg.Invite`

**Files:** `internal/ui/reader/msgs.go`,
`internal/ui/account/cmds.go`

- Extend `reader.BodyLoadedMsg` with
  `Invite *icalendar.Invite // nil when absent`.
- In `account.fetchBodyCmd` (currently the body-walk loop near
  line 188), add a third branch in the part-type switch for
  `text/calendar` and `application/ics` (case-insensitive,
  parameter-tolerant — `gomail.InlineHeader.ContentType` already
  strips params, so a direct match works for the canonical
  types; lowercase compare against `"text/calendar"` and
  `"application/ics"`).
- Read part bytes (the existing `io.ReadAll(p.Body)` already
  produces them — branch the switch *after* the read).
- Call `icalendar.ParseInvite`. On any error, leave invite nil.
- Thread the resulting `*icalendar.Invite` onto the
  `BodyLoadedMsg` constructor.
- Update `account/model.go`'s `BodyLoadedMsg` handler if it
  needs to forward the invite into reader state (it currently
  routes the msg into `reader.Model` via the standard child
  Update — the reader handles the field).
- Add a unit test covering the body cmd's invite-extraction
  branch using a fixture from Task 1.

### Task 3 — Reader state + render

**Files:** `internal/ui/reader/model.go`,
`internal/ui/reader/styles.go`,
`internal/ui/uicore/layout.go`

- Add to `reader.Model`:
  ```go
  invite       *icalendar.Invite
  inviteRow    string
  inviteHeight int
  ```
- On `BodyLoadedMsg`, store `msg.Invite`, then call a private
  `rerenderInvite()` that produces `inviteRow` / `inviteHeight`.
- Add `renderInviteBlock(*icalendar.Invite, IconSet, Styles, width int) (string, int)`
  in a new `reader/invite.go`. Pure function — no I/O, no
  state. Layout (top to bottom):
  - `[CANCELLED]` row when `Method == "CANCEL"` (styled
    `InviteCancelled`).
  - `<icon>  <Summary>` (icon styled `InviteIcon`, Summary
    `InviteSummary`).
  - 4-cell indented rows for When / Where / Organizer /
    Attendees / Recurrence (each styled `InviteField`,
    omitted when source is empty).
- When formatting:
  - Same-day end → `Wed 2026-05-14, 3:00 PM – 4:00 PM`.
  - Cross-day end → both dates.
  - All-day (`Start.Hour()==0 && Start.Minute()==0 &&
    End.Sub(Start)==24*time.Hour`) → `Wed 2026-05-14 (all day)`.
  - Zero start → `When: (no start time)`.
- Width math via `ansix.Width` and truncation via
  `ansix.Truncate` — every row fits within the panel-width
  budget.
- Update `Layout()` so body height = `height - panelHeight -
  inviteHeight - chipHeight`. The invite row joins the
  assembled view between panel and chips, with the chip-row
  blank-line separator preserved when both are present.
- Add `InviteIcon`, `InviteSummary`, `InviteField`,
  `InviteCancelled` to reader styles.
- Add `Calendar` field to `uicore.IconSet`. Populate
  `SimpleIcons.Calendar = "[i]"`,
  `FancyIcons.Calendar = "📅"` (verify class invariants:
  `[i]` is Na/N width 1; `📅` is the SPUA-A range — actually
  `📅` is U+1F4C5, an emoji, **not** SPUA-A. The class test
  expects fancy icons in `[U+F0000, U+FFFFD]`. Pick a Nerd Font
  glyph in that range instead — `nf-md-calendar_check` U+F0E68
  works. Confirm against `internal/ui/uicore/layout_test.go`
  invariants and update simple/fancy values accordingly.)
- Golden tests in `internal/ui/reader/model_test.go` (or a new
  `invite_test.go`) for: invite present, invite absent,
  cancelled, all-day, recurring, with attachments, without.
  Cover the height-arithmetic invariant.

### Task 4 — Docs (styling, wireframes, invariants)

**Files:** `docs/poplar/styling.md`,
`docs/poplar/wireframes.md`,
`docs/poplar/invariants.md`

- Append a row group for the four invite styles to
  `styling.md` mapping palette tokens → surfaces.
- Update the viewer wireframe in `wireframes.md` to show the
  invite band between header panel and chip row in a sample
  message with both present.
- Update `invariants.md` Viewer §: rewrite the existing chip-
  row fact to read "Invite block + chip row sit between header
  panel and body, in that order, both optional."

### Task 5 — ADR-0186

**File:** `docs/poplar/decisions/0186-ics-viewer.md`

- Title: `.ics calendar invite viewer (display-only)`.
- Context: aerc-aligned plain-text inline block; spec
  rationale on first-VEVENT-only and no-RSVP-in-v1.
- Decision: single binding paragraph that ends in invariants.md.
- Consequences: unlocks future RSVP / `internal/calendar/` pass;
  forecloses chip+popover migration without a layout shift.
- Update `docs/poplar/decisions/INDEX.md` with the new entry
  under the Viewer / mail-rendering theme.

### Task 6 — Verify + pass-end ritual

- `make check` green.
- `/simplify` against the diff; apply genuine wins.
- Idiomatic-bubbletea checklist (§10) against the reader diff.
  Capture tmux at 80×24 and 120×40 — once with a real Fastmail
  invite delivered to `geoff@907.life`, once without.
- Archive plan + spec via `git mv`.
- Update STATUS.md: mark Pass 12 done; write Pass 13 starter
  prompt for Search (#38).
- Commit + push + `make install` per ship workflow.

## Risks / open seams

- `arran4/golang-ical`'s API for TZID and ORGANIZER access may
  require helper funcs against `*ical.Property`. Task 1
  surveys the library upfront; if the API is hostile, document
  a thin internal accessor and continue — no library swap in
  scope.
- The icon-class invariant (fancy in U+F0000–U+FFFFD) blocks
  emoji `📅`. Task 3 picks a Nerd Font calendar glyph; flag if
  none exists in the Nerd Font set the project uses.
- Layout height math interacts with the existing chip-row
  blank-line separator; golden tests in Task 3 lock it down.

## Subagent-driven dispatch

Tasks 1 and 2 are independent and dispatched in parallel.
Task 3 depends on both (uses the value type from 1 and the
msg field from 2). Tasks 4–6 sequence after 3. Pass-end
ritual is the controller's responsibility, not a subagent's.
