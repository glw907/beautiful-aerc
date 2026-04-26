# Pass 3 — JMAP direct-on-rockorager backend, fork removal, live wiring

**Status:** approved 2026-04-25
**Implements:** ADR-0075 (direct-on-libraries mail stack)
**Closes:** BACKLOG #10 (already closed by ADR-0075), BACKLOG #11 (MIME-aware FetchBody)

## Goal

Rewrite `internal/mailjmap/` directly against `git.sr.ht/~rockorager/go-jmap`
under a synchronous `mail.Backend`. Vendor minimal auth/keepalive snippets
into `internal/mailauth/`. Delete `internal/mailworker/` in its entirety.
Wire the prototype to a live Fastmail account.

## Non-goals

- Gmail IMAP. Deferred to Pass 8.
- On-disk blob/state cache. In-memory only this pass.
- Idle prefetch (warming the next folder).
- Multi-account simultaneous push infrastructure (Pass 11).
- Compose / send. Pass 9.
- Viewer changes. The Pass 2.5b-4 viewer already accepts `io.Reader`.

## Settled decisions (from brainstorming)

| Question | Decision |
|---|---|
| Push/EventSource shape | Dedicated push goroutine inside `mailjmap`, owned by `Connect`/`Disconnect`, feeds the existing `Updates()` channel. |
| Blob/state cache | In-memory LRU body cache (~64 entries / ~32 MB) plus in-memory `state` strings; cleared on `Disconnect`. |
| Connection state | New `UpdateConnState` type on `mail.Update` with a `ConnState` field. App's existing `Updates()` pump routes it to the status bar. |
| Large-mailbox pagination | Lazy-load on scroll. 500-message initial window via `QueryFolder(offset, limit)`; `MessageList.AppendMessages` plus a UID-anchored cursor restore handles the merge. |

## Package layout after the pass

```
internal/
  mail/                # interface + types (existing)
  mailjmap/            # rewritten — direct on rockorager/go-jmap
  mailauth/            # NEW — vendored xoauth2.go + keepalive/
  mailimap/            # not created this pass (Pass 8)
  mailworker/          # DELETED
```

## `mail.Backend` interface changes

Two additions to `internal/mail/`:

```go
// backend.go — add to Backend
QueryFolder(name string, offset, limit int) (uids []UID, total int, err error)

// types.go — extend Update by adding UpdateConnState as the next
// member of the existing const block (after UpdateFolderInfo).

type ConnState int

const (
    ConnOffline ConnState = iota
    ConnReconnecting
    ConnConnected
)

type Update struct {
    Type      UpdateType
    Folder    string
    UIDs      []UID
    ConnState ConnState // populated only when Type == UpdateConnState
}
```

`OpenFolder(name)` is reduced to "remember the current folder name." No
work. The first `QueryFolder` call performs the `Email/query` +
`Email/get` round trip.

`mail.MessageInfo` stays as-is on the wire. The JMAP backend stores the
JMAP `id` in `UID` and stores `blobId` in a private adjacent map keyed by
`UID` so `FetchBody` can locate the blob without widening the wire type.

## `internal/mailjmap/` design

Single `Backend` struct:

```go
type Backend struct {
    name    string
    cfg     config.AccountConfig
    client  *jmap.Client          // rockorager/go-jmap
    session *jmap.Session

    mu       sync.Mutex
    current  string               // current folder name
    folders  map[string]*folder   // by canonical poplar name
    blobIDs  map[mail.UID]string  // UID -> JMAP blobId
    states   map[string]string    // per-collection state cursor

    bodies   *lru.Cache[string, []byte] // blobId -> raw RFC822 (hashicorp/golang-lru/v2)
    updates  chan mail.Update

    pushCancel context.CancelFunc
    pushDone   chan struct{}
}
```

Construction: `New(cfg config.AccountConfig) *Backend`. The `*jmap.Client`
is built in `Connect` so retries can rebuild it.

### Lifecycle

`Connect(ctx)`:
1. Build the JMAP `*jmap.Client` against `cfg.Host` (Fastmail
   `https://api.fastmail.com/jmap/session`).
2. Inject XOAUTH2 bearer via `internal/mailauth` using
   `os.Getenv("FASTMAIL_API_TOKEN")`. The token is read once at connect
   time. Wrong/missing token surfaces as `mail.ErrorMsg{Op: "connect"}`.
