package compose

import gomail "github.com/emersion/go-message/mail"

// Mode tags how a Draft was originated. compose.Model uses it to
// pick the tab title and seed dispatch. AssembleMIME ignores it.
type Mode int

const (
	ModeNew Mode = iota
	ModeReply
	ModeReplyAll
	ModeForward
)

// Draft is the full state of an outbound message. Body is raw
// CommonMark, which AssembleMIME renders to text/plain plus
// text/html. InReplyTo and References are RFC 5322 message IDs, not
// poplar UIDs. Seed* parses them off the parent body. Attachments
// are filesystem paths read at assembly time. The cache outbox
// stores the assembled bytes, so a draft is shippable even if the
// file moves between save and send.
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
