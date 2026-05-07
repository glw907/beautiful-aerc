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
// metadata. bodyStructure carries every part with disposition + cid,
// which is enough to classify inline-vs-attachment ourselves without
// the server's precomputed `attachments` subset.
var attachmentProperties = []string{"id", "bodyStructure"}

// Attachments issues one Email/get for
// bodyStructure, walks the part tree, and returns the non-body
// parts. Side effect: populates b.partBlobIDs[uid].
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
			Disposition: mail.ClassifyDisposition(p.Disposition, p.CID),
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

// FetchAttachment resolves (uid, partID) to
// a blobID via the cached partBlobIDs map (issuing one Email/get
// when the cache is cold), then downloads via downloadBlob.
func (b *Backend) FetchAttachment(uid mail.UID, partID string) ([]byte, error) {
	b.mu.Lock()
	parts := b.partBlobIDs[uid]
	dl := b.downloadBlob
	b.mu.Unlock()

	if parts == nil {
		// Cold map: populate via Attachments. Discard the metadata;
		// the caller already has it. The side-effect of populating
		// partBlobIDs is what we need.
		if _, err := b.Attachments(uid); err != nil {
			return nil, fmt.Errorf("fetch attachment %s/%s: prime: %w", uid, partID, err)
		}
		b.mu.Lock()
		parts = b.partBlobIDs[uid]
		b.mu.Unlock()
	}
	blobID, ok := parts[partID]
	if !ok {
		return nil, fmt.Errorf("fetch attachment %s/%s: unknown partID", uid, partID)
	}
	if dl == nil {
		return nil, fmt.Errorf("fetch attachment %s/%s: not connected", uid, partID)
	}
	body, err := dl(blobID)
	if err != nil {
		return nil, fmt.Errorf("fetch attachment %s/%s: download: %w", uid, partID, err)
	}
	return body, nil
}
