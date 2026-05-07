package compose

import (
	"testing"
	"time"

	gomail "github.com/emersion/go-message/mail"
)

func TestRoundTrip_AssembleParse(t *testing.T) {
	d := Draft{
		From:    gomail.Address{Name: "Geoff", Address: "geoff@907.life"},
		To:      []gomail.Address{{Address: "alice@example.com"}},
		Cc:      []gomail.Address{{Address: "bob@example.com"}},
		Subject: "test draft",
		Body:    "hello\n\n> quoted\n",
	}
	mime, err := AssembleMIME(d, time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("AssembleMIME: %v", err)
	}
	got, err := ParseDraftMIME(mime)
	if err != nil {
		t.Fatalf("ParseDraftMIME: %v", err)
	}
	if got.Subject != d.Subject {
		t.Errorf("Subject = %q, want %q", got.Subject, d.Subject)
	}
	if got.From.Address != d.From.Address {
		t.Errorf("From = %q, want %q", got.From.Address, d.From.Address)
	}
	if len(got.To) != 1 || got.To[0].Address != "alice@example.com" {
		t.Errorf("To = %+v, want alice@example.com", got.To)
	}
	if len(got.Cc) != 1 || got.Cc[0].Address != "bob@example.com" {
		t.Errorf("Cc = %+v, want bob@example.com", got.Cc)
	}
	if got.Body != d.Body {
		t.Errorf("Body = %q, want %q", got.Body, d.Body)
	}
}

func TestParseDraftMIME_HeadersOnly(t *testing.T) {
	mime := []byte("From: a@x\r\nTo: b@y\r\nSubject: hi\r\n\r\nBody text\r\n")
	got, err := ParseDraftMIME(mime)
	if err != nil {
		t.Fatalf("ParseDraftMIME: %v", err)
	}
	if got.Subject != "hi" || got.Body != "Body text\n" {
		t.Errorf("got = %+v, want subject=hi body=%q", got, "Body text\n")
	}
}
