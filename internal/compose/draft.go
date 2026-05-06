// SPDX-License-Identifier: MIT

package compose

import gomail "github.com/emersion/go-message/mail"

// Mode tags how a Draft was originated. ComposeTab uses it to label
// the tab title and to drive seed dispatch. AssembleMIME ignores it.
type Mode int

const (
	ModeNew Mode = iota
	ModeReply
	ModeReplyAll
	ModeForward
)

// Draft is the full state of an outbound message. The body is raw
// CommonMark. AssembleMIME renders it to text/plain plus text/html.
// The reply-thread headers (InReplyTo, References) are RFC 5322
// message IDs, not poplar UIDs. Seed* parses them off the parent
// body.
//
// Attachments are filesystem paths. AssembleMIME reads the bytes and
// classifies the MIME type at assembly time. The cache outbox stores
// the assembled bytes rather than the paths, so a draft remains
// shippable even if the file moves between save and send.
type Draft struct {
	From    gomail.Address
	To      []gomail.Address
	Cc      []gomail.Address
	Bcc     []gomail.Address
	Subject string
	Body    string

	InReplyTo  string
	References []string

	Attachments []string
}
