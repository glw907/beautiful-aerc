package account

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// ErrorMsg aliases uicore.ErrorMsg so the case arms read without the
// uicore prefix. Producers (cmds in this package) emit uicore.ErrorMsg
// directly. The alias is for the consumer side.
type ErrorMsg = uicore.ErrorMsg

// foldersLoadedMsg carries the result of an initial sync + ListFolders
// call. The cache emits canonical display names already classified.
type foldersLoadedMsg struct {
	classified []mail.ClassifiedFolder
}

// FolderLoadedMsg replaces the previous open→query→fetch chain. The
// cache returns headers in one call. App reads it to commit a pending
// toast on a fresh folder load. Account routes it into the message
// list.
type FolderLoadedMsg struct {
	Name  string
	Msgs  []mail.MessageInfo
	Total int
}

// folderAppendedMsg is the load-more counterpart of folderLoadedMsg.
type folderAppendedMsg struct {
	name  string
	msgs  []mail.MessageInfo
	total int
}

// CacheEventMsg wraps a cache.CacheEvent for tea routing. Account
// consumes it to refresh the active folder. App consumes it to
// refresh the outbox depth segment and any open outbox or conflict
// overlay.
type CacheEventMsg struct{ Event cache.CacheEvent }

// TriageStartedMsg is emitted after an optimistic triage flip. App
// receives it, sets the toast, and schedules a tea.Tick for the undo
// timer. The cache owns the optimistic state, so Inverse is the sole
// undo Cmd, a compensating QueueOp.
type TriageStartedMsg struct {
	Op      uicore.TriageOp
	N       int
	Dest    string
	UIDs    []mail.UID
	Inverse tea.Cmd
}

// emptyFolderDoneMsg reports a successful manual empty.
type emptyFolderDoneMsg struct {
	folder string
	source string
	n      int
}

// sweepCompletedMsg reports a retention sweep's destroyed UIDs.
type sweepCompletedMsg struct {
	folder string
	uids   []mail.UID
}

// OpenConfirmEmptyMsg asks App to open the empty-folder confirm modal.
// Source is passed through so it can be handed to emptyFolderCmd later.
type OpenConfirmEmptyMsg struct {
	Folder string // display name shown in modal title and toast
	Total  int    // message count shown in modal body
	Source string // canonical folder name passed to QueueOp
}

// EmptyFolderConfirmedMsg signals the user pressed `y` in the confirm modal.
type EmptyFolderConfirmedMsg struct {
	Folder string
	Source string
}
