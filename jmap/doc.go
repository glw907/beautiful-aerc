// Package jmap implements the JMAP wire protocol: the data model of
// RFC 8620 (JMAP) and RFC 8621 (JMAP for Mail), the request and
// response envelope, back-references, and the typed capability
// objects a client reads out of a session.
//
// The package is types and pure functions. It opens no connection,
// reads no file, and logs nothing. Every failure comes back as an
// error, and the caller decides what it means.
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
// # Provenance
//
// The data model derives from go-jmap v0.5.3, which is MIT licensed.
// THIRD-PARTY-NOTICES.md alongside this file carries the notice.
package jmap
