// Package mail defines poplar's mail backend interface and types.
package mail

// UID is a message unique identifier.
type UID string

// Flag represents email message flags.
type Flag uint32

const (
	FlagSeen Flag = 1 << iota
	FlagRecent
	FlagAnswered
	FlagForwarded
	FlagDeleted
	FlagFlagged
	FlagDraft
)

// Folder represents a mail folder with summary counts.
type Folder struct {
	Name   string
	Exists int
	Unseen int
	Role   string
}

// UpdateType classifies asynchronous backend updates.
type UpdateType int

const (
	UpdateNewMail UpdateType = iota
	UpdateFlagsChanged
	UpdateExpunge
	UpdateFolderInfo
	UpdateConnState
)

// ConnState classifies the backend's transport state. Carried on
// Update.ConnState only when Type == UpdateConnState.
type ConnState int

const (
	ConnOffline ConnState = iota
	ConnReconnecting
	ConnConnected
)

// Update represents an asynchronous update from the backend. The
// ConnState field is populated only for UpdateConnState; for every
// other Type it is the zero value (ConnOffline) and ignored.
type Update struct {
	Type      UpdateType
	Folder    string
	UIDs      []UID
	ConnState ConnState
}
