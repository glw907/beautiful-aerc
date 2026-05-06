// SPDX-License-Identifier: MIT

package compose

import (
	"strings"
	"testing"

	gomail "github.com/emersion/go-message/mail"

	pmail "github.com/glw907/poplar/internal/mail"
)

func TestEnsurePrefix(t *testing.T) {
	cases := []struct{ in, prefix, want string }{
		{"Hello", "Re:", "Re: Hello"},
		{"Re: Hello", "Re:", "Re: Hello"},
		{"Re: Re: Hello", "Re:", "Re: Hello"},
		{"re: re: hi", "Re:", "Re: hi"},
		{"", "Re:", "Re:"},
		{"foo", "Fwd:", "Fwd: foo"},
		{"Fwd: foo", "Fwd:", "Fwd: foo"},
	}
	for _, c := range cases {
		if got := ensurePrefix(c.in, c.prefix); got != c.want {
			t.Errorf("ensurePrefix(%q, %q) = %q, want %q", c.in, c.prefix, got, c.want)
		}
	}
}

func TestQuoteLinesDepthPreserving(t *testing.T) {
	in := "hi\n> nested\n\nbye"
	got := quoteLines(in)
	want := "> hi\n> > nested\n>\n> bye"
	if got != want {
		t.Errorf("quoteLines:\n got %q\nwant %q", got, want)
	}
}

func TestStripAngles(t *testing.T) {
	if got := stripAngles("<abc@x>"); got != "abc@x" {
		t.Errorf("stripAngles = %q", got)
	}
	if got := stripAngles("  abc@x "); got != "abc@x" {
		t.Errorf("stripAngles plain = %q", got)
	}
}

const replyFixture = `From: Alice <alice@example.com>
To: Bob <bob@example.org>
Subject: Project update
Message-Id: <parent-id@example.com>
References: <root@example.com>
Date: Mon, 04 May 2026 12:00:00 +0000
Content-Type: text/plain; charset=utf-8

Hi Bob,

Thoughts on Q2?

Alice
`

func TestSeedReplyFields(t *testing.T) {
	parent := pmail.MessageInfo{
		UID:     pmail.UID("42"),
		Subject: "Project update",
		From:    "Alice <alice@example.com>",
		Date:    "Mon, 04 May 2026 12:00:00 +0000",
	}
	d := SeedReply(parent, []byte(replyFixture))

	if d.Subject != "Re: Project update" {
		t.Errorf("Subject = %q", d.Subject)
	}
	if len(d.To) != 1 || d.To[0].Address != "alice@example.com" {
		t.Errorf("To = %+v, want alice@example.com", d.To)
	}
	if d.InReplyTo != "parent-id@example.com" {
		t.Errorf("InReplyTo = %q", d.InReplyTo)
	}
	wantRefs := []string{"root@example.com", "parent-id@example.com"}
	if !equalSlice(d.References, wantRefs) {
		t.Errorf("References = %v, want %v", d.References, wantRefs)
	}
	if !strings.Contains(d.Body, "On Mon, 04 May 2026 12:00:00 +0000, Alice <alice@example.com> wrote:") {
		t.Errorf("body missing attribution: %q", d.Body)
	}
	if !strings.Contains(d.Body, "> Hi Bob,") {
		t.Errorf("body missing quoted line: %q", d.Body)
	}
	if !strings.HasPrefix(d.Body, "\n\n") {
		t.Errorf("body should lead with blank cursor row, got %q", d.Body[:4])
	}
}

func TestSeedReplyAllExcludesSelf(t *testing.T) {
	parent := pmail.MessageInfo{
		Subject: "thread",
		From:    "Alice <alice@example.com>",
		To:      "Bob <bob@example.org>, Carol <carol@example.net>",
		Cc:      "dave@example.io",
	}
	self := gomail.Address{Address: "bob@example.org"}
	d := SeedReplyAll(parent, []byte(replyFixture), self)
	for _, a := range d.Cc {
		if strings.EqualFold(a.Address, "bob@example.org") {
			t.Errorf("Cc must not include self: %+v", d.Cc)
		}
	}
	gotAddrs := map[string]bool{}
	for _, a := range d.Cc {
		gotAddrs[a.Address] = true
	}
	if !gotAddrs["carol@example.net"] || !gotAddrs["dave@example.io"] {
		t.Errorf("Cc missing expected addresses: %+v", d.Cc)
	}
}

func TestSeedForwardEmptyThreadHeaders(t *testing.T) {
	parent := pmail.MessageInfo{
		Subject: "Project update",
		From:    "Alice <alice@example.com>",
		Date:    "Mon, 04 May 2026 12:00:00 +0000",
	}
	d := SeedForward(parent, []byte(replyFixture))
	if d.Subject != "Fwd: Project update" {
		t.Errorf("Subject = %q", d.Subject)
	}
	if d.InReplyTo != "" || len(d.References) != 0 {
		t.Errorf("forward must not chain: InReplyTo=%q References=%v", d.InReplyTo, d.References)
	}
	if len(d.To) != 0 || len(d.Cc) != 0 {
		t.Errorf("forward To/Cc must be empty: To=%v Cc=%v", d.To, d.Cc)
	}
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
