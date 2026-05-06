// SPDX-License-Identifier: MIT

package sidebar

// ClearSearchMsg tells AccountTab to clear an active sidebar search and
// return to SearchIdle. Fired when the user navigates to a different folder
// (folder jumps commit the current search).
type ClearSearchMsg struct{}
