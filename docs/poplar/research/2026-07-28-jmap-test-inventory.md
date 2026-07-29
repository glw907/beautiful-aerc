# Test plan and validation setup for poplar's `jmap` library

Status: research deliverable, ready to consume as a build task.
Date: 2026-07-28.
Subject: package `jmap` at `poplar/jmap`, destined for its own repository.

Every claim below carries a source. RFC line numbers refer to the plain-text
RFCs saved at
`/tmp/claude-1000/-home-glw907-Projects-poplar/47f73ae1-3feb-488d-a9f3-147d090563bd/scratchpad/research/rfc8620.txt`
and `.../rfc8621.txt`, fetched from rfc-editor.org. go-jmap file paths are
relative to `$(go env GOMODCACHE)/git.sr.ht/~rockorager/go-jmap@v0.5.3`.

---

## 0. How a build task consumes this document

**Test IDs are stable.** Each inventory row is `JT-nn`. A build task may cite
`JT-05` as an acceptance criterion and a reviewer may check it off. Divergence
tests are `DV-nn`.

**Tiering is by silent-data-loss risk, not by build order.** Tier 4 is the
wire-format foundation. It ranks last on risk and first on build order, because
nothing above it compiles without it. Build 4, then 1, then 2, then 3.

**Fixture convention.** JSON fixtures live in `jmap/testdata/`, one file per
fixture, mirroring the existing convention in
`/home/glw907/Projects/poplar/internal/backend/jmap/testdata/` (29 files,
committed at `7d29fe1`). Fixtures transcribed from an RFC take the file name
`rfc8620-3.7-backref-wildcard.json` and so on, so the source is visible in the
name. Tests are table-driven per `go-conventions`.

**Three test surfaces, three gates.**

| Surface | Build tag | Runs in | What it proves |
|---|---|---|---|
| Unit and fixture tests | none | `make check` | poplar reads the RFC correctly |
| Fake-transport tests (`httptest.Server`) | none | `make check` | poplar's transport handles the unhappy paths |
| Conformance suite | `conformance` | `make conformance`, local only | poplar agrees with a second real server |
| Live account suite | `live` | manual only | poplar agrees with Fastmail |

The `live` tag and the token-missing skip already exist at
`internal/backend/jmap/live_test.go:1` and `:28`. Reuse that idiom exactly.
CI never touches a live account, per ADR-0014.

**A note on what fixture tests can and cannot prove.** A fixture transcribed
from an RFC proves poplar reads the specification the way its authors wrote it.
It does not prove a server behaves that way. Sections 3 and 4 close that gap.
Section 5 states what stays open.

---

## 1. The test inventory

### Tier 1: silent data loss

Getting these wrong destroys or misplaces mail without an error reaching the
user. Build them first among the risk tiers and review them hardest.

---

**JT-01. A `/set` response with both successes and failures surfaces every
outcome, and the API makes ignoring the failure maps awkward.**

- Source: RFC 8620 §5.3. The response carries `created`/`notCreated`,
  `updated`/`notUpdated`, `destroyed`/`notDestroyed` as six independent maps.
- Assert: a fixture with three creates, one succeeding and two failing with
  different `SetError` types, decodes to three distinguishable outcomes. The
  exported result type offers no single `ok bool` or `err error` that reads as
  "all three worked". A caller that inspects only the error return must not
  compile into a false success.
- Protects against: an email believed sent, flagged, or moved when one of a
  batch silently failed. This is the single highest-value test in the plan,
  because the failure is invisible at every layer above the library.
- Fixture: `set-partial-failure.json`, synthesized from the §5.3 field list.
- Corroboration: Apache James `EmailSetMethodContract.scala` (Apache-2.0, 118
  tests) treats per-record failure as a first-class assertion throughout.

---

**JT-02. A PatchObject patches one leaf. A `null` value removes or defaults that
leaf, never its parent.**

- Source: RFC 8620 §5.3 (PatchObject rules), RFC 8621 §4.6 (keywords and
  mailboxIds patch syntax).
- Assert: setting `$seen` marshals to `{"keywords/$seen": true}`. Clearing it
  marshals to `{"keywords/$seen": null}`. Neither form ever produces
  `{"keywords": null}` or `{"keywords": {...}}` unless the caller explicitly
  asked for a whole-property replacement.
- Protects against: a one-flag keyword change that clears the entire keyword
  set, or a mailbox move that empties `mailboxIds` and functionally deletes the
  message. Cyrus names this hazard three separate times in its own corpus
  (`email_set_patch`, `email_set_move_keywords`, `email_set_move_multiuid_patch`
  under `cassandane/tiny-tests/JMAPEmail/`).
- Note: go-jmap proves the explicit-null wire form works
  (`mail/mailbox/set_test.go`) using `jmap.Patch` as a
  `map[string]interface{}`. That is the one idea worth carrying (section 2).
  The typed-API question of whether a caller can build a destructive patch by
  accident is untested anywhere and is poplar's to answer.

---

**JT-03. poplar refuses to construct an illegal patch.**

- Source: RFC 8620 §5.3. A PatchObject MUST NOT contain an array index in a
  pointer, and MUST NOT contain two pointers where one is a prefix of the
  other. Servers reject both with `invalidPatch`.
- Assert: building `{"mailboxIds/0": ...}` fails at the library boundary.
  Building `{"keywords": {...}, "keywords/$seen": null}` in one patch fails at
  the library boundary. Neither reaches the wire.
- Protects against: a round trip that appears to succeed locally and then
  either fails opaquely at the server or, worse, is applied by a lenient server
  in an order poplar did not intend.

---

**JT-04. Moving an email between mailboxes patches the specific keys and leaves
the rest.**

- Source: RFC 8621 §4.6 plus the §4.10 worked narrative.
- Assert: a move from A to B marshals `{"mailboxIds/A": null, "mailboxIds/B":
  true}`. A whole-object replacement is available but requires an explicit,
  differently named call.
- Protects against: functional deletion. An email in no mailbox is invisible.

---

**JT-05. Back-reference resolution, including the `*` wildcard flattening,
matches the RFC's own four-hop example byte for byte.**

- Source: RFC 8620 §3.7, lines 1241 to 1400. The RFC gives two complete
  chains: a one-hop `Foo/changes` to `Foo/get` resolving `/created`, and a
  four-call `Email/query` to `Email/get` to `Thread/get` to `Email/get` chain
  that exercises `"*"` array-of-arrays flattening end to end, with full
  intermediate JSON at every step.
