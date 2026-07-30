# What this package leaves out

One row per method or package deliberately not modelled in
`poplar/jmap`. The list travels with the package, so a reader can see
the surface boundary without reading poplar's own plan documents.

**One cost applies to every row and is worth stating once.** The
method registry in `method.go` is a build-time table, and an
unregistered method name in a response fails the **whole** `Response`
decode, not that one invocation. A trimmed method is therefore not
merely unavailable: a batch that contains one is undecodable.
Reinstating a row is adding a type plus a registry line, which is
small. Discovering the need in production is not.

| Trimmed | Why it is out | What brings it back |
|---|---|---|
| `Thread/get`, `Thread/changes` | poplar reads `Email.threadId` and never groups messages itself, so nothing needed the Thread object. RFC 8621 section 3 makes the grouping a SHOULD, so there is nothing to verify against. | A thread-view screen that wants the server's own message list for a conversation, rather than a query filtered by `threadId`. Also any batch a server answers with a `Thread/get`, which today fails the whole response decode. |
| `Email/copy` | poplar moves messages between mailboxes within one account, which is a `mailboxIds` patch, not a copy. Cross-account copy has no user-facing feature. | Multi-account support with a "copy to other account" action. |
| `Email/parse` | poplar parses MIME itself in `internal/mail`, which it must do for local drafts anyway. | Rendering a `message/rfc822` attachment as a message without downloading and parsing the blob. |
| `EmailSubmission/get`, `EmailSubmission/changes` | the outbox tracks its own sends and hears the outcome through `Email/changes` on the Sent mailbox. | A delivery-status view that shows per-recipient `deliveryStatus`, or an undo-send flow that has to read `undoStatus` back. |
| `Identity/set`, `Identity/changes` | poplar reads identities and never edits them; Fastmail's identities are managed in Fastmail. | An identity editor, or a signature-editing feature. |
| `Blob/copy` | **no longer trimmed.** It was unowned between briefs and landed with the second-server validation, in `blob.go`. | — |
| `SearchSnippet/get` | poplar's search highlights locally out of the body it already holds. | Server-side search over messages whose bodies are not in the local store. |
| `VacationResponse/get`, `VacationResponse/set` | out of scope for v1. go-jmap's own implementation carries a confirmed defect here (`MarshalJson`), so it is a rewrite rather than a port. | A vacation-responder setting. |
| `MDN/send`, `MDN/parse` | read receipts are a v1 non-goal. go-jmap's implementation also carries a confirmed tag defect (`reportinUA`). | Read-receipt support, which would need a policy decision before a type. |
| `PushSubscription/get`, `/set` (RFC 8620 section 7.2) | poplar is a terminal client with no public URL for a server to post to. EventSource covers the same ground and needs no registration. | A daemon mode with a reachable endpoint, or a server that offers push and no event source. |
| `Quota/*`, `principals`, `calendars`, `contacts`, `filenode`, `sieve`, `websocket` (RFC 8887) | Stalwart advertises sixteen capability URIs and this package models four. Everything else survives untyped in `Session.RawCapabilities`, which costs nothing. | The pass that needs the feature. Calendars and contacts have their own transports in poplar today. |

`Email/queryChanges` is not in this list. It was trimmed, found to be
needed for the query-changes splice, and reinstated. It is the
precedent that made the list worth keeping.
