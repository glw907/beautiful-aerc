// Package jmap implements the JMAP wire protocol: the data model of
// RFC 8620 (JMAP) and RFC 8621 (JMAP for Mail), the request and
// response envelope, back-references, and the typed capability
// objects a client reads out of a session.
//
// [Client] carries those types over HTTP: the session resource, the
// method calls, and the blob endpoints beside them. Everything else in
// the package is types and pure functions.
//
// The package reads no file and logs nothing, and it neither retries
// nor backs off. Every failure comes back as an error, and the caller
// decides what it means and whether to ask again.
//
// # Optional Booleans
//
// A Boolean property whose absence differs from false is typed *bool,
// which new(false) and new(true) build. A [Comparator] with no
// IsAscending sorts ascending, RFC 8620 section 5.5's default, while
// one holding new(false) reverses. An [EmailFilterCondition] with no
// HasAttachment constrains nothing, while one holding new(false)
// matches only messages carrying no attachment.
//
// # What a downstream cannot extend
//
// Two seams are closed on purpose, and both matter to a project
// importing this package for a server with vendor extensions. The
// method registry is a build-time table with no registration call, so
// one unmodelled method name in a response fails the whole [Response]
// decode rather than that one entry. [Filter] seals its
// implementations, so a vendor filter condition cannot be expressed,
// nested in a [FilterOperator] or otherwise. Opening either is the
// work of the task that needs it.
//
// # Provenance
//
// The data model derives from go-jmap v0.5.3, which is MIT licensed.
// THIRD-PARTY-NOTICES.md alongside this file carries the notice.
package jmap
