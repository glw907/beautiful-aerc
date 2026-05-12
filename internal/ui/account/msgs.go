package account

import (
	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// ErrorMsg aliases uicore.ErrorMsg so consumer case arms read without the
// uicore prefix. Cmds in this package still emit uicore.ErrorMsg directly.
type ErrorMsg = uicore.ErrorMsg

// foldersLoadedMsg carries the result of an initial sync + ListFolders
// call. Names arrive already classified to canonical display names.
type foldersLoadedMsg struct {
	classified []mail.ClassifiedFolder
}

// FolderLoadedMsg carries headers for the active folder from the cache.
// App uses it to commit a pending toast on a fresh load. Account routes
// it into the message list.
type FolderLoadedMsg struct {
	Name  string
	Msgs  []mail.MessageInfo
	Total int
}

type folderAppendedMsg struct {
	name  string
	msgs  []mail.MessageInfo
	total int
}

// CacheEventMsg wraps a cache.Event for tea routing. Account refreshes
// the active folder on it. App refreshes the outbox depth segment and any
// open outbox or conflict overlay.
type CacheEventMsg struct{ Event cache.Event }

// TriageStartedMsg is emitted after an optimistic triage flip. App sets
// the toast and schedules the undo timer. Inverse is the sole undo Cmd
// (a compensating QueueOp) since the cache owns the optimistic state.
type TriageStartedMsg struct {
	Op      uicore.TriageOp
	N       int
	Dest    string
	UIDs    []mail.UID
	Inverse tea.Cmd
}

type emptyFolderDoneMsg struct {
	folder string
	source string
	n      int
}

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

// JumpFolderMsg asks the account model to select the named canonical folder
// and fire the load Cmd, exactly like J/K navigation. Used by App to restore
// the previous folder after leaving the outbox view.
type JumpFolderMsg struct{ Canonical string }

// searchResultsMsg carries cross-folder search hits back to AccountTab.
// AccountTab drops them into the messagelist via SetSearchResults; the
// origin folder rides on each hit.
type searchResultsMsg struct {
	Hits []cache.SearchHit
}
