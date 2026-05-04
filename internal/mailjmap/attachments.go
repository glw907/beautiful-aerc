// SPDX-License-Identifier: MIT

package mailjmap

import (
	"fmt"
	"strings"

	"git.sr.ht/~rockorager/go-jmap"
	jmapmail "git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"

	"github.com/glw907/poplar/internal/mail"
)

// attachmentProperties is the Email/get property set for attachment
// metadata. bodyStructure carries every part with disposition + cid;
// attachments is the server-precomputed non-body subset, useful as a
// hint but not authoritative for inline-vs-attachment classification.
var attachmentProperties = []string{"id", "bodyStructure", "attachments"}

// Attachments satisfies mail.Backend. Issues one Email/get with
// bodyStructure + attachments, walks the part tree, and returns the
// non-body parts. Side effect: populates b.partBlobIDs[uid].
func (b *Backend) Attachments(uid mail.UID) ([]mail.Attachment, error) {
	b.mu.Lock()
	accountID := b.session.PrimaryAccounts[jmapmail.URI]
	b.mu.Unlock()

	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	req.Invoke(&email.Get{
		Account:    accountID,
		IDs:        []jmap.ID{jmap.ID(uid)},
		Properties: attachmentProperties,
	})
	resp, err := b.do(req)
	if err != nil {
		return nil, fmt.Errorf("attachments %s: %w", uid, err)
	}

	for _, inv := range resp.Responses {
		gr, ok := inv.Args.(*email.GetResponse)
		if !ok || len(gr.List) == 0 {
			continue
		}
		e := gr.List[0]
		atts, partMap := walkBodyStructure(e.BodyStructure)
		b.mu.Lock()
		b.partBlobIDs[uid] = partMap
		b.mu.Unlock()
		return atts, nil
	}
	return nil, fmt.Errorf("attachments %s: no Email/get response", uid)
}

// walkBodyStructure flattens the JMAP body structure into a list of
// non-body parts, applying the spec classification rule. The returned
// map carries partID→blobID for every walked leaf so FetchAttachment
// can resolve without a second roundtrip.
func walkBodyStructure(bp *email.BodyPart) ([]mail.Attachment, map[string]string) {
	if bp == nil {
		return nil, map[string]string{}
	}
	var atts []mail.Attachment
	parts := map[string]string{}
	var walk func(p *email.BodyPart, isTopLevelBody bool)
	walk = func(p *email.BodyPart, isTopLevelBody bool) {
		if p == nil {
			return
		}
		if len(p.SubParts) > 0 {
			for _, sp := range p.SubParts {
				walk(sp, false)
			}
			return
		}
		mt := strings.ToLower(p.Type)
		// Skip the displayable body candidates at the top level.
		if isTopLevelBody && (mt == "text/plain" || mt == "text/html") {
			return
		}
		parts[p.PartID] = string(p.BlobID)
		atts = append(atts, mail.Attachment{
			PartID:      p.PartID,
			Filename:    p.Name,
			MIMEType:    mt,
			Size:        uint32(p.Size),
			ContentID:   strings.Trim(p.CID, "<>"),
			Disposition: classifyDisposition(p),
		})
	}
	if len(bp.SubParts) > 0 {
		for _, sp := range bp.SubParts {
			walk(sp, true)
		}
	} else {
		walk(bp, true)
	}
	return atts, parts
}

// classifyDisposition implements Q1: trust Content-Disposition; when
// missing, ContentID != "" → inline, else attachment.
func classifyDisposition(p *email.BodyPart) mail.Disposition {
	if d, err := mail.ParseDisposition(p.Disposition); err == nil {
		return d
	}
	if strings.TrimSpace(p.CID) != "" {
		return mail.DispInline
	}
	return mail.DispAttachment
}
