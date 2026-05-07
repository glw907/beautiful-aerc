---
title: Pass 9.1b — Contact edit form
status: planned
date: 2026-05-07
---

# Goal

Land the contact edit form (Person + Business). Two render contexts:
centered modal over dimmed mail chrome (popover "no match" → `n`), or
right-pane replacement in Contacts mode (`n`/`e` from the list).

Reference:
- Spec §"Contact edit form" (`docs/superpowers/specs/2026-05-06-addressbook-design.md`).
- Archived plan Task 8 (`docs/superpowers/archive/plans/2026-05-07-address-book-mockups.md`).
- ADR-0166 locks package shape and Msg contracts.

# Files

- Create `internal/ui/contacts/form.go` — `Form` model, `NewForm`,
  `Update`, `View`, `Validate`. `bubbles/textinput` per single-line
  field; `bubbles/textarea` for note. ModalShell when
  `fromPopover`, plain panel otherwise.
- Create `internal/ui/contacts/form_test.go` — kind toggle, row
  add/remove, primary promotion, validation matrix, save/cancel msgs.
- Modify `internal/ui/contacts/keys.go` — add form-local bindings
  (Tab/Shift+Tab/Ctrl+S/Space + ←/→ on toggle, ★/− buttons).
- Modify `internal/ui/app.go` — `form *contacts.Form` field; route
  keys + WindowSizeMsg while non-nil; handle `OpenFormMsg`,
  `ContactSaveMsg`, `ContactCancelMsg`; wire ConfirmModal "Discard
  changes?" path.
- Modify `docs/poplar/wireframes.md` — Person + Business form
  wireframes (already in spec; mirror into wireframes doc for
  on-demand reading).
- Modify `docs/poplar/keybindings.md` — Form section: Tab cycle,
  Ctrl+S save, Esc cancel, Space/← / → on kind, +/★ /− on rows.

# Form shape

```go
type Form struct {
    styles      Styles
    initial     Contact
    kind        Kind
    fromPopover bool
    saveTo      []string
    saveIdx     int

    // Person fields.
    first textinput.Model
    last  textinput.Model
    org   textinput.Model
    title textinput.Model

    // Business field.
    bizName textinput.Model

    emails []emailRow
    phones []phoneRow
    note   textarea.Model

    focusIdx int
    err      string
    width, height int
}

type emailRow struct {
    input textinput.Model
    label int // 0=Work 1=Home 2=blank
}

type phoneRow struct {
    input textinput.Model
    label int // 0=Mobile 1=Work 2=Home 3=Fax 4=blank
}
```

Focus order:
- Person: KindToggle, First, Last, Org, Title, [Email rows: input,
  cycler, ★, −] for each, +AddEmail, [Phone rows: input, cycler, ★,
  −] for each, +AddPhone, Note, SaveTo radio.
- Business: KindToggle, Name, [Email rows...], +AddEmail, [Phone
  rows...], +AddPhone, Note, SaveTo radio.

`primary email` is always row 0 of `emails`; ★ on row 0 is inert
(rendered as `★` filled). Rows 1+ render `☆`; pressing Enter on the
☆ button promotes that row to index 0 (slice rotate). `−` deletes the
row; disabled when only 1 email remains.

Phones are the same except minimum is 0 (zero rows allowed).

`Tab`/`Shift+Tab` cycle the focusable widget list. The focused widget
receives `Enter` and other inputs; cyclers receive `Space` /
`←` / `→` to cycle.

Validation (run on `Ctrl+S`):
- Person: first or last non-empty; Business: bizName non-empty.
- ≥1 email row, all addresses parse via `mail.ParseAddress`.
- All non-empty phones must be reasonable (Pass 9.1b accepts any
  non-blank string — strict E.164 lands when phonenumbers ships in
  9.2 per spec §Library: phonenumbers).
- saveIdx in range.

Failure: set `f.err` to a one-line message naming the offending
field; refocus the offending field; return no Cmd. Success: emit
`ContactSaveMsg{Contact: assemble(f), SaveTo: f.saveTo[f.saveIdx]}`.

`Esc` returns `ContactCancelMsg{Dirty: f.dirty()}` where dirty is
`!equal(initial, current)` (re-derived from form state — no shadow
flag, easier to reason about).

# App lifecycle

```go
type App struct {
    // ...
    form *contacts.Form
}
```

- `OpenFormMsg`: build saveTo `["Local file", account.Name()]`
  (Pass 9.1 hard-codes single-account; multi-account in 9.2). Pass
  9.1b: account name is `m.acct.AccountEmail()` since `account.Model`
  exposes that already; see `account_email.go`. Mount form,
  `m.popover = nil`.
- WindowSizeMsg while form != nil: forward via `form.SetSize(w, h)`.
- KeyMsg routing: form is highest priority *after* confirm modal.
  Cascade: confirm > conflict > outbox > help > linkpicker >
  attachpicker > movepicker > **form** > popover > compose.
- `ContactSaveMsg`: log+discard for 9.1b (no contacts cache yet).
  Clear form. Pre-beta is fine with this — 9.2 wires the cache.
- `ContactCancelMsg{Dirty}`: if !Dirty, clear form. If Dirty, open
  ConfirmModal "Discard changes?" and set
  `pendingFormDiscard = true` (mirrors the existing
  `pendingComposeSave bool` discriminator pattern).
  - Yes → clear form.
  - No / Esc → keep form open.

# Render

- Right-pane mode (`!fromPopover` and contactsMode): build the
  contacts frame (sidebar + list still drawn, inert), then replace
  the right (detail) column with `form.RightPane()` — a column-fit
  rendering using `Styles.Border` panel. Sidebar/list stay drawn
  but updated keys never reach them while form is open.
- Modal mode (`fromPopover`): underlying account frame is dimmed
  via `DimANSI` and `form.Box(width, height)` is composited via
  `PlaceOverlay`. Position via `CenterOverlay`. Use `ModalShell`
  for chrome — `Form` embeds `uicore.ModalShell` like the popover.

Width budget for the modal: 60 cells content (matches popover) when
terminal allows, else `width - 4` floor.

# Self-review

- Tests cover the validation matrix and msg shapes.
- All width math via `lipgloss.Width`/`uicore.PadOrTruncate`; no
  `len()` on user-visible strings.
- `Ctrl+S` is text-entry exempt per ADR-0076.
- Font of icons: `★` `☆` `−` `+` are EAW Na/N runes (lipgloss.Width
  == 1 in both icon modes); confirm in tests.
- ConfirmModal multiplexing: introduce `pendingConfirmKind` rather
  than juggling three distinct boolean fields (`pendingComposeSave`,
  `pendingEmpty.folder != ""`, plus new form-discard). Inline
  refactor — pre-beta posture endorses this and the existing fields
  are awkward. Do as part of this pass.
