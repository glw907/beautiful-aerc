# RFC obligations map for `poplar/jmap`

Date: 2026-07-29. Subject: package `jmap` at `/home/glw907/Projects/poplar/jmap/`,
as committed. Sources fetched from rfc-editor.org and html.spec.whatwg.org during
this pass, not recalled.

The package was read at commit `ef0c986` plus the working tree as it stood
mid-morning on 2026-07-29, then re-read once the blob work landing in the same
tree (`jmap/blob.go`, `jmap/blob_test.go`, `jmap/conformance_test.go`, and edits
to `jmap/method.go`) settled, to close out the three rows that depended on it.

Companion to `docs/poplar/research/2026-07-28-jmap-test-inventory.md`. That
document asks "what would lose data if it were wrong". This one asks "what does
the specification require of a client". Where a JT or DV item already covers a
row, the status column cites the test that proves it and the item id.

---

## Summary

**281 rows.** By strength: 213 MUST (counting MUST NOT, REQUIRED, and SHALL),
33 SHOULD (counting SHOULD NOT and RECOMMENDED), 20 MAY, and 15 rows carrying no
RFC 2119 keyword. Those last 15 are the scope markers for unmodelled sections
plus four implementation obligations that matter here but are nobody's MUST.

By status: **115 rows are GAP**, 120 are proven by a named test, 30 are N/A, and
16 are partial. A partial row names a test that proves one half of an obligation
and states what the other half still owes; three of those name an outright GAP
inside, so 118 rows carry an unproven obligation of some size.

The count came out above the 150 to 200 estimate, and the excess is almost all
MUSTs. Two scope filters were applied. Only client-binding obligations appear:
where a section imposes a duty on the server, the row records the client's duty
to tolerate the result, and where there is no such duty the row is N/A with the
reason. Only sections the package implements appear: `Email/copy`,
`Email/parse`, `SearchSnippet`, `VacationResponse`, `PushSubscription`, and SRV
autodiscovery are each recorded once as N/A and not enumerated further.

The GAP count is high because the package is a wire library. Roughly half the
gaps are one class: a client MUST that constrains what may go into a `create`,
a `filter`, or a patch, which this package models the wire shape of without
policing the content. That is a defensible boundary. It is not a documented one:
`doc.go` names two closed seams and says nothing about validation, so a
downstream reading the package doc cannot tell which of the 115 gaps it inherits.
Deciding that boundary explicitly, and writing it into `doc.go`, would convert a
large block of these rows from "unproven" to "out of scope by declaration".

### The five gaps I judge most consequential

**1. One Go type per record serves both the fetched record and the create
payload, so every server-set property is expressible on a create (RFC 8620
sections 1.1 and 5.3, RFC 8621 section 7, 6 rows).** `MailboxSet.Create` takes a `*Mailbox`,
`EmailSet.Create` takes an `*Email`, `EmailSubmissionSet.Create` takes an
`*EmailSubmission`. RFC 8620 section 1.1 says the client MUST NOT send a
server-set property on a create. `omitempty` saves the zero-valued case and not
the fetch-modify-recreate case, which is what a resend or a draft-from-template
flow does. `EmailSubmission.SendAt` is the sharpest instance: RFC 8621 section 7
marks `sendAt` immutable and server-set, and the type lets a caller set it on a
create anyway, since nothing below the wire enforces the rule. The doc comment
on the field was checked against this and corrected rather than the type: it now
names FUTURERELEASE (RFC 4865), asked for through an envelope address's
parameters and bounded by `maxDelayedSend`, as the RFC's actual delayed-send
lever, rather than presenting `SendAt` itself as one. See the survey after the
RFC 8621 section 4.6 rows below for the other properties in the same shape.

**2. The S/MIME capability is correctly named in `using`, but the values it
unlocks have no semantics once they arrive (RFC 9219 section 4.1, 3 rows).**
`EmailGet.Requires()` and `EmailQuery.Requires()` both call `withSMIME`, which
adds `urn:ietf:params:jmap:smimeverify` to `using` whenever a caller asks for one
of the four S/MIME properties or sets one of the three S/MIME filter conditions,
proven down to the wire bytes by `TestRequestUsing` (JT-39). What that naming
actually buys, or costs, against a real server was checked live against Stalwart
too; see the interop note after the RFC 9219 table. What is missing is on
the read side: `SMIMEStatus` is a bare string with no constants, so a caller that
renders it verbatim shows an unrecognised value instead of downgrading it to
`unknown`, as section 4.1 requires, and shows a bare `signed` as though it were
verified, which section 4.1 also forbids. Both are security misreports, not
cosmetic ones, in a package whose capability-naming mechanism is otherwise solid.

**3. `Thread/get` cannot be decoded at all (RFC 8621 section 3, RFC 8620 section
3.7, 2 rows).** `methodResponses` in `method.go` has no `Thread/get` entry and
the package defines no `Thread` type. `Invocation.UnmarshalJSON` fails the whole
`Response` on an unregistered method name, by design. So a response containing a
`Thread/get` result fails every other call in the same request with it. The
resolver's own wildcard test replays RFC 8620 section 3.7's four-hop
`Email/query` to `Email/get` to `Thread/get` to `Email/get` chain from a
hand-decoded fixture that bypasses the registry, which means the package's
flagship back-reference proof exercises a chain the package cannot run against a
real server. JT-46's requirement that poplar consume `Thread/get` output rather
than recompute grouping is not satisfiable today.

**4. An unrecognised method error is required to be treated as `serverFail`, and
the package pins the opposite (RFC 8620 section 3.6.2, 1 row).** Section 3.6.2:
"Should a client receive an error type it does not understand, it MUST treat it
the same as the 'serverFail' type." `TestResponseDecodesMethodError` asserts that
an unregistered type does not match `ErrServerFail`. Preserving the type is the
right call, but the library offers no seam that satisfies the MUST, so a caller
who writes the obvious `errors.Is(err, ErrServerFail)` gets `false` on exactly
the errors the RFC wrote that sentence for. DV-06 makes this concrete: Stalwart
substitutes `invalidArguments` where the RFC calls for `accountNotFound` and
`anchorNotFound`, so unrecognised and mis-typed errors are the live case, not the
theoretical one. `errors.go` does not document the divergence.

**5. The `Email/set` create constraint cluster is entirely unenforced (RFC 8621
section 4.6, 11 rows).** Eleven MUSTs and MUST NOTs govern what a client may put
in an `Email` create: no `headers` property, no two properties for one header
field, no `Content-` header on the Email object, no `textBody`/`htmlBody`/
`attachments` beside a `bodyStructure`, exactly one part in each of `textBody`
and `htmlBody`, `partId` or `blobId` but never both, no `charset` or `size`
beside a `partId`, no `Content-Transfer-Encoding`, and `isTruncated` and
`isEncodingProblem` false or omitted. The package enforces none of them and no
test covers any. Section 4.6 then says a server MAY modify the Email to comply
rather than rejecting it, so the failure mode is not a loud `invalidProperties`
but a message sent with a body the client did not compose. Every compose feature
in pass 3 lands on this surface.

### How to read the status column

- A test name means that test asserts the required behaviour, not merely that it
  exercises the code path.
- `GAP` means no test proves it. The note says what proving it would take, and
  where the behaviour is absent rather than untested, it says so.
- `N/A` means the obligation does not bind this package, with the reason.
- Several rows are marked `partial`. Those name a test that proves one half and a
  gap for the other, most often where the library decodes a value correctly and
  the decision the RFC requires belongs to a consumer.

---

## RFC 8620, JMAP core

### Section 1, data types and model

