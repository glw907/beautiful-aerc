# Pass 9h.6 — IMAP PushDraft + discard race + conflict banner

Closes #39 (drafts discard race). Implements `mail.Backend.PushDraft`
on IMAP, lifts the `IsJMAP()` gate from compose lifecycle, and
surfaces a one-shot banner when a server push lands on a row the
user has already discarded.

## Settled (do not re-brainstorm)

- Schema v7 stays. No new migration.
- Last-write-wins semantics for PushDraft (ADR-0164).
- APPEND on the IMAP cmd connection (per IMAP invariants).
- UIDPLUS is a Connect-time invariant; APPENDUID is guaranteed.
- `imapclient.AppendCommand.Wait()` returns
  `*imap.AppendData{UID, UIDValidity}` — the parsing path is just
  to call `Wait()` and read `UID`.
- Race fix: split `cache.UpsertDraft` into explicit `CreateDraft`
  (insert-on-open) and `UpdateDraft` (UPDATE-only; 0 rows = no-op).
  After `DeleteDraft`, any in-flight autosave Cmd's `UpdateDraft`
  call is a benign no-op. Ticker-cancellation is rejected — the
  in-flight `tea.Cmd` already captured state, so a model-side flag
  cannot reach it.
- Conflict-superseded banner: emit once per session per draftID.
  Triggered by `MarkDraftPushed` returning "draft not found" from
  the drainer's PushDraft post-success path.

## Tasks

### 1. Widen `imapClient.Append` to return APPENDUID

`internal/mailimap/client.go`: change

```
Append(folder string, mime []byte, flags []string) error
```

to

```
Append(folder string, mime []byte, flags []string) (mail.UID, error)
```

`realclient.go`: read `data, err := cmd.Wait()` and return
`imapUID(data.UID)`.

`fake_test.go`: extend the fake to record the next-UID counter
and return synthesized UIDs.

`smtp.go` (`Backend.Append`): discard the UID — Append-from-cache
doesn't need it.

### 2. Implement `Backend.PushDraft` (IMAP)

Replace the stub in `internal/mailimap/smtp.go`:

```go
func (b *Backend) PushDraft(folder string, mime []byte, prevUID mail.UID) (mail.UID, error) {
    if folder == "" {
        return "", errors.New("push-draft: empty folder")
    }
    b.mu.Lock()
    cmd := b.cmd
    b.mu.Unlock()

    newUID, err := cmd.Append(folder, mime, []string{"\\Draft"})
    if err != nil {
        return "", fmt.Errorf("push-draft: append: %w", classifyErr(err))
    }
    if prevUID == "" {
        return newUID, nil
    }
    // Best-effort prior-image cleanup. APPEND succeeded; an EXPUNGE
    // failure orphans the prior image but the new draft is good.
    if _, err := cmd.Select(folder, false); err != nil {
        return newUID, nil
    }
    if err := cmd.Store([]mail.UID{prevUID}, "+FLAGS.SILENT", []string{"\\Deleted"}); err != nil {
        return newUID, nil
    }
    _ = cmd.UIDExpunge([]mail.UID{prevUID})
    return newUID, nil
}
```

Test: `TestPushDraft_IMAP_FirstPush`, `..._ReplacesPrev`,
`..._AppendError`. Drop the existing `_Unsupported` test.

### 3. Lift `IsJMAP()` gate from compose lifecycle

`internal/ui/app.go`: drop the four `m.acct.Backend().IsJMAP()`
branches at lines 506, 561, 628, 644. Compose uses cache-backed
persistence on every backend now.

The IMAP-side "discard draft?" legacy modal at line 568 collapses
into the JMAP-side "Save draft?" modal — single code path.

### 4. Split `UpsertDraft` for race-immune discard

`internal/cache/drafts.go`:

- Rename current `UpsertDraft` to `CreateDraft`. Same body —
  INSERT … ON CONFLICT UPDATE keeps the open-race (concurrent open
  of the same draftID) benign.
- Add `UpdateDraft(ctx, draftID, payload)`: `UPDATE drafts SET
  payload = ?, dirty = 1, updated_at = ? WHERE draft_id = ?`. No
  RowsAffected check — 0 rows is the discard-race no-op.

`internal/cache/drafts_test.go`: rename existing tests; add
`TestUpdateDraft_DeletedRow_NoOp`.

`internal/ui/compose/cache.go` (or wherever `CacheStore` lives):
extend the interface to declare both methods.

`internal/ui/compose/model.go`:

- `Init()` calls `CreateDraft` (first creation, idempotent).
- `autosaveTickMsg` calls `UpdateDraft`.

`internal/ui/cmds.go`:

- `upsertAndPushDraftCmd` (save-on-close path) calls `UpdateDraft`
  — by this point the row exists.

### 5. Conflict-superseded banner

`internal/cache/drafts.go`: define
`var ErrDraftSuperseded = errors.New("draft superseded by another client")`.
`MarkDraftPushed` returns `ErrDraftSuperseded` (wrapping) instead
of the current `"mark draft pushed %s: draft not found"` when
RowsAffected == 0.

`internal/cache/drainer.go`: in the `KindPushDraft` dispatch,
when `MarkDraftPushed` returns `ErrDraftSuperseded`, emit a
`CacheEvent` with a new `Kind`-level signal — or simpler, route
through the existing `OpDone` path (the server push *did*
succeed) but include the error string in the event so the UI can
banner. Concrete shape: extend `CacheEvent` with an optional
`Note string` field; populate "draft superseded by another
client" once per draftID per session, gated by an
`Account`-scoped `supersedeOnce map[string]bool`.

`internal/ui/app.go`: in the `account.CacheEventMsg` handler,
when `Note != ""`, set `m.lastErr = ErrorMsg{Op: "draft", Err:
errors.New(Note)}`.

### 6. Outbox visibility for PushDraft

Verify `OutboxSummary` already groups KindPushDraft (it should —
the kind enum is total). Add a fixture row to
`outbox_summary_test.go` confirming.

### 7. Update STATUS, invariants, ADRs

- ADR-0165: IMAP PushDraft (APPEND \Draft + UIDPLUS-scoped
  prevUID expunge; gate-lift; race-immune Create/Update split;
  ErrDraftSuperseded banner).
- `docs/poplar/invariants.md`: rewrite the "Drafts persistence
  (JMAP)" row in the decision index to "Drafts persistence" and
  drop the "(IMAP stub returns ErrUnsupported)" / "App gates on
  IsJMAP()" clauses. Add the IMAP impl shape and Create/Update
  split.
- `BACKLOG.md`: close #39.
- `docs/poplar/STATUS.md`: mark 9h.6 done; next pass = 9.1
  (address autocomplete from CardDAV, #34).

## Out of scope

- Signatures / multiple identities (9.4)
- Address autocomplete (9.1)
- Schedule send / outbox controls (9.2)
- Attachments-rich compose (9.5)
- IDLE-driven sync of server-side draft mutations (push parity is
  one-way for v1 — last-write-wins is the documented contract)
