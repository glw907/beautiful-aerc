# go-jmap: adopt, keep, or write fresh

Every claim below is labelled **[field]** (verified against source, gates,
or repo state) or **[judgment]** (my call on the evidence).

## Verdict

**Adopt it: copy the source into poplar's tree, trimmed to the surface
poplar uses plus the surfaces the design has already committed to, with
the known defects fixed in the landing commit.** Keep the MIT notice and
both copyright lines intact. Do not keep the frozen module dependency,
and do not write the protocol layer from scratch.

The one correction to the mechanics research: **do not write poplar's
own EventSource client off as future work, and do not trim
`statechange.go` and `core/push` on the grounds that nothing calls them
today.** ADR-0005 binds poplar to the EventSource push stream, so those
files are call surface with a date on it, not dead code. Details in
section 3.

## 1. Recommendation, in one sentence

Fork go-jmap v0.5.3 into `internal/backend/jmap/internal/gojmap`,
trimmed to about 1,900 lines, fix the seven or so known defects on the
way in, and own it from that point forward.

## 2. Does the code clear the owner's bar?

**It is adequate with named fixes.** Not "good" without qualification,
and not "not good enough to own." The distinction matters, so here is
what each half rests on.

What earns adoption:

- The **data model is right where it counts**. Spot checks against the
  RFC text (not against names) confirmed `ResultReference`'s
  `resultOf`/`name`/`path` against RFC 8620 §3.7, `RequestError` against
  §3.6.1/§3.6.2, `SetError` against §5.3, and `identity.Get.Requires()`
  returning the submission capability against RFC 8621 §1.3.2. **[field]**
  Getting the JMAP object graph, the back-reference composition, and the
  capability negotiation right is the part that is genuinely expensive to
  redo, and it is the part that is already correct. **[judgment]**
- **Poplar is not fighting it.** The back-reference mechanism composes
  cleanly for the changes-plus-get pattern at
  `internal/backend/jmap/changes.go:106-115, 181-192, 231-240`, and the
  typed-capability decode gives Core's numeric limits and
  EmailSubmission's delayed-send ceiling without protocol-string parsing
  at `internal/backend/jmap/session.go:135-150`. **[field]**
- **The size is ownable.** About 1,740 lines trimmed to the current call
  surface, 29 files; roughly 1,900 with the push types and client added
  back. That is smaller than several single files in poplar's own
  `internal/` tree, and it is data-shape code, not algorithmic code.
  **[field on the counts, judgment on "ownable"]**
- **The licence is clean and unambiguous.** MIT, two copyright lines
  (fox.cpp 2019, Culverhouse 2022), no copyleft, no propagation into
  poplar's own LICENSE. **[field]**

What blocks "as-is" and forces the fixes:

- Seven-plus real defects in a tagged release, three known going in and
  four found in review: `fmt.Sprintf(e.Detail)` treating server-supplied
  prose as a format string (`errors.go:25`); `Account.ID` declared and
  never populated by any code path; `omitempty` on booleans whose RFC
  default is `true` or whose explicit `false` is meaningful
  (`mail/mailbox/mailbox.go:34`, `mail/email/filter.go:47,63-65`);
  `SetError` not implementing `error` unlike its two siblings in the same
  file. **[field]**
- Two literal debug leftovers shipped in v0.5.3: `"error? %v"` at
  `client.go:176` and `t.Logf("TIMBUG %T", ...)` in
  `invocation_test.go:52`. **[field]** A release that ships those did not
  get a review pass. **[judgment]**
- Zero test coverage of the network-facing code. All 619 test lines are
  marshal-shape assertions; `Do`, `Authenticate`, `Upload*`, `Download*`,
  and `ResultReference` resolution end-to-end have none. **[field]** The
  known race lives in exactly the code nobody tested. **[judgment]**

Why "not good enough to own" is the wrong reading: **[judgment]** every
defect above is local. Each is a one-line or one-function fix that
touches no type signature and no caller. Nothing found requires
reshaping the API, which is the signal that would have said the design
was wrong rather than the code being unfinished.

Why "good" is also the wrong reading: **[judgment]** you would not ship
this code under your own name today without the sweep, and adopting it
means it *is* under your own name. The verdict is conditional on doing
the sweep in the landing commit rather than deferring it.

## 3. Scope of work, and where it sits in the pass sequence

### Correction to the trim: keep the push surface

The mechanics research trims `statechange.go` and `core/push` as
uncalled. **They are called by the design, just not yet by the code.**
ADR-0005 (`0005-sync-engine.md:17,36-38,47-49`) makes the EventSource
push stream the supported sync path, with poll as fallback, and records a
live probe confirming Bearer auth works against Fastmail. **[field]** A
trim that removes them buys nothing and costs a re-import later. Keep
`statechange.go` (25 lines: `StateChange`, `TypeState`, `EventType`) as
adopted code. **[judgment]**

