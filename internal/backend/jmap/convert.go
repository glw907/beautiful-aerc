package jmap

// toInt narrows a JMAP uint64 counter (a server limit, a mailbox
// total) to int. Every such counter this package reads comes from a
// real JMAP server and fits comfortably within int's range; there is
// no untrusted input path where it could overflow.
//
//nolint:gosec // G115: bounded by real JMAP server values, see doc comment
func toInt(u uint64) int { return int(u) }

// toInt64 narrows a JMAP uint64 counter to int64, poplar's field
// vocabulary convention for a signed count. See toInt.
//
//nolint:gosec // G115: bounded by real JMAP server values, see toInt
func toInt64(u uint64) int64 { return int64(u) }

// jmapLimit widens a caller-supplied page limit to JMAP's uint64
// MaxChanges/Limit fields. A negative limit is a caller bug, not
// untrusted input.
//
//nolint:gosec // G115: limit is caller-controlled and non-negative by convention
func jmapLimit(limit int) uint64 { return uint64(limit) }