| § | Obligation | Strength | Status |
|---|---|---|---|
| 1.1 | Client does not send a server-set property when creating an object | MUST | GAP. `Mailbox`, `Email`, and `EmailSubmission` each serve as both the fetched record and the create payload, so `id`, `blobId`, `threadId`, `size`, `receivedAt`, `preview`, `hasAttachment`, `myRights`, the four mailbox counts, `sendAt`, `undoStatus`, and `deliveryStatus` all marshal into a `create`. Proving it needs either a create-only type per record or a marshal-time rejection of the server-set set. |
| 1.1 | Client does not patch an immutable property | MUST (derived) | GAP. `Patch` is `map[string]any`, so `Patch{"threadId": x}` marshals. `checkSegment` knows nothing of the record schema. Proving it needs a per-record immutable-property table consulted at marshal time. |
| 1.2 | An Id is 1 to 255 octets from the URL-and-filename-safe base64 alphabet without the pad | MUST | `TestIDValid` (JT-41), all eleven boundary and character-class rows. |
| 1.2 | A creation-id reference carrying a leading `#` still reaches the wire | MUST (section 5.3) | `TestIDMarshalAcceptsCreationReference` (JT-41). |
| 1.2 | An id the client mints is checked against the grammar before it is sent | MUST | GAP, deliberate. `jmap.go` documents that no method calls `Valid` and why. Nothing warns a caller who mints a creation id with a dot in it. Proving it needs either a validating constructor or a marshal-time check that exempts the `#` form. |
| 1.3 | An UnsignedInt the client sends is in 0 to 2^53-1 | MUST | GAP. `Limit`, `MaxChanges`, `MaxBodyValueBytes`, `Size`, `SortOrder`, and `AddedItem.Index` are `uint64`, which reaches 2^64-1 and leaves I-JSON range with no error. Proving it needs a bound check at marshal time or a narrower type. |
| 1.3 | An Int the client sends is in -(2^53-1) to 2^53-1 | MUST | GAP. `Position` and `AnchorOffset` are `int64`. Same shape as the row above. |
| 1.4 | A Date omits the fractional second when it is zero and uppercases its letters | MUST | `TestDateMarshalsUTC` (JT-35), including RFC 8620 section 1.4's own worked example. |
| 1.4 | A UTCDate carries a `Z` offset | MUST | `TestDateMarshalsUTC`, `TestDateFieldsMarshalUTC` across all seven date-carrying fields (JT-35). |
| 1.5 | Everything the client sends is valid I-JSON in UTF-8 | MUST | GAP, structurally satisfied. Go maps cannot emit duplicate member names and `encoding/json` emits UTF-8. The number-range half is the two section 1.3 rows above. No test asserts any of it. |
| 1.6.2 | Client tolerates an account id disappearing and being reissued after a server-side reallocation | MUST (derived) | partial. `TestFetchSessionInstallsAtomically` proves a refetch replaces the whole account map. Acting on a vanished account id belongs to the store, and no test covers it. |
| 1.6.3 | Client never keys a cache by record id alone across types or accounts | MAY (derived from the id-clash allowance) | N/A. The package holds no cache. |
| 1.7 | Every HTTP request uses the `https://` scheme | MUST | GAP. `NewClient` accepts any scheme and `expandTemplate` uses whatever the session gives. The fake-transport suite runs entirely over `http://`, so a scheme check would need a test-only exemption. Neither the check nor the decision to omit it exists. |
| 1.7 | Every HTTP request is authenticated | MUST | N/A. Delegated to the caller's `http.Client`, documented on `Client`. No credential ever enters the package. |
| 1.8 | Client opts in to a capability by naming its URI in `using` | MUST | `TestRequestUsing` (JT-39), `TestMethodInvoke` for all fifteen method cases, `TestRelianceForcedCoreCapability`. Proven for all four modelled capabilities, including `smimeverify`'s conditional naming; see the RFC 9219 section 4.1 and 4.2 rows below. |
| 1.8 | A vendor capability the client does not know costs it nothing | MUST (tolerance) | `TestSessionUnknownCapabilitySurvives` (JT-20), including the round trip. |

### Section 2, the session resource

| § | Obligation | Strength | Status |
|---|---|---|---|
| 2 | Client ignores Session properties it does not understand | MUST | GAP. `encoding/json` ignores them on decode, but a re-marshal drops them, and `TestSessionUnknownCapabilitySurvives` compares the whole fixture against a re-marshal. The fixture carries no unmodelled top-level property, so the round-trip assertion never tests this. Adding one property to the fixture proves or breaks it in one line. |
| 2 | Client ignores Account properties it does not understand | MUST | GAP, same shape and same fixture. |
| 2 | Client reads the eight `urn:ietf:params:jmap:core` limits | MUST (server duty to supply) | `TestSessionCapabilitiesDecode` (JT-19), `TestRelianceCoreLimits`, both asserting every field non-zero. |
| 2 | Client reads a `null` account limit as no limit rather than as zero | MUST | `TestSessionNullLimitIsNotZero` (JT-19), both the null and the 1 case. |
| 2 | Client works within `maxSizeRequest`, `maxCallsInRequest`, `maxObjectsInGet`, `maxObjectsInSet` | MUST (derived, section 3.6.1 limit) | GAP. The limits decode and nothing consults them. `Do` will post a 200-call request against a `maxCallsInRequest` of 32. Proving it needs either a check in `Do` or a documented handoff to the caller. |
| 2 | Client substitutes `accountId`, `blobId`, `type`, and `name` into the download template | MUST | `TestDownloadExpandsTheURLTemplate` (JT-32), including percent-encoding of a slash, a hash, a space, and a non-ASCII character. |
| 2 | Client substitutes `accountId` into the upload template | MUST | GAP. `Upload` calls `expandTemplate` and no test asserts the expanded path. `TestUploadSendsTheCallersContentType` reads the header and the body only, and the three other upload tests register a prefix handler. A one-line path assertion closes it. |
| 2 | Client substitutes `types`, `closeafter`, and `ping` into the event-source template | MUST | `TestListenExpandsTheURLTemplate`, three rows. |
| 2 | Client treats the session `state` string as opaque | RECOMMENDED (derived) | `TestSessionStateIsOpaque` (JT-18) with Fastmail's captured value. Note that RFC 8620 has no explicit MUST NOT parse; JT-18's citation of one is loose. The opacity is poplar's design choice and the right one, since Fastmail's value visibly encodes a Cyrus generation number. |
| 2 | Client refetches the session when the state changes | RECOMMENDED | GAP, delegated. `FetchSession` is the only writer and `TestFetchSessionInstallsAtomically` proves the install is whole. Nothing decides when to call it, and `client.go` documents that as the caller's. JT-21's trigger half is unproven anywhere. |
| 2 | Client does not let an HTTP layer cache the session resource | RECOMMENDED | GAP. `FetchSession` sets no `Cache-Control` on the request. Go's default transport caches nothing, so the common case holds by accident. Proving it needs the request header and a test. |
| 2.2 | Client may autodiscover from the domain part of an email-address username | MAY | N/A. `NewClient` takes a session URL. No discovery is implemented. |

### Section 3, the structure of an API request

