package header

import (
	"io"
	"net/mail"
	"regexp"
	"strings"
)

var rePrefixes = regexp.MustCompile(`(?i)^(Re|Fwd):\s*`)

// ExtractFrom reads an RFC 2822 message from r and returns the sender
// email address. Returns empty string if From header is missing.
func ExtractFrom(r io.Reader) string {
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return ""
	}
	addr, err := msg.Header.AddressList("From")
	if err != nil || len(addr) == 0 {
		return ""
	}
	return addr[0].Address
}

// ExtractTo reads an RFC 2822 message from r and returns all recipient
// email addresses from the To and Cc headers combined (To first, then Cc).
// Returns nil if neither header is present.
func ExtractTo(r io.Reader) []string {
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return nil
	}

	var addrs []string
	for _, hdr := range []string{"To", "Cc"} {
		list, err := msg.Header.AddressList(hdr)
		if err != nil {
			continue
		}
		for _, a := range list {
			addrs = append(addrs, a.Address)
		}
	}
	return addrs
}

// ExtractSubject reads an RFC 2822 message from r and returns the
// subject line with Re:/Fwd: prefixes stripped.
func ExtractSubject(r io.Reader) string {
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return ""
	}
	subj := msg.Header.Get("Subject")
	for rePrefixes.MatchString(subj) {
		subj = rePrefixes.ReplaceAllString(subj, "")
	}
	return strings.TrimSpace(subj)
}