- Assert: both chains, transcribed verbatim, resolve to the exact id sets the
  RFC shows.
- Protects against: a wrong id set feeding a destructive call. If poplar ever
  chains a query into a `/set destroy` or a move, a flattening bug destroys or
  moves the wrong messages and reports success.
- Fixture: `rfc8620-3.7-backref-simple.json`,
  `rfc8620-3.7-backref-wildcard.json`.
- Corroboration: Stalwart isolates back-reference resolution into its own
  module (`crates/jmap-proto/src/references/{eval,jsptr,resolve}.rs`), which is
  a structural signal that the team judged it hard enough to separate. That is
  an observation about their architecture, not reusable material: Stalwart is
  AGPL-3.0-only, so no code or test from it may be read into poplar.

---

**JT-06. Every back-reference failure is loud, and never resolves to an empty
but valid list.**

- Source: RFC 8620 §3.7, lines 1239 to 1241: "If any result reference fails to
  resolve, the whole method MUST be rejected with an `invalidResultReference`
  error."
- Assert, as a table: `resultOf` naming no prior call id; `name` not matching
  the referenced call's method; a path that does not resolve; `*` applied to a
  non-array; and a reference into a call that itself returned an `"error"`
  invocation. Each produces a typed `invalidResultReference`, and none produces
  an empty id slice with a nil error.
- Protects against: a destroy or a move that silently no-ops, or that operates
  on the wrong set, because a broken reference degraded to "no ids".
- Corroboration: James `BackReferenceContract.scala` has six tests including
  `wildcardRequiresAnArray` and `resolvingAnUnexistingMethodCallIdShouldFail`.
  Apache-2.0, so its assertion logic may be read and adapted with attribution.
  A copy is at `.../scratchpad/research/james-BackReferenceContract.scala`.

---

**JT-07. An arguments object containing both `foo` and `#foo` is rejected with
`invalidArguments`.**

- Source: RFC 8620 §3.7, lines 1241 to 1243, verbatim: "If an arguments object
  contains the same argument name in normal and referenced form (e.g., 'foo'
  and '#foo'), the method MUST return an 'invalidArguments' error."
- Assert: poplar refuses to build such a request, and decodes the server's
  `invalidArguments` correctly if one is returned anyway.
- Protects against: a server picking whichever of the two it likes, so poplar's
  intended argument is silently discarded. No implementation surveyed (James,
  Cyrus, jmapc, go-jmap) has a named test for this case.

---

**JT-08. A reused creation id maps to the most recently created item.**

- Source: RFC 8620 §5.3, lines 2001 to 2003, verbatim: "If a creation id is
  reused, the server MUST map the creation id to the most recently created item
  with that id."
- Assert: two creates in one request both using `#k1`; a later reference to
  `#k1` resolves to the second one. Explicitly assert last-write-wins, not
  first-write-wins.
- Protects against: new data attached to a stale, already-superseded record. A
  naive `map[string]ID` populated in call order is correct here only if read
  after the full pass, which is exactly the kind of ordering assumption a test
  must pin.

---

**JT-09. Creation ids substitute across calls, and the `createdIds` request seed
and response echo round-trip.**

- Source: RFC 8620 §3.3, §3.4, §5.3 (lines 2003 to 2008), and the §5.7 `Todo`
  narrative, which shows a create-with-`"#k15"` foreign-key reference in a
  later call.
- Assert: an id created in call 1 resolves in call 2; a `createdIds` map passed
  in on the request appears in the response; the response's final map reflects
  every creation made.
- Protects against: a foreign key pointing at nothing, which surfaces as a
  message filed into a mailbox that does not exist. Also required if poplar
  ever proxies (§5.8), even though it does not today.
- Fixture: transcribe the §5.7 six-request narrative. It is the richest single
  fixture source in either RFC.

---

**JT-10. `EmailSubmission/set` returns two responses under one call id, and the
implicit `Email/set` is matched by creation id, not by position.**

- Source: RFC 8621 §7.5.1, lines 4655 to 4790. The example shows an
  `onSuccessUpdateEmail` create whose response array contains both the
  `EmailSubmission/set` response and an implicit `Email/set` response sharing
  one call id. Lines 4735 to 4785 give the rejected variant, where the
  submission fails with `forbiddenToSend` and carries a localized German
  description.
- Assert: both responses are retrievable; the implicit `Email/set` is matched
  back to its submission by the `#creationId` sharp reference; and when the
  submission fails, poplar does not apply the `onSuccessUpdateEmail` effects.
  Test `onSuccessDestroyEmail` on the same shape.
- Protects against: a draft that stays flagged `$draft` in Drafts after it was
  actually sent, or is marked sent and moved out of Drafts after the send
  failed. Both are silent.
- Corroboration: James names this hazard four times, including
  `onSuccessUpdateEmailShouldTriggerAnImplicitEmailSetCall` and
  `setShouldFailWhenOnSuccessUpdateEmailMissesTheCreationIdSharp`, in
  `EmailSubmissionSetMethodContract.scala` (Apache-2.0, copy in the scratchpad).

---

**JT-11. `methodResponses` is indexed by call id, tolerating duplicates and
reordering.**

- Source: RFC 8620 §3.4, plus two deliberate same-call-id patterns: `Todo/copy`
  with `onSuccessDestroyOriginal` producing an implicit `Todo/set` (§5.7) and
  the `EmailSubmission/set` case above (RFC 8621 §7.5.1). RFC 8620 §3.3.1 and
  §3.4.1 also show one method returning two responses
  (`anotherResponseFromMethod2`) under one call id.
- Assert: a fixture with two responses sharing a call id yields both; a fixture
  whose responses appear out of array order still resolves each to the right
  call; a `map[callID]Invocation` shape that drops the second entry fails the
  test.
- Protects against: the JT-10 failure mode reached by a different route, and
  any future chained-call feature silently reading the wrong response.

---

**JT-12. Typed `SetError` extra fields survive decoding.**

- Source: RFC 8620 §5.3 for the base taxonomy. RFC 8621 §4.6 lines 2932 to
  2949 for `blobNotFound` with its mandatory `notFound` `Id[]`,
  `tooManyKeywords` and `tooManyMailboxes`. RFC 8621 §7.5 lines 4600 to 4640
  for `invalidEmail` with `properties`, `tooManyRecipients` with a mandatory
  `maxRecipients`, `invalidRecipients` with a mandatory `invalidRecipients`
  `String[]`, `noRecipients`, `forbiddenMailFrom`, `forbiddenFrom`,
  `forbiddenToSend`, and `cannotUnsend`. RFC 8621 §10.6.1 to §10.6.12 is the
  registry.
