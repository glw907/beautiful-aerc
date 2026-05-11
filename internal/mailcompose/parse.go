package mailcompose

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	gomessage "github.com/emersion/go-message"
	gomail "github.com/emersion/go-message/mail"
)

// ParseDraftMIME reverses AssembleMIME for the fields Draft carries.
// HTML siblings are dropped and re-assembled from markdown on the
// next push. Attachments come back as filenames only. The outbox
// payload still carries the full bytes, so a local round-trip loses
// nothing the user can see in compose.
func ParseDraftMIME(raw []byte) (Draft, error) {
	mr, err := gomail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return parsePlain(raw)
	}
	defer mr.Close()

	var d Draft
	hdr := mr.Header
	if from, _ := hdr.AddressList("From"); len(from) > 0 {
		d.From = *from[0]
	}
	if to, _ := hdr.AddressList("To"); len(to) > 0 {
		for _, a := range to {
			d.To = append(d.To, *a)
		}
	}
	if cc, _ := hdr.AddressList("Cc"); len(cc) > 0 {
		for _, a := range cc {
			d.Cc = append(d.Cc, *a)
		}
	}
	if bcc, _ := hdr.AddressList("Bcc"); len(bcc) > 0 {
		for _, a := range bcc {
			d.Bcc = append(d.Bcc, *a)
		}
	}
	d.Subject, _ = hdr.Subject()
	if ids, err := hdr.MsgIDList("In-Reply-To"); err == nil && len(ids) > 0 {
		d.InReplyTo = ids[0]
	}
	if refs, err := hdr.MsgIDList("References"); err == nil {
		d.References = refs
	}

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return d, fmt.Errorf("read part: %w", err)
		}
		switch h := p.Header.(type) {
		case *gomail.InlineHeader:
			ct, _, _ := h.ContentType()
			if d.Body == "" && strings.EqualFold(ct, "text/plain") {
				body, err := io.ReadAll(p.Body)
				if err != nil {
					return d, fmt.Errorf("read body: %w", err)
				}
				// Draft body is bare-LF markdown. Strip MIME's CRLF.
				d.Body = strings.ReplaceAll(string(body), "\r\n", "\n")
			}
		case *gomail.AttachmentHeader:
			fn, _ := h.Filename()
			if fn != "" {
				d.Attachments = append(d.Attachments, fn)
			}
		}
	}
	return d, nil
}

// parsePlain reads a non-multipart message: headers-only or
// hand-typed drafts that never went through AssembleMIME.
func parsePlain(raw []byte) (Draft, error) {
	m, err := gomessage.Read(bytes.NewReader(raw))
	if err != nil {
		return Draft{}, fmt.Errorf("parse mime: %w", err)
	}
	var d Draft
	hdr := gomail.Header{Header: m.Header}
	if from, _ := hdr.AddressList("From"); len(from) > 0 {
		d.From = *from[0]
	}
	if to, _ := hdr.AddressList("To"); len(to) > 0 {
		for _, a := range to {
			d.To = append(d.To, *a)
		}
	}
	d.Subject, _ = hdr.Subject()
	body, err := io.ReadAll(m.Body)
	if err != nil {
		return d, fmt.Errorf("read body: %w", err)
	}
	d.Body = string(body)
	return d, nil
}
