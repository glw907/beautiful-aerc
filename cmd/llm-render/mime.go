package main

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"

	_ "github.com/emersion/go-message/charset"
	gomail "github.com/emersion/go-message/mail"
)

// bodyPart holds extracted email body content, its MIME type, and header
// fields needed for synthesis when the body is empty.
type bodyPart struct {
	content     []byte
	contentType string // "text/html" or "text/plain"
	subject     string
	fromDisplay string
}

// extractBody parses a raw .eml message and returns the first text/html part,
// falling back to text/plain. Returns an error when neither is present.
func extractBody(raw []byte) (bodyPart, error) {
	mr, err := gomail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return bodyPart{}, fmt.Errorf("parse message: %w", err)
	}
	defer mr.Close()

	h := mr.Header
	subject, _ := h.Subject()
	fromList, _ := h.AddressList("From")
	var fromDisplay string
	if len(fromList) > 0 {
		fromDisplay = fromList[0].Name
		if fromDisplay == "" {
			fromDisplay = fromList[0].Address
		}
	}

	var htmlContent, plainContent []byte
	for {
		p, perr := mr.NextPart()
		if perr == io.EOF {
			break
		}
		if perr != nil {
			slog.Warn("MIME part read", "err", perr)
			break
		}
		ih, ok := p.Header.(*gomail.InlineHeader)
		if !ok {
			io.Copy(io.Discard, p.Body)
			continue
		}
		ct, _, _ := ih.ContentType()
		body, _ := io.ReadAll(p.Body)
		switch ct {
		case "text/html":
			if htmlContent == nil {
				htmlContent = body
			}
		case "text/plain":
			if plainContent == nil {
				plainContent = body
			}
		}
	}

	switch {
	case htmlContent != nil:
		return bodyPart{content: htmlContent, contentType: "text/html", subject: subject, fromDisplay: fromDisplay}, nil
	case plainContent != nil:
		return bodyPart{content: plainContent, contentType: "text/plain", subject: subject, fromDisplay: fromDisplay}, nil
	default:
		return bodyPart{}, fmt.Errorf("no text part found")
	}
}
