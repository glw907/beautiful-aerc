package compose

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"mime"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	gomail "github.com/emersion/go-message/mail"

	"github.com/glw907/poplar/internal/filter"
)

// AssembleMIME renders d into an RFC 5322 message. The body is
// multipart/alternative with text/plain (markdown verbatim) and
// text/html (goldmark render). With attachments, the alternative
// is wrapped in multipart/mixed and each attachment is a sibling
// part. now stamps the Date header and Message-Id suffix.
func AssembleMIME(d Draft, now time.Time) ([]byte, error) {
	from := d.From
	if from.Address == "" {
		return nil, fmt.Errorf("compose: From address required")
	}

	htmlBody, err := filter.MarkdownToHTML([]byte(d.Body))
	if err != nil {
		return nil, fmt.Errorf("compose: render html: %w", err)
	}

	var buf bytes.Buffer

	h := gomail.Header{}
	h.SetDate(now)
	h.SetAddressList("From", []*gomail.Address{&from})
	if len(d.To) > 0 {
		h.SetAddressList("To", addrPtrs(d.To))
	}
	if len(d.Cc) > 0 {
		h.SetAddressList("Cc", addrPtrs(d.Cc))
	}
	if len(d.Bcc) > 0 {
		h.SetAddressList("Bcc", addrPtrs(d.Bcc))
	}
	h.SetSubject(d.Subject)
	h.Set("Message-Id", newMessageID(from.Address, now))
	if d.InReplyTo != "" {
		h.Set("In-Reply-To", angleAddr(d.InReplyTo))
	}
	if len(d.References) > 0 {
		refs := make([]string, len(d.References))
		for i, r := range d.References {
			refs[i] = angleAddr(r)
		}
		h.Set("References", strings.Join(refs, " "))
	}
	h.Set("MIME-Version", "1.0")
	h.Set("User-Agent", "poplar")

	mw, err := gomail.CreateWriter(&buf, h)
	if err != nil {
		return nil, fmt.Errorf("compose: create writer: %w", err)
	}

	if err := writeAlternative(mw, d.Body, htmlBody); err != nil {
		return nil, err
	}
	for _, path := range d.Attachments {
		if err := writeAttachment(mw, path); err != nil {
			return nil, err
		}
	}

	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("compose: close writer: %w", err)
	}
	return buf.Bytes(), nil
}

func writeAlternative(mw *gomail.Writer, plain, html string) error {
	iw, err := mw.CreateInline()
	if err != nil {
		return fmt.Errorf("compose: create inline: %w", err)
	}

	plainHdr := gomail.InlineHeader{}
	plainHdr.SetContentType("text/plain", map[string]string{"charset": "utf-8"})
	pw, err := iw.CreatePart(plainHdr)
	if err != nil {
		return fmt.Errorf("compose: create plain part: %w", err)
	}
	if _, err := io.WriteString(pw, plain); err != nil {
		return fmt.Errorf("compose: write plain: %w", err)
	}
	if err := pw.Close(); err != nil {
		return fmt.Errorf("compose: close plain: %w", err)
	}

	htmlHdr := gomail.InlineHeader{}
	htmlHdr.SetContentType("text/html", map[string]string{"charset": "utf-8"})
	hw, err := iw.CreatePart(htmlHdr)
	if err != nil {
		return fmt.Errorf("compose: create html part: %w", err)
	}
	if _, err := io.WriteString(hw, html); err != nil {
		return fmt.Errorf("compose: write html: %w", err)
	}
	if err := hw.Close(); err != nil {
		return fmt.Errorf("compose: close html: %w", err)
	}

	if err := iw.Close(); err != nil {
		return fmt.Errorf("compose: close inline: %w", err)
	}
	return nil
}

func writeAttachment(mw *gomail.Writer, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("compose: read %s: %w", path, err)
	}
	name := filepath.Base(path)
	ctype := mime.TypeByExtension(filepath.Ext(name))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	media, params, err := mime.ParseMediaType(ctype)
	if err != nil {
		media, params = "application/octet-stream", nil
	}

	ah := gomail.AttachmentHeader{}
	ah.SetContentType(media, params)
	ah.SetFilename(name)

	aw, err := mw.CreateAttachment(ah)
	if err != nil {
		return fmt.Errorf("compose: create attachment %s: %w", name, err)
	}
	if _, err := aw.Write(data); err != nil {
		return fmt.Errorf("compose: write attachment %s: %w", name, err)
	}
	if err := aw.Close(); err != nil {
		return fmt.Errorf("compose: close attachment %s: %w", name, err)
	}
	return nil
}

// newMessageID returns a "<hex-random.unix-nano@host>" Message-Id.
// The random suffix avoids same-instant collisions across processes.
func newMessageID(fromAddr string, now time.Time) string {
	host := "localhost"
	if at := strings.LastIndex(fromAddr, "@"); at >= 0 && at+1 < len(fromAddr) {
		host = fromAddr[at+1:]
	}
	var rb [8]byte
	_, _ = rand.Read(rb[:])
	return fmt.Sprintf("<%x.%d@%s>", rb[:], now.UnixNano(), host)
}

// angleAddr wraps s in <...> if it isn't already. Idempotent.
func angleAddr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		return s
	}
	return "<" + s + ">"
}

func addrPtrs(addrs []gomail.Address) []*gomail.Address {
	out := make([]*gomail.Address, len(addrs))
	for i := range addrs {
		out[i] = &addrs[i]
	}
	return out
}

func parseHeaders(body []byte) (mail.Header, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return msg.Header, nil
}
