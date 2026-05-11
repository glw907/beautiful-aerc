package mailcompose

import (
	"errors"
	"net/url"
	"strings"

	gomail "github.com/emersion/go-message/mail"
)

// SeedFromMailto parses a mailto: URL and returns a Draft seeded with
// the first recipient and any subject/body query parameters. Multiple
// addresses are tolerated (first wins, RFC 6068 reading). fromEmail
// populates the From field.
func SeedFromMailto(raw, fromEmail string) (Draft, error) {
	if !strings.HasPrefix(strings.ToLower(raw), "mailto:") {
		return Draft{}, errors.New("not a mailto URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return Draft{}, err
	}
	if u.Opaque == "" {
		return Draft{}, errors.New("mailto URL missing recipient")
	}
	addrs := strings.Split(u.Opaque, ",")
	first := strings.TrimSpace(addrs[0])
	if first == "" {
		return Draft{}, errors.New("mailto URL missing recipient")
	}
	d := Draft{
		From: gomail.Address{Address: fromEmail},
		To:   []gomail.Address{{Address: first}},
	}
	q := u.Query()
	d.Subject = q.Get("subject")
	d.Body = q.Get("body")
	return d, nil
}