- Assert: a table with one fixture per error type; each decodes to a
  discriminated Go value carrying its extra fields. An unknown error type
  preserves its raw type string and its raw payload rather than being flattened
  to a generic error.
- Protects against: poplar telling the user "send failed" when the server said
  "these three recipients are invalid" and named them. go-jmap's `SetError`
  (`errors.go:47`) carries only `Type`, `Description`, and `Properties`, so
  every one of those extras is dropped today. Nothing in its suite would catch
  that.

---

### Tier 2: silent sync and state corruption

Wrong here and the mailbox drifts out of agreement with the server without an
error. The user sees a stale or duplicated view.

---

**JT-13. `/changes` never reports a record destroyed before a response that
reports it created or updated, across a paged sequence.**

- Source: RFC 8620 §5.2. The RFC gives a five-state `A` to `E` walkthrough of
  intermediate-state paging.
- Assert: replay the walkthrough as a synthesized multi-response fixture and
  check the invariant holds at every step of poplar's consumption.
- Protects against: a message deleted locally and then never recreated, or the
  reverse.

---

**JT-14. `hasMoreChanges` paging terminates and the state never regresses.**

- Source: RFC 8620 §5.2, line 1769.
- Assert: a three-page fixture drives the loop to completion; a malicious
  fixture that returns the same `newState` forever is detected rather than
  looping.
- Protects against: an infinite sync loop, or a client that believes it is
  caught up while `hasMoreChanges` was true.
- Existing material: `internal/backend/jmap/testdata/changes_page1.json` and
  `changes_page2.json` already encode this shape for the consumer. Lift the
  shape, not the file.

---

**JT-15. A record created and destroyed since `sinceState` is handled whether
the server omits it or lists it as destroyed.**

- Source: RFC 8620 §5.2. Omission is a SHOULD, so a compliant server may still
  include it in `destroyed`.
- Assert: both fixtures produce the same final local state.
- Protects against: a phantom deletion of a record poplar never had.

---

**JT-16. `cannotCalculateChanges` is a typed sentinel that forces a full
resync.**

- Source: RFC 8620 §5.2 line 1826, and the registry entry at line 4559.
- Assert: the error is distinguishable from every other method error by type,
  not by string matching, and the consumer contract test proves it triggers
  invalidation rather than an empty diff.
- Protects against: a mailbox that quietly stays stale forever after a long
  offline period.
- Existing material: `internal/backend/jmap/testdata/changes_cannot_calculate.json`.

---

**JT-17. `/queryChanges` splice math is exact, including `upToId` and the
mutable-property reinsertion rule.**

- Source: RFC 8620 §5.6.
- Assert: `removed` then `added` applied in the RFC's prescribed order
  reproduces the expected list; an `added` entry whose index exceeds the
  current length is an error, not a silent append.
- Protects against: an off-by-one that duplicates or drops a row in a mailbox
  listing, which looks like a rendering bug and is actually a sync bug.

---

**JT-18. State strings are opaque bytes, compared only for equality.**

- Source: RFC 8620 §2, line 704: state is "a (preferably short) string" and
  clients "should not attempt to parse it".
- Assert: a fixture carrying Fastmail's real captured value,
  `cyrus-77;j-1;p-30c616ea00;s-69951158a7dcb38d`, round-trips unchanged, and no
  code path splits, parses, or orders it.
- Protects against: a client that infers meaning from structure that happens to
  be there today. Fastmail's value visibly encodes a Cyrus generation number.
  A future Fastmail change to that format would silently break any parser.

---

**JT-19. Every capability type the library defines decodes its own JSON tags.**

- Source: RFC 8620 §2 for `urn:ietf:params:jmap:core`, RFC 8621 §1.3 for the
  mail capabilities and the EmailSubmission capability object.
- Assert: a table over every capability type poplar ships, each decoding a
  fixture built from the RFC's own field names, with every field asserted
  non-zero.
- Protects against: a mistyped struct tag that silently reads zero forever.
  This is not hypothetical. go-jmap has exactly two confirmed instances:
  `mail/mdn/mdn.go` tags `ReportingUA` as `json:"reportinUA,omitempty"`, and
  `mail/vacationresponse/vacationresponse.go:43` defines `MarshalJson` rather
  than `MarshalJSON`, so its UTC coercion is dead code that `encoding/json`
  never calls. Both live in packages with no test file. go-jmap's
  `session_test.go` proves the capability dispatch mechanism works only through
  a synthetic `testCapability`; not one of the library's real capability types
  is decoded by any test in the suite. Note also that its fixture spells
  `maxConcurrentRequest` (singular) at `session_test.go:16` while the RFC and
  the Go field both say `MaxConcurrentRequests`.

---

**JT-20. An unknown capability URI and an unknown account capability decode
without error and survive a round trip.**

- Source: RFC 8620 §2.1. The RFC's own example session includes
  `https://example.com/apis/foobar`.
- Assert: the session decodes, the unknown URI is preserved in raw form, and
  re-marshaling emits it unchanged.
- Protects against: poplar refusing to talk to a server that offers an
  extension it does not know. go-jmap's fixture contains the unknown URI but
  never asserts that it is tolerated rather than dropped.

---

**JT-21. A changed `sessionState` triggers a session refetch.**

- Source: RFC 8620 §2 and §3.4 (`sessionState` on every response).
- Assert: a fake transport returning a new `sessionState` causes exactly one
  refetch, and the refetched session replaces the cached one atomically.
- Protects against: poplar using stale account ids or a stale upload URL after
  the server reconfigures.

---

### Tier 3: loud failures with a silent subset

These usually produce an error. Each has one path where it does not.

---

**JT-22. EventSource reconnect sends `Last-Event-ID`, and the unknown-id case
has a documented, tested policy.**

- Source: RFC 8620 §7.3, lines 3960 to 3968, verbatim: "a client following the
  server-sent events specification will send a Last-Event-ID HTTP header field
  with the last id it saw, which the server can use to work out whether the
  client has missed some changes. If so, it SHOULD send these changes
  immediately on connection."
