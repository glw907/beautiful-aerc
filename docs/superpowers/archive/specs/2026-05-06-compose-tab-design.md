# Pass 9h — ComposeTab design

*Spec frozen 2026-05-06. Implementation plan follows.*

## Goal

Land `ComposeTab`, the inline compose surface that wraps a Catkin
body editor and stacked address/subject text inputs. Wire `c`,
`r`, `R`, `f` to open it. Send goes through the cache outbox
(Pass 9g, ADR-0158). Drafts persistence, autocomplete,
signatures, undo-send, and richer attach UI are explicitly out —
each owns a later pass.

## Scope

In:

- `internal/ui/compose_tab.go` — new `ComposeTab` model.
- `internal/ui/app.go` — `compose *ComposeTab` field, `composeOpen`
  active-screen flag, key wiring for `c` (and routing for `r`/`R`/
  `f` to seed via `compose.Seed*`).
- `internal/ui/keys.go` — Catkin-context bindings on compose
  (`Ctrl+X` send, `Ctrl+C` cancel).
- `internal/cache/account.go` — new `QueueOutbound(ctx, sentFolder,
  env, mime)` helper that branches IMAP (two ops) vs JMAP (one op)
  internally, so ComposeTab does not learn the protocol shape.
- Tidy seam — `TidyFn` function pointer on `App`, no-op identity
  for 9h. Pass 9i replaces.
- Tests + golden tmux captures at 80×24 and 120×40.

Out:

- Drafts persistence — Pass 9h.5.
- Address autocomplete — Pass 9.1.
- Signatures + multiple identities — Pass 9.4.
- Undo-send + scheduled send — Pass 9.2.
- Attachments-richer compose UI — Pass 9.5 (raw attach via path
  list works through `Draft.Attachments` and `compose.AssembleMIME`,
  but no picker UI in 9h).

## Architecture

### App-level shape

`App` grows two fields:

```go
compose     *ComposeTab // nil when not open
composeOpen bool        // active-screen selector
tidy        TidyFn      // body rewriter; no-op in 9h
```

