//go:build live

package jmapsource_test

import (
	"context"
	"fmt"
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

// TestLivePushNotifiesOnAMailboxChange is the push transport against
// the real server. Two things only Fastmail can answer: that its event
// source opens and reports the connection at all, and that the account
// id its StateChange names is the one primaryAccounts assigned mail,
// which is what Listen filters a change on. A fake server proves the
// filter works; only this proves poplar is filtering on the right id.
//
// The mailbox round trip is the same one cmd/poplar's live runner
// uses, and for the same reason: it touches none of the real mail.
func TestLivePushNotifiesOnAMailboxChange(t *testing.T) {
	token, err := keyring.Token("")
	if err != nil {
		t.Skipf("no fastmail token: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	session, err := jmapsource.Dial(ctx, fastmailSessionURL, jmapsource.NewStaticCredentials(token))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	push := session.Push()
	if push == nil {
		t.Fatal("Push() = nil; the live session advertises no event source")
	}

	ch, err := push.Listen(ctx)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}

	// ADR-0018's flush-on-connect, live: the connection itself reports,
	// before the server has pushed anything.
	select {
	case n := <-ch:
		if n.Scope == "" {
			t.Error("the connect notification carries no scope")
		}
	case <-ctx.Done():
		t.Fatal("no notification for the connection itself")
	}

	name := fmt.Sprintf("poplar-live-push-%d", time.Now().UnixNano())
	id, err := session.Mail().CreateMailbox(ctx, name, "")
	if err != nil {
		t.Fatalf("CreateMailbox: %v", err)
	}
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer ccancel()
		if err := session.Mail().DeleteMailbox(cctx, id); err != nil {
			t.Errorf("DeleteMailbox(%s): %v", id, err)
		}
	})

	select {
	case n := <-ch:
		if n.Scope != session.Capabilities().AccountIDs["mail"] {
			t.Errorf("notification scope = %q, want the mail account id %q",
				n.Scope, session.Capabilities().AccountIDs["mail"])
		}
	case <-ctx.Done():
		t.Fatal("the server pushed nothing for a mailbox this test just created")
	}
}
