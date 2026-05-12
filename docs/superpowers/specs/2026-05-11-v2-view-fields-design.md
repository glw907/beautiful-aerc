# Pass 32 — v2 declarative View fields

**Date:** 2026-05-11
**Pass:** 32
**Initiative:** ROADMAP `v2-view-fields`
**Supersedes:** —
**ADR (to be written at consolidation):** ADR-0217

## Goal

Wire the three `tea.View` fields poplar already has access to but does
not set: `ProgressBar` (OSC 9;4), `ReportFocus` + `FocusMsg`/`BlurMsg`,
and `KeyboardEnhancements` (Kitty keyboard protocol). Each field is
set per-frame in `App.View()` alongside the existing `AltScreen` and
`WindowTitle`. Each gets exactly one user-visible consumer in this
pass; future passes deepen the consumers.

## Scope

In:

- `App.View()` returns a `tea.View` with all five declarative fields
  set per frame (`AltScreen`, `WindowTitle`, `ProgressBar`,
  `ReportFocus`, `KeyboardEnhancements`).
- `ProgressBar` driven by a fixed priority ladder over three op
  classes (attachment download > outbox drain > sync).
- `ReportFocus` consumer: a coalesced new-mail toast that fires only
  while the terminal is unfocused.
- `[ui] new-mail-toast` config flag, default `true`, opt-out for
  focus-mode users.
- `KeyboardEnhancements`: request `DisambiguateEscapeCodes` and
  `ReportEventTypes`; store negotiated capabilities on `App`; tag
  catkin's protocol-dependent chord bindings; capability-aware help
  filtering.
- ADR-0217, invariants update, INDEX update, keybindings.md update,
  ui-invariants.md update, bubbletea-conventions.md checklist update.

Out:

- Mouse support (Pass 33; ROADMAP `mouse-support`).
- Backend-side IDLE / push pause on blur (deferred — pre-beta lets a
  future pass add server-side pause if measurement justifies it).
- `IsRepeat` consumers (held-key acceleration in messagelist /
  reader) — landing the negotiation now lets future passes consume
  without re-doing the App.View wiring.
- `ShiftedCode` / `BaseCode` consumers (non-US keyboard catkin work).
- Determinate-percent reporting for attachment downloads and sync —
  both ride as `ProgressBarIndeterminate` until the underlying ops
  gain progress reporting in a future pass.
- Bell on new mail. Toast carries content; bell carries none. Future
  config flag is a one-line addition if asked.

## Settled decisions

These are inputs to the design, not open questions:

- **Progress source: priority ladder** (attachment > outbox > sync).
  Rationale: predictable, cheap, no aggregation math, mirrors user
  attention. A long sync is visually preempted while a brief outbox
  drain finishes; the bar returns to sync state when drain completes.
- **Focus resume: re-arm only.** No backend-side pause this pass; the
  IDLE loop / JMAP push keep running on blur. The only thing gated by
  focus is the new-mail toast. (Re-arm semantics apply to a future
  pass that adds backend pause.)
- **Catkin chord help: capability-aware filtering.** One binding
  table, one render path, one filter predicate. Bindings tagged
  `RequiresKittyKbd` are skipped by `helppopover` when the protocol
  isn't negotiated. Catkin's status footer collapses to a discreet
  one-line hint when the protocol is absent.
- **Incoming-mail signal: toast only, gated on blur, opt-out via
  config.** Focused users already see the messagelist update; the
  toast targets unfocused users specifically. Bell is not added.

## Architecture

### ProgressBar wiring

`App.View()` calls `m.frameProgressBar()` and assigns the result to
`v.ProgressBar`. The accessor:

```go
func (m App) frameProgressBar() *tea.ProgressBar {
    if pct, ok := m.acct.AttachmentDownloadProgress(); ok {
        return tea.NewProgressBar(tea.ProgressBarIndeterminate, pct)
    }
    if pct, ok := m.acct.OutboxDrainProgress(); ok {
        return tea.NewProgressBar(tea.ProgressBarDefault, pct)
    }
    if pct, ok := m.acct.SyncProgress(); ok {
        return tea.NewProgressBar(tea.ProgressBarIndeterminate, pct)
    }
    if state, ok := m.recentProgressError(); ok {
        return tea.NewProgressBar(state, 0)
    }
    return nil
}
```

New methods on `cache.Account`:

- `AttachmentDownloadProgress() (pct int, active bool)` — returns
  `(0, true)` while a save is in flight; `(0, false)` otherwise.
  Backed by an `atomic.Int32` counter incremented around the save
  cmd path. Indeterminate-only this pass.
- `OutboxDrainProgress() (pct int, active bool)` — returns the
  current burst's `(done * 100 / total, true)` while the drainer is
  working a non-empty queue. State stored on `Account`: `burstTotal`
  set when transitioning from idle to draining; `burstDone`
  incremented per OpDone tx; both reset on transition back to idle.
  Determinate.
