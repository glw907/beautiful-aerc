// SPDX-License-Identifier: MIT

// Package mail defines poplar's mail backend interface and types.
package mail

import (
	"fmt"
	"strings"
)

// UID is a message unique identifier.
type UID string

// Flag represents email message flags.
type Flag uint32

const (
	FlagSeen Flag = 1 << iota
	_              // RFC 3501 \Recent is server-set, never client-passed; bit retained for cache compatibility
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

// Disposition classifies a MIME part as a user-visible attachment or
// an inline body fragment. Source of truth is the Content-Disposition
// header (JMAP `disposition`, IMAP body-structure dispositional ext);
// when missing, ContentID != "" implies DispInline.
type Disposition uint8

const (
	DispAttachment Disposition = iota
	DispInline
)

// String returns the canonical lowercase token. The string form is
// what the cache stores in attachments.disposition and what backends
// emit on the wire.
func (d Disposition) String() string {
	switch d {
	case DispInline:
		return "inline"
	default:
		return "attachment"
	}
}

// ParseDisposition is the inverse of String. Empty / unknown input
// returns an error so callers can apply their own fallback.
func ParseDisposition(s string) (Disposition, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "attachment":
		return DispAttachment, nil
	case "inline":
		return DispInline, nil
	default:
		return 0, fmt.Errorf("unknown disposition %q", s)
	}
}

// Attachment carries metadata for a non-body MIME part. PartID is
// protocol-native (JMAP partId, IMAP section "2", "2.1") and is
// opaque to consumers — pass it back to FetchAttachment unchanged.
type Attachment struct {
	PartID      string
	Filename    string
	MIMEType    string
	Size        uint32
	ContentID   string
	Disposition Disposition
}