| § | Obligation | Strength | Status |
|---|---|---|---|
| 3.1 | The request body is `application/json` | MUST | GAP. `Do` sets the header and no test asserts it. `echoRequest` captures the body only. |
| 3.1 | Client decodes an `application/json` response | MUST (tolerance) | `TestDoStreamsTheResponse`, `TestRelianceThreeErrorShapes`. The package decodes regardless of the response content type, which is the tolerant reading. |
| 3.2 | An Invocation is the three-element array `[name, arguments, callId]` | MUST | `TestMethodInvoke` for every method type, `TestInvocationUnmarshal`, `TestInvocationUnmarshalRejectsWrongArity` with four rows (JT-36). |
| 3.2 | The arguments element is a JSON object | MUST | `TestMethodInvoke`, byte-for-byte, for all fifteen cases. `checkReferenceCollision` passes a non-object silently, which is correct: the server reports it. |
| 3.2 | Client tolerates one method call answering with more than one response under one call id | MUST (tolerance) | `TestResponseKeepsEveryInvocation` (JT-11), `TestResponseImplicitEmailSetMatchesByCreationID` (JT-10). |
| 3.2 | Client matches responses to calls by call id, not by array position | MUST (derived) | `TestResponseOutOfOrderResolvesByCallID`. |
| 3.3 | `using` and `methodCalls` are present on every request, as arrays | MUST | `TestRequestShape` (JT-40), including the empty request emitting both. |
| 3.3 | Client may name a capability it does not use | MAY | `TestRequestUsing`, the three seeded rows. |
| 3.3 | Calls are ordered so a back-reference names an earlier call | MUST (from the sequential-processing rule) | GAP. `Request.Invoke` assigns increasing ids and nothing rejects a `ResultReference` naming a later or absent one. `TestRequestBackReferenceNamesTheRightCall` proves the id Invoke returns is the right one; it does not prove a wrong one is refused. Proving it needs a request-level validator at marshal time. |
| 3.3 | Client may seed `createdIds` and reads the echoed map back | MAY | `TestRequestShape`, `TestResponseCreatedIDsRoundTrip` (JT-09). |
| 3.4 | Client preserves the order of `methodResponses` | MUST (tolerance) | `TestResponseKeepsEveryInvocation`, which asserts the submission response precedes the implicit set. |
| 3.4 | Client reads `createdIds` covering every creation the request made | MUST (server duty) | `TestResponseCreatedIDsRoundTrip` (JT-09). |
| 3.4 | Client compares `sessionState` and refetches on a change | RECOMMENDED | partial. `TestResponseStateIsOpaque` proves the value survives. The comparison and the refetch are the caller's, and untested. |
| 3.5 | Omitting an argument is exactly sending its default | MUST | `TestComparatorIsAscending` (JT-42), `TestQueryCollapseThreads` (JT-45), `TestOptionalBooleanStates`, `TestMailboxIsSubscribed`. The `*bool` decision is what makes this true and every one of those tests pins it. |
| 3.5 | Client tolerates a response omitting an argument that holds its default | MAY (server) | partial. Every response field carries `omitempty`, so an absent property decodes to the Go zero. Two response fields break the equivalence and get their own rows below: `total` on a `/query` and `notFound` on a `/get`. |
| 3.6.1 | Client parses an RFC 7807 problem-details body on an HTTP error | SHOULD (server) / MUST (tolerance) | `TestDoDecodesProblemDetails` with four content-type rows (JT-29, DV-10), `TestRequestErrorDecodesProblemDetails`, `TestFetchSessionSurfacesTheServerRefusal`, `TestDownloadSurfacesAServerRefusal`, `TestRelianceUploadStatusHandling`, `TestListenStopsOnAServerRefusal`. |
| 3.6.1 | Client reads the mandatory `limit` property on the limit problem type | MUST | `TestDoDecodesProblemDetails`, `TestRelianceThreeErrorShapes` request-level row. |
| 3.6.1 | Client recognises `unknownCapability`, `notJSON`, `notRequest`, and `limit` | MUST (derived) | GAP for two of four. `RequestError.Type` is a bare string with no sentinel constants, unlike `MethodError`. `unknownCapability` and `limit` decode under test; `notJSON` and `notRequest` appear nowhere. A caller must string-match. |
| 3.6.1 | A body that is not problem details still reaches the caller as a typed failure | MUST (derived) | `TestDoDegradesToAnHTTPError`, three rows including an empty body. |
| 3.6.2 | Client reads an `error` response under its own call id in place of the method's own | MUST | `TestResponseDecodesMethodError` (JT-31), `TestRelianceThreeErrorShapes` method-level row. |
| 3.6.2 | A method-level error is not a request-level failure and the calls around it still ran | MUST | `TestRelianceThreeErrorShapes` method-level row, which asserts the call after the failure decoded. |
| 3.6.2 | Client reads the mandatory `type` property on an error response | MUST | `TestResponseDecodesMethodError`, four rows. |
| 3.6.2 | Client resynchronises impacted data on `serverPartialFail` | MUST | GAP. `ErrServerPartialFail` exists and `TestMethodErrorIsMatchesOnlyItsOwnType` proves it matches only its own type. Nothing forces or tests a resync. Proving it belongs to the store's contract test. |
| 3.6.2 | Client treats an error type it does not understand the same as `serverFail` | MUST | GAP, and the package pins the opposite. `TestResponseDecodesMethodError` asserts an unregistered type does not match `ErrServerFail`. See gap 4 in the summary. |
| 3.6.2 | Client does not show an `invalidArguments` description to the user | MUST (derived from "not intended to be shown directly to end users") | GAP. `MethodError.Error()` concatenates the type and the description with nothing marking it unsafe for display. `SetError.Description` carries a doc comment about localisation; `MethodError.Description` carries none. |
| 3.6.2 | Client tolerates server state being unchanged when a method errors | MUST (server) | N/A. Server duty with no client action. |
| 3.7 | A referenced argument is named with a leading `#` | MUST | `TestInvocationAllowsAReferenceWithoutItsNormalForm`, `TestMethodInvoke` Email/get row, `TestRelianceBackReferenceBatch` asserting the wire bytes. |
| 3.7 | Client never builds an arguments object holding both `foo` and `#foo` | MUST | `TestInvocationRefusesReferenceCollision` (JT-07), including that the reference alone still marshals. |
| 3.7 | A reference that does not resolve fails loudly and never yields an empty list | MUST | `TestResolveFailsLoudly` (JT-06), fifteen rows covering a missing call id, a name mismatch, a missing property, a scalar descent, a wildcard over a non-array, an index past the end, a leading zero, three integer-overflow shapes, a signed index, a non-pointer path, and a reference into a failed call. |
| 3.7 | The path is an RFC 6901 pointer with `*` mapping over an array and flattening one level | MUST | `TestResolveSimpleChain` and `TestResolveWildcardChain` against both of section 3.7's worked chains (JT-05). |
| 3.7 | Resolution reads the arguments as the server sent them | MUST (derived) | `TestResolveReadsRawArguments`, `TestInvocationKeepsRawArguments`, both of which assert the re-marshalled form has already lost the property. |
| 3.7 | Step 2 checks the name of the first response under the call id and fails rather than looking further | MUST | `TestResolveFailsLoudly` name-mismatch and failed-call rows. |
| 3.8 | Client sends `Accept-Language` when it wants localised user-visible strings | SHOULD (server duty, client-triggered) | GAP. No request in the package sets the header. RFC 8621 section 7.5.1's German `forbiddenToSend` description is the case that matters, and `TestResponseRejectedSubmissionHasNoImplicitSet` reads a localised description from a fixture that no request could have provoked. |
| 3.8 | Client reads `Content-Language` on the response | SHOULD | GAP. Never read. |
| 3.10 | Client tolerates data changing between two calls in one request | MUST (derived) | GAP. `IfInState` exists on all four /set-shaped methods and `TestRequestTags` observes the tag. No test sends it and there is no `stateMismatch` sentinel. |

### Section 5, standard methods