- `SyncProgress() (pct int, active bool)` — returns `(0, true)`
  while a `Backend` Connect or refresh fetch is mid-flight (a flag
  set by `pumpUpdatesCmd` callsites). Indeterminate-only this pass.

`App` also tracks `recentProgressError`: when an op-class transitions
from active to errored within the last ~3s, return
`ProgressBarError`. Decay timer fires `progressErrorClearMsg`. State
on `App`: `progressErrorUntil time.Time`.

### ReportFocus + new-mail toast

`App.View()` sets `v.ReportFocus = true`. App.Update handles:

- `tea.FocusMsg` → `m.focused = true`; clears any active new-mail
  toast (the messagelist now shows the truth).
- `tea.BlurMsg` → `m.focused = false`.

Initial state: `App.focused = true`. The terminal sends a Focus/Blur
event on whatever transition happens first.

**Toast variant.** Extend the toast pipeline (`internal/ui/toast.go`)
with a `newMail` variant on `pendingAction`:

```go
type pendingAction struct {
    op            triageOp
    // existing fields ...
    newMailCount    int
    newMailSender   string  // most-recent sender, "" if mixed
    newMailFolder   string  // folder name when mixed senders
}
```

Renderer: `· N new from <sender> · ◉` when single sender,
`· N new in <folder> ·` when mixed. 4s decay; `Esc` dismisses early.

**Coalesce.** `pumpUpdatesCmd` already surfaces `mail.Update` events
into App.Update. New gate:

```go
case mail.Update:
    m.acct = m.acct.WithUpdate(msg)
    cmds = append(cmds, refreshAfterUpdateCmd(...))
    if !m.focused && m.cfg.UI.NewMailToast && msg.HasNewArrivals() {
        m, c := m.queueNewMailToast(msg)
        cmds = append(cmds, c)
    }
```

`queueNewMailToast` either starts a fresh 1s coalesce window
(`coalesceTimerMsg` tick) or extends the pending one. When the timer
fires, the accumulated arrivals collapse into one toast.

**Config.** New field on the `[ui]` table:

```toml
[ui]
new-mail-toast = true   # set false to silence; default true
```

Decoded by `config.LoadUI` in the existing `[ui]` table walk; default
`true`. Strict-decode picks up unknown variants per ADR-0211.

### KeyboardEnhancements + capability-aware help filtering

`App.View()` sets:

```go
v.KeyboardEnhancements.DisambiguateEscapeCodes = true
v.KeyboardEnhancements.ReportEventTypes = true
```

App.Update handles `tea.KeyboardEnhancementsMsg` once at startup:

```go
case tea.KeyboardEnhancementsMsg:
    m.kbdCaps = msg
```

