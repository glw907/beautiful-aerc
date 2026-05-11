---
title: Audit A remediation — strict-TOML decoding, JMAP connection sentinel, lock-release across HTTP, Gmail Destroy folder restore, defensive-clamp sweep
status: accepted
date: 2026-05-11
---

## Context

ADR-0210 logged Audit A as 0 blocking / 11 non-blocking and waved
Batch 2 through without a remediation pass. The decision was
revisited: bundling the five entries as a 10–12 task pass before
Pass 27 keeps the remediation cadence consistent with the
audit-plan §Mechanics rule (non-blocking findings land within two
passes of being logged) and lands the strict-TOML binding fact
before Catkin Elm conformance rewrites the compose state path.

The five entries:

- **#54** mailjmap `classifyErr` had no `mail.ErrConnection`
  branch — transport drops fell through, breaking sentinel parity
  with mailimap.
- **#55** `mailjmap.refreshFoldersLocked` held `b.mu` across the
  `Mailbox/get` HTTP round-trip; readers and the push loop's
  `handleStateChange` stalled together.
- **#56** mailimap Destroy's Gmail branch internally
  `Select(trash)`s and never restored the prior folder. A
  subsequent redial would re-Select the wrong mailbox.
- **#57** Config validators accepted unknown TOML keys silently
  (no strict decode), plus missing enum and emptiness checks on
  `oauth-store`, `auth`/`smtp.auth`, bare-IMAP `port`, contacts
  credentials post-fallback, and the `contacts.url` empty-vs-
  unparseable message.
- **#58** Defensive nil/clamp checks in `internal/cache/` and
  `internal/ui/status_bar.go` that violated the no-defensive-
  checks rule.

## Decision

Land all five inline.

**Connection-dead classification moves to `internal/mail/`.** A
new `mail.IsConnectionDead(err) bool` consolidates the
`io.EOF` / `io.ErrClosedPipe` / `net.ErrClosed` / `*net.OpError` /
`net.Error.Timeout()` / `*url.Error` recursion that previously
lived only in mailimap. Both backends route their `classifyErr`
through it before falling back to library-specific shapes.

**Lock-and-call inversion in mailjmap.** `refreshFoldersLocked`
becomes `refreshFolders`: take `b.mu` to snapshot
`client`/`session`, release, run `fetchFolders` lock-free, re-
acquire to write `b.folders` / `b.states["Mailbox"]`. Mirrors the
existing `resolvedPassword` pattern. `ListFolders` is the only
caller; it adopts the same snapshot-release shape on its empty-
folders fast path.

**Gmail Destroy restores `b.current`.** The Gmail branch captures
`prev := b.current` under the lock at entry, sets `b.current =
trash` after the internal Select, and installs a deferred re-
Select of `prev` that runs after the STORE+EXPUNGE complete (or
errors). The defer fires only when `prev != ""` and
`prev != trash`. Re-Select errors route through `maybeDropOnConn`
rather than masking the Destroy success.

**Strict TOML decoding is now a binding fact for the config
package.** `internal/config/strict.go` adds `strictDecode(data,
v)` wrapping `toml.NewDecoder(...).Decode(v)` and walking
`md.Undecoded()`. Unknown keys surface as
`*config.ConfigError{Field, Message, Suggest}` with a Levenshtein
suggestion (via `internal/strdist`) of the nearest sibling field
of the destination struct. Reflection helpers (`outOfScope`,
`dropsIntoMap`, `suggestSibling`) handle the four call sites
(`accounts.go`, `ui.go`, `cache.go`) sharing a single TOML file
across `[[account]]`/`[ui]`/`[cache]` sections — each Load*
ignores unknowns rooted at a sibling section that another loader
owns. `map[string]string` fields (e.g. `[account.params]`) are
accept-anything by design and bypass the strict check.
`writer.go:FolderKeys` keeps `toml.Unmarshal` — it is an
intentional partial-decode helper, not a validator.

**Enum + emptiness validators.** `validateAuth` (allows `plain` /
`login` / `cram-md5` / `xoauth2` / `bearer` / empty) and
`validateOAuthStore` (allows `keyring` / `age-file` / empty) fire
in `toAccountConfig`. Bare `provider = "imap"` now requires
`port != 0`. Contacts credentials are validated post-
`finalizeContacts` (username + password-or-password-cmd present
after parent-account fallback) in `toAccountConfig`, not in
`ContactsConfig.validate()` — keeps the value-type validate()
focused on URL/refresh shape. `contacts.url == ""` now surfaces
as "required" rather than "not parseable".

**Defensive-clamp sweep.** Seven internal-to-internal guards in
`internal/cache/` and `internal/ui/status_bar.go` deleted:
`a.Backend == nil` (×5), `ChangeTracker == nil`, `args == nil`
(×2), `sqlPlaceholders n <= 0`, `Search limit <= 0`, status-bar
outbox-depth `< 0` and scroll-pct `< 0`/`> 100` clamps. The
`ContactsWriter == nil` guard in the drainer's contact dispatch
arms stays — that field is legitimately optional for accounts
without CardDAV configured — with a one-line rationale comment.
The drainer's `default` arm gets an inline note that
`mail.ErrConnection` rides the transient/backoff curve there
(backends drop their dead clients and redial on the next
attempt) — closes the F1.4 inline finding from Audit A.

## Consequences

- **Sentinel parity.** JMAP and IMAP backends now route transport
  drops through the same `mail.ErrConnection` path. The drainer's
  `default` arm sees consistent shapes across backends; the
  `pumpUpdatesCmd` reconnect lens covers both equally.
- **Push-loop concurrency improves.** Refreshing the folder map
  no longer blocks `handleStateChange` or any reader for the
  duration of a `Mailbox/get` round-trip. Worst-case stall on a
  slow JMAP server drops from "round-trip duration" to "local
  copy + write".
- **Strict TOML is now the contract.** A typo in any decoded
  config file (`passwrd`, `forlder-sort`, `oath-store`) surfaces
  immediately with a Levenshtein suggestion of the right key.
  Documentation in `docs/poplar/config-format.md` (if written
  later) becomes load-bearing — there is no silent-drop escape
  hatch left. Map-typed sections (`[account.params]`,
  `[ui.folders.<name>]`) keep their accept-anything semantics.
- **Adjacent #57 leaf checks tighten the wizard's pre-save probe
  surface.** Bare-IMAP `port = 0` now fails at config-load
  rather than at first dial. Contacts misconfiguration surfaces
  as a typed `ConfigError` rather than a CardDAV 401 at runtime.
- **No-defensive-checks rule reasserted.** The deleted guards
  match the catalogue in BACKLOG #58; if a deletion turns out to
  be load-bearing under some future code path, the package test
  suite (which now exercises every removed-guard surface) is the
  signal — not a re-added clamp.
- **No invariant deletes.** The remediation is purely additive
  to the binding facts in `invariants.md` (one new line about
  strict TOML; the existing config / sentinel / Destroy facts
  pick up the new shapes without rewrites).
