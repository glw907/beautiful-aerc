package mailcompose

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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

func TestQuoteLinesWrapsLongParagraph(t *testing.T) {
	long := strings.Repeat("the quick brown fox jumps over the lazy dog ", 8)
	long = strings.TrimRight(long, " ")
	got := quoteLines(long)
	for _, line := range strings.Split(got, "\n") {
		if !strings.HasPrefix(line, "> ") {
			t.Errorf("wrapped line missing %q prefix: %q", "> ", line)
		}
		if n := utf8.RuneCountInString(line); n > quoteWrapWidth {
			t.Errorf("wrapped line exceeds %d cells (%d): %q", quoteWrapWidth, n, line)
		}
	}
	// Continuation lines exist; single-line input is the bug we're fixing.
	if !strings.Contains(got, "\n") {
		t.Errorf("expected wrap into multiple lines, got %q", got)
	}
}

func TestQuoteLinesNestedDepthBudget(t *testing.T) {
	// At depth 2 ("> > ") prefix eats 4 cells; content budget is 68.
	long := strings.Repeat("alpha bravo charlie delta echo foxtrot golf ", 4)
	in := "> " + strings.TrimRight(long, " ")
	got := quoteLines(in)
	for _, line := range strings.Split(got, "\n") {
		if !strings.HasPrefix(line, "> > ") {
			t.Errorf("nested wrap dropped depth: %q", line)
		}
		if n := utf8.RuneCountInString(line); n > quoteWrapWidth {
			t.Errorf("nested wrap line exceeds %d cells (%d): %q", quoteWrapWidth, n, line)
		}
	}
}

func TestExtractPlainBodyNormalizesCRLF(t *testing.T) {
	raw := []byte("Subject: x\r\nContent-Type: text/plain\r\n\r\npara1\r\n\r\npara2\r\n")
	got := extractPlainBody(raw)
	if strings.Contains(got, "\r") {
		t.Errorf("extractPlainBody left CR in output: %q", got)
	}
	if want := "para1\n\npara2\n"; got != want {
		t.Errorf("extractPlainBody = %q, want %q", got, want)
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
		SentAt:  time.Date(2026, time.May, 4, 12, 0, 0, 0, time.UTC),
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
	if !strings.Contains(d.Body, "On Mon, May 4 2026 at 12:00 PM, Alice <alice@example.com> wrote:") {
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
		SentAt:  time.Date(2026, time.May, 4, 12, 0, 0, 0, time.UTC),
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
