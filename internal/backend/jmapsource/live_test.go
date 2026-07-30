//go:build live

package jmapsource_test

import (
	"context"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/jmapsource"
	"github.com/glw907/poplar/internal/keyring"
)

// fastmailSessionURL is Fastmail's JMAP session discovery endpoint
// (~/.claude/instructions/fastmail-api.md).
const fastmailSessionURL = "https://api.fastmail.com/jmap/session"

// TestLiveChangesAndBody runs against Geoff's real Fastmail account,
// gated by FASTMAIL_API_TOKEN. It never runs in CI or make check: the
// live build tag keeps the file out of a plain `go test ./...`, and
// the missing-token skip covers a checkout where the secret was never
// sourced.
func TestLiveChangesAndBody(t *testing.T) {
	token, err := keyring.Token("")
	if err != nil {
		t.Skipf("no fastmail token: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session, err := jmapsource.Dial(ctx, fastmailSessionURL, jmapsource.NewStaticCredentials(token))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	caps := session.Capabilities()
	if caps.Limits.MaxObjectsInGet == 0 {
		t.Error("Capabilities().Limits.MaxObjectsInGet = 0, want the live session's real limit")
	}
	if caps.AccountIDs["mail"] == "" {
		t.Error("Capabilities().AccountIDs[mail] is empty")
	}

	changes, err := session.Mail().Changes(ctx, backend.ObjectKindMessage, "", 5)
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	if len(changes.Created) == 0 {
		t.Fatal("Changes returned no messages; account has none, or the baseline pull is broken")
	}

	seq, err := session.Mail().FetchBodies(ctx, []string{changes.Created[0].ID})
	if err != nil {
		t.Fatalf("FetchBodies: %v", err)
	}
	for chunk := range seq {
		if chunk.Err != nil {
			t.Fatalf("FetchBodies %s: %v", chunk.ID, chunk.Err)
		}
		if len(chunk.Raw) == 0 {
			t.Fatalf("FetchBodies %s: empty body", chunk.ID)
		}
	}
}