`core/push/eventsource.go` (129 lines) is a different call. Read it and
it is the weakest code in the library: **[field]** `Listen()` takes no
context and blocks until the stream dies; there is no reconnection and no
SSE `Last-Event-ID` resume, so a dropped connection loses the resume
point that ADR-0005's 30s p95 push-recovery criterion depends on; the
`bufio.Scanner` carries the default 64KB line cap on server-controlled
data lines; `Close()` discards its error; the `id:` field and multi-line
`data:` payloads are ignored outright. **[field]** Adopt the types, write
the client. **[judgment]** That is a poplar-authored file in the sync
pass, not an adoption task, but the trim decision has to be made with it
in view so `statechange.go` survives.

The clean generalization: **[judgment]** the library's value is
concentrated in the wire types, the method registry, and the capability
model. Its transport code (`Do`'s `io.ReadAll`, `Authenticate`'s missing
context, the oauth2-bound auth helpers, `EventSource.Listen`) is the part
poplar has already been rewriting or bypassing. Poplar has already
hand-rolled session fetching (`session.go:34-40, 58-86`) and its own
`http.RoundTripper` (`session.go:106-118`) for exactly this reason.
**[field]** Adoption should be read as taking the data model and
progressively owning the transport, not as taking a whole client.

### The work

| Item | Scope |
|---|---|
| Files adopted | 30 of 46 in the seven imported packages, plus `statechange.go` retained **[field]** |
| Lines adopted | ~1,765 (~1,740 trimmed set + 25 for `statechange.go`) **[field]** |
| Import blocks rewritten | 30 vendored files + 4 poplar callers + `go.mod`/`go.sum` **[field]** |
| Dependencies dropped | `git.sr.ht/~rockorager/go-jmap` and `golang.org/x/oauth2`; `go mod why` confirms go-jmap is oauth2's only path into poplar **[field]** |
| Licence artifacts | `LICENSE` verbatim, both copyright lines; `NOTICE.md` recording v0.5.3, the trim date, and the delta list; one provenance sentence in the package doc comment **[field]** |

Defects fixed in the landing commit, all of them:

1. `Do()` reads `c.Session.APIURL` outside the mutex. Latent race.
2. `Do()` buffers with `io.ReadAll` instead of decoding off the stream.
3. `UploadWithContext` rejects any non-200; Cyrus returns 201. Poplar
   calls it at `internal/backend/jmap/mail.go:240`.
4. `errors.go:25` format-string bug.
5. `Account.ID` dead field: delete it.
6. `omitempty` on the four RFC-meaningful booleans.
7. `SetError` gets an `Error()` method, and
   `internal/backend/jmap/errors.go:97-103` stops reconstructing one by
   hand and stops dropping the server's `Description`.

Plus the mechanical gate fixes the mechanics research verified by
actually running poplar's linters: errcheck ×4, govet printf ×1,
modernize ×6, staticcheck ST1005 ×1, and three `//nolint:gosec // G704`
justifications matching the existing `convert.go` pattern. **[field]**
Vale runs clean on the adopted prose today at suggestion level, verified
with a positive control. **[field]**

Two tests are worth writing with the adoption rather than later, because
they cover the code the fixes touch and the composition poplar leans on
hardest, and neither exists upstream: `Do`/`Upload`/`Download` against an
`httptest` server, and `ResultReference` resolution end-to-end.
**[judgment]**

### Sequencing: two tasks, adoption first, both in pass 1b

**Two tasks, not one.** **[judgment]** They touch the same package but
different halves of it, and bundling them produces a diff nobody can
review. Adoption is mechanical and verifiable by "the tests that passed
before still pass." Typed models are a semantic change to poplar's own
`backend.Record` vocabulary. Mixed together, a wire-type regression hides
inside a model refactor.

**Adoption first**, for three reasons:

- The typed-model work rewrites `messageFields`/`mailboxFields`
  (`changes.go:297-397`) and their siblings in `mail.go`, which are the
  only places a go-jmap wire type is read. **[field]** Writing them
  against the post-fix wire types means writing them once. Doing typed
  models first means touching the same functions twice, the second time
  for the `SetError` and `omitempty` fixes.
- The nested-`internal` placement makes the translation boundary
  compiler-enforced rather than conventional: nothing under
  `internal/sync`, `internal/store`, or `internal/ui` can import the wire
  types at all. **[field]** That guarantee is worth more before the typed
  models are designed than after, because it removes the failure mode
  where a `backend.Message` field ends up holding a `jmap.ID`.
- The import-boundary analyzer's `forbidden` map does not currently stop
  `internal/sync` from importing a backend package directly. **[field]**
  Adoption closes that with the compiler instead of a lint rule.

**Not in pass 1's consolidation.** **[judgment]** Pass 1 is closing; a
dependency-graph change and a seven-defect sweep landing during
consolidation puts new risk in front of the pass gate for no gain.
Adoption is the first task of the pass 1b plan, ahead of the typed-model
tasks in the same plan.

## 4. What poplar gives up

