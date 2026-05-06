// SPDX-License-Identifier: MIT

package reader

import (
	"github.com/glw907/poplar/internal/content"
	"github.com/glw907/poplar/internal/mail"
)

// BodyLoadedMsg carries the parsed-block representation of a fetched
// message body. AccountTab compares uid against the viewer's current
// UID and drops mismatches (user closed and reopened on a different
// UID before the Cmd resolved).
type BodyLoadedMsg struct {
	UID    mail.UID
	Blocks []content.Block
}

// AttachmentsLoadedMsg carries metadata for the viewer's current UID.
// Stale UIDs are dropped at the AccountTab boundary like BodyLoadedMsg.
type AttachmentsLoadedMsg struct {
	UID   mail.UID
	Items []mail.Attachment
}

// AttachmentSavedMsg carries the final filesystem path of a saved attachment.
type AttachmentSavedMsg struct {
	Path string
}

// OpenLinkPickerMsg requests App open the link picker with the given
// harvested URLs. Emitted by Model when the user presses Tab on a
// message that has at least one harvested link.
type OpenLinkPickerMsg struct {
	Links []string
}

// LinkPickerClosedMsg signals the picker has closed (Esc, Tab, Enter,
// or numeric launch). Handled at the App level to flip linkPicker.open.
type LinkPickerClosedMsg struct{}

// LaunchURLMsg requests App fire launchURLCmd for the given URL.
// Emitted by the link picker on Enter or 1-9 in-range.
type LaunchURLMsg struct {
	URL string
}

// OpenAttachPickerMsg requests App open the attachment picker.
type OpenAttachPickerMsg struct {
	UID   mail.UID
	Items []mail.Attachment
}

// AttachPickerClosedMsg signals the attachment picker has closed.
type AttachPickerClosedMsg struct{}

// OpenAttachmentMsg requests App open the attachment via xdg-open.
type OpenAttachmentMsg struct {
	UID mail.UID
	Att mail.Attachment
}

// SaveAttachmentMsg requests App save the attachment to download_dir.
type SaveAttachmentMsg struct {
	UID mail.UID
	Att mail.Attachment
}