- Assert: after a dropped connection, the reconnect carries the header with the
  last seen id. A fake server that ignores the header causes poplar to fall
  back to a full `/changes` pull rather than assuming it is caught up.
- Protects against: a reconnect that silently drops every change from the
  disconnected window. The RFC defines no error for "I do not remember that
  event id", unlike the analogous `/changes` case which has
  `cannotCalculateChanges`. That means poplar must decide unilaterally, and the
  decision belongs in a test and in the package documentation.
- Uncharted territory warning: a grep of the whole `apache/james-project` repo
  for `Last-Event-ID` returns exactly one hit, in a specification document, and
  zero in server code or tests. go-jmap never reads an SSE `id:` field at all
  (`core/push/eventsource.go:89` to `:118` handles only `event` and `data`).
  poplar is writing first-party fixtures here, not borrowing.

---

**JT-23. SSE framing follows the WHATWG event-stream rules.**

- Source: the WHATWG server-sent events specification, referenced by RFC 8620
  §7.3 and cited in go-jmap's own source comment at
  `core/push/eventsource.go:91`.
- Assert, as a table: an event dispatches on the blank line, not on the data
  line; multi-line `data:` fields accumulate with newlines between them; `id:`
  is captured and retained; a line beginning with `:` is a comment; a line with
  no colon is a field with an empty value; a `retry:` field is honored or
  explicitly ignored by documented choice.
- Protects against: a state event silently lost because its payload spanned two
  lines. go-jmap dispatches on the `data` line
  (`core/push/eventsource.go:106` to `:116`), so any multi-line data breaks it
  and no test exists.

---

**JT-24. A server line larger than the read buffer does not silently truncate
the stream.**

- Source: implementation hazard, confirmed at `core/push/eventsource.go:89`,
  which builds a `bufio.Scanner` with the default 64 KB maximum token and never
  checks `scanner.Err()` before returning nil at line 118.
- Assert: a fake server sending a 128 KB `data:` line either parses it or
  returns an error. It never returns nil with the event dropped.
- Protects against: a push stream that appears healthy and delivers nothing. A
  `StateChange` covering many accounts and types can be large.

---

**JT-25. `Listen` takes a context, reconnects with backoff, and `Close` returns
its error.**

- Source: the gaps that made poplar write its own transport. go-jmap's
  `Listen()` takes no context (`core/push/eventsource.go:84`), never
  reconnects, and `Close()` discards the body-close error (`:125` to `:129`).
- Assert: canceling the context unblocks `Listen` promptly; a dropped
  connection reconnects with bounded backoff; `Close` surfaces a close error.

---

**JT-26. The `ping` interval poplar uses is the one the server reports, not one
poplar assumed.**

- Source: RFC 8620 §7.3, lines 4006 to 4022. The server MAY clamp a requested
  interval. Servers MUST NOT set a minimum above 30 or a maximum below 300.
  The ping event's data is a JSON object with an `interval` property giving the
  interval the server actually chose.
- Assert: poplar requests an interval, receives a clamped one in the ping
  payload, and adapts its liveness timeout to the received value.
- Protects against: poplar treating a healthy connection as dead, or a dead one
  as healthy, because it assumed its requested interval was honored. Note that
  the `[30, 300]` figures constrain the server's limits, not the value the
  client may use; asserting a client-side bound would be a misreading.

---

**JT-27. `closeafter=state` terminates the stream after one state event.**

- Source: RFC 8620 §7.3, lines 3995 to 4004.
- Assert: poplar's one-shot mode consumes exactly one event and returns.
- Corroboration: James `EventSourceContract.scala` has 20 tests covering
  `types`, `ping`, and `closeafter` validation. Apache-2.0, adaptable with
  attribution. Copy at `.../scratchpad/research/james-EventSourceContract.scala`.

---

**JT-28. `StateChange` fans out per account and per type.**

- Source: RFC 8620 §7.1.1 (a StateChange across two accounts), RFC 8621 §1.5.1
  (the mail TypeState example).
- Assert: a two-account fixture yields two independent per-type state maps; an
  unknown type name in the map is preserved, not dropped.

---

**JT-29. Request-level RFC 7807 problem details parse, including the
`application/problem+json` content type.**

- Source: RFC 8620 §3.6.1, lines 1080 to 1105, listing `unknownCapability`,
  `notJSON`, `notRequest`, and `limit`, with `limit` carrying a mandatory
  `limit` property naming the limit applied. §3.6.1.1 gives two complete bodies
  at lines 1108 to 1140.
- Assert, as a table: a `problem+json` content type parses; a plain
  `application/json` content type parses; a content type with a charset
  parameter parses; a non-JSON body degrades to a typed HTTP error carrying the
  status. The `limit` body's `limit` property is retained.
- Protects against: throwing away the only diagnostic the server sent.
  go-jmap's `decodeHttpError` (`client.go:300` to `:312`) requires the content
  type to equal `application/json` exactly, so a spec-conformant
  `application/problem+json` response is reduced to `HTTP 400 400 Bad Request`
  and the structured detail is discarded. A charset parameter breaks it too.
- Fixtures: `rfc8620-3.6.1.1-unknowncapability.json`,
  `rfc8620-3.6.1.1-limit.json`, transcribed verbatim.

---

**JT-30. An error detail string is never used as a format string.**

- Source: implementation hazard, confirmed at go-jmap `errors.go:56`, which
  calls `fmt.Sprintf(e.Detail)`.
- Assert: a `detail` containing a literal `%s` and a literal `%!` renders
  unchanged in the error text.
- Protects against: a corrupted error message that hides the real problem. Note
  that `go vet`'s printf analyzer catches this class, and poplar's `make check`
  runs analyzers, so this test is belt to that suspenders.

---

**JT-31. The method-level error taxonomy is typed, and unknown types survive.**

- Source: RFC 8620 §3.6.2 for the shape and the enumeration
  (`serverUnavailable`, `serverFail`, `serverPartialFail`, `unknownMethod`,
  `invalidArguments`, `invalidResultReference`, `forbidden`, `accountNotFound`,
  `accountNotSupportedByMethod`, `accountReadOnly`). The registry is §7 of the
  IANA section, with `cannotCalculateChanges` at line 4559 and
  `invalidResultReference` at line 4670.
- Assert: a response containing an `"error"` invocation decodes to a typed
  method error at the right call id; an unregistered error type is preserved
  rather than dropped.
