// SPDX-License-Identifier: MIT

package compose

import mailcompose "github.com/glw907/poplar/internal/compose"

// SeededMsg carries a pre-filled Draft from r/R/f. App opens
// Model and calls Seed when this msg arrives.
type SeededMsg struct {
	Draft mailcompose.Draft
}

// SentMsg fires after QueueOutbound returns. App stages a
// non-undoable "Sending…" toast.
type SentMsg struct{}