3. `Session/get` → cache `*jmap.Session`.
4. `Mailbox/get` → populate `folders` keyed by canonical poplar name via
   `mail.Classify`.
5. Initialize `states["Email"]`, `states["Mailbox"]` from the session
   state strings.
6. Allocate `updates` (buffered 64), `bodies` (`lru.New[string, []byte](64)`).
7. Spawn `pushLoop(ctx)`. Emit
   `Update{Type: UpdateConnState, ConnState: ConnConnected}`.

`Disconnect()`:
1. `pushCancel()`; wait on `pushDone`.
2. `close(updates)`. The App's pump treats a closed channel as `ConnOffline`
   if no explicit terminal `UpdateConnState` was sent.
3. Drop `bodies`, `blobIDs`, `states`, `folders`.

### Push loop

```go
func (b *Backend) pushLoop(ctx context.Context) {
    defer close(b.pushDone)
    backoff := newBackoff() // 1s..30s exponential
    for {
        if err := b.runEventSource(ctx); err != nil && ctx.Err() == nil {
            b.emit(mail.Update{Type: UpdateConnState, ConnState: ConnReconnecting})
            select {
            case <-ctx.Done():
                return
            case <-time.After(backoff.next()):
            }
            continue
        }
        return
    }
}
```

`runEventSource` opens the JMAP push EventSource via `rockorager/go-jmap`'s
push subpackage, blocks reading `StateChange` events. On each event:

1. Compare incoming `state` against `b.states[type]`. If unchanged, ignore.
2. For changed types, run `Email/changes` (or `Mailbox/changes`) from the
   stored cursor, then `Email/get` for the changed IDs.
3. Translate to `mail.Update` values:
   - New IDs → `UpdateNewMail`.
   - Updated → `UpdateFlagsChanged`.
   - Removed → `UpdateExpunge`.
   - Mailbox change → `UpdateFolderInfo`.
4. Update `b.states[type]` to the new state string only after successful
   processing — partial failures keep the old cursor so the next event
   retries the gap.
5. Emit on `updates` (non-blocking with a drop-and-log fallback if the
   buffer is full; the buffer is sized so the App always drains promptly).

On EventSource open success after a prior failure, emit
`UpdateConnState{ConnConnected}`. On any open failure, return error so
`pushLoop` retries.

### Sync RPC methods

Each method builds a JMAP method call, runs it through `b.client.Do`,
translates the result.

- `ListFolders()` — `Mailbox/get` (or returns the cached `folders`).
- `OpenFolder(name)` — `b.current = name`. No RPC.
- `QueryFolder(name, offset, limit)` —
  `Email/query` with `inMailbox = folder.id`, `position = offset`,
  `limit`, `sort: [{property: "receivedAt", isAscending: false}]`,
  `calculateTotal: true`. Returns the IDs and total. Stores `blobId` for
  each returned ID via a co-issued `Email/get` (the same request can use
  `#emailIds` reference syntax to chain).
- `FetchHeaders(uids)` — `Email/get` with `properties = headers minimal
  set`. Translate to `mail.MessageInfo`. Populate `b.blobIDs[uid] = blobId`.
- `FetchBody(uid)` — look up `blobId`. Cache hit returns
  `bytes.NewReader(buf)`. Cache miss → `Blob/download` (or `Email/get`
  with full body if the JMAP server prefers; we use `Blob/download` per
  rockorager/go-jmap conventions) → store → return reader.
- `Search(criteria)` — `Email/query` with translated `Filter`.
- `Move(uids, dest)` — `Email/set` updating `mailboxIds`.
- `Copy(uids, dest)` — `Email/copy`.
- `Delete(uids)` — `Email/set` moving to Trash mailbox (Fastmail
  convention) or `Email/set` with `destroyed = uids` if hard delete.
  Soft-delete to Trash matches v1 expectations.
- `Flag/MarkRead/MarkAnswered` — `Email/set` updating `keywords`.
- `Send(...)` — returns `errors.New("send not implemented in pass 3")`.
  `EmailSubmission/set` lands in Pass 9.
- `Updates()` — returns `b.updates`.

### Body LRU