- Gap it closes: go-jmap registers the `"error"` pseudo-method in `jmap.go`'s
  `init`, and no test in the suite ever decodes one.

---

**JT-32. Blob upload, download, and copy.**

- Source: RFC 8620 §6.1 (upload response: `accountId`, `blobId`, `type`,
  `size`), §6.2 (download URL template), §6.3 (`Blob/copy`, `notCopied`,
  `fromAccountNotFound`).
- Assert: any 2xx upload status is accepted (see DV-01); an extra field beyond
  the RFC's four is tolerated; the download URL template expands all four
  variables with correct percent-escaping of a blob id containing reserved
  characters; `Blob/copy` decodes `notCopied` as a map of typed SetErrors.
- Corroboration: James `UploadContract.scala` (6 tests) and
  `DownloadContract.scala` (26 tests), Apache-2.0, copies in the scratchpad.

---

**JT-33. `Email/set create` constraint violations decode correctly.**

- Source: RFC 8621 §4.6. Fastmail's own `JMAP-TestSuite` has 30 files under
  `t/Email/set/create/` naming these exact constraints, including
  `cannot-have-blobid-and-partid.t`, `only-one-part-in-text-body.t`, and
  `no-duplicate-header-representations.t`.
- Assert: each constraint violation produces its documented error shape. Note
  that `JMAP-TestSuite` has no LICENSE file and GitHub reports `license: null`,
  so none of its code may be copied. The file listing is an uncopyrightable
  inventory of what to test, and that is all it is being used for here.

---

**JT-34. Mailbox destroy errors.**

- Source: RFC 8621 §2.6 and lines 855 to 870, plus §10.6.1 `mailboxHasChild`
  and §10.6.2 `mailboxHasEmail`.
- Assert: both errors decode as discriminated types and are surfaced to the
  caller with enough information to explain the refusal.

---

**JT-35. Every date field marshals as UTC.**

- Source: RFC 8620 §1.4 (`UTCDate` type). The pattern appears in go-jmap at
  `mail/email/filter.go:74` and `mail/emailsubmission/emailsubmission.go:62`.
- Assert, as a table over every type carrying a date: a value constructed in a
  non-UTC location marshals with a `Z` suffix and the correct instant.
- Protects against: the exact class of bug go-jmap shipped. Its
  `VacationResponse` UTC coercion never runs because the method is misspelled
  (JT-19), and nothing else proves the working instances work either.

---

### Tier 4: the wire-format foundation

Low risk, high build priority. Everything above depends on these.

**JT-36.** Invocation marshals and unmarshals as the `[name, args, callId]`
3-tuple, dispatching args by registered method name. RFC 8620 §3.2.

**JT-37.** An unregistered method name in a response is an error naming the
method, and does not panic or silently produce nil args. Untested in go-jmap
despite the branch existing in its `UnmarshalJSON`.

**JT-38.** `Request.Invoke` assigns sequential call ids across multiple calls,
and a back-reference to call *n* resolves. go-jmap generates `fmt.Sprintf("%x",
len(r.Calls))` and never tests the sequence past one call.

**JT-39.** Required capability URIs merge into `using`, deduplicated, and the
core URI is always present. RFC 8620 §3.3.

**JT-40.** Request and Response object shapes: `using`, `methodCalls`,
`createdIds` omitted when empty; `methodResponses`, `sessionState`,
`createdIds` on the way back. RFC 8620 §3.3, §3.4.

**JT-41.** ID validity. RFC 8620 §1.2 defines an Id as 1 to 255 characters from
the base64url alphabet. Assert the exact boundaries (1 valid, 255 valid, 0
invalid, 256 invalid) and make an explicit, documented decision about the
character class. go-jmap declares `idRegexp` at `jmap.go:21` and never
references it: commit `f6efa21` ("id: remove check for valid id") disabled the
check because it rejected result-reference ids like `"#0"`, and left the regex
in place implying a validation that does not happen. poplar should either
validate ids and exempt the `#` sharp form structurally, or not validate and say
so. Either way, test it and delete any dead validator.

**JT-42.** `SortComparator.isAscending` defaults to true when omitted. RFC 8620
§5.5. A Go `bool` field without `omitempty` always serializes, so a caller who
does not set it silently sends `false` and gets descending order. go-jmap has
this behavior and its `mail/mailbox/sort_test.go` asserts `"isAscending":false`
for a zero value, documenting the footgun without flagging it. poplar must
choose deliberately: either a pointer or a defaulting constructor. Test the
choice.

**JT-43.** Compound `FilterOperator` nesting: `AND`, `OR`, `NOT`, nested two
deep, and an empty condition. RFC 8621 §4.4.1. go-jmap's entire `mail/email`
test coverage is one assertion that an empty `FilterCondition` marshals to
`"{}"`.

**JT-44.** Anchor-based paging: `anchor`, `anchorOffset`, and the
`anchorNotFound` error. RFC 8620 §5.5. See DV-06.

**JT-45.** `collapseThreads` on `Email/query`. RFC 8621 §4.4. See DV-07.

**JT-46.** Thread grouping expectations are treated as server-side and not
reimplemented. The governing text is RFC 8621 §3 and, specifically, the
unnumbered prose before §3.1, which describes the message-id plus
normalized-subject heuristic as a SHOULD rather than a mandated algorithm.
Assert that poplar consumes the server's grouping and never recomputes it.

Two corrections, both verified during task 5 (2026-07-29):

- An earlier draft of this row, and the charter, cited RFC 8621 §1.3.2 for
  Thread semantics. That is wrong. §1.3.2 is the EmailSubmission capability
  object (`maxDelayedSend`, `submissionExtensions`). Thread is §3, and the
  heuristic is the unnumbered prose before §3.1, where it is a SHOULD.
- The row's original wording asked for an assertion that poplar "consumes
  `Thread/get` output". `Thread/get` is trimmed out of `poplar/jmap`, so no
  test can make that assertion. The assertion in force is the one the
  conformance suite makes: the `threadId` poplar reports is byte for byte the
  one the server sent.

---

## 2. go-jmap reuse: verbatim, rewrite, discard

**The attribution requirement, stated once and concretely.**

go-jmap is MIT with two copyright holders: Max Mazurov (2019) and Tim
Culverhouse (2022), per its `LICENSE`. MIT requires the copyright notice and
permission notice to be included in all copies or substantial portions. Because
poplar's data model was derived by reading that source, and because several
expected-JSON literals below are lifted verbatim, treat the whole package as
MIT-derived and satisfy the licence once, at the repository level:

1. Ship `THIRD-PARTY-NOTICES.md` in the `jmap` repository root, containing
   go-jmap's full LICENSE text and a sentence naming what was derived: the data
   model shape and several test fixtures.
2. Add one line to the package doc comment in `jmap/doc.go` pointing at that
   file.
3. Note the derivation in the commit that first lands the ported code.

No per-file header is required and none should be added. Three artifacts, once.

**Carry the idea close to verbatim (three items, attributed above).**

| go-jmap file | What carries | Why |
|---|---|---|
| `core/echo_test.go` | The "one test per concrete method type, driven through `Request.Invoke`" template | It is the only test in the suite that runs a real `Method` with a real `Name()` and `Requires()` end to end. go-jmap defines roughly 30 method types and tests exactly this one. Generalize the template to all of them. |
| `mail/mailbox/changes_test.go`, `get_test.go` | The direct-versus-manual construction agreement pattern, where a `t.Run("manual", ...)` subtest builds the same `Invocation` by hand and asserts identical bytes | It pins the convenience API to the wire format, which is exactly the invariant a library other projects depend on must not drift on. |
| `mail/mailbox/set_test.go` | The explicit-null patch case | It is the only proof anywhere that a `null` in a patch survives Go marshaling rather than being dropped by `omitempty`. Feed it into JT-02. |

**Rewrite, do not patch (seven files).**

`invocation_test.go`, `request_test.go`, `response_test.go`, `session_test.go`,
`jmap_test.go`, `mail/mailbox/query_test.go`, `mail/mailbox/sort_test.go`.

Each of these identifies something real that needs proving: the tuple shape,
the envelope shape, the capability dispatch mechanism, ID bounds, sort
defaults. Each also has a gap that patching would preserve. `jmap_test.go`
tests two ID cases and neither boundary nor the character class.
`invocation_test.go` never exercises the unregistered-method branch, and
carries a debug leftover at line 52 (`t.Logf("TIMBUG %T", inv.Args)`, introduced
in commit `ea777b6` and never removed). `request_test.go` never invokes twice.
`response_test.go` never decodes an error invocation. `session_test.go` proves
the capability mechanism only through a synthetic type. Treat all seven as a
checklist and author fresh against JT-36 through JT-43.

**Discard (two files).**

`example_test.go` has two `Example()` functions with no `// Output:` comment, so
`go test` compiles them and never runs them. Write real executable examples
instead, with output comments, so the documentation is verified.
`mail/email/filter_test.go` is a single assertion that an empty struct marshals
to `"{}"`. It is not a foundation.

**No material exists for ten of fourteen packages.** `mail`, `mail/thread`,
`mail/identity`, `mail/mdn`, `mail/searchsnippet`, `mail/vacationresponse`,
`mail/emailsubmission`, `core/blob`, `core/push`, and `core/push/subscription`
have no test file at all. Those are written from the RFCs. Apply extra scrutiny
to `mail/vacationresponse` and `mail/mdn` when porting their data models,
because both contain confirmed defects that no test could catch (JT-19).

**Transport is not ported at all.** `client.go` and `core/push/eventsource.go`
are the reason poplar is writing its own. Read them for the failure catalogue
in JT-22 through JT-30, and write nothing across.

---

## 3. Second-server validation setup

### The server: Stalwart

Stalwart is the pick, for one reason above the others. It shares no lineage
with Fastmail. Fastmail's live `sessionState` value,
`cyrus-77;j-1;p-30c616ea00;s-69951158a7dcb38d`, shows its backend is
Cyrus-derived, so testing against upstream Cyrus risks agreeing with Fastmail
because they are cousins. Stalwart is an independent Rust implementation, so
agreement between poplar-against-Fastmail and poplar-against-Stalwart is real
evidence rather than a shared assumption.

Stalwart also has the lowest setup cost of the candidates and the widest RFC
coverage, including PushSubscription webhooks and JMAP WebSocket (RFC 8887).
Current release is v0.16.15, published 2026-07-27.

The image is `docker.io/stalwartlabs/stalwart`. The older
`stalwartlabs/mail-server` name stops at v0.11.8 and must not be used: it
predates the v0.16 rewrite this document's setup now assumes. Any surviving
reference to it anywhere is stale.

Runner-up is Cyrus IMAP, and section 5 explains why it is still worth adding
later.

Apache James is a viable third. It is Apache-2.0 and its contract suite is the
best mail-JMAP reference material in existence, but running it costs a JVM and,
for the distributed profile, Cassandra plus OpenSearch. Read its tests; do not
run its server for this purpose.

mox is ruled out. JMAP appears only under "Roadmap" on `xmox.nl/features`.

### How it runs locally

The paragraph below was rewritten during task 5 (2026-07-29) against the real
v0.16.15 image. The TOML configuration, the `/api/account` endpoint, and
`stalwart-cli` that this section originally carried are all v0.11 shapes and
none of them survives into v0.16. `make conformance` is the executable version
of what follows.

- **The configuration file is JSON, not TOML**, and it holds only the data
  store. `crates/store/src/registry/local.rs` reads it with `serde_json` and
  decodes it into one `DataStore` value, so an extension of `.toml` changes
  nothing. Everything else lives in a registry inside that store.
- **The instance boots into a setup mode** that serves nothing but its own
  `Bootstrap` object, and leaves that mode only when it restarts against a
  configuration file it wrote itself. The suite's configuration is therefore
  the Bootstrap object, at `jmap/testdata/conformance/stalwart.json`.
- **Management moved onto JMAP** under the `urn:stalwart:jmap` capability, as
  `x:Object/method` calls with capitalised object names: `x:Bootstrap/set`,
  `x:Domain/query`, `x:Account/set`. There is no principal REST API.
- **The suite needs a real account.** `STALWART_RECOVERY_ADMIN` pins an
  administrator that authenticates and holds a JMAP session, but its account id
  is synthetic and `Email/query` against it fails inside the server. Create a
  user through `x:Account/set` and run as that.
- **`STALWART_PUBLIC_URL` is required.** Without it the session advertises
  `https://<container hostname>/jmap/`, which nothing outside the container can
  reach.

