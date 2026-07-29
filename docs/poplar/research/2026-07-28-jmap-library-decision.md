# Should poplar keep git.sr.ht/~rockorager/go-jmap?

Decision doc, 2026-07-28. Every claim below is labelled **[field]** (verified by me
against a remote, a clone, or a live API call, with the command or path named) or
**[judgment]** (my reasoning over those facts).

---

## 1. Recommendation

**Keep it, pinned at v0.5.3, and reclassify it from "a dependency poplar tracks" to
"a frozen protocol layer poplar will eventually own."** Do not replace it. Do not
hand-roll now. Do not fork yet, but pre-authorize the fork and write down its
procedure, because the fork is the likely end state and the trigger for it is
knowable in advance (§4).

The two rejected options, briefly:

- **Replace** is not available. **[field]** There is no second Go JMAP library with
  mail-client coverage. `foxcpp/go-jmap` (the fork origin) last pushed 2023-01-31 and
  still self-describes as WIP; `cwinters8/gomap` documents a send-only surface with no
  Mailbox sync, changes/query pagination, push, or blob download. Every fork of either
  repo has zero independent commits.
- **Hand-roll now** spends a few thousand lines to buy nothing poplar needs today.
  **[field]** The library is 3,096 non-test Go lines plus 619 test lines
  (`find … -name '*.go' | xargs wc -l` over a fresh clone of the sourcehut origin).
  **[judgment]** Rewriting that today is a pure cost: the back-reference/JSON-pointer
  resolution and the RFC 8620 errata handling are exactly the parts that are easy to
  get subtly and silently wrong, and poplar would be reintroducing that risk in
  exchange for control it does not yet have a use for.

**[judgment]** The long-term-maintainability directive does not point at "own the code
now." It points at "make sure you can own the code the moment you need to, and know
exactly what owning it means." That is what §3 buys, cheaply.

---

## 2. The concrete risk of keeping it

The phrase "bus factor" is doing no work here. Three specific things are true.

### 2a. The upstream repository cannot currently accept a patch from poplar

This is the real risk, and it is sharper than either input report stated.

**[field] I must correct the alternatives research on a load-bearing point.** It
reported the GitHub mirror as "17 months of untagged commits *ahead* of the sourcehut
tag." The relationship is the reverse. Verified by cloning both and reading the logs:

| Remote | HEAD | Date | What it is |
|---|---|---|---|
| `git.sr.ht/~rockorager/go-jmap` `main` | `feb034f8` | 2025-02-01 | tag `v0.5.3`, the current release |
| `github.com/rockorager/go-jmap` `main` | `25f7b38d` | 2026-07-13 | one community commit sitting on top of `91aad88` |

**[field]** `25f7b38d`'s parent is `91aad88` (2023-10-01), which `git ls-remote` shows
is tag `v0.4.0`. The GitHub mirror is therefore a stale October 2023 snapshot with a
single 2026 patch bolted on; it is *missing* everything from v0.4.0 to v0.5.3,
including all three January 2025 mutex fixes. **[field]** This also explains the
`kagisearch` fork's reported `ahead_by: 29` over `rockorager:main` — those 29 commits
are the v0.4.0→v0.5.3 range absent from the mirror, not Kagi patches.

**[judgment]** The consequence matters more than the correction. That 2026-07-13 merge
is the only evidence the maintainer has touched the project in 18 months, and what it
shows is a maintainer who clicked merge on a one-line fix, put it on a branch 21
months behind his own release, did not forward-port it to the canonical repo, and did
not tag it. That is a sign of life, not a sign of maintenance. Read alongside the two
April 2025 mailing-list patches that have sat unmerged for 15 months **[field, from the
health research]**, the honest statement is: **poplar has no working channel to land a
fix in this library.** Not "the maintainer is slow" — there is no path at all today.

### 2b. Two known defects already ship inside poplar's pinned version

Both verified by reading `client.go` at `v0.5.3` in a fresh clone.

- **[field] `Do()` dereferences `c.Session.APIURL` outside the mutex.** The capability
  check takes `c.Lock()`, then `c.Unlock()`s, and the very next use of the session —
  `http.NewRequestWithContext(req.Context, "POST", c.Session.APIURL, …)` — is unguarded,
  while `Authenticate()` reassigns `c.Session` under lock. That is a live data race in
  the library. **[field]** It does not bite poplar today: `internal/backend/jmap/session.go`
  fetches the session itself (`session.go:65`, its own `httpClient.Do`), assigns
  `client.Session` once at dial time, and never calls the library's `Authenticate()`.
  **[judgment]** A future 401-refresh pass that calls `Authenticate()` from a goroutine
  while requests are in flight converts this into a real race that poplar cannot fix
  upstream (§2a).
- **[field] `Do()` does `io.ReadAll` then `json.Unmarshal`,** not a streaming
  `json.Decoder`, so a batched `Email/changes` + `Email/get` response is fully buffered
  before poplar sees a byte. This is precisely what the April 2025 unmerged patch would
  fix. **[judgment]** A latency and memory concern at poplar's page size, not a
  correctness bug.

### 2c. The one live-relevant portability defect, measured

**[field]** `UploadWithContext` still requires `resp.StatusCode != 200` to mean failure
at v0.5.3; the July 2026 mirror commit relaxes that to any 2xx because Cyrus returns
201. **[field]** Poplar does use this call — `internal/backend/jmap/mail.go:240`,
`m.session.client.UploadWithContext(...)` on the submit path. **[field]** I tested the
actual server: a live upload to Fastmail's `uploadUrl` with `$FASTMAIL_API_TOKEN`
returned **HTTP 200**, so this does not affect poplar today.

