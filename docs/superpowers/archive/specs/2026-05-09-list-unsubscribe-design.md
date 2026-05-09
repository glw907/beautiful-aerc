# List-Unsubscribe (BACKLOG #36)

Date: 2026-05-09
Status: Approved
Pass: 11

## Problem

Bulk and mailing-list senders advertise an unsubscribe path in the
`List-Unsubscribe` header (RFC 2369), and well-behaved senders also
opt into one-click POST per RFC 8058 via
`List-Unsubscribe-Post: List-Unsubscribe=One-Click`. Poplar today
ignores both. Users who want off a list have to scroll to the
footer, click an HTML link, and bounce out to a browser — exactly
the friction other clients have removed.

The matrix consensus (Thunderbird, Apple Mail, Outlook, mutt,
aerc, K-9, Geary, Evolution) is uniform: surface a single
unsubscribe affordance whenever the header is present, fire on
click, no client-side memory of prior unsubscribes. Poplar follows
that shape.

## Decisions settled in brainstorm

- **Key = `U`.** Free in `keybindings.md`; modifier-free uppercase
  per ADR-0076; mnemonic for "unsubscribe". The search-shelf
  typing state already steals printable runes including `U` per
  ADR-0064, so no conflict.
- **mailto fallback opens compose, pre-filled.** Poplar is the
  mail client; routing the mailto form through `xdg-open` would
  hand it to a different client. The compose seam already exists.
- **Both forms present → prefer the https one-click POST.** That's
  the whole point of RFC 8058. The mailto form is fallback only
  when no one-click endpoint is offered.
- **Plain http: links route through the existing `URLOpener`
  seam.** No POST, no compose — the header is treated as a
  vanilla web link.
- **No memory of prior unsubscribes.** Universal across the
  matrix. The unsub endpoint is the source of truth (idempotent
  in practice); a well-behaved list stops sending after a
  successful unsub, so client-side memory is moot. Revisit
  post-1.0 if real usage shows users want a "you've already done
  this" signal.

## Architecture

### Header parsing

Pure function, new file `internal/content/listunsubscribe.go`:

```go
type Unsubscribe struct {
    OneClick string // https URL when both List-Unsubscribe-Post
                    // and an https form are present, else ""
    Mailto   string // first mailto: URL, else ""
    HTTP     string // first http(s) URL when not promoted to
                    // OneClick, else ""
}

func (u Unsubscribe) Available() bool

func Parse(headers textproto.MIMEHeader) Unsubscribe
```

`List-Unsubscribe` carries a comma-separated list of angle-bracketed
URIs (RFC 2369 §3.2). The parser tolerates whitespace, missing
brackets, and mixed schemes. `List-Unsubscribe-Post` promotes the
first https URL to `OneClick` when its value is exactly
`List-Unsubscribe=One-Click` (case-insensitive on the key per RFC
8058 §3); any other value leaves the URL in `HTTP`. http (non-TLS)
URLs never promote to `OneClick` — RFC 8058 §3 requires https.

Unit-tested table-driven against the canonical RFC 8058 §6
examples plus poplar fixtures (mailto only, https + mailto, malformed
brackets, multiple https URLs, http+https mix, missing post header,
non-One-Click post value).

### Wire path

Parsing runs at viewer body-load time in `internal/ui/cmds.go`'s
existing body-fetch flow. The raw RFC 5322 bytes already pass
through that Cmd. The parsed `Unsubscribe` value rides back on a
new `reader.UnsubscribeLoadedMsg{Unsub content.Unsubscribe}` batched
in the same `tea.Batch` as `BodyLoadedMsg` — keeps `bodyLoadedMsg`'s
shape stable for downstream consumers.

`reader.Model` grows one field:

```go
unsub content.Unsubscribe
```

with an accessor `Unsubscribe() content.Unsubscribe`. Cleared when
the viewer closes (same lifecycle as body).

`mail.MessageInfo` is **not** extended. The affordance is
viewer-only; carrying `List-Unsubscribe` headers on every list row
would bloat the wire shape for no list-side consumer.

### Dispatch

`U` keypress in the reader (inert when `!unsub.Available()`) emits
`reader.OpenUnsubscribeConfirmMsg{Unsub}`. App opens
`ConfirmModal` with body text:

```
Send unsubscribe request to <host>?
                                         y / n
```

`<host>` is the parsed URL's host for OneClick / HTTP, or the
mailto address for Mailto-only. Yes-confirm emits
`UnsubscribeConfirmedMsg`; App routes by precedence:

