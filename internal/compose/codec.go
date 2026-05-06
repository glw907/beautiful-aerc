// SPDX-License-Identifier: MIT

package compose

import (
	"bytes"
	"encoding/gob"

	gomail "github.com/emersion/go-message/mail"
)

// gobDraft is the gob-serializable shape of Draft. gomail.Address
// is a type alias for net/mail.Address (two exported string fields)
// and encodes cleanly with gob; json.Marshal mishandles non-ASCII
// Name values — gob avoids that roundtrip failure.
//
// Parallel name/addr slices avoid the gobAddress mirror type that
// would otherwise be required to satisfy gob's exported-field rule
// for aliased types across packages.
type gobDraft struct {
	FromName    string
	FromAddr    string
	ToNames     []string
	ToAddrs     []string
	CcNames     []string
	CcAddrs     []string
	BccNames    []string
	BccAddrs    []string
	Subject     string
	Body        string
	InReplyTo   string
	References  []string
	Attachments []string
}

func splitAddrs(addrs []gomail.Address) (names, addrsOut []string) {
	if len(addrs) == 0 {
		return nil, nil
	}
	names = make([]string, len(addrs))
	addrsOut = make([]string, len(addrs))
	for i, a := range addrs {
		names[i] = a.Name
		addrsOut[i] = a.Address
	}
	return names, addrsOut
}

func joinAddrs(names, addrs []string) []gomail.Address {
	if len(names) == 0 {
		return nil
	}
	out := make([]gomail.Address, len(names))
	for i := range names {
		out[i] = gomail.Address{Name: names[i], Address: addrs[i]}
	}
	return out
}

// EncodeDraft serializes d to gob bytes.
func EncodeDraft(d Draft) ([]byte, error) {
	toN, toA := splitAddrs(d.To)
	ccN, ccA := splitAddrs(d.Cc)
	bccN, bccA := splitAddrs(d.Bcc)
	g := gobDraft{
		FromName:    d.From.Name,
		FromAddr:    d.From.Address,
		ToNames:     toN,
		ToAddrs:     toA,
		CcNames:     ccN,
		CcAddrs:     ccA,
		BccNames:    bccN,
		BccAddrs:    bccA,
		Subject:     d.Subject,
		Body:        d.Body,
		InReplyTo:   d.InReplyTo,
		References:  d.References,
		Attachments: d.Attachments,
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(g); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecodeDraft deserializes gob bytes produced by EncodeDraft.
func DecodeDraft(b []byte) (Draft, error) {
	var g gobDraft
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&g); err != nil {
		return Draft{}, err
	}
	return Draft{
		From:        gomail.Address{Name: g.FromName, Address: g.FromAddr},
		To:          joinAddrs(g.ToNames, g.ToAddrs),
		Cc:          joinAddrs(g.CcNames, g.CcAddrs),
		Bcc:         joinAddrs(g.BccNames, g.BccAddrs),
		Subject:     g.Subject,
		Body:        g.Body,
		InReplyTo:   g.InReplyTo,
		References:  g.References,
		Attachments: g.Attachments,
	}, nil
}
