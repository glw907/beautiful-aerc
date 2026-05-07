package compose

import (
	"reflect"
	"testing"

	gomail "github.com/emersion/go-message/mail"
)

func TestDraftGobRoundTrip(t *testing.T) {
	d := Draft{
		From:        gomail.Address{Name: "Géoff", Address: "geoff@907.life"},
		To:          []gomail.Address{{Name: "Alice", Address: "alice@example.com"}},
		Cc:          []gomail.Address{{Address: "cc@example.com"}},
		Bcc:         []gomail.Address{{Name: "Bcc User", Address: "bcc@example.com"}},
		Subject:     "Re: héllo",
		Body:        "quoted body\n\n> original",
		InReplyTo:   "<abc123@example.com>",
		References:  []string{"<ref1@example.com>", "<ref2@example.com>"},
		Attachments: []string{"/tmp/file.pdf", "/tmp/image.png"},
	}

	b, err := EncodeDraft(d)
	if err != nil {
		t.Fatalf("EncodeDraft: %v", err)
	}

	got, err := DecodeDraft(b)
	if err != nil {
		t.Fatalf("DecodeDraft: %v", err)
	}

	if !reflect.DeepEqual(d, got) {
		t.Fatalf("roundtrip mismatch\nwant: %+v\n got: %+v", d, got)
	}
}