1. **`OneClick != ""`** → `unsubscribePostCmd(ctx, url)` in
   `internal/ui/cmds.go`. Body: form-encoded
   `List-Unsubscribe=One-Click`,
   `Content-Type: application/x-www-form-urlencoded`,
   `Content-Length` set. 10-second timeout via
   `http.Client{Timeout: 10 * time.Second}`. 2xx → success;
   anything else (non-2xx, network, TLS, timeout) →
   `uicore.ErrorMsg{Op: "unsubscribe", Err: ...}`.
2. **`OneClick == "" && Mailto != ""`** → seed compose with the
   mailto URL parsed via `net/url.Parse` for path (address) +
   query (subject/body). Existing compose seam, no new path. App
   opens the compose surface as for `c`/`r`.
3. **Only `HTTP != ""`** → route through `App.URLOpener` (existing
   seam used by `1`–`9` link launch). Fire-and-forget.

Success surfaces as a one-shot `Unsubscribed from <host>` notice
in the existing chrome banner row, with the same precedence
rules as triage toasts (error wins, then notice, else collapse).
The exact emit mechanism — extending the toast slot vs. a
dedicated notice channel — is a plan-level decision. No separate
success modal.

### Footer

Viewer footer gains a conditional `U unsub` hint at drop rank 6,
visible only when `reader.Unsubscribe().Available()`. Drop rank 6
sits between primary triage and tertiary actions; the hint
disappears entirely on messages without the header rather than
rendering inert. Footer composition logic in
`internal/ui/footer.go` (or wherever the viewer hint slice lives)
checks `reader.Unsubscribe().Available()` when assembling.

## Module placement

| File | Purpose |
|------|---------|
| `internal/content/listunsubscribe.go` | Pure `Parse`, `Unsubscribe` value type |
| `internal/content/listunsubscribe_test.go` | Table-driven parser tests |
| `internal/ui/cmds.go` | `unsubscribePostCmd` (new) and Unsub plumbing on body fetch |
| `internal/ui/reader/model.go` | `unsub` field + `Unsubscribe()` accessor |
| `internal/ui/reader/msgs.go` | `UnsubscribeLoadedMsg`, `OpenUnsubscribeConfirmMsg` |
| `internal/ui/reader/keys.go` | `U` binding |
| `internal/ui/app.go` | Confirm-modal cascade entry; mailto→compose route; URLOpener route; toast emit |
| `docs/poplar/keybindings.md` | `U` row under Viewer |
| `docs/poplar/wireframes.md` | Confirm-modal capture at 80×24 |
| `docs/poplar/invariants.md` | Reader / UX invariants for the affordance |
| `docs/poplar/decisions/0185-list-unsubscribe.md` | New ADR |

## Error & edge handling

- **No header, or only malformed entries** → `Available()` is
  false, `U` is inert, no footer hint.
- **POST timeout / non-2xx** → `ErrorMsg` banner, no toast. The
  user can press `U` again; the affordance stays live.
- **mailto with multiple addresses** → first address wins, others
  drop on the floor. (Matrix clients all do this.)
- **mailto with no subject/body params** → compose opens with To
  pre-filled, blank Subject/Body.
- **List-Unsubscribe-Post present but no https URL** → no
  promotion to OneClick; falls through to mailto/http precedence.
- **Network offline (`ConnState`)** → POST still attempts and
  fails fast; ErrorMsg surfaces the underlying `net.OpError`.

## Testing

- Unit: `Parse` table-driven, no testify. ≥10 cases covering RFC
  8058 §6 canonical examples and poplar edge cases.
- Unit: `unsubscribePostCmd` against an `httptest.Server` —
  success (200, 202), failure (404, 500), redirect handled by
  default policy, timeout via `httptest` slow handler.
- Live: tmux capture of confirm modal at 80×24 and the success
  toast at 120×40. Verify against a real list message in
  Fastmail (`geoff@907.life` reliably has bulk-list traffic).

## Out of scope

- Memory of prior unsubscribes (per-Message-ID or per-List-Id).
- Bulk unsubscribe across multiple messages.
- `List-Subscribe` support (no precedent in matrix; rare).
- Automatic detection on message-list rows.
- Confirm-modal "always allow" toggle.

## ADR

`docs/poplar/decisions/0185-list-unsubscribe.md` — codifies:

- RFC 8058 https one-click POST is preferred; mailto is fallback;
  http link routes through `URLOpener`.
- No client-side memory of prior unsubscribes (matches matrix).
- Affordance is viewer-only; `mail.MessageInfo` is not extended.

## Pass-end ritual

Standard: `/simplify`, idiomatic-bubbletea checklist (UI changes
are small but real — one new key, one new conditional footer
hint, one confirm modal trigger), ADR-0185, invariants update,
plan + spec archive, `make check`, commit/push/install.
