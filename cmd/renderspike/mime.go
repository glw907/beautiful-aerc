package main

import (
	"bytes"
	"fmt"
	"io"

	_ "github.com/emersion/go-message/charset"
	gomail "github.com/emersion/go-message/mail"
)

// bodyPart holds extracted email body content and its MIME type.
type bodyPart struct {
	content     []byte
	contentType string // "text/html" or "text/plain"
}

// extractBody parses a raw .eml message and returns the first text/html part,
// falling back to text/plain. Returns an error when neither type is present.
func extractBody(raw []byte) (bodyPart, error) {
	mr, err := gomail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return bodyPart{}, fmt.Errorf("parse message: %w", err)
	}
	defer mr.Close()

	var htmlContent, plainContent []byte
	for {
		p, perr := mr.NextPart()
		if perr != nil {
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
		return bodyPart{content: htmlContent, contentType: "text/html"}, nil
	case plainContent != nil:
		return bodyPart{content: plainContent, contentType: "text/plain"}, nil
	default:
		return bodyPart{}, fmt.Errorf("no text part found")
	}
}