```bash
podman run -d --name poplar-jmap-conformance -p 19080:8080 \
  -e STALWART_RECOVERY_ADMIN=admin:conformance \
  -e STALWART_PUBLIC_URL=http://localhost:19080 \
  docker.io/stalwartlabs/stalwart:v0.16.15

go run ./scripts/conformance -step setup   -url http://localhost:19080
podman restart poplar-jmap-conformance
go run ./scripts/conformance -step account -url http://localhost:19080
```

Session endpoint: `http://localhost:19080/.well-known/jmap`. The host port is
high because rootless podman cannot bind below 1024.

The seed corpus should be license-clean. poplar already plans one as a Phase 5
pass-3 artifact per ADR-0014. Reuse it rather than minting a second.

### How poplar's suite points at it

Add a build tag `conformance` and a single env-driven target selector, so the
same test bodies run against Stalwart, Fastmail, or any future server:

```
POPLAR_JMAP_SESSION_URL=http://localhost:19080/.well-known/jmap
POPLAR_JMAP_USER=user1@conformance.test
POPLAR_JMAP_PASSWORD=poplar-conformance-9f2c
POPLAR_JMAP_SERVER=stalwart     # names the expected-divergence profile
```

The `live` suite keeps its existing shape. It points at
`https://api.fastmail.com/jmap/session` and skips when the token is missing, as
`internal/backend/jmap/live_test.go` already does at lines 19 and 28 to 31.

Add one Makefile target. It starts the container, waits for the session
endpoint, runs `go test -tags conformance ./jmap/...`, and tears down. It is not
part of `make check`, because `make check` must not require Docker.

The divergence profile named by `POPLAR_JMAP_SERVER` selects which of the DV
tests assert "this server does X" versus "poplar tolerates X or Y". A test that
needs the profile to decide what is correct is a test that has found a real
divergence, and it belongs in section 4.

### What Stalwart can exercise

Everything in Tier 1, Tier 2, and most of Tier 3 against real server behavior:
the full `/set` partial-failure surface, patch semantics, back-references,
creation ids, `/changes` and `/queryChanges` paging under real state
transitions, EmailSubmission with `onSuccessUpdateEmail`, blob upload and
download, EventSource including `closeafter` and `ping`, and PushSubscription.

### What Stalwart cannot exercise

- Fastmail's own quirks, by construction. Only the live suite catches those.
- `Last-Event-ID` resumption behavior in the wild. Stalwart may implement it;
  James demonstrably does not. Test poplar's fallback (JT-22) against a
  purpose-built fake that ignores the header, because neither real server is a
  reliable negative control.
- Real rate limiting, `maxConcurrentRequests` enforcement, and
  `urn:ietf:params:jmap:error:limit` under load. A local single-user instance
  never hits them.
- Blob expiry. RFC 8620 §6.1 guarantees only a one-hour minimum retention. A
  local instance under test never ages a blob out.
- Localized error descriptions. RFC 8621 §7.5.1 shows a German
  `forbiddenToSend` description driven by `Accept-Language`. Whether a given
  server localizes at all is server-specific.

### The Fastmail-only parts

Three things only the live suite validates: Fastmail's actual blob upload
status code and response fields, Fastmail's `sessionState` format assumptions
(JT-18), and Fastmail's EventSource behavior including whether it honors
`Last-Event-ID`. Keep the live suite small, tagged, manual, and pointed at
exactly those. ADR-0014 already scopes it that way.

---

## 4. The divergence tests

Each pins poplar's behavior so a future server cannot silently break it. Every
one is a normal unit test against a fake transport, plus an assertion in the
conformance suite where a real server can confirm it.

**DV-01. Blob upload status code, 200 versus 201.**

RFC 8620 §6.1 never specifies a success status code. It mandates only the JSON
body. Cyrus returns 201: `imap/http_jmap.c`, function `jmap_upload`, line 1232,
`ret = HTTP_CREATED;`, and it adds a fifth `expires` field beyond the RFC's
four. Stalwart returns 200: `crates/http-proto/src/lib.rs`, `JsonResponse::new`
defaults to `StatusCode::OK`, and `crates/jmap/src/api/mod.rs` returns
`UploadResponse` through it with no override. Fastmail returns 200.

Measured during task 5 (2026-07-29): Stalwart v0.16.15 answers 200 with exactly
the four properties. Fastmail answers 200 and sends the fifth `expires`
property too, which this section attributed to Cyrus alone.

Test: a table over `{200, 201, 202, 200-with-extra-fields}`; every 2xx with a
valid body succeeds; the extra `expires` field is preserved or ignored without
error. This is go-jmap's live bug: `client.go:232` rejects anything that is not
exactly 200, so uploading to Cyrus fails.

**DV-02. Sort order of an optional boolean, `hasKeyword`.**

Genuinely unspecified. From `stalwartlabs/stalwart` Discussion #2772 ("Meta:
JMAP Conformance tests", opened by `josephg` on 2026-02-06, pulled via
`gh api graphql` to `.../scratchpad/research/discussion_2772.json`): sorting
`Email/query` by `hasKeyword: "$flagged"` produced different orders on Fastmail
and Stalwart, because there is no place in the specification where the order of
boolean, let alone optional boolean, values is fixed. Stalwart adopted
Fastmail's ordering.

Test: poplar performs no client-side reordering of a server-returned query
result. Assert that the id order poplar exposes equals the server's order
exactly, for both orderings. A conformant server is free to pick either.

**DV-03. `filter: null`.**

Stalwart rejected `filter: null` in queries; RFC 8620 does not forbid it. Fixed
in v0.16.10 per `mdecimus`'s 2026-06-19 comment in the same discussion.

Test: poplar omits `filter` entirely when there is no filter, and never emits
`filter: null`. Also assert poplar accepts a server that rejects the null form,
by never producing it. This is a "do not depend on the ambiguity" pin.

**DV-04. Missing `notFound` arrays on `/get`.**

Stalwart omitted `notFound` from `/get` responses where the RFC calls for the
unfound ids to be echoed back. Fixed in v0.16.10.

Test: poplar computes its own missing set as requested ids minus returned ids,
and treats an absent `notFound` as "the server did not tell me" rather than as
"nothing was missing". Fixture with `notFound` absent, and a second with it
present and disagreeing with the computed set.

Protects against: poplar believing a message exists because the server neither
returned it nor said it was missing.

**DV-05. `sessionState` carries internal structure.**

Covered by JT-18. Listed here because it is a divergence in fact even though
the specification is clear: Fastmail's value is structured, Stalwart's is not,
and the RFC says clients should not parse either.