`kbdCaps` is a value on `App`; threaded into children that need it
(`helppopover`, `compose`/catkin's footer) via App accessors —
`m.KbdCaps()`. No Msg fan-out, since caps don't change post-negotiation.

**Capability tag.** `key.Binding` is third-party and not extensible.
A thin local type sits beside it:

```go
// internal/ui/uicore/keys.go
type GatedBinding struct {
    Binding          key.Binding
    RequiresKittyKbd bool
}
```

Catkin's chord set declares `RequiresKittyKbd: true` for the chords
the legacy ASCII mapping confuses with Tab/Enter/Backspace/etc.:
`^B`, `^I`, `^L`, `^Q`, `^@`/`^Space`, `^K`. `Ctrl+Backspace` (the
wordnav binding in `internal/catkin/wordnav.go`) carries the same
tag.

**Help filtering.** `helppopover.Model` takes `tea.KeyboardEnhancementsMsg`
at construction (or via `WithKbdCaps(msg) helppopover.Model`). Its
render iterates `GatedBinding` entries and skips any with
`RequiresKittyKbd && !caps.DisambiguateEscapeCodes`. One render path,
one filter predicate.

**Catkin status footer.** The dim chord-hint row already on the
wireframe renders the active subset:

- Protocol negotiated:
  `^B bold · ^I italic · ^K link · ^L list · ^Q quote · ^@ task`
- Protocol absent:
  `markdown: type **bold** *italic* [link](url) — richer chords in Kitty-protocol terminals`

One render function, two branches by `caps.DisambiguateEscapeCodes`.

**Documentation.** `docs/poplar/keybindings.md` gets a new section,
"Catkin chords (Kitty keyboard protocol)", listing the gated set
with one paragraph naming the protocol and the terminals that
support it (Kitty, Ghostty, WezTerm, foot, recent tmux).

## Data flow

```
Backend update           → mail.Update          → App.Update
                                                    ↓
                                              (new arrival? + !focused
                                                 + cfg.UI.NewMailToast)
                                                    ↓
                                              queue / extend coalesce
                                                    ↓
                                              coalesceTimerMsg fires
                                                    ↓
                                              pendingAction{newMail, ...}
                                                    ↓
                                              App.View renderToast → row

Outbox drainer state     → Account.OutboxDrainProgress
Save attachment cmd      → Account.AttachmentDownloadProgress
Connect / refresh fetch  → Account.SyncProgress
                           ↓
                     App.frameProgressBar (priority ladder)
                           ↓
                     v.ProgressBar in App.View → OSC 9;4 to terminal

Terminal focus event     → tea.FocusMsg / tea.BlurMsg → m.focused
Kitty kbd negotiation    → tea.KeyboardEnhancementsMsg → m.kbdCaps
                                                          ↓
                                                helppopover, catkin footer
```

## Testing

Unit:

- `App.frameProgressBar` table test: each combination of (attach,
  outbox, sync, error) returns the expected bar / nil.
- `cache.Account.OutboxDrainProgress` test: simulate a drain burst,
  assert `(done * 100 / total, true)` for each step, `(0, false)`
  after queue empties.
- New-mail toast coalesce: feed three `mail.Update`s within 1s, assert
  one toast with combined count.
- New-mail toast gate: with `m.focused = true`, no toast even with
  arrivals; with `m.focused = false` + `cfg.UI.NewMailToast = false`,
  no toast.
- Catkin help filter: with `caps.DisambiguateEscapeCodes = false`,
  `helppopover` render omits the gated entries; with `true`, all
  entries present.
- Config decode: `new-mail-toast = false` round-trips through
  `config.LoadUI` / `config.Render`; default is `true` when absent.

Live (tmux, per `.claude/docs/tmux-testing.md`):

- 120×40 capture: outbox with one queued send showing the
  `ProgressBar` driving the terminal title (verify OSC 9;4 in the
  emitted byte stream — capture via `tmux capture-pane -p` plus a raw
  byte log).
- 120×40 capture: help popover on a Kitty-protocol terminal vs plain
  xterm; the gated chords are present in the first, absent in the
  second.
- 80×24 capture: compose surface footer chord hint in both modes.
- Manual: blur the terminal, send mail to the test account from
  another client, observe the toast row appears with the correct
  sender + count after the 1s coalesce window. Re-focus mid-decay,
  observe the toast clears immediately.

## Pass-end deliverables

Per the `poplar-pass` consolidation ritual:

1. Write **ADR-0217 — v2 declarative View fields** covering all
   three sub-decisions in one document (priority ladder, focus toast
   gate, capability-tagged bindings).
2. **`docs/poplar/invariants.md`** — edit in place:
   - *Elm architecture & idiomatic bubbletea* section: extend the
     `tea.View` line to enumerate all five declarative fields now set
     per frame.
   - Add a one-sentence statement of the priority ladder.
   - Add a one-sentence statement of the `RequiresKittyKbd` tag.
   - *Config & theming* section: add `new-mail-toast` to the `[ui]`
     table description.
3. **`docs/poplar/decisions/INDEX.md`** — new row pointing the four
   binding facts (View-field set, progress priority, focus toast,
   kbd capability tag) to ADR-0217.
4. **`.claude/rules/ui-invariants.md`** — add the new toast variant
   + `RequiresKittyKbd` tag to the auto-loaded UI rules.
5. **`docs/poplar/keybindings.md`** — new "Catkin chords (Kitty
   keyboard protocol)" section.
6. **`docs/poplar/bubbletea-conventions.md`** §10 checklist — extend
   the parenthetical listing the `tea.View` declarative fields.
7. **`docs/poplar/STATUS.md`** — mark Pass 32 done, write the Pass 33
   starter prompt (mouse support — already gated and described in
   ROADMAP `mouse-support`).
8. **`ROADMAP.md`** — mark `v2-view-fields` initiative done in the
   Done section; note deferred consumers (`IsRepeat` for held-key
   accel, server-side IDLE pause).
9. **Archive** this spec and the implementation plan to
   `docs/superpowers/archive/`.
10. `make check` green; commit; push; `make install`.

## Pass shape

Eight implementation tasks plus the consolidation step:

1. ProgressBar source accessors on `cache.Account` (drain%, attach
   binary, sync binary).
2. `App.frameProgressBar` + error decay timer; wire into `App.View`.
3. `ReportFocus = true`; `FocusMsg` / `BlurMsg` handlers; `App.focused`.
4. New-mail toast variant + coalesce timer + config gate.
5. `KeyboardEnhancements` field set; `KeyboardEnhancementsMsg`
   handler; `App.kbdCaps`.
6. `GatedBinding` type; tag catkin chords; `helppopover` filter.
7. Catkin status footer adapts to caps.
8. tmux capture verification (120×40 + 80×24).
9. Consolidation: ADR + invariants + INDEX + STATUS + archive +
   commit + push + install.

Fits the 8–12 budget comfortably.