`golang.org/x/sync/singleflight.Group` keyed on `blobId` so two viewers
opening the same message in rapid succession produce one network call.
LRU implementation: `github.com/hashicorp/golang-lru/v2` (`lru.New[K,V]`).
This is the only new third-party dependency added by Pass 3.
Cache invalidation: when `Email/get` returns a different `blobId` for a
known `UID` (server-side rewrite), the new blobId becomes the cache key
and the old entry ages out naturally.

LRU sized at 64 entries with no byte cap in v1 — body sizes vary
widely; cap-by-count keeps the implementation simple. If a single body
is unreasonably large (>32 MB) we still cache it but log a warning.
A byte cap can land via BACKLOG if the count cap proves wrong.

## `internal/mailauth/`

Two files vendored from aerc with provenance comments:

```
internal/mailauth/
  README.md           # vendor provenance and license summary
  xoauth2.go          # ~80 LOC — emersion/go-sasl XOAUTH2 mech
  keepalive/
    keepalive.go      # ~32 LOC — TCP keepalive helper
```

Each file carries:

```go
// Vendored from git.sr.ht/~rjarry/aerc <commit-hash> (MIT).
// Modifications: <list of edits, or "none">.
```

Tested only at the build-and-link level; the upstream tests cover
behavior. We add a single sanity test that the XOAUTH2 challenge bytes
match a known-good vector.

## `internal/mailworker/` deletion

Removed in one commit:

```
git rm -r internal/mailworker/
```

Followed by `goimports -w` across `internal/` to drop any stale
imports. Build must be green after the deletion (the Pass 2.5b prototype
was already wired against `mail.Backend`, not against the worker
directly).

## UI changes

### `AccountTab`

Gains per-folder pagination state:

```go
type folderPage struct {
    loaded, total    int
    loadMoreInFlight bool
}

type AccountTab struct {
    // existing fields...
    pages map[string]*folderPage
}
```

Flow:
- `OpenFolder` Cmd → on success, dispatch
  `QueryFolderCmd(name, 0, 500)` and a "loading messages" spinner.
- `QueryFolderCmd` returns `headersLoadedMsg{name, uids, total}` →
  AccountTab calls `FetchHeadersCmd(uids)` → on success returns
  `headersAppliedMsg{name, headers}` → `messagelist.SetMessages(headers)`,
  `pages[name] = {loaded: len, total: total}`.
- On every cursor move, AccountTab calls `messagelist.IsNearBottom(20)`.
  If true and `!loadMoreInFlight && loaded < total`, dispatch
  `LoadMoreCmd(name, loaded, 500)`. Returned `headersAppendedMsg{name,
  headers}` → `messagelist.AppendMessages(headers)`,
  `pages[name].loaded += len(headers)`, drop in-flight flag.

`folder switches` reset the cursor; we keep the loaded set across
switches so jumping back to Inbox does not re-fetch — until the next
folder open or push event invalidates it.

### `MessageList`

Two new methods:

```go
func (m *MessageList) AppendMessages(extra []mail.MessageInfo) {
    cursorUID := m.cursorUID()       // capture before mutate
    m.source = append(m.source, extra...)
    m.now = time.Now()                // refresh clock for new rows
    m.rebuild()                       // existing pipeline
    m.snapToUID(cursorUID)            // restore cursor by UID
}

func (m *MessageList) IsNearBottom(k int) bool {
    return m.cursor >= len(m.rows)-k
}
```

`cursorUID()` returns the UID under the cursor or empty if rows empty.
`snapToUID(uid)` finds the row index by UID, falling back to clamp at
`len(rows)-1`. Both are package-private helpers added alongside the new
methods.

The UID-anchor approach is reused by an upcoming push-driven refresh:
new-mail arrival via `UpdateNewMail` will call a similar
`MergeMessages([]MessageInfo)` later. Out of scope for this pass; the
shape just needs to accommodate it.

### Status bar / connection state

The `App` already holds `connState ConnState` (currently zero-valued).
The pump goroutine that drains `backend.Updates()` gains:

```go
case mail.UpdateConnState:
    return connStateMsg(u.ConnState)
```

`App.Update` on `connStateMsg` stores the value; the existing status-bar
View derives `●/◐/○` from it.

### Command footer hint

One new hint, drop rank 8:

```
{Key: "", Desc: fmt.Sprintf("%d/%d", page.loaded, page.total), Rank: 8}
```

Empty key (it is informational, not a binding). Hidden on narrow
terminals.

### Loading spinner