| § | Obligation | Strength | Status |
|---|---|---|---|
| 5.1 | Client asks only for properties the type defines | MUST (server rejects with invalidArguments) | GAP. `Properties []string` is unchecked in `MailboxGet`, `EmailGet`, and `IdentityGet`. Proving it needs a per-type property table. |
| 5.1 | Client either discards its cache or calls `/changes` when the state string differs | MUST | GAP at the consumer. The state strings decode. The decision belongs to the store and is untested there. |
| 5.1 | Client does not assume `list` is in the order of the requested ids | MAY (server may reorder) | GAP, and one test assumes the opposite. `TestRelianceBackReferenceBatch` asserts the `Mailbox/get` list order equals the referenced id order, which the fixture makes true and the RFC does not require. That assertion pins a fixture coincidence. |
| 5.1 | Client tolerates a repeated requested id appearing once in the response | MUST (server) | N/A. Server duty with no client action. |
| 5.1 | Client treats an absent `notFound` as "the server did not say" rather than as "nothing was missing" | MUST (derived, DV-04) | GAP. `NotFound []ID` distinguishes nil from empty after decoding, so the information survives, but nothing asserts it and the package offers no requested-minus-returned helper. Stalwart omitted `notFound` before v0.16.10. Proving it needs two fixtures and a helper. |
| 5.2 | `maxChanges`, when supplied, is a positive integer greater than zero | MUST | `TestMethodInvoke` Email/changes row proves a zero `MaxChanges` is omitted rather than sent. The upper bound is the section 1.3 row. |
| 5.2 | Client tolerates a paged `/changes` never reporting a record destroyed before it was created | MUST (server) | GAP. JT-13 is unbuilt. No fixture replays section 5.2's five-state walkthrough. |
| 5.2 | Client keeps calling `/changes` while `hasMoreChanges` is true, and terminates | MUST (derived) | GAP. JT-14 is unbuilt. `HasMoreChanges` decodes and nothing loops on it in this package. A same-state-forever fixture is the test. |
| 5.2 | Client reaches the same local state whether a created-then-destroyed record is omitted or listed as destroyed | SHOULD (server) | GAP. JT-15 is unbuilt. |
| 5.2 | Client invalidates its cache for the type on `cannotCalculateChanges` | MUST | partial. `TestResponseDecodesMethodError`, `TestRelianceThreeErrorShapes`, and `TestQueryChangesCannotCalculate` prove the sentinel is distinguishable by type rather than by string. The invalidation is the store's and untested. |
| 5.3 | Client omits server-set properties from a create | MUST | GAP. See the section 1.1 row and gap 1 in the summary. |
| 5.3 | A PatchObject pointer never references inside an array | MUST | `TestPatchRefusesIllegalPointer` (JT-03), the index, append-token, and nested-index rows. `TestPatchRefusalReachesTheRequest` proves the refusal survives being nested in a `Request`. |
| 5.3 | Every part of a patch pointer before the last already exists on the object | MUST | GAP. `Patch.validate` cannot check it without a record schema, and `patch.go` documents the schema problem only for the array rule. A strict server answers `invalidPatch`. |
| 5.3 | No two patch pointers where one is the prefix of the other | MUST | `TestPatchRefusesIllegalPointer`, the overlap and third-key rows; `TestPatchAllowsAKeyThatOnlyLooksNested` proves the rule does not misfire on siblings. |
| 5.3 | Client does not patch a server-set property to a different value | MUST | GAP. Same shape as the immutable row in section 1.1. |
| 5.3 | Client reads all six result maps and never reads success off the method's own outcome | MUST | `TestSetPartialFailure` (JT-01), `TestMailboxSetPartialFailure`, `TestRelianceThreeErrorShapes` record-level row, each asserting the state advanced while records failed. |
| 5.3 | Client recognises the `notFound` SetError on an update or destroy of an absent id | MUST (server) | GAP. `SetError.Type` is a bare string with no sentinels. The same shortfall as the section 3.6.1 request-error row. |
| 5.3 | A foreign key to a record created in the same request uses the creation id with a leading `#` | MUST | `TestIDMarshalAcceptsCreationReference`, `TestMethodInvoke` both EmailSubmission/set rows. |
| 5.3 | The referenced record is created in the same or an earlier method call | MUST | GAP. Nothing checks the ordering of a `#` reference against the call that creates it. Proving it needs a request-level validator that walks creation ids across calls. |
| 5.3 | Client does not reuse a creation id anywhere in one request | SHOULD | GAP. `Create` is a map per method, so two methods may both use `k1` and nothing notices. JT-08 is unbuilt. |
| 5.3 | Client tolerates a reused creation id resolving to the most recently created item | MUST (server) | GAP. JT-08 is unbuilt. |
| 5.3 | Client reads the `properties` list on an `invalidProperties` SetError | SHOULD (server) | `TestSetErrorTypedExtras`, `TestRelianceThreeErrorShapes`, `TestMailboxSetPartialFailure`. |
| 5.3 | Client preserves an unregistered SetError type and its whole payload | MAY (server may define more) | `TestSetErrorTypedExtras` unregistered-type row asserting `Raw` byte for byte (JT-12). |
| 5.3 | The value for every key in a keyword or mailbox set is `true` | MUST (section 5.7, RFC 8621 section 4.1.1) | GAP. `map[string]bool` and `map[ID]bool` accept `false`, and `Patch{Pointer("keywords","$seen"): false}` marshals. A lenient server may store the false. Proving it needs a marshal-time check or a set type. |
| 5.4 | `/copy` obligations | n/a | N/A. This section describes a generic create-with-a-source-id shape, with `create`, `ifFromInState`, `ifInState`, and `onSuccessDestroyOriginal`; no `Foo/copy` method using that shape is modelled on any type. `Blob/copy` (section 6.3, next row) is a distinct, simpler method that does not use it. `Email/copy` (RFC 8621 section 4.7), the one type in this package's scope that would use it, is deliberately unmodelled; see that row. |
| 5.4 | Client reads the mandatory `existingId` on an `alreadyExists` SetError | MUST | GAP. `SetError` has no `ExistingID` field, so the id lands in `Raw` alone. This is the one place an import that hit a duplicate can point the user at the message that already exists. |
| 5.5 | The filter operator is exactly one of AND, OR, or NOT | MUST | partial. `TestFilterOperatorNesting` covers all three and two-deep nesting (JT-43). `Operator` is a string type with no validation, so an arbitrary value marshals. |
| 5.5 | A FilterCondition carries no `operator` property | MUST | `TestEveryJSONTagIsObserved`, which fails if a tag is added to a condition type that its case does not name. Structurally satisfied today. |
| 5.5 | Client performs no client-side reordering of a server-returned result list | MUST (derived from the stable-sort rule) | `TestQueryResponseKeepsServerOrder` (DV-02). |
| 5.5 | Client asks only for a collation the server advertised in `collationAlgorithms` | MUST (derived) | GAP. `Comparator.Collation` is never checked against `Core.CollationAlgorithms`, which decode in the same session. |
| 5.5 | Client never sends a negative `limit` | MUST | `Limit uint64` makes it unrepresentable. `TestMethodInvoke` shows the positive case. Proven by the type. |
| 5.5 | Client may send a negative `position`, which the server clamps | MUST (server) | GAP. `Position int64` allows it and no test exercises a negative value. |
| 5.5 | Client does not send `position` alongside `anchor`, nor `anchorOffset` without one | MUST (server ignores) | GAP. `EmailQuery` and `MailboxQuery` allow both pairs and nothing warns. `TestQueryAnchorPaging` sends anchor and anchorOffset only. |
| 5.5 | Client handles `anchorNotFound` as its own error type | MUST (server) | `TestQueryAnchorPaging` (JT-44), matching `ErrAnchorNotFound` by type. |
| 5.5 | Client either discards the cached query or calls `/queryChanges` when `queryState` differs | MUST | GAP at the consumer. `queryState` decodes and nothing acts on it here. |
| 5.5 | Client distinguishes "total not requested" from "total is zero" | MUST (server omits unless asked) | GAP. `Total uint64` with `omitempty` collapses the two, and so do `Position` and `Limit`. A query that matched nothing and asked for the total is indistinguishable from one that never asked. Proving it needs `*uint64` or a presence flag. |
| 5.5 | Client suggests simplifying the search on `unsupportedFilter` | SHOULD | GAP. No sentinel for `unsupportedFilter`, and the suggestion is a UI act. |
| 5.6 | Client applies `removed` first, then `added` one by one from the lowest index | MUST | `TestSpliceFollowsTheRFCWalkthrough` against section 5.6's own sparse worked example, `TestSpliceOrdersItsWork` six rows (JT-17). |
| 5.6 | Client applies `added` in ascending index order and refuses an unsorted array | MUST | `TestSpliceRefusesAnUnusableChange` ascending row, plus the two past-the-end rows (JT-17). |
| 5.6 | Client truncates or extends the spliced list to the new total | MUST | GAP, delegated. `Splice` documents that it does neither and why. No caller is tested doing it, so the third of section 5.6's three splice steps is unproven anywhere in the repo. |
| 5.6 | Client tolerates `removed` naming ids it never held | MAY (server) | `TestSpliceOrdersItsWork`, the removal-the-cache-never-held row. |
| 5.6 | Client reinserts a row that appears in both `removed` and `added` because a mutable property moved it | MUST (server includes it) | `TestSpliceOrdersItsWork`, the moved-row row. |
| 5.6 | Client sends `upToId` only when it holds a contiguous prefix and the filter and sort are immutable | SHOULD (server) | GAP. `UpToID` marshals under `TestMethodInvoke` and the semantics are untested. JT-17's `upToId` half is unbuilt. |
| 5.6 | Client repeats the original query's `filter`, `sort`, and `collapseThreads` on a `/queryChanges` | MUST (derived) | GAP. Documented in the type comments and untested. Nothing ties a `queryChanges` to the query it continues. |
| 5.6 | Client distinguishes "total not requested" from zero on a `/queryChanges` | MUST | GAP. Same `uint64` collapse as the `/query` row. |
| 5.6 | Client retries with a higher `maxChanges` or invalidates on `tooManyChanges` | MAY | GAP. No sentinel for `tooManyChanges`, so a caller string-matches. |
| 5.6 | Client invalidates its cached query results on `cannotCalculateChanges` | MUST | partial. `TestQueryChangesCannotCalculate` proves the sentinel; the invalidation is the store's. |

### Section 6, binary data

| § | Obligation | Strength | Status |
|---|---|---|---|
| 6 | Client reads a reassigned `blobId` out of the created or updated record rather than reusing the uploaded one | MUST (server returns it) | GAP. `EmailSetResponse.Created` carries it and no test asserts the reassignment is read. A resend that reuses the upload's blob id fails against a server that reassigns. |
| 6 | Client does not rely on an unreferenced blob living beyond one hour | MUST (server floor) | GAP. Named in the inventory's honest-gaps section as production-only. Nothing in the package records an upload time. |
| 6 | Client may issue a create and a destroy referencing one blob in the same call | MUST (server) | N/A. Server duty with no client action. |
| 6.1 | Client accepts any 2xx with a decodable body as a successful upload | MUST (derived, no status specified) | `TestRelianceUploadStatusHandling` (DV-01, JT-32), rows for 200, 201, 202, and an extra property beyond the four. |
| 6.1 | Client reads `accountId`, `blobId`, `type`, and `size` from the upload response | MUST | `TestRelianceUploadStatusHandling`, every field asserted. |
| 6.1 | Client sends the blob's real media type as the request `Content-Type` | MUST (derived, the server takes it from there) | `TestUploadSendsTheCallersContentType`, which also asserts the body bytes. |
| 6.1 | Client parses problem details on an upload error | SHOULD (server) | `TestRelianceUploadStatusHandling`, the refusal row asserting the `limit` property. |
| 6.1 | Client works within `maxSizeUpload` and `maxConcurrentUpload` | MUST (derived) | GAP. Both decode under `TestRelianceCoreLimits` and `Upload` consults neither. |
| 6.2 | Client expands the download template with all four variables, percent-encoded per RFC 6570 simple string expansion | MUST | `TestDownloadExpandsTheURLTemplate` (JT-32). |
| 6.2 | Client sends `name` so the server can set `Content-Disposition` | MUST (server) | `TestDownloadExpandsTheURLTemplate`. |
| 6.2 | Client parses problem details on a download error and returns no reader alongside | SHOULD (server) | `TestDownloadSurfacesAServerRefusal`, asserting both the typed error and the nil reader. |
| 6.2 | Client streams a download rather than buffering it | n/a (implementation) | `TestRelianceStreamingDownload`, which holds the tail back and asserts the head arrives. |
| 6.3 | `Blob/copy` obligations | n/a | partial. Modelled as `BlobCopy`/`BlobCopyResponse`. `TestMethodInvoke`'s Blob/copy case proves the wire shape byte for byte, `TestBlobCopyRequiresNoMailCapability` proves the method needs no capability beyond core, and `TestBlobCopyResponseDecodes` proves both the `copied` and `notCopied` maps decode, including an unregistered SetError type keeping its `Raw` payload. The conformance suite's `TestConformanceBlobRoundTrip` proves it live against Stalwart: an uploaded blob and a blob id that does not exist, copied in one call, land correctly in `copied` and `notCopied`. GAP: every proof, including the live one, copies within a single account. `fromAccountId` differing from `accountId`, the operation section 6.3 exists for, is untested anywhere. |

