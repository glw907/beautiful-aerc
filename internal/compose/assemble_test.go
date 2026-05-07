package compose

import (
	"bytes"
	"io"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gomail "github.com/emersion/go-message/mail"
)

func fixedClock() time.Time {
	return time.Date(2026, 5, 5, 10, 30, 0, 0, time.UTC)
}

func TestAssembleMIMERequiresFrom(t *testing.T) {
	if _, err := AssembleMIME(Draft{}, fixedClock()); err == nil {
		t.Fatal("AssembleMIME with empty From: want error, got nil")
	}
}

func TestAssembleMIMEHeadersAndAlternative(t *testing.T) {
	d := Draft{
		From:       gomail.Address{Address: "alice@example.com"},
		To:         []gomail.Address{{Address: "bob@example.org"}},
		Subject:    "Hello",
		Body:       "**hi** bob",
		InReplyTo:  "abc123@example.org",
		References: []string{"root@example.org", "abc123@example.org"},
	}
	raw, err := AssembleMIME(d, fixedClock())
	if err != nil {
		t.Fatalf("AssembleMIME: %v", err)
	}

	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse assembled message: %v", err)
	}
	if got := msg.Header.Get("From"); !strings.Contains(got, "alice@example.com") {
		t.Errorf("From header = %q", got)
	}
	if got := msg.Header.Get("To"); !strings.Contains(got, "bob@example.org") {
		t.Errorf("To header = %q", got)
	}
	if got := msg.Header.Get("Subject"); got != "Hello" {
		t.Errorf("Subject = %q, want %q", got, "Hello")
	}
	if got := msg.Header.Get("In-Reply-To"); got != "<abc123@example.org>" {
		t.Errorf("In-Reply-To = %q, want %q", got, "<abc123@example.org>")
	}
	if got := msg.Header.Get("References"); got != "<root@example.org> <abc123@example.org>" {
		t.Errorf("References = %q", got)
	}
	if got := msg.Header.Get("Message-Id"); !strings.Contains(got, "@example.com") {
		t.Errorf("Message-Id = %q, want @example.com host", got)
	}

	mr, err := gomail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("multipart reader: %v", err)
	}
	defer mr.Close()
	var sawPlain, sawHTML bool
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextPart: %v", err)
		}
		ih, ok := p.Header.(*gomail.InlineHeader)
		if !ok {
			continue
		}
		ct, _, _ := ih.ContentType()
		body, _ := io.ReadAll(p.Body)
		switch ct {
		case "text/plain":
			sawPlain = true
			if string(body) != "**hi** bob" {
				t.Errorf("text/plain body = %q, want raw markdown", string(body))
			}
		case "text/html":
			sawHTML = true
			if !strings.Contains(string(body), "<strong>hi</strong>") {
				t.Errorf("text/html missing rendered bold: %q", string(body))
			}
		}
	}
	if !sawPlain || !sawHTML {
		t.Errorf("missing alternative parts: plain=%v html=%v", sawPlain, sawHTML)
	}
}

func TestAssembleMIMEWithAttachment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("the body"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := Draft{
		From:        gomail.Address{Address: "a@x.test"},
		To:          []gomail.Address{{Address: "b@y.test"}},
		Subject:     "with file",
		Body:        "see attached",
		Attachments: []string{path},
	}
	raw, err := AssembleMIME(d, fixedClock())
	if err != nil {
		t.Fatalf("AssembleMIME: %v", err)
	}
	mr, err := gomail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("CreateReader: %v", err)
	}
	defer mr.Close()
	var sawAttachment bool
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextPart: %v", err)
		}
		ah, ok := p.Header.(*gomail.AttachmentHeader)
		if !ok {
			continue
		}
		fn, _ := ah.Filename()
		if fn != "note.txt" {
			t.Errorf("attachment filename = %q, want %q", fn, "note.txt")
		}
		body, _ := io.ReadAll(p.Body)
		if string(body) != "the body" {
			t.Errorf("attachment body = %q", string(body))
		}
		sawAttachment = true
	}
	if !sawAttachment {
		t.Error("no attachment part found")
	}
}

func TestAngleAddrIdempotent(t *testing.T) {
	cases := []struct{ in, want string }{
		{"abc", "<abc>"},
		{"<abc>", "<abc>"},
		{"  abc ", "<abc>"},
		{"", ""},
	}
	for _, c := range cases {
		if got := angleAddr(c.in); got != c.want {
			t.Errorf("angleAddr(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
