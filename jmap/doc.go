// Package jmap implements the JMAP wire protocol: the data model of
// RFC 8620 (JMAP) and RFC 8621 (JMAP for Mail), the request and
// response envelope, back-references, and the typed capability
// objects a client reads out of a session.
//
// [Client] carries those types over HTTP: the session resource, the
// method calls, the blob endpoints beside them, and the push stream.
// Everything else in the package is types and pure functions.
//
// The package reads no file and logs nothing. A method call neither
// retries nor backs off: every failure comes back as an error, and the
// caller decides what it means and whether to ask again.
// [Client.Listen] is the one place a failure is retried here. The
// server-sent events standard makes reconnecting part of that
// protocol. Listen still decides nothing about when to give up or what
// to do while the stream is down.
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
// # What is not modelled
//
// The package covers what poplar sends, not everything the two RFCs
// define. TRIMMED.md alongside this file lists every method left out,
// why, and what would bring it back. Read it before assuming a method
// is missing by accident: a method with no registered response fails
// the whole [Response] decode, so the list is also the set of calls a
// batch must not contain.
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
// The data model derives from git.sr.ht/~rockorager/go-jmap v0.5.3
// (2025-02-01), which is MIT licensed and, as of this writing, remains
// the head of upstream's main branch. THIRD-PARTY-NOTICES.md alongside
// this file names exactly what was read from it and rewritten, and
// carries the license notice in full.
package jmap
