// SPDX-License-Identifier: MIT

package compose

// SeedKind selects which seeding function composeSeedCmd invokes.
type SeedKind int

const (
	SeedReply    SeedKind = iota
	SeedReplyAll SeedKind = iota
	SeedForward  SeedKind = iota
)
