package mailimap

import (
	"fmt"
	"strings"

	"github.com/glw907/poplar/internal/mail"
)

// Attachments issues UID FETCH BODYSTRUCTURE
// for uid, walks the part tree, and returns the non-body parts.
// Top-level text/plain and text/html parts are dropped (they are
// the displayable body, not attachments).
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

// walkBodyStructure flattens bs to leaves, applying the Q1
// classification rule. Direct children of the outermost multipart
// that are text/plain or text/html are skipped (displayable body).
// Parts nested inside inner multiparts are never suppressed.
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
// children of the outermost multipart. Inner multipart children
// always receive false.
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

// FetchAttachment issues UID FETCH BODY[<part>]
// and returns the decoded bytes. The transfer encoding (base64,
// quoted-printable) is decoded by the go-imap client adapter.
// Returns raw decoded bytes ready to write to disk.
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
