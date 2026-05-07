package compose

import (
	"bytes"
	"io"
	"net/mail"
	"strings"

	gomail "github.com/emersion/go-message/mail"

	"github.com/glw907/poplar/internal/content"
	pmail "github.com/glw907/poplar/internal/mail"
)

// SeedReply builds a Draft replying to parent. body is the parent's
// raw RFC 5322 bytes, mined for Message-Id, References, and a
// quotable text/plain body.
func SeedReply(parent pmail.MessageInfo, body []byte) Draft {
	d := seedCommon(parent, body)
	d.Subject = ensurePrefix(parent.Subject, "Re:")
	if from := firstAddress(parent.From); from != nil {
		d.To = []gomail.Address{*from}
	}
	d.Body = quoteAttribution(parent, body)
	return d
}

// SeedReplyAll is SeedReply plus parent's To and Cc, minus self,
// added to Cc.
func SeedReplyAll(parent pmail.MessageInfo, body []byte, self gomail.Address) Draft {
	d := SeedReply(parent, body)
	cc := mergeAddresses(parent.To, parent.Cc)
	d.Cc = filterAddress(cc, self.Address)
	return d
}

// SeedForward builds a Draft forwarding parent. To and Cc are empty
// for the user to fill, and forwards start a fresh thread, so
// In-Reply-To and References stay empty.
func SeedForward(parent pmail.MessageInfo, body []byte) Draft {
	return Draft{
		Subject: ensurePrefix(parent.Subject, "Fwd:"),
		Body:    quoteAttribution(parent, body),
	}
}

func seedCommon(parent pmail.MessageInfo, body []byte) Draft {
	d := Draft{}
	hdr, err := parseHeaders(body)
	if err != nil {
		return d
	}
	if mid := stripAngles(hdr.Get("Message-Id")); mid != "" {
		d.InReplyTo = mid
	}
	refs := splitRefs(hdr.Get("References"))
	if d.InReplyTo != "" {
		refs = append(refs, d.InReplyTo)
	}
	d.References = refs
	return d
}

// quoteAttribution returns an empty row, an attribution line, and
// the parent body with one additional "> " level. The empty row
// puts the cursor above the attribution so reply text bottom-posts.
func quoteAttribution(parent pmail.MessageInfo, body []byte) string {
	plain := extractPlainBody(body)
	quoted := quoteLines(plain)
	attribution := "On " + parent.Date + ", " + parent.From + " wrote:"
	return "\n\n" + attribution + "\n" + quoted
}

// quoteLines prefixes every line with "> ", deepening any existing
// quote depth by one.
func quoteLines(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l == "" {
			lines[i] = ">"
		} else {
			lines[i] = "> " + l
		}
	}
	return strings.Join(lines, "\n")
}

// extractPlainBody returns the first text/plain inline part, or the
// raw body when go-message can't parse it as multipart.
func extractPlainBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	if mr, err := gomail.CreateReader(bytes.NewReader(body)); err == nil {
		defer mr.Close()
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
			ih, ok := p.Header.(*gomail.InlineHeader)
			if !ok {
				continue
			}
			ct, _, _ := ih.ContentType()
			if ct == "text/plain" {
				b, _ := io.ReadAll(p.Body)
				return string(b)
			}
		}
	}
	msg, err := mail.ReadMessage(bytes.NewReader(body))
	if err != nil {
		return string(body)
	}
	b, err := io.ReadAll(msg.Body)
	if err != nil {
		return ""
	}
	return string(b)
}

// ensurePrefix attaches prefix to subject, collapsing repeated runs
// case-insensitively. "Re: Re: foo" becomes "Re: foo".
func ensurePrefix(subject, prefix string) string {
	s := strings.TrimSpace(subject)
	for strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix)) {
		s = strings.TrimSpace(s[len(prefix):])
	}
	if s == "" {
		return prefix
	}
	return prefix + " " + s
}

func splitRefs(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	fields := strings.Fields(s)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if id := stripAngles(f); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func stripAngles(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "<")
	s = strings.TrimSuffix(s, ">")
	return strings.TrimSpace(s)
}

func firstAddress(display string) *gomail.Address {
	addrs := content.ParseAddressList(display)
	if len(addrs) == 0 {
		return nil
	}
	return &gomail.Address{Name: addrs[0].Name, Address: addrs[0].Email}
}

func mergeAddresses(displays ...string) []gomail.Address {
	var out []gomail.Address
	seen := map[string]bool{}
	for _, d := range displays {
		for _, a := range content.ParseAddressList(d) {
			key := strings.ToLower(a.Email)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, gomail.Address{Name: a.Name, Address: a.Email})
		}
	}
	return out
}

func filterAddress(addrs []gomail.Address, drop string) []gomail.Address {
	drop = strings.ToLower(strings.TrimSpace(drop))
	if drop == "" {
		return addrs
	}
	out := addrs[:0]
	for _, a := range addrs {
		if strings.ToLower(a.Address) == drop {
			continue
		}
		out = append(out, a)
	}
	return out
}
