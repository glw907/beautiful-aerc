package mailimap

import (
	"fmt"
	"strings"

	"github.com/glw907/poplar/internal/mail"
)

// Attachments runs UID FETCH BODYSTRUCTURE on uid and walks the part
// tree. Top-level text/plain and text/html parts are dropped because
// they are the displayable body.
func (b *Backend) Attachments(uid mail.UID) ([]mail.Attachment, error) {
	b.mu.Lock()
	cmd := b.cmd
	b.mu.Unlock()

	bs, err := cmd.FetchBodyStructure(uid)
	if err != nil {
		return nil, fmt.Errorf("attachments %s: %w", uid, err)
	}
	return walkBodyStructure(bs), nil
}

// walkBodyStructure flattens bs to leaves. Direct children of the
// outermost multipart that are text/plain or text/html are dropped
// as displayable body. Parts nested deeper are never suppressed.
func walkBodyStructure(bs BodyStructure) []mail.Attachment {
	if len(bs.Children) > 0 {
		var out []mail.Attachment
		for _, c := range bs.Children {
			out = append(out, walkPart(c, true)...)
		}
		return out
	}
	return walkPart(bs, true)
}

// walkPart processes one part. isTopLevel is true only for direct
// children of the outermost multipart.
func walkPart(bs BodyStructure, isTopLevel bool) []mail.Attachment {
	if len(bs.Children) > 0 {
		var out []mail.Attachment
		for _, c := range bs.Children {
			out = append(out, walkPart(c, false)...)
		}
		return out
	}
	mt := strings.ToLower(bs.MIMEType)
	if isTopLevel && (mt == "text/plain" || mt == "text/html") {
		return nil
	}
	return []mail.Attachment{{
		PartID:      bs.Section,
		Filename:    bs.Filename,
		MIMEType:    mt,
		Size:        bs.SizeBytes,
		ContentID:   strings.Trim(bs.ContentID, "<>"),
		Disposition: mail.ClassifyDisposition(bs.Disposition, bs.ContentID),
	}}
}

// FetchAttachment runs UID FETCH BODY[partID] and returns the
// decoded bytes. The go-imap adapter handles base64 and
// quoted-printable transfer encoding.
func (b *Backend) FetchAttachment(uid mail.UID, partID string) ([]byte, error) {
	b.mu.Lock()
	cmd := b.cmd
	b.mu.Unlock()

	body, err := cmd.FetchBodyPart(uid, partID)
	if err != nil {
		return nil, fmt.Errorf("fetch attachment %s/%s: %w", uid, partID, err)
	}
	return body, nil
}