Upstream fixes are worth nothing, so set that aside. **[field: no release
since 2025-02-01, GitHub `main` sits on the v0.4.0 tag, no working patch
channel.]** What is actually lost:

- **Provenance leaves the module graph.** Once the source is in
  `internal/`, `go list -m all`, `go.sum`, licence scanners, and any
  advisory keyed to the module path stop seeing it. **[field]** The
  nightly `govulncheck` job still analyzes the code as poplar source, but
  a hypothetical advisory filed against `git.sr.ht/~rockorager/go-jmap`
  would never match. **[judgment]** `NOTICE.md` is the mitigation, and it
  is a weaker one than `go.sum` was.
- **Permanent ownership of the protocol layer.** Every RFC erratum, every
  server-interop quirk (Cyrus, Stalwart, Dovecot's JMAP), every new
  datatype poplar reaches for later is poplar's to write. **[judgment]**
  The trim makes this concrete: `thread`, `searchsnippet`,
  `vacationresponse`, `core/blob`, and `mdn` are dropped, so a future
  threading or snippet feature means writing those types rather than
  importing them. Small work each time, but it never stops.
- **A one-way door on merge legibility.** After the trim, the rewritten
  imports, and the defect fixes, the tree no longer diffs cleanly against
  v0.5.3. **[judgment]** If a maintained fork appears in a year, taking
  it is a manual reconciliation, not a version bump. This is the real
  cost of adoption over keeping the dependency, and it is the reason
  `NOTICE.md` must record the delta list rather than just the version.
- **No inherited tests.** Upstream's 619 test lines use `testify` and are
  not carried across, so the vendored subtree ships with no unit tests of
  its own on day one. **[field]** Poplar's existing
  `mail_test.go`/`changes_test.go`/`session_test.go` exercise it
  end-to-end, which is coverage, but not of the transport code the fixes
  touch. **[judgment]** Hence the two tests above.
- **The gate now runs against 1,765 more lines.** Marginal, but every
  future `golangci-lint` version bump, every `modernize` rule addition,
  and every Go version upgrade now has more surface to satisfy in code
  poplar did not write. **[judgment]**

What poplar does *not* give up, contrary to the usual argument against
vendoring: **[judgment]** no shared bug-finding, because there is no
upstream community; no ecosystem compatibility, because nothing else in
poplar's graph imports go-jmap; and no update cadence, because there is
none.

## 5. What would make this the wrong call

Four things, with the evidence that would show each.

**A maintained fork exists or appears.** The strongest counter-evidence.
Look for a fork with commits after 2025-02-01, a merged patch, and a
maintainer who answers. Concretely: the GitHub network graph on the
mirror, the sourcehut mailing list archive, and `pkg.go.dev`'s importers
list for a fork path with real traffic. **[judgment on the test]** If one
turns up before the landing commit, depend on it instead; if it turns up
after, the one-way door above means poplar keeps its fork and cherry
-picks. The pre-adoption check is cheap and should be run once more the
day the task starts.

**The day-one fixes turn out not to be local.** The verdict in section 2
rests entirely on every defect being a one-line or one-function change.
The abort signal is concrete: **[judgment]** if fixing the seven requires
changing a type signature that ripples into poplar's own
`internal/backend/jmap/*.go` callers, or if the fix diff exceeds roughly
30% of the adopted lines, the code was not "adequate with named fixes"
and the honest re-read is "write fresh." Stop the task and re-decide
rather than pushing through.

**Poplar's JMAP surface grows well past the trimmed set.** If threads,
search snippets, blob handling, vacation responses, MDN, and
WebSocket push (RFC 8887, which go-jmap does not implement at all, per
ADR-0005:36) all land, the owned protocol layer roughly doubles and
starts needing a per-server compatibility matrix. **[field on the
absence, judgment on the threshold]** At that point poplar is maintaining
a JMAP library as a side effect of maintaining an email client, which is
a different job than the one being signed up for here. The evidence would
be the adopted subtree becoming a churn hotspot: more than a couple of
protocol-layer commits per pass, sustained over two or more passes.

**The wire types themselves prove wrong against a second server.** The
RFC checks in section 2 were spot checks against the paths poplar
exercises, not a full audit. **[field]** If bringing up a second JMAP
server surfaces structural problems in the types rather than the
transport, the "data model is right, transport is weak" premise
collapses and the whole calculus changes. **[judgment]** Fastmail is the
only server poplar has probed, so this risk is genuinely open. It argues
for one thing in the near term: when a second server is first tried, read
the failures as evidence about the adoption decision, not just as bugs.

## Summary

Adopt. The data model earns it, the licence permits it, the size makes it
ownable, and the frozen upstream removes the only real argument for
keeping the dependency. Land it as one mechanical task at the head of
pass 1b, with all seven defects and the gate findings fixed in the same
commit, `statechange.go` retained against ADR-0005, and the push client
written by poplar rather than adopted. Then do typed models against the
finished wire types.
