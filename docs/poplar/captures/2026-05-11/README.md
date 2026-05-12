# Pass 32 captures (2026-05-11)

Live verification artifacts for the v2 declarative View fields pass
(`TERM=xterm-256color` tmux sessions; no Kitty keyboard protocol).

## What's captured here

### `helppopover-account-noproto-120x40.txt`

Account (Message List) context help popover at 120×40,
`TERM=xterm-256color`. Demonstrates the Account-context binding
table. No chord rows appear — chord rows are scoped to the Compose
context and are filtered by `RequiresKittyKbd` in any case.

### `compose-footer-noproto-120x40.txt`

Compose open, body focused, 120×40, `TERM=xterm-256color`. The
footer group reads:

```
Ctrl+X send  Ctrl+C cancel  Tab field  Ctrl+O attach  Ctrl+L later ┊  **bold** *italic* [link](url) markdown
```

The markdown nudge (`**bold** *italic* [link](url) markdown`) is the
protocol-absent path from `chordFooterHints`: `caps.SupportsKeyDisambiguation()`
returns false, so the plain markdown hint renders instead of the
full chord vocabulary. Correct behavior.

### `compose-footer-noproto-80x24.txt`

Compose open, body focused, 80×24, `TERM=xterm-256color`. The footer
reads:

```
Ctrl+X send  Ctrl+C cancel  Tab field  Ctrl+O attach  Ctrl+L later …
```

The markdown nudge (rank 7) is dropped at 80 columns — the footer
width budget is exhausted at rank 6. The `…` truncation glyph
confirms the footer system is operating correctly; the hint would
appear at wider terminals (confirmed by the 120×40 capture above).

## Deferred to manual smoke

The following capture paths require either a Kitty-protocol-capable
terminal or live mail traffic and were not reproducible in the
automation session.

### Compose-context help popover (`helppopover-compose-*.txt`)

**Why deferred:** when compose is open, the `routeOverlayKey`
dispatcher in `App` routes all key events to `compose.Update` before
the app-level `?` → help-open branch. The `?` character is consumed
by the focused text-input or Catkin body editor before it can reach
the help key handler. The Compose context in helppopover is exercised
by `TestPopoverFiltersGatedBindings` in
`internal/ui/helppopover/model_test.go`; live capture requires a UX
path that bypasses the compose key-route (not yet wired — a future
pass could add a help chord to compose's key dispatch or lift the
help check above the compose route).

**To verify manually:** open compose in any terminal, type `?` in the
body — confirm it inserts a literal `?`. The compose-context help
filter is tested in unit tests; the live capture path does not exist
in the current architecture.

### Kitty-protocol help popover + compose footer

Run poplar inside Ghostty or Kitty (Kitty keyboard protocol
negotiated). Open compose; navigate to the body. The compose footer
should show the full chord vocabulary:

```
^B bold · ^I italic · ^K link · ^L list · ^Q quote · ^@ task
```

Press `?` from the account view (not while compose is open) and
confirm the Account-context popover shows. The Compose-context
popover with chord rows requires the architectural path noted above.

### Outbox drain ProgressBar

Needs a queued outbound message draining. Compose a message, send
it, and watch the OS taskbar progress segment during drain. The OSC
9;4 byte stream is not visible in tmux pane captures.

### New-mail toast (focus gate)

Alt-tab away from poplar; trigger a real `mail.Update` arrival (send
mail to `geoff@907.life` from another client). The toast should
appear within ~1.5s reading `· N new from Foo ·` or `· N new in
Inbox ·`. Re-focus mid-decay clears the toast. Set
`[ui] new-mail-toast = false` in `~/.config/poplar/config.toml`
and restart to confirm the opt-out path.

## Unit-test coverage for deferred paths

| Feature | Test file | Test name |
|---|---|---|
| helppopover Compose filter | `internal/ui/helppopover/model_test.go` | `TestPopoverFiltersGatedBindings` |
| catkin ChordSet gating | `internal/catkin/bindings_test.go` | `TestChordSet_GatedSubset` |
| new-mail toast gate | `internal/ui/app_test.go` | `TestNewMailToast_GatedOnFocusAndConfig` |
| new-mail toast coalesce | `internal/ui/app_test.go` | `TestNewMailToast_CoalesceCollapses` |
| focus/blur toggle | `internal/ui/app_test.go` | `TestFocusBlurTogglesAppFocused` |
| ProgressBar priority ladder | `internal/ui/app_view_test.go` | `TestFrameProgressBarPriorityLadder` |

These paths have unit-test coverage in the suite (`make check`
passes). Live verification is part of pre-soak readiness, not Pass
32 sign-off.
