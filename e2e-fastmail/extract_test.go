package e2e

import (
	"os"
	"strings"
	"testing"
)

func TestExtractFrom(t *testing.T) {
	msg, err := os.ReadFile("testdata/sample-message.eml")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	r := run(t, string(msg), "rules", "extract", "from")
	if r.err != nil {
		t.Fatalf("extract from failed: %v\nstderr: %s", r.err, r.stderr)
	}
	if r.stdout != "seanwalsh144@gmail.com" {
		t.Errorf("stdout = %q, want %q", r.stdout, "seanwalsh144@gmail.com")
	}
}

func TestExtractSubject(t *testing.T) {
	msg, err := os.ReadFile("testdata/sample-message.eml")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	r := run(t, string(msg), "rules", "extract", "subject")
	if r.err != nil {
		t.Fatalf("extract subject failed: %v\nstderr: %s", r.err, r.stderr)
	}
	if strings.TrimSpace(r.stdout) != "Should have been adult class" {
		t.Errorf("stdout = %q, want %q", r.stdout, "Should have been adult class")
	}
}

func TestExtractBadField(t *testing.T) {
	r := run(t, "From: a@b.com\r\n\r\n", "rules", "extract", "bogus")
	if r.err == nil {
		t.Error("expected error for unknown field")
	}
}

func TestExtractTo(t *testing.T) {
	msg, err := os.ReadFile("testdata/sample-message.eml")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	r := run(t, string(msg), "rules", "extract", "to")
	if r.err != nil {
		t.Fatalf("extract to failed: %v\nstderr: %s", r.err, r.stderr)
	}

	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 addresses, got %d: %v", len(lines), lines)
	}
	if lines[0] != "matt.flickinger@aksailingclub.org" {
		t.Errorf("first address = %q, want matt.flickinger@aksailingclub.org", lines[0])
	}
	if lines[1] != "program-committee@aksailingclub.org" {
		t.Errorf("second address = %q, want program-committee@aksailingclub.org", lines[1])
	}
}

func TestExtractToMissing(t *testing.T) {
	r := run(t, "From: a@b.com\r\nSubject: Hi\r\n\r\n", "rules", "extract", "to")
	if r.err == nil {
		t.Error("expected error for missing to/cc headers")
	}
}