`OpenFolder` triggers a "Loading messages…" placeholder in the message
list panel built via `ui.NewSpinner(theme)` (Pass 2.5b-6 helper).
Cleared when `headersAppliedMsg` lands.

## Error handling

All `tea.Cmd` closures wrap errors as `mail.ErrorMsg{Op, Err}`. New
verb-phrases this pass:

| Op | Source |
|---|---|
| `connect` | `Backend.Connect` |
| `list folders` | `Backend.ListFolders` |
| `open folder` | `Backend.OpenFolder` (no-op, but reserved) |
| `query folder` | `Backend.QueryFolder` |
| `fetch headers` | `Backend.FetchHeaders` |
| `load more` | tail-window `QueryFolder` + `FetchHeaders` |
| `fetch body` | `Backend.FetchBody` (already in Pass 2.5b-4) |
| `mark read` | already in Pass 2.5b-6 |
| `move` | `Backend.Move` (action wiring) |
| `delete` | `Backend.Delete` (action wiring) |

Push-loop errors emit `UpdateConnState{ConnReconnecting}` rather than
banner text — connection churn is chrome, not user-facing alarm. A
permanent auth failure (HTTP 401) still emits `ErrorMsg{Op: "connect"}`
in addition, so the user sees why the indicator is stuck on `◐`.

## Config

`config.AccountConfig` already exposes `Protocol`, `Host`, `Username`,
`Password`. Pass 3 adds environment-variable substitution: when
`Password == "$VAR"`, `config.ParseAccounts` substitutes
`os.Getenv("VAR")`. This enables `password = "$FASTMAIL_API_TOKEN"` in
`accounts.toml` without committing the token. Substitution is plain
literal `$VAR` — no `${VAR}` form, no `$VAR_with_extra` ambiguity. If
the env var is unset, ParseAccounts returns an error mentioning the
missing variable name.

## Testing

- **`internal/mailjmap/`** — tests use a fake `*jmap.Client` shim that
  records issued method calls and returns scripted responses. Cover
  `QueryFolder` (offset/limit translation, total propagation),
  `FetchHeaders` (header → MessageInfo mapping, blobId capture),
  `FetchBody` (cache hit / miss / singleflight collapse),
  `pushLoop` (StateChange dispatch, state cursor advance, reconnect).
- **`internal/mail/`** — table-driven test for the new `UpdateConnState`
  field round-tripping through a `chan Update`. (Trivial; mostly a
  compile-time check.)
- **`internal/mailauth/`** — a single XOAUTH2 challenge vector test.
- **`internal/ui/`** — unit tests for `AppendMessages` (cursor by UID,
  thread merge across windows), `IsNearBottom`, and `AccountTab`'s
  load-more dispatch (in-flight guard, terminal at total).
- **Integration** — manual: `make install`, run `poplar`, verify Inbox
  loads, scroll to bottom triggers lazy load, push event from a phone
  send arrives within seconds. No automated live-Fastmail test.

## Acceptance checklist

- [ ] `internal/mailjmap/` rewritten on `rockorager/go-jmap`. No imports
  of `internal/mailworker/`.
- [ ] `internal/mailauth/` exists with vendored xoauth2 + keepalive and
  provenance comments.
- [ ] `internal/mailworker/` deleted; `git ls-files | grep mailworker`
  is empty.
- [ ] `mail.Backend` gains `QueryFolder`; `mail.Update` gains
  `UpdateConnState` + `ConnState`.
- [ ] `MessageList.AppendMessages` + `IsNearBottom` land with tests.
- [ ] `AccountTab` lazy-loads on cursor proximity to bottom.
- [ ] Status bar shows `●` once `pushLoop` reports `ConnConnected`.
- [ ] `make check` green.
- [ ] Live Fastmail Inbox loads; scrolling past 500 fetches the next
  window; sending from another client surfaces a new row within seconds.
- [ ] BACKLOG #11 closed.

## Out of scope / BACKLOG entries to add

- BACKLOG: on-disk JMAP blob/state cache (cold-start latency).
- BACKLOG: idle prefetch — warm next-folder window when the account is
  idle for N seconds.
- BACKLOG: byte-cap (in addition to count-cap) on the body LRU.
- BACKLOG: gracefully degraded mode when EventSource is permanently
  blocked (corporate firewall) — fall back to a poll loop.
