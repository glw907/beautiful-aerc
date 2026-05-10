package content

import (
	"bytes"
	"io"
	"strings"

	gomail "github.com/emersion/go-message/mail"
	"github.com/glw907/poplar/internal/filter"
)

// ExtractPlainText returns a plain-text projection of the message
// body suitable for full-text indexing. The first text/plain part
// wins; absent that, the first text/html falls through CleanHTML.
// Non-RFC822 input passes through unchanged. Backend fixtures and
// other pre-cleaned bytes flow straight to the caller.
//
// Returns "" with nil error when the message has no text body
// (e.g., calendar-only invites). A parse failure on RFC822 framing
// returns the raw bytes as a string with nil error; treating that
// as "indexable text" is the right call for FTS5. A malformed MIME
// part is still searchable on whatever readable runes it carries.
func ExtractPlainText(buf []byte) (string, error) {
	if !IsRFC822Frame(buf) {
		return string(buf), nil
	}
	mr, err := gomail.CreateReader(bytes.NewReader(buf))
	if err != nil {
		return string(buf), nil
	}
	var plain, html string
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
		body, rerr := io.ReadAll(p.Body)
		if rerr != nil {
			continue
		}
		switch ct {
		case "text/plain":
			if plain == "" {
				plain = string(body)
			}
		case "text/html":
			if html == "" {
				html = string(body)
			}
		}
	}
	mr.Close()
	switch {
	case plain != "":
		return filter.CleanPlain(plain), nil
	case html != "":
		return filter.CleanHTML(html), nil
	default:
		return "", nil
	}
}

// IsRFC822Frame sniffs buf for an RFC 5322 header line (a Field-Name:
// before the first newline). Used to gate MIME walks against
// pre-cleaned bytes from fixtures.
func IsRFC822Frame(b []byte) bool {
	s := string(b)
	if i := strings.IndexByte(s, '\n'); i > 0 {
		s = s[:i]
	}
	colon := strings.IndexByte(s, ':')
	if colon <= 0 || colon > 78 {
		return false
	}
	for _, r := range s[:colon] {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