When `composeOpen`, `App.View()` renders chrome + sidebar column
+ `ComposeTab.View()` in the right pane (in place of
`AccountTab`'s msglist/viewer pane). Chrome stays drawn so the
status bar / footer / error banner / outbox segment remain
visible. Overlays (`Q`, `!`, confirm modal) work over compose
the same way they work over the account view.

`Update`'s key routing: while `composeOpen`, keys flow into
`ComposeTab.Update` first. `Ctrl+C` from compose triggers
discard-confirm via the existing `ConfirmModal` (empty draft
closes immediately, no confirm). `Ctrl+X` from compose triggers
send — emits `composeSendMsg{draft}` which `App` translates into
a `QueueOutbound` Cmd, then closes compose on success.

ComposeTab does not hold a `*cache.Account` reference; it's
presentation-only. The send path is a `tea.Msg` returned to App
which owns the cache handle. This matches the
`MessageList`/`Viewer` boundary already established in 9g.

### Single-instance for 9h

One compose at a time. Opening a new one while compose is open
is rejected (key inert) — multi-compose / draft list is 9h.5.
The `ComposeTab` value is heap-allocated (`*ComposeTab`) so a
nil pointer represents "no compose" cleanly; this matches
existing optional-overlay patterns (`pendingEmptyConfirm`).

### Header layout

Five rows above the body, all bubbles `textinput`:

```
From:    geoff@907.life                  (read-only label)
To:      ▍                               (textinput)
Cc:                                      (textinput, may be empty)
Bcc:                                     (textinput, may be empty)
Subject: ▍                               (textinput)
─────────                                (rule)
<catkin body fills remaining height>
```

`From` is a `lipgloss`-styled label, not an input — single
identity in v1. Cc and Bcc rows are always visible, empty when
unused (aerc precedent; avoids a reveal-toggle whose key
conflicts with typing).

Address inputs accept comma-separated raw RFC 5322 ("Name
<addr>" or bare email). Validation happens at send via
`content.ParseAddressList` — parse failure surfaces as an inline
red row above the rule with the offending field name. No
real-time validation.

### Focus model

Six focusable fields: `To`, `Cc`, `Bcc`, `Subject`, `Body`. Each
header is a `bubbles/textinput` with its own `Focused()`. Body
focus is the Catkin editor's own `Focused()` via the
`compose.Editor` seam.

`ComposeTab` holds `focus int` (0..4) and a `focusable []focusItem`
slice; `setFocus(i)` blurs the previous and focuses the new.
Header inputs have `KeyTab`/`KeyShiftTab` disabled (we own
cycling); `KeyEnter` advances to the next field.

Cycling:

- **`Tab`** — next field, wraps. Headers → Body wraps to To.
- **`Shift+Tab`** — previous field, wraps.
- **`Enter` in a header** — advances to next field (line break
  only fires inside Body).
- **`Esc` in Body** — focus returns to Subject. Esc never closes
  compose.
- **`Esc` in a header** — focus returns to Body.

Closing compose is exclusively `Ctrl+C` (with confirm-if-dirty).
This eliminates the two-stage-Esc idea proposed earlier — Esc is
purely a focus key.

### Send key + cancel

Per ADR-0076, text-entry surfaces are exempt from the
modifier-free rule. Catkin already uses `Ctrl+B/I/K/L/Q/Space`;
adding `Ctrl+X`/`Ctrl+C` extends the same modifier-allowed
pocket to compose chrome.

- **`Ctrl+X`** — send. Pine convention. Available from any focus.
- **`Ctrl+C`** — cancel. Pine convention. Confirms via
  `ConfirmModal` when the draft is dirty (any header value or
  body content). Empty draft closes silently. `Ctrl+C` also
  surfaces as the standard terminal-kill on the account view —
  in compose context it's a deliberate rebind, advertised in
  the compose footer.

The compose footer drops the account-context hints and
advertises `^X send  ^C cancel  Tab next  ⇧Tab prev`. Footer
height is 1 row (matches account context), drop-rank policy
unchanged.

### Send path

ComposeTab assembles bytes via `compose.AssembleMIME(draft,
time.Now())`. The send op is a single `tea.Msg`:

```go
type composeSendMsg struct {
    sentFolder string
    env        mail.Envelope
    mime       []byte
}
```

App's handler invokes `tidy` on the body before assembly (no-op
in 9h), calls `acct.QueueOutbound(ctx, sentFolder, env, mime)`,
sets `composeOpen = false`, drops `compose = nil`, and emits a
"Sending…" toast (reuses existing `pendingAction` chrome but
without an undo affordance — outbox visibility lives in `Q`).

`*cache.Account.QueueOutbound` is a thin protocol-aware helper:

```go
func (a *Account) QueueOutbound(ctx context.Context, sentFolder string,
    env mail.Envelope, mime []byte) error {

    if _, err := a.QueueSend(ctx, sentFolder, env, mime); err != nil {
        return err
    }
    if a.Backend.IsJMAP() {
        return nil // JMAP server lands the Sent copy atomically
    }
    _, err := a.QueueAppend(ctx, sentFolder, mail.FlagSeen, mime)
    return err
}
```

The IMAP/JMAP branch lives in `cache`, not in UI. The
`mail.Backend.IsJMAP()` accessor is added in 9h (one-line
predicate per backend; trivial impls).

`sentFolder` resolution: classify the cached folder list
(`acct.ListFolders()`) and pick the `RoleSent` folder. If none
classified, fall back to first folder named `Sent` (case-fold).
If that's also missing, abort send with an inline error row in
the compose ("no Sent folder configured"). Real edge case for
self-hosted IMAP without SPECIAL-USE.

### Tidy seam

```go
// TidyFn rewrites the markdown body before MIME assembly. Pass 9h
// ships a no-op identity. Pass 9i swaps in Claude Tidy.
type TidyFn func(ctx context.Context, body string) (string, error)
```

Stored on `App`; `App.WithTidy(fn) App` setter mirrors
`WithOpener`. `cmd/poplar/root.go` plumbs the production
implementation. 9h wires `func(_ context.Context, b string)
(string, error) { return b, nil }`. The seam exists so the next
pass replaces a function reference rather than rewiring
ComposeTab. No interface, per the no-single-impl-interfaces
invariant.

### Seed paths

Reply/forward routing lives in App's account-view key handler
(unchanged for delegation). When `r`/`R`/`f` fires:

1. Read selected message UID + raw bytes via cache (`acct.FetchBody`).
2. Build `Draft` via `compose.SeedReply(parent, body, self)` /
   `SeedReplyAll` / `SeedForward`.
3. Construct `ComposeTab` seeded with the draft, set
   `composeOpen = true`.

`self` is the configured account email (`acct.AccountEmail()`).
For SeedReplyAll, this is the address removed from the synthesized
recipient list per ADR-0156. The parent body fetch reuses the
existing `loadBodyCmd` codepath (Cmd-returning); compose opens
into a "loading…" placeholder until `bodyLoadedMsg` arrives,
mirroring the viewer.

### Empty `c` (new compose)

Compose opens immediately with a fresh `Draft{From: <self>}`
and focus on `To`. No async work.

## Components

### `ComposeTab` (new, `internal/ui/compose_tab.go`)

```go
type ComposeTab struct {
    styles  Styles
    icons   IconSet
    draft   compose.Draft
    from    string

    to      textinput.Model
    cc      textinput.Model
    bcc     textinput.Model
    subject textinput.Model
    editor  compose.Editor // CatkinEditor

    focus  int
    err    string // inline error row above rule, empty when none
    width  int
    height int
}
```

Methods: `New(styles, t, self string, icons) ComposeTab`,
`Init() tea.Cmd`, `Update(tea.Msg) (ComposeTab, tea.Cmd)`,
`View() string`, `SetSize(w, h int)`, `IsDirty() bool`,
`Draft() compose.Draft` (rebuilds from current input values),
`Seed(d compose.Draft)`.

`View()` self-enforces width via `clipPane` (matches existing
component contract). Width math through `displayCells` for
icon-bearing strings; pure text uses `lipgloss.Width`.

Header label column is fixed-width 9 cells (`Subject:` is the
longest at 8 + space). Inputs occupy `width - 9 - 1` cells
(account for right border). Body inherits the editor's wordwrap
+ hardwrap.

### `ComposeKeys` (new, `internal/ui/keys.go`)

```go
type ComposeKeys struct {
    Send         key.Binding // ctrl+x
    Cancel       key.Binding // ctrl+c
    NextField    key.Binding // tab
    PrevField    key.Binding // shift+tab
    EscapeBody   key.Binding // esc
    AdvanceField key.Binding // enter (in headers only)
}
```

Dispatched via `key.Matches` per the conventions doc.

### `App` deltas

- New fields `compose *ComposeTab`, `composeOpen bool`, `tidy TidyFn`.
- `WithTidy(fn) App` setter.
- `c`/`r`/`R`/`f` key routing — `c` synchronous, `r`/`R`/`f`
  through the body-fetch Cmd path.
- `composeSendMsg` handler — invokes tidy, calls
  `acct.QueueOutbound`, closes compose, emits toast.
- `ConfirmModalYesMsg` handler grows a `pendingComposeDiscard`
  branch (mirrors `pendingEmptyConfirm`).
- `View()` selects compose vs. acct in the right pane based on
  `composeOpen`.
- `WindowSizeMsg` forwards into compose (when present) per the
  conventions checklist.

### `cache.Account.QueueOutbound`

Thin wrapper over `QueueSend` + conditional `QueueAppend`, as
shown above. Tested at the cache layer with a fake Backend that
flips `IsJMAP()` between true/false.

### `mail.Backend.IsJMAP() bool`

New one-line predicate. JMAP impl returns true; IMAP impl
returns false. Surfaced from the existing backend handles.

## Wireframe (120×40, compose open, fresh `c`)

```
┌─ Inbox ─────────────────────────┬─ New message ────────────────────╮
│ ●●○ Connected                   │                                  │
│                                 │ From:    geoff@907.life          │
│ Inbox            ┃              │ To:      ▍                       │
│ Drafts                          │ Cc:                              │
│ Sent                            │ Bcc:                             │
│ Archive                         │ Subject:                         │
│                                 │ ─────────                        │
│ Trash                           │                                  │
│ Spam                            │                                  │
│                                 │                                  │
│ /                               │                                  │
│                                 │                                  │
└─────────────────────────────────┴──────────────────────────────────╯
 ^X send  ^C cancel  Tab next  ⇧Tab prev                          ?
```

## Testing

### Unit

- Focus cycling — Tab from To → Cc → Bcc → Subject → Body → To;
  Shift+Tab reverse.
- Esc behavior — from Body returns to Subject; from a header
  returns to Body; never closes.
- Ctrl+C — empty draft closes immediately; dirty draft opens
  ConfirmModal; Yes closes, No keeps open.
- Ctrl+X — emits `composeSendMsg` with assembled bytes matching
  `compose.AssembleMIME(draft, frozenTime)`.
- Cc/Bcc — typed addresses appear in `Draft()` output; empty
  rows produce empty slices.
- Sent folder resolution — classified `RoleSent` wins; name-
  fallback works; missing Sent surfaces inline error.
- Seed paths — `r`/`R`/`f` produce drafts matching
  `compose.Seed*` golden output.
- Tidy seam — App with no-op tidy passes body through unchanged;
  test with a fake tidy that rewrites verifies the body change
  reaches AssembleMIME.

### Cache

- `QueueOutbound` with `IsJMAP() == true` enqueues exactly one
  Send op.
- `QueueOutbound` with `IsJMAP() == false` enqueues Send then
  Append, in order.
- Send queue failure short-circuits (Append not enqueued).

### Golden tmux

- 80×24 fresh compose, `c` from inbox.
- 80×24 dirty compose, Ctrl+C → ConfirmModal overlay.
- 120×40 reply seeded from a real fixture (quoted body visible).
- 120×40 send → toast → outbox `Q` shows the queued op.

## Risks + open spots

- **Body focus + Catkin Tab** — verified Catkin owns Tab for
  indent. Compose disables textinput's own Tab handling so our
  cycle works in headers; in body we never see Tab at the
  ComposeTab layer because Catkin consumes it before bubbling.
- **Ctrl+C as cancel rebind** — terminal sends SIGINT-shaped
  KeyMsg; bubbletea delivers as `tea.KeyCtrlC`. We handle it in
  compose; outside compose, the existing global Ctrl+C → Quit
  path is preserved.
- **Sent folder not configured** — surfaces inline. Self-hosted
  IMAP without SPECIAL-USE and without a `Sent`-named folder is
  the realistic edge case; the inline error directs the user to
  config.
- **Tidy long-running** — 9h ships no-op so this isn't a problem
  yet. 9i needs a tidy progress indicator and cancellation; not
  a 9h concern but called out so the seam shape (returns
  `(string, error)`, takes `context.Context`) supports it.

## Out of scope (explicit)

- Drafts persistence to server / local cache (9h.5).
- Address autocomplete from CardDAV (9.1).
- Signature insertion + multiple identities (9.4).
- Undo-send window + scheduled send (9.2).
- Attach picker overlay in compose (9.5).
- Reply-to header.
- BCC self.
- Per-folder default identity.
- HTML signature stripping when quoting.

## ADR list (written at pass end)

- **ADR-0159** — ComposeTab as App-level mode + single-instance
  for 9h; focus model + Tab cycling rule; Ctrl+X send / Ctrl+C
  cancel rebind in compose context; Esc as focus-only key.
- **ADR-0160** — `cache.Account.QueueOutbound` + `Backend.IsJMAP()`
  predicate; protocol branch lives in cache, not UI.
- **ADR-0161** — `TidyFn` function-pointer seam on App (no
  interface) for the 9i Claude Tidy entry point.

Three ADRs is at the upper bound for one pass; if the tidy seam
discussion is trivial in retrospect, fold it into 0159.

## Pass-end checklist (non-binding preview)

- `/simplify` clean.
- Idiomatic-bubbletea checklist run against the diff (size
  contract, JoinHorizontal avoided where SPUA cell-width is
  non-1, key.Binding + key.Matches, WindowSizeMsg forwarded).
- ADRs 0159–0161 written.
- `docs/poplar/invariants.md` — add Compose section under
  Architecture (currently has a stub under "Compose"); update
  decision index.
- `.claude/rules/ui-invariants.md` — flesh out the Compose
  component subsection (focus model, key map, send/cancel).
- `docs/poplar/keybindings.md` — Ctrl+X send, Ctrl+C cancel,
  Tab/Shift+Tab cycling, Esc-as-focus advertised under a new
  Compose context.
- `docs/poplar/wireframes.md` — add fresh-compose and
  reply-seeded wireframes.
- STATUS.md — Pass 9h done; promote 9h.5 (drafts) starter prompt.
- `make check` green; `make install`.
