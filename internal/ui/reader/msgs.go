package reader

import (
	"github.com/glw907/poplar/internal/content"
	"github.com/glw907/poplar/internal/mail"
)

// BodyLoadedMsg carries the parsed-block body and any List-Unsubscribe
// headers harvested from the raw RFC 5322 envelope. AccountTab drops
// mismatches against the viewer's current UID (user closed and reopened
// on a different UID before the Cmd resolved).
type BodyLoadedMsg struct {
	UID    mail.UID
	Blocks []content.Block
	Unsub  content.Unsubscribe
}

// AttachmentsLoadedMsg carries metadata for the viewer's current UID.
// Stale UIDs are dropped at the AccountTab boundary.
type AttachmentsLoadedMsg struct {
	UID   mail.UID
	Items []mail.Attachment
}

// AttachmentSavedMsg carries the final filesystem path of a saved attachment.
type AttachmentSavedMsg struct {
	Path string
}

// OpenLinkPickerMsg asks App to open the link picker with harvested URLs.
// Emitted on Tab when the message has at least one harvested link.
type OpenLinkPickerMsg struct {
	Links []string
}

// LinkPickerClosedMsg signals Esc, Tab, Enter, or a numeric launch closed
// the picker. App flips linkPicker.open.
type LinkPickerClosedMsg struct{}

// LaunchURLMsg asks App to fire launchURLCmd for the given URL. Emitted by
// the link picker on Enter or 1-9 in-range.
type LaunchURLMsg struct {
	URL string
}

// OpenAttachPickerMsg asks App to open the attachment picker.
type OpenAttachPickerMsg struct {
	UID   mail.UID
	Items []mail.Attachment
}

type AttachPickerClosedMsg struct{}

// OpenAttachmentMsg asks App to open the attachment via xdg-open.
type OpenAttachmentMsg struct {
	UID mail.UID
	Att mail.Attachment
}

// SaveAttachmentMsg asks App to save the attachment to download_dir.
type SaveAttachmentMsg struct {
	UID mail.UID
	Att mail.Attachment
}