### Section 7, push

| § | Obligation | Strength | Status |
|---|---|---|---|
| 7.1.1 | Client validates that a pushed object's `@type` is `StateChange` | MUST | GAP. `TestStateChangeFansOut` asserts the field on a fixture. `listener.consume` does not check it before calling `OnChange`, so a `state` event carrying some other `@type` is delivered as a StateChange with an empty `changed` map. |
| 7.1.1 | Client fans a StateChange out per account and per type, keeping types it does not model | MUST | `TestStateChangeFansOut` (JT-28), including a CalendarEvent state and an account that reports no Email state. |
| 7.2 | PushSubscription obligations | n/a | N/A. Not modelled, stated in `eventsource.go`. DV-08's fallback question does not arise, since the package never uses PushSubscription. |
| 7.3 | Client makes an authenticated GET to the event-source URL with the variables substituted | MUST | `TestListenExpandsTheURLTemplate`. Authentication rides the caller's `http.Client`, as everywhere else. |
| 7.3 | `types` is a comma-separated list of type names or the single character `*` | MUST | `TestListenExpandsTheURLTemplate`, the empty and named rows. |
| 7.3 | `closeafter` is exactly `state` or `no` | MUST | `TestListenExpandsTheURLTemplate`, both rows. |
| 7.3 | `ping` is a positive integer number of seconds, or `0` for none | MUST | partial. `TestListenExpandsTheURLTemplate` proves 300 and 0. GAP: `EventSource.Ping` is a `time.Duration` and `Listen` computes `Ping/time.Second`, so 500ms silently becomes `ping=0`, which is "no ping" and also disables the stall detector. A negative Duration sends a negative integer. |
| 7.3 | Client adapts its liveness expectation to the interval the ping payload reports | MUST (derived from the clamping allowance) | `TestListenAdaptsToTheServersPingInterval` (JT-26) with a real elapsed-time bound, `TestPingCadence` eight rows, `TestStallWindow` six rows, `TestListenSurvivesAnAbsurdPingInterval`. |
| 7.3 | Client does not read the `[30, 300]` figures as a bound on the value it may request | n/a | N/A by the inventory's own warning. Those constrain the server's limits. `TestListenExpandsTheURLTemplate` requests 300 and `TestListenAdaptsToTheServersPingInterval` accepts a granted 1. |
| 7.3 | With `closeafter=state` the client stops after one state event rather than reconnecting | MUST (server ends the response) | `TestListenStopsAfterOneStateEvent` (JT-27), asserting exactly one change and exactly one connection. |
| 7.3 | Client sends `Last-Event-ID` on a reconnect with the last id it saw | SHOULD (server benefit) / MUST (SSE) | `TestListenResumesFromTheLastEventID` (JT-22), `TestListenKeepsTheResumeIDAcrossAConnection` across a connection that dispatched nothing. |
| 7.3 | Client assumes nothing about whether the server replays what was missed | SHOULD (server) | `TestListenResumesFromTheLastEventID`, both the honours-the-header and ignores-the-header rows, asserting no change is invented or dropped (JT-22, DV-09). |
| 7.3 | Client tolerates a server that sets a new event id on a ping | MUST (server must not) | GAP. The reader treats a ping's `id:` like any other, so a non-conforming server advances the resume point past changes the client never saw. Tolerance is arguably right and no test says so either way. |
| 7.3 | Client uses a single event-source connection | SHOULD | `Listen` runs one connection at a time in a serial loop. Proven by construction; `TestListenSurvivesAnAbsurdPingInterval` asserts exactly one connection over the run. |
| 7.3 | Client reports each new connection as a gap the caller must close with `/changes` | n/a (poplar's own rule, ADR-0018) | `TestListenResumesFromTheLastEventID` asserts one `OnConnect` per connection and at least one `OnDisconnect`. |

### Section 8, security

| § | Obligation | Strength | Status |
|---|---|---|---|
| 8.1 | Every request uses TLS 1.2 or later | MUST | GAP. Delegated to the caller's `http.Client` and not documented as delegated. |
| 8.1 | Client validates TLS certificate chains | MUST | GAP. Satisfied by Go's default transport and silently defeated by a caller passing `InsecureSkipVerify`. Nothing in `client.go` says the caller owns this. |
| 8.2 | Basic authentication is not recommended | NOT RECOMMENDED | N/A. The package holds no credential and chooses no scheme. |
| 8.5 | Server implements sensible limits | MUST (server) | N/A. |
| 8.7 | Client specifies encryption keys on a PushSubscription routed through a third party | MUST | N/A. PushSubscription is not modelled. |

---

## RFC 8621, JMAP Mail

### Section 1, capabilities and push

| § | Obligation | Strength | Status |
|---|---|---|---|
| 1.3.1 | Client reads all six mail account-capability properties | MUST (server supplies) | `TestSessionCapabilitiesDecode` (JT-19), every field asserted. |
| 1.3.1 | Client reads `maxMailboxesPerEmail` null as no limit | MUST | `TestSessionNullLimitIsNotZero`. |
| 1.3.1 | Client ignores `emailQuerySortOptions` entries it does not recognise | MUST | `EmailQuerySortOptions []string` cannot reject one, and `TestSessionCapabilitiesDecode` reads a three-entry list. Satisfied by the type. |
| 1.3.1 | Client sorts only on a property the account advertises | MUST (derived) | GAP. `Comparator.Property` is never checked against `EmailQuerySortOptions`. |
| 1.3.1 | Client works within `maxMailboxesPerEmail`, `maxMailboxDepth`, `maxSizeMailboxName`, `maxSizeAttachmentsPerEmail` | MUST (derived) | GAP. All four decode and none is consulted. |
| 1.3.2 | Client reads `maxDelayedSend` and `submissionExtensions` | MUST (server supplies) | `TestSessionCapabilitiesDecode`, including the FUTURERELEASE ehlo-args. |
| 1.3.2 | Client sends only envelope parameters the server advertised | MUST (derived) | GAP. `EnvelopeAddress.Parameters` is unchecked, and its doc comment asserts the constraint ("Only parameters the server advertised ... are accepted") without enforcing it. |
| 1.4 | Client reads per-account capabilities rather than assuming the session's | MUST | `TestSessionCapabilitiesDecode` reads the mail and submission objects off `Account.Capabilities`; `TestSessionFields` asserts the shared read-only account. |
| 1.5 | Client subscribes to the `EmailDelivery` push type to learn that new mail arrived | MUST (server supports it) | GAP. `EventType` is a free string so the name is expressible, and nothing names it, no constant exists, and no test subscribes to it. `Email` state changes on every flag change; `EmailDelivery` is the only push that means new mail. For a mail client this is the push type that matters most. |
| 1.6 | Ids match an IMAP RFC 8474 interface | SHOULD (server) | N/A. |

### Section 2, mailboxes

| § | Obligation | Strength | Status |
|---|---|---|---|
| 2 | Client never leaves an Email in zero mailboxes | MUST | partial. `TestPatchLeaf` proves a move patches two leaves and never the parent (JT-04), and `patch.go` documents that an empty `mailboxIds` map hides the message. Nothing rejects `Patch{"mailboxIds": map[ID]bool{}}`. |
| 2 | A mailbox name is a Net-Unicode string of at least one character within `maxSizeMailboxName` | MUST | GAP. `Mailbox.Name` is unchecked on create. |
| 2 | No two sibling mailboxes share a parent and a name | MUST | GAP, and DV-11 records that servers disagree on which error this produces. No test. |
| 2 | A mailbox parent chain has no loop | MUST | GAP. Nothing checks a `parentId` update against the tree. |
| 2 | A mailbox has at most one role, and no two mailboxes in an account share a role | MUST | GAP. `Role` is a bare string field on a create. |
| 2 | A role is one of the IANA IMAP mailbox name attributes, lowercased | MUST | partial. The eighteen `Role` constants enumerate the registry and `TestMailboxDecode` asserts `RoleInbox`. `Role` is a string type, so an arbitrary value marshals. |
| 2 | `sortOrder` is an integer in 0 to 2^31-1 | MUST | GAP. `SortOrder uint64` admits far more. |
| 2 | Mailboxes with equal `sortOrder` are displayed alphabetically by name | SHOULD | N/A to this package. A rendering rule that belongs to the UI layer. |
| 2 | Client may ignore `isSubscribed` entirely, and can express its three states | MAY | `TestMailboxIsSubscribed`, absent, false, and true. |
| 2 | An Email in trash and another mailbox is treated as existing in both | SHOULD | N/A to this package. A store and UI rule. |
| 2.2 | Client reads a null `updatedProperties` as "refetch everything" | MUST (server sets null when it cannot tell) | GAP. `UpdatedProperties []string` distinguishes nil from empty after decoding and `omitempty` loses the distinction on a re-marshal. No test. |
| 2.3 | Client sorts mailboxes only on properties the server must support | MUST (server duty) | `TestMethodInvoke` Mailbox/query row sorts on `name`. |
| 2.5 | Client removes child mailboxes before deleting the parent | MUST | GAP. `MailboxSet.Destroy` sends whatever it is given. `TestMailboxSetPartialFailure` decodes `mailboxHasEmail` and never `mailboxHasChild`, and neither has a sentinel. |
| 2.5 | Client understands that `onDestroyRemoveEmails` removes messages from the mailbox being destroyed | n/a (semantics) | GAP. The tag is observed by `TestRequestTags` and the behaviour is untested. |

### Section 3, threads

| § | Obligation | Strength | Status |
|---|---|---|---|
| 3 | Client reads `threadId` and does not recompute thread grouping | SHOULD (the grouping heuristic is a server SHOULD) | partial. `TestEmailDecode` reads `threadId` and nothing in the package groups messages. JT-46's second half is unprovable: see the next row. |
| 3 | Client consumes `Thread/get` output | n/a (the only way to read a thread) | GAP, and currently impossible. No `Thread` type and no `Thread/get` entry in `methodResponses`, so a response carrying one fails the whole `Response` decode. See gap 3 in the summary. |
| 3 | Client tolerates an Email id changing when a server merges threads by delete and reinsert | MUST (server) | GAP. Untested, and the store is where it would bite. |

### Section 4, email

| § | Obligation | Strength | Status |
|---|---|---|---|
| 4.1.1 | Every key in `mailboxIds` and `keywords` maps to `true` | MUST | GAP. See the RFC 8620 section 5.3 row. |
| 4.1.1 | A keyword is 1 to 255 characters of ASCII %x21-%x7e excluding a listed set | MUST | GAP. `Keywords map[string]bool` is unchecked and `Pointer` escapes without validating. |
| 4.1.1 | Client sends keywords lowercased and compares case-insensitively | MUST (servers return lowercase) | GAP. No constants, no normalisation, no test. Tests use `$seen` and `$draft` literals. |
| 4.1.1 | Client warns and disables links and attachments on `$phishing` | SHOULD | N/A to this package. A UI rule; the keyword is expressible. |
| 4.1.1 | Client sets `$junk` and `$notjunk` when the user reports spam or legitimacy | SHOULD | GAP at the consumer. The keywords are expressible and nothing sets them. |
| 4.1.1 | Client tolerates the `\Deleted` and `\Recent` IMAP keywords being absent | MUST (server) | N/A. |
| 4.1.2 | Client tolerates best-effort header parsing, including an `email` that is not a valid addr-spec | MAY (server) | partial. `Address.Email` is a plain string, so a malformed address survives. `Address.String()` renders it unchanged. No test uses a malformed address. |
| 4.1.3 | A `header:Name:asX:all` property spells its two suffixes in that order | MUST | GAP. The parsed-header property syntax is not modelled at all, so a caller hand-spells the string into `Properties` and nothing checks it. |
| 4.1.3 | Client tolerates a null or empty-array value for a header property with no matching field | n/a | N/A. Not modelled. |
| 4.1.4 | Client does not recompute `hasAttachment` | MAY (server heuristics) | `TestEmailDecode` reads the server's value. Nothing recomputes. |
| 4.1.4 | Client tolerates `preview` changing between fetches without the Email being marked changed | MAY (server) | N/A. Server duty with no client action. |
| 4.2 | Client reads `isTruncated` whenever it sets `maxBodyValueBytes` | MUST (server truncates) | `TestBodyValueFlags`, four rows, plus the doc comment on `BodyValue.IsTruncated`. |
| 4.2 | Client knows the default property list it gets when `properties` is omitted | MUST (server applies it) | GAP. The default list excludes `bodyStructure` and every `header:` form, so a caller who omits `properties` and reads `BodyStructure` gets nil with no signal. The list appears nowhere in the package. |
| 4.2 | Client does not request a parsed header form forbidden for that field | MUST | GAP. Same as the section 4.1.3 row. |
| 4.2 | Client matches its own capitalisation when reading a header property back | MUST (server echoes it) | N/A. Header properties are not modelled. |
| 4.2 | Client takes care fetching properties outside the fast set | SHOULD | N/A. A caller's judgement. |
| 4.4.1 | The `header` filter array holds exactly one or two elements | MUST | GAP. `Header []string` is unchecked, and DV-07 records header filters as a live Stalwart divergence. |
| 4.4.1 | Client knows an empty FilterCondition evaluates to true for every message | MUST (server) | partial. `TestFilterOperatorNesting` proves `EmailFilterCondition{}` marshals to `{}`. Nothing warns that an accidentally empty condition matches every message, which is the shape that turns a chained destroy into a mailbox wipe. |
| 4.4.1 | Client escapes the listed characters with a backslash inside a quoted phrase | MUST | GAP. No escaping helper exists, so a caller's quote or backslash reaches the server raw. |
| 4.4.1 | Client tolerates server-side stemming, case folding, and markup stripping | SHOULD/MAY (server) | N/A. Server duties with no client action. |
| 4.4.2 | A `hasKeyword`, `allInThreadHaveKeyword`, or `someInThreadHaveKeyword` sort also carries a `keyword` | MUST | GAP. `Comparator.Keyword` is optional and unchecked. `TestComparatorIsAscending` sets both in one row and asserts only the bytes, so removing the check that does not exist changes nothing. |
| 4.4.2 | Client may rely on `receivedAt` being sortable | MUST (server) | `TestMethodInvoke` Email/query row. |
| 4.5 | Client repeats `collapseThreads` from the original query | MUST (derived) | GAP. Documented in the type and untested. |
| 4.6 | No `headers` property on a create, on the Email or a body part | MUST NOT | GAP. `EmailSet.Create` takes an `*Email` whose `Headers` field marshals. |
| 4.6 | No two properties representing the same header field | MUST NOT | GAP. Unchecked. |
| 4.6 | No `Content-` header field on the Email object | MUST NOT | GAP. Unchecked. |
| 4.6 | No `textBody`, `htmlBody`, or `attachments` beside a `bodyStructure` | MUST NOT | GAP. `Email` carries all four with no check. |
| 4.6 | A `bodyStructure` part does not repeat a header field set on the Email | MUST NOT | GAP. Unchecked. |
| 4.6 | `textBody` holds exactly one part, of type `text/plain` | MUST | GAP. Unchecked. |
| 4.6 | `htmlBody` holds exactly one part, of type `text/html` | MUST | GAP. Unchecked. |
| 4.6 | A body part carries `partId` or `blobId`, never both, and a `partId` appears in `bodyValues` | MUST | GAP. `BodyPart` carries both fields. JT-33 names this constraint by its Fastmail test-suite file name. |
| 4.6 | `charset` and `size` are omitted when a `partId` is given | MUST | GAP. Unchecked. |
| 4.6 | No `Content-Transfer-Encoding` header field on a body part | MUST NOT | GAP. Unchecked. |
| 4.6 | `isEncodingProblem` and `isTruncated` are false or omitted on a create | MUST | GAP. `BodyValue` is shared between the fetched record and the create, so a fetched truncated body round-trips into a create with the flag set. |
| 4.6 | Client tolerates a server modifying the Email rather than rejecting it | MAY (server) | GAP. This is what makes the eleven rows above quiet rather than loud, and nothing reads the created record back to see what changed. |
| 4.6 | Client does not set `Message-ID` or `Date` and lets the server generate them | MUST (server generates when absent) | GAP. Both are settable through `Email.MessageID` and `Email.SentAt`, and nothing says which the server owns. |
| 4.6 | When emptying the trash the client does not destroy Emails also in another mailbox | SHOULD NOT | GAP at the consumer. A store rule; the package expresses both operations. |
| 4.6 | Client reads the mandatory `notFound` list on a `blobNotFound` SetError | MUST | `TestSetErrorTypedExtras`, `TestSetPartialFailure` (JT-12, JT-01). |
| 4.7 | `Email/copy` obligations | n/a | N/A, deliberate. poplar moves messages between mailboxes within one account with a `mailboxIds` patch, not a copy, and has no cross-account copy feature (`jmap/TRIMMED.md`). |
| 4.8 | At least one mailbox is given on an import | MUST | GAP. `EmailImportItem.MailboxIDs` carries `omitempty`, so an empty map vanishes and the server rejects the import. Proving it needs a marshal-time check. |
| 4.8 | Client reads the mandatory `existingId` on an `alreadyExists` import rejection | MUST | GAP. See the RFC 8620 section 5.4 row. `SetError` has no field for it. |
| 4.8 | Client tolerates the server returning a different `blobId` than the one imported | MUST (server) | GAP. `EmailImportResponse.Created` carries the record and no test reads the reassigned blob id. |
| 4.9 | `Email/parse` obligations | n/a | N/A. Not modelled. |
| 5 | `SearchSnippet` obligations | n/a | N/A. Not modelled. |

### Server-set properties on a create

This package models every JMAP object as one struct that serves both the
fetched record and, where the package models a create, the create payload too.
The same shape recurs across roughly fifteen properties: `Email.ID`, `.BlobID`,
`.ThreadID`, `.Size`, `.HasAttachment`, `.Preview`; `Mailbox.TotalEmails`,
`.UnreadEmails`, `.TotalThreads`; and `EmailSubmission.ID`, `.ThreadID`,
`.UndoStatus`, `.DeliveryStatus`, `.DSNBlobIDs`, `.MDNBlobIDs`, each expressible
on a create today because `EmailSet.Create`, `MailboxSet.Create`, and
`EmailSubmissionSet.Create` all take the same struct type the fetch decodes
into. `Identity.MayDelete` carries the same server-set shape but stays dormant:
the package models no `Identity/set` at all (`jmap/TRIMMED.md`), so the property
cannot reach a create until that method does, and it will inherit the same gap
the day it is added.

A create carrying one of these earns `invalidProperties` on the wire, and the
type system does nothing to stop a caller from building one; `omitempty` only
hides the zero value, not a value copied forward from a fetched record, which is
what a resend or a draft-from-template flow does.

`EmailSubmission.SendAt` was checked against this pattern as a possible defect
and ruled not one. RFC 8620 section 1.6 says the client MUST NOT send a
server-set property on a create, and RFC 8621 section 7 marks `sendAt`
`(immutable; server-set)`. JMAP immutability forbids an update, though, not a
create, and section 7's own text gives `sendAt` a default of the time of
creation on the server. The RFC's actual delayed-send lever is the
FUTURERELEASE extension (RFC 4865), asked for through an envelope address's
parameters and bounded by `maxDelayedSend`; the session fixture already carries
that extension. So the field was left as it stands. What was wrong was a doc
comment describing the wrong mechanism, and that comment is now corrected.

This is a standing constraint on the compose path pass 3 will build, not a
defect in the library today. This map proposes no fix; the decision belongs to
whichever pass designs the create side of compose.

### Section 6, identities

| § | Obligation | Strength | Status |
|---|---|---|---|
| 6 | Client uses the identity's `email` as the From address, honouring a `*` local part | MUST | GAP. `Identity.Email` decodes under `TestIdentityDecode` and nothing parses the `*` form or enforces the address on compose. |
| 6 | Client uses the identity's `name` in From | SHOULD | GAP at the compose layer. Decoded and unused. |
| 6 | Client sets Reply-To and Bcc from the identity | SHOULD | GAP at the compose layer. `TestIdentityDecode` asserts both decode. |
| 6 | Client inserts the identity's text and HTML signatures, or deliberately ignores them | SHOULD / MAY | GAP at the compose layer. Both decode. |
| 6 | Client keys identities by id, since several may share an email address | MAY (server allows duplicates) | GAP. `IdentityGetResponse.List` is a slice and nothing indexes by address. Untested. |
| 6 | Client reads `mayDelete` before offering to destroy an identity | n/a (semantics) | `TestIdentityDecode` asserts it decodes. The check belongs to the UI. |

### Section 7, email submission

| § | Obligation | Strength | Status |
|---|---|---|---|
| 7 | Client omits `envelope` and lets the server derive one from the message | MUST (server derives) | `TestMethodInvoke`, both EmailSubmission/set rows omit it. |
| 7 | Client does not set `sendAt`, which is immutable and server-set | MUST | GAP. `EmailSubmission.SendAt` still marshals into a create; nothing at the type level blocks it. The doc comment on the field now correctly names FUTURERELEASE as the RFC's actual delayed-send mechanism. See the server-set-property survey after the section 4.6 rows below. |
| 7 | Client does not set `undoStatus` on a create | MUST (server-set on create) | GAP. Same shared type; `undoStatus` marshals into a create. |
| 7 | Client sets `undoStatus` to `canceled` only on an update, and only while pending | MUST (the three allowed values) | GAP. No constants for the three values and no test cancels a submission. |
| 7 | Client tolerates `deliveryStatus` staying null on a server that does not track it | MAY (server) | `TestEmailSubmissionDecode` reads a populated one; the null case is untested but a nil map is the natural decode. |
| 7 | Client tolerates `delivered` being one of queued, yes, no, unknown, and `displayed` one of unknown, yes | MUST (server) | partial. `TestEmailSubmissionDecode` asserts `yes`. No constants exist, so a caller string-matches. |
| 7 | Client does not rely on refetching an EmailSubmission, which a server may destroy at any time | MAY (server) | GAP. Untested, and the outbox is where it bites. |
| 7 | Client reads `dsnBlobIds` and `mdnBlobIds` when the server exposes delivery status that way | SHOULD (server) | `TestEmailSubmissionDecode` asserts both decode. |
| 7.5 | Client reads both responses under one call id, the submission's first and the implicit `Email/set` second | MUST | `TestResponseKeepsEveryInvocation` (JT-11), `TestResponseImplicitEmailSetMatchesByCreationID` (JT-10), asserting the order. |
| 7.5 | Client does not treat a refused submission as sent, and knows no implicit set ran | MUST (derived) | `TestResponseRejectedSubmissionHasNoImplicitSet` (JT-10), asserting one response, the `forbiddenToSend` type, and an unmoved state. |
| 7.5 | `onSuccessUpdateEmail` and `onSuccessDestroyEmail` are keyed by the submission creation id with a leading `#` | MUST | `TestMethodInvoke`, both EmailSubmission/set rows, byte for byte. |
| 7.5 | Client reads the mandatory `maxSize` on a `tooLarge` SetError | MUST | GAP. `SetError` has no `MaxSize` field, so the number lands in `Raw` alone. The user hears "too large" without the limit. |
| 7.5 | Client reads the `properties` list on an `invalidEmail` SetError | SHOULD | `TestSetErrorTypedExtras`, the invalidEmail row. |
| 7.5 | Client reads the mandatory `maxRecipients` on `tooManyRecipients` | MUST | `TestSetErrorTypedExtras` (JT-12). |
| 7.5 | Client reads the mandatory `invalidRecipients` list on `invalidRecipients` | MUST | `TestSetErrorTypedExtras` (JT-12). |
| 7.5 | Client displays the `description` on a `forbiddenToSend` | MAY (server supplies) | `TestResponseRejectedSubmissionHasNoImplicitSet` asserts it is non-empty. Provoking a localised one needs the `Accept-Language` row in RFC 8620 section 3.8. |
| 7.5 | Client tolerates the server removing Bcc and altering headers during delivery | MUST (server) | N/A. Server duty with no client action. |
| 8 | VacationResponse obligations | n/a | N/A. Not modelled. |

### Section 9, security considerations

| § | Obligation | Strength | Status |
|---|---|---|---|
| 9.3 | Client renders each body part in isolation and never concatenates raw text values | MUST | N/A to this package, and binding on poplar's render layer. The EFAIL citation makes it security-relevant, so it belongs in the pass 3 plan rather than here. `Email.TextBody` and `HTMLBody` are lists precisely so a renderer can keep them apart. |
| 9.5 | Client treats data it cannot access as absent | MUST (server) | N/A. Server duty with no client action. |
| 9.6 | Client sends only from an address the identity permits | SHOULD (server rejects otherwise) | GAP. Same as the section 6 From row. |
| 10.4 | Client may silently ignore IMAP-only or reserved keywords | MAY | `Keywords map[string]bool` keeps everything the server sends. Satisfied by the type. |

---

## RFC 9219, S/MIME signature verification

| § | Obligation | Strength | Status |
|---|---|---|---|
| 3 | Client reads the presence of `urn:ietf:params:jmap:smimeverify` in the session capabilities before using the extension | MUST (server advertises) | GAP. `smimeVerify` is in the capability table and is an empty struct with no JSON tags, so `TestEveryJSONTagIsObserved` does not cover it and no fixture carries the URI. Nothing decodes it under test. |
| 4.1 | A request that asks for `smimeStatus`, `smimeStatusAtDelivery`, `smimeErrors`, or `smimeVerifiedAt` names the capability in `using` | MUST (RFC 8620 section 1.8) | `TestRequestUsing` (JT-39), the "a get asking for an S/MIME property asks for the capability" row and its negative counterpart. `EmailGet.Requires()` calls `withSMIME(smimeProperties(m.Properties))`. |
| 4.1 | Client reads the four S/MIME response properties | MUST (server returns them when asked) | `TestRequestTags` and `TestResponseTags` observe all four tags on `Email`. No semantic test reads them. |
| 4.1 | Client treats an unrecognised `smimeStatus` value as `unknown` or `signed/failed` | MUST | GAP. `SMIMEStatus` is a bare string with no constants and no mapping. A UI that renders the raw value reports a status the RFC requires to be downgraded, which is a security misreport rather than a cosmetic one. |
| 4.1 | Client tolerates `smimeStatus` changing over time and being cached up to 24 hours | MAY (server caches) | GAP. Nothing records when a status was read. |
| 4.1 | Client does not treat the `signed` status as verification | SHOULD (server should verify) | GAP. No constants distinguish `signed` from `signed/verified`. |
| 4.2 | A query filtering on `hasSmime`, `hasVerifiedSmime`, or `hasVerifiedSmimeAtDelivery` names the capability in `using` | MUST (RFC 8620 section 1.8) | `TestRequestUsing`, the top-level and nested-condition filter rows, the `queryChanges` row, and their two negative counterparts; `TestRequestMarshalRecomputesUsingForALateMutation` for a filter set on the method after `Invoke` already ran. `EmailQuery.Requires()` and `EmailQueryChanges.Requires()` both call `withSMIME(smimeFilter(m.Filter))`. |
| 4.2 | The three S/MIME conditions distinguish absent from explicitly false | MUST (derived, they are Booleans) | `TestRequestTags` observes all three as `*bool` set to false. `TestOptionalBooleanStates` proves the three-state behaviour on `HasAttachment` only, but the mechanism is the same field type. |
| 4.3 | Client tolerates `smimeVerifiedAt` changing without the Email appearing in `Email/changes` | MUST (server) | N/A. Server duty with no client action. |

### Interop note: an unnamed or a named-but-unsupported capability both fail loudly

Verified live against Stalwart. A filter condition whose capability is missing
from `using` does not silently match every message; the server answers
`unsupportedFilter` (RFC 8620 section 5.5), which section 1.8 ties to the rule
that a server behaves as though it does not implement a capability the request
never named. That failure is scoped to the call that carried the condition.

Naming `urn:ietf:params:jmap:smimeverify` in `using` trades that scoped failure
for a request-level one, against a server that lacks the capability entirely.
Stalwart rejects the whole request with HTTP 400 and
`urn:ietf:params:jmap:error:notRequest` the moment the URI appears in `using` at
all, which ends every other call batched into the same request, not only the
S/MIME one. Both outcomes are loud; neither is a silent pass-through.

This matters to whichever pass builds a query grammar that can emit an S/MIME
filter: batching an S/MIME condition alongside unrelated calls means a server
without the capability takes the whole request down with it, not just the one
filter.

---

## RFC 6901, JSON Pointer

Scope: the back-reference resolver in `resolve.go` and the patch-pointer builder
in `patch.go`.

| § | Obligation | Strength | Status |
|---|---|---|---|
| 3 | `~` is encoded as `~0` and `/` as `~1` when building a reference token | MUST | `TestPatchLeaf`, the escape-a-slash and escape-a-tilde rows. |
| 4 | Unescaping transforms `~1` first and then `~0`, so `~01` becomes `~1` and not `/` | MUST | GAP. `strings.NewReplacer` is single-pass so the hazard cannot arise, and the code comment claims the ordering is load-bearing. `TestResolveUnescapesPointerTokens` covers `~1` and `~0` separately and never `~01`. One table row closes it. |
| 4 | An array reference token is digits without a leading zero | MUST | `TestResolveFailsLoudly` rows for a leading zero, a plus sign, a minus sign, and three integer-overflow widths. `arrayIndex` carries a comment explaining why both the digit scan and `strconv.Atoi` are load-bearing. |
| 4 | The token `-` names the position past the end and always raises an error | MUST (derived) | partial. `TestPatchRefusesIllegalPointer` covers `-` on the build side. `TestResolveFailsLoudly` has no `-` row, so the resolve side is untested. |
| 4 | Object member lookup is byte-by-byte equal with no Unicode normalisation | MUST | Go map lookup is byte equality. Satisfied by the language, untested. |
| 4 | Evaluation raises an error when a token fails to resolve to a concrete value | MUST | `TestResolveFailsLoudly`, every row, asserting a nil value alongside the error. |
| 5 | A pointer represented in a JSON string escapes `"`, `\`, and control characters | MUST | Handled by `encoding/json`. Untested. |
| 7 | The application specifies the impact and handling of each error type | SHOULD | RFC 8620 section 3.7 specifies it as `invalidResultReference`. `TestResolveFailsLoudly` asserts every failure carries that type. |

---

## WHATWG HTML, server-sent events (section 9.2)

Scope: `eventstream.go` and the connection loop in `eventsource.go`. RFC 8620
section 7.3 defers to this specification for stream framing and reconnection.

| § | Obligation | Strength | Status |
|---|---|---|---|
| 9.2.3 | On a response whose status is not 200, or whose Content-Type is not `text/event-stream`, the connection fails rather than being read | MUST | partial. `TestListenStopsOnAServerRefusal` covers a 401, a 503, and a 200 carrying HTML, each ending the call after exactly one attempt. GAP: `refusal` accepts any 2xx, so a 204 with a stream content type would be read as an open connection. |
| 9.2.3 | On a network error the connection is reestablished | MUST | `TestListenResumesFromTheLastEventID`, `TestListenKeepsTheResumeIDAcrossAConnection`, both of which drive several reconnects. |
| 9.2.3 | The client waits the reconnection time before reestablishing, and may add exponential backoff | MUST / MAY | `TestReconnectBackoffSchedule` including the uptime reset, `TestJitterStaysUnderItsBound`, `TestConnectTimesTheOpenStream` for what the schedule measures. The reconnection time is poplar's own, per the `retry` row below. |
| 9.2.4 | A reconnect carries `Last-Event-ID` when the last event ID string is not empty, and omits it when it is | MUST | `TestListenResumesFromTheLastEventID` (JT-22), `TestListenExpandsTheURLTemplate` asserting the first connection sends none. |
| 9.2.5 | The stream is decoded as UTF-8 with one leading byte order mark stripped | MUST | `TestEventReaderFraming`, three BOM rows covering the first line, a second mark, and a mark on a later line. GAP: invalid UTF-8 is passed through rather than replaced, which is a tolerant reading and untested. |
| 9.2.5 | Lines end with CRLF, a bare LF, or a bare CR | MUST | `TestEventReaderFraming` CRLF and bare-CR rows, `TestEventReaderJoinsALineSplitAcrossReads` for a CR landing on a read boundary. |
| 9.2.6 | A blank line dispatches the event | MUST | `TestEventReaderFraming`, the dispatch row and the no-blank-line row. |
| 9.2.6 | A line starting with a colon is ignored | MUST | `TestEventReaderFraming`, the comment row. |
| 9.2.6 | A line with a colon splits at the first one, and one leading space is removed from the value | MUST | `TestEventReaderFraming`, the one-space-stripped-second-kept row and the no-space row. |
| 9.2.6 | A line with no colon is the whole line as field name with an empty value | MUST | `TestEventReaderFraming`, the no-colon row. |
| 9.2.6 | Field names are compared literally with no case folding | MUST | Satisfied by the `switch` on the exact strings. No row sends `Data:` or `EVENT:`. |
| 9.2.6 | `event` sets the event type buffer | MUST | `TestEventReaderFraming`, the named-event and two-events rows. |
| 9.2.6 | `data` appends the value and a line feed to the data buffer | MUST | `TestEventReaderFraming`, the multi-line row. |
| 9.2.6 | `id` sets the last event ID buffer unless the value holds a NUL, in which case the field is ignored | MUST | `TestEventReaderFraming`, the id-retained and NUL rows. |
| 9.2.6 | `retry`, when all ASCII digits, sets the reconnection time | MUST | GAP, deliberate. `eventstream.go` documents the refusal and the reason: a server naming an hour would take push down for an hour with no way to disagree. `TestEventReaderFraming`'s retry row pins the divergence. This is a knowing departure from a MUST, and it is the only one in the package that is written down. |
| 9.2.6 | Any other field is ignored | MUST | `TestEventReaderFraming`, the unknown-field row. |
| 9.2.6 | On dispatch, the last event ID string is set from the buffer first, and the buffer is not reset | MUST | `TestEventReaderFraming` id-retained, id-with-no-data, and never-dispatched rows; `TestListenKeepsTheResumeIDAcrossAConnection` across a connection that dispatched nothing. |
| 9.2.6 | An empty data buffer resets the buffers and dispatches nothing | MUST | `TestEventReaderFraming`, the event-field-with-no-data and blank-line rows. |
| 9.2.6 | A single trailing line feed is removed from the data buffer before dispatch | MUST | `TestEventReaderFraming`, the multi-line row asserting `first\nsecond`. |
| 9.2.6 | An event with no `event` field has type `message` | MUST | `TestEventReaderFraming`, the absent-event-field row. |
| 9.2.6 | At end of stream, pending data is discarded and an incomplete event is not dispatched | MUST | `TestEventReaderFraming` two discard rows, `TestEventReaderSurfacesAReadError`. |
| n/a | A line or an event larger than the reader's buffer is an error, never a silent truncation | n/a (implementation, JT-24) | `TestEventReaderCarriesALargeEvent` at 128 KB, `TestEventReaderRefusesAnOversizedEvent` for both the one-line and the accumulating case. |