**[judgment]** But that is the whole shape of the problem in one example. The fix
poplar would need the day it points at a self-hosted Cyrus or Stalwart server exists,
is two characters wide, is already written, and is unreachable through any released
version. That is what "the upstream channel is closed" costs in practice.

### 2d. What makes the risk material

**[judgment]** Not the calendar gap — poplar's v1 routes calendar to CalDAV by design,
so go-jmap having zero calendar coverage costs nothing until v2. Not source
availability — `go.sum` pins the hash and proxy.golang.org has v0.5.3 cached
permanently, so sourcehut going dark does not break poplar's build. Not another quiet
year on its own.

The risk goes material at exactly one moment: **the first time poplar needs the
library's behavior changed and cannot express the workaround at its own boundary.**
Everything above is dormant until then.

---

## 3. What poplar should do now, cheaply

### 3a. Write the reliance spec — as a test, not a prose list. (Do this.)

**[field]** Poplar's surface is small and now enumerated: ~40 exported identifiers
across 8 packages (root, `core`, `mail`, `mail/email`, `mail/mailbox`,
`mail/identity`, `mail/emailsubmission`), from `jmap.Client`/`Request`/`Response`/
`ResultReference`/`Patch`/`ID`/`MethodError`/`RequestError`/`CoreURI`, through
`core.Core`, the `Get`/`Set`/`Query`/`Changes` quartets for email and mailbox,
`email.Import`/`EmailImport`, `identity.Get`, and `emailsubmission.Set`.

**[judgment]** The right artifact is not a document. It is a test file inside
`internal/backend/jmap` that drives every library behavior poplar depends on against a
scripted `httptest` server: back-reference resolution in a changes+get batch, the
forced `urn:ietf:params:jmap:core` entry in `using`, `Patch` encoding on `/set`, the
three distinct error shapes (HTTP problem-details, per-call `MethodError`, per-object
`SetError`), streaming `DownloadWithContext`, `UploadWithContext`'s accepted status
codes, and the `core.Core` limit fields. That file is simultaneously the specification
a replacement must meet, a regression suite against a future upgrade, and the
acceptance test for a hand-rolled layer. A prose list of relied-on behaviors rots
silently; a test suite fails loudly. **[judgment]** The approved typed-model boundary
work is the correct moment, because that pass already touches every translation point.

### 3b. Do not fork yet; pre-decide it. (One paragraph in the ADR.)

**[judgment]** A fork carrying zero patches is pure overhead — a module path rewrite or
a `replace` directive, plus a sync obligation nobody will honor. Its only value is the
ability to land a patch, and poplar has no patch to land today (§2c: Fastmail returns
200). Record in ADR-0004 or a new ADR that the fork is pre-authorized, name the trigger
(§4), and name the mechanics so the first needed patch is a mechanical action rather
than a re-litigation of this decision.

### 3c. Skip vendoring.

**[judgment]** It buys nothing here. `go.sum` already pins the content hash and the
module proxy already guarantees availability; `go mod vendor` would only add 3k lines
to poplar's diffs and reviews without adding any ability to change them.

### 3d. One WHY-comment, now.

**[judgment]** Poplar's single-assignment, never-call-`Authenticate()` pattern in
`session.go` is currently load-bearing safety (§2b) that reads like an arbitrary
choice. It deserves a comment saying so, in poplar's own comment idiom — a why that is
genuinely non-obvious, and the guardrail against a future session-refresh pass
introducing a race. Cheap and permanent.

---

## 4. The trigger to revisit

**Fork trigger (observable, primary).** Poplar needs a behavior change inside go-jmap
that it cannot work around at its own boundary. Three concrete instances, any one of
which fires it: a server responds outside go-jmap's accepted status codes on a path
poplar uses (the Cyrus 201 case, §2c); a page-size or memory measurement makes the
`io.ReadAll` buffering in `Do()` a real cost; or poplar adds session refresh requiring
`Authenticate()` concurrent with in-flight requests, which needs the unlocked
`c.Session` read fixed. **[judgment]** At that point: fork, land the patch, upstream it
as a courtesy, and expect nothing back.

**Replace-or-hand-roll trigger (observable, secondary).** Either poplar targets a
second JMAP server implementation (Stalwart, Cyrus, self-hosted), or the JMAP calendars
draft finalizes and Fastmail exposes JMAP calendars. **[judgment]** Either one converts
poplar from "one library, one server" into "a project maintaining a JMAP protocol
layer," and at that point the reliance test suite from §3a is the specification and the
fork should already exist. The calendar case is the more likely of the two, and
`go-jmap` has no calendar package and not even a tracking row in its own README status
table **[field, from the health research]** — so that work is greenfield regardless of
which base it sits on.

**Explicitly not triggers. [judgment]** The passage of time. Another twelve silent
months. The sourcehut origin becoming unreachable. A GitHub mirror commit that never
reaches a release. None of these changes what poplar can build or ship, and treating
them as triggers would spend real effort against no real exposure.

---

## Verification commands used

- `git ls-remote https://github.com/rockorager/go-jmap` and `…/git.sr.ht/~rockorager/go-jmap`
- fresh clones of both remotes; `git log --format='%h %ad %an %s' --date=short`;
  `git show 25f7b38 --stat`
- `client.go` at `v0.5.3` read directly for the `Do()` lock scope, the `io.ReadAll`
  path, and the `UploadWithContext` status check
- `grep` over `/home/glw907/Projects/poplar/internal/backend/jmap/*.go` for the
  imported-identifier inventory; `go.mod` confirms the `v0.5.3` pin
- live `POST` to Fastmail's `uploadUrl` with `$FASTMAIL_API_TOKEN`, observed HTTP 200

No files in the poplar repository were modified.
