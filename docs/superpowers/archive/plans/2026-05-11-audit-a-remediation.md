# Pass 26.1 — Audit A remediation

Bundle the five non-blocking Audit A findings (#54–#58, ADR-0210)
into one remediation pass before Batch 2 opens. Fix shapes are
already specified in the BACKLOG entries; this plan is a task list.

## Tasks

### #54 — mailjmap classifyErr network-error branch

`internal/mailjmap/errors.go`. Today only `*jmap.RequestError`
401/403/404 is classified; plain transport drops fall through, so
`mail.ErrConnection` parity with mailimap is broken.

- Extend `classifyErr` with a connection-dead branch mirroring
  `internal/mailimap/errors.go:isConnectionDead`: `io.EOF`,
  `io.ErrClosedPipe`, `io.ErrUnexpectedEOF`, `net.ErrClosed`,
  `*net.OpError`, `net.Error.Timeout()`, and `*url.Error` whose
  unwrapped error matches any of the above. Wrap via
  `mail.WrapSentinel(err, mail.ErrConnection)`.
- Factor the shared shape into `mail.IsConnectionDead(err) bool` if
  the mailimap helper extracts cleanly — otherwise keep two
  package-local helpers (mailjmap already imports `net`/`io` once
  this lands).
- Test: fake `jmap.Client` (the test seam already used in
  `jmap_test.go`) returning a `*url.Error` wrapping `io.EOF`;
  assert `errors.Is(err, mail.ErrConnection)`.

### #55 — mailjmap refreshFoldersLocked releases lock across HTTP

`internal/mailjmap/jmap.go:316-324`. Holds `b.mu` across
`fetchFolders → b.client.Do`, stalling every reader and the push
loop's `handleStateChange` for the network round-trip.

- Rename `refreshFoldersLocked` to `refreshFolders` (it will no
  longer be lock-held).
- New shape: acquire `b.mu`, snapshot `client := b.client`,
  `session := b.session`, release; call `fetchFolders(client,
  session)` lock-free; re-acquire to write `b.folders` and
  `b.states["Mailbox"]`. Pattern mirrors `resolvedPassword`.
- Update the single caller (`ListFolders`) and verify
  `handleStateChange` / `dispatchMailboxChanges` paths don't
  re-enter `refreshFolders` under the lock. Inspect each
  `b.mu.Lock()` in `jmap.go` and `push.go` for read-then-call
  patterns that previously assumed the lock was held across
  the call — they all already release before issuing RPCs.
- Test: drive `refreshFolders` against a fake client that blocks
  on a channel; assert a concurrent `ListFolders` (or `b.mu`
  consumer) can acquire while the round-trip is parked.

### #56 — mailimap Destroy (Gmail) restores b.current

`internal/mailimap/actions.go:95-105`. The Gmail branch issues
`cmd.Select(trash, false)` but never restores `b.current`, so a
subsequent redial re-Selects the pre-Destroy folder mid-cmd
sequence.

- After the trash Select succeeds and the +FLAGS / UID EXPUNGE
  complete, re-Select the pre-Destroy folder (captured from
  `b.current` under the lock at function entry) before
  returning. On the re-Select error path, still return success
  for the Destroy itself but route the re-Select error through
  `maybeDropOnConn` and wrap as a non-fatal mailbox-state error.
- Defer-restore pattern: capture `prev := b.current` at the top,
  install a deferred `cmd.Select(prev, false)` only when `gmail
  && prev != "" && prev != trash`.
- Test: stub `cmdClient` with a recorder fake (`mailimap.imapClient`
  test seam) sequencing the calls. Assert the final Select target
  equals the pre-Destroy folder.

### #57 — config validator completeness

`internal/config/`. Six gaps — strict TOML is the root.

- **F2.0 (root).** Replace the four `toml.Unmarshal(data, &v)`
  call sites with `toml.NewDecoder(bytes.NewReader(data)).Decode(&v)`
  (or `toml.Decode(string(data), &v)`), capturing the returned
  `MetaData`. After Decode, scan `md.Undecoded()`; for each
  unknown key, build a `*ConfigError` with `Field` = the dotted
  key path and `Suggest` filled via
  `internal/strdist.Levenshtein` against the static set of valid
  keys at that depth. Use the existing `suggestProvider` pattern.
  Sites: `accounts.go:366`, `ui.go:155`, `cache.go:57`,
  `writer.go:111`.
  - Build a small known-keys table per top-level section (only
    needed shape: a flat `[]string` of valid dotted paths per
    decoded struct). Generate from struct tags at first use via
    `sync.OnceValue`.
- **F2.1.** Validate `oauth-store` against `{"keyring",
  "age-file"}` in `toAccountConfig` (post-fallback). Empty is
  legal; unknown raises `*ConfigError` with `Suggest` listing
  the two valid values.
- **F2.2.** Validate `auth` and `smtp.auth` against `{"plain",
  "login", "cram-md5", "xoauth2", "bearer"}`. Empty legal.
- **F2.3.** Distinguish empty `contacts.url` from un-parseable:
  in `ContactsConfig.validate`, return a clearer `"url: required"`
  when `c.URL == ""`.
- **F2.4.** For bare `provider = "imap"` (no preset), require
  `port != 0` next to the `host == ""` check.
- **F2.5.** Run `contacts.validate` after `finalizeContacts` and
  *also* check that resolved credentials are non-empty when
  `[account.contacts]` is present (i.e., post-fallback). The
  current `validate()` only catches missing URL.
- Tests next to each new check: table-driven cases in
  `accounts_test.go`/`ui_test.go`/`cache_test.go` showing the
  new error class fires and existing valid configs still pass.

### #58 — defensive-clamp sweep

Delete the guards from BACKLOG #58. Per file:

- `cache/syncer.go:17,72,115`: drop the `ChangeTracker == nil`
  and `a.Backend == nil` guards.
- `cache/drainer.go:130`: drop `a.Backend == nil`.
- `cache/drainer.go:231,241` (`ContactsWriter == nil`): **keep**;
  add a one-line doc comment that the field is legitimately
  optional. The F1.4 inline addition: a single-line comment
  in `cache/drainer.go:executeOne` noting that `ErrConnection`
  routing is implicit via the `default` arm with backoff.
- `cache/reads.go:262`: drop `a.Backend == nil`.
- `cache/reads.go:393`: drop `n <= 0` early return in
  `sqlPlaceholders` (callers already gate on `len == 0`).
- `cache/attachments.go:26,116`: drop `a.Backend == nil`.
- `cache/ops.go:65,114`: drop `args == nil` guards.
- `cache/search.go:34`: move the `limit <= 0 → 200` default to
  the single call site; drop the clamp.
- `ui/status_bar.go:72,75`: drop `< 0 → 0` clamps in
  `SetOutboxDepth`.
- `ui/status_bar.go:100,103`: drop the `< 0` / `> 100` clamps in
  `SetScrollPct`.

After each deletion, run the package tests; any regression
means the guard was load-bearing and the deletion needs revisit.

## Pass-end ritual

1. `/simplify`.
2. ADR-0211 — Strict TOML decoding and validator completeness.
   The strict-TOML switch is a new binding fact (config files now
   reject unknown keys with a Levenshtein suggestion). The other
   #57 sub-items ride on the same ADR. #54–#56 are bug fixes that
   don't shift any invariant; no separate ADR.
3. Update `docs/poplar/invariants.md` — narrow the config section
   to mention strict decoding. Update `INDEX.md` with the new ADR.
4. Update `STATUS.md` — mark 26.1 done, promote Pass 27 to
   next-up (its starter prompt is already in STATUS).
5. Archive this plan to `docs/superpowers/archive/plans/`.
6. `make check`.
7. Commit, push, `make install`.