**DV-06. Error-type substitution.**

From the live conformance report (`seph.au/jmap-report.html`, generated
2026-02-18, snapshot at `.../scratchpad/research/jmap-report.html`, parsed
directly from its raw table): Stalwart returned `invalidArguments` where the
RFC calls for `accountNotFound` (`core/error-account-not-found`), and returned
`invalidArguments` where the RFC calls for `anchorNotFound`
(`email/paging-anchor-not-found`). Also `core/error-empty-using`,
`core/error-not-request`, and `core/error-wrong-content-type` diverged.

Test: poplar never treats an unrecognized method error as success, and never
branches on error type for a correctness-critical decision where servers are
known to substitute. The one place branching is required is
`cannotCalculateChanges` (JT-16), so test that one explicitly against both
servers and document that poplar depends on it.

**DV-07. `collapseThreads` and filter conditions.**

The same report shows `email/collapse-threads-basic`,
`email/collapse-threads-calculate-total`, `email/filter-has-attachment-true`,
`email/filter-has-attachment-false`, `email/filter-header-name-only`, and
`email/filter-header-value` failing against Stalwart at that snapshot.

Test: poplar sends the RFC 8621 §4.4 and §4.4.1 forms correctly and does not
compensate client-side for a server that ignores them. A wrong result set here
is the server's bug, and poplar should surface it rather than paper over it.
Re-check these against v0.16.15 when the conformance suite first runs; several
may already be fixed.

**DV-08. PushSubscription absent.**

Cyrus's own RFC-support page lists RFC 8620 as complete except the
PushSubscription object, which is "not yet supported". EventSource still works
there.

Test: poplar detects the capability's absence from the session and falls back to
EventSource, without an error and without assuming push exists. Never sniff;
read the session.

**DV-09. EventSource resumption absent.**

Covered by JT-22. Listed here because "the server ignores `Last-Event-ID`" is a
divergence poplar will meet in production and has no error code for.

**DV-10. Problem-details content type.**

Covered by JT-29. A server sending `application/problem+json`, which RFC 7807
specifies, must not cause poplar to discard the error body.

**DV-11. Duplicate mailbox name under one parent.**

`mailbox/set-duplicate-name-same-parent` failed against both Fastmail and
Stalwart in the report, which suggests either the test is wrong or the
`alreadyExists` versus `invalidProperties` boundary is genuinely underspecified.

Test: poplar accepts either error type for this case and presents the same
message to the user. Do not pin one.

**DV-12. Push subscription tests failing against Fastmail.**

The report shows `push-eventsource/eventsource-receives-state-change` and all
`push-subscription/*` failing against real Fastmail while passing against
Stalwart. The report's own author calls some of these "spurious", for example
localhost push registration being disallowed by policy. Treat this as needing a
live re-check before writing an assertion, and do not encode a Fastmail defect
that may not exist.

**Framing, from the standards body.** IETF draft
`draft-ietf-jmap-portability-extensions-01` states that a JMAP server "might
only have partially implemented the JMAP standard or design decisions might
have been taken that let the server deviate from what is actually required by
RFC8620", and that JMAP offers no standardized way to identify the server
product. The divergence problem is acknowledged and unsolved at the protocol
level. Testing against a second independent server is the mitigation, not a
nicety.

---

## 5. Honest gaps

**What this plan does not cover.**

- *Fastmail-fork quirks that upstream Cyrus does not share.* Stalwart is an
  independent implementation, which is why it was chosen, and that is exactly
  why it cannot tell poplar which of Fastmail's behaviors are Cyrus behaviors
  and which are Fastmail patches. Adding the Cyrus Docker image
  (`ghcr.io/cyrusimap/cyrus-docker-test-server`, users `user1` to `user5`,
  seedable with `curl -T testserver/examples/userdata.json
  http://localhost:8001/api/eve`) as a third target costs almost nothing and
  answers that specific question. The blob-upload 200 versus 201 split is the
  shape of bug it catches. Do this after the Stalwart suite is green.
- *Thread grouping correctness.* RFC 8621 §3 makes the grouping heuristic a
  SHOULD. Two conformant servers may thread the same mailbox differently, and
  no test can call either wrong. JT-46 pins only that poplar does not
  reimplement it.
- *Search relevance and `SearchSnippet/get` quality.* Untestable as
  correctness. Only shape can be asserted.
- *Performance at scale.* A local instance with a seed corpus never exercises a
  100,000-message mailbox, `maxObjectsInGet` clamping under real limits, or
  paging behavior when the state changes mid-page. ADR-0014's QA-1/2/3 perf
  harnesses cover poplar's side; the server side is production-only.
- *Real concurrency limits.* `maxConcurrentRequests` and the
  `urn:ietf:params:jmap:error:limit` problem type only fire under real load
  against a real service.
- *Blob lifetime.* RFC 8620 §6.1 guarantees a one-hour minimum. A blob id
  cached past its real server lifetime surfaces only on the next send attempt,
  in production, weeks later.
- *Authentication.* RFC 8620 §8 leaves the scheme to the server. Bearer tokens
  against Fastmail and basic auth against Stalwart cover two shapes. OAuth
  refresh, token expiry mid-stream, and re-auth on a 401 during a long-lived
  EventSource connection are production-only until a server that does them is
  in the loop.
- *Localization.* RFC 8621 §7.5.1's `Accept-Language`-driven error descriptions
  are visible only against a server that localizes.

**What would only be caught in production.**

A server that changes behavior after a deploy. A `sessionState` format change.
A push stream that degrades rather than disconnects. Rate limiting that arrives
as a 429 with a `Retry-After` rather than a JMAP `limit` error. Each of these
argues for the same thing: every user-visible error reaching the log through
one seam, which is already a standing poplar rule.

**One thing this plan deliberately does not do.** It does not vendor
`jmapio/jmap-test-suite`, the live conformance suite at
`github.com/jmapio/jmap-test-suite` that generated the report cited throughout
section 4. It has no LICENSE file. Its roughly 300 test cases are facts about
the protocol and are fair to be inspired by, and several are cited above by
name for exactly that reason. Its code may not be copied. The same applies to
`fastmail/JMAP-TestSuite`, which also reports `license: null`. Apache James
(Apache-2.0) and Cyrus (BSD-style CMU licence) are the two corpora whose
assertion logic may be adapted, with attribution.
