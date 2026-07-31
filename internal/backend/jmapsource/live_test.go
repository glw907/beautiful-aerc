//go:build live

package jmapsource_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
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

// TestLiveMailboxNameConflictAndLookup is the outbox's create-replay
// reconcile against the server poplar ships for. Two things only
// Fastmail can answer, and the reconcile rests on both: that it
// refuses a duplicate sibling name at all, RFC 8621 section 2
// forbidding two sibling mailboxes with the same parent and the same
// name, since a Fastmail that started allowing one would leave the
// reconcile never firing and the duplicate folder back; and that
// FindMailboxes compensates for section 2.3's name filter, which
// matches a mailbox whose name "contains the given string", so a live
// account holding both a name and a longer one built on it answers
// with the exact one alone.
func TestLiveMailboxNameConflictAndLookup(t *testing.T) {
	token, err := keyring.Token("")
	if err != nil {
		t.Skipf("no fastmail token: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	session, err := jmapsource.Dial(ctx, fastmailSessionURL, jmapsource.NewStaticCredentials(token))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	mail := session.Mail()

	// Each mailbox is destroyed when the test ends, so a run leaves the
	// account as it found it.
	create := func(name string) string {
		t.Helper()
		id, err := mail.CreateMailbox(ctx, name, "")
		if err != nil {
			t.Fatalf("CreateMailbox %q: %v", name, err)
		}
		t.Cleanup(func() {
			cctx, ccancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer ccancel()
			if err := mail.DeleteMailbox(cctx, id); err != nil {
				t.Errorf("DeleteMailbox(%s): %v", id, err)
			}
		})
		return id
	}

	name := fmt.Sprintf("poplar-live-conflict-%d", time.Now().UnixNano())
	id := create(name)
	create(name + "-extended")

	if _, err := mail.CreateMailbox(ctx, name, ""); !errors.Is(err, backend.ErrMailboxNameExists) {
		t.Fatalf("a second create of %q returned %v, want it wrapping backend.ErrMailboxNameExists", name, err)
	}

	found, err := mail.FindMailboxes(ctx, name, "")
	if err != nil {
		t.Fatalf("FindMailboxes: %v", err)
	}
	if !slices.Equal(found, []string{id}) {
		t.Errorf("FindMailboxes(%q, root) = %v, want only %s: the longer name is a substring match on the wire", name, found, id)
	}

	// A name the account does not hold answers with nothing, which is
	// what tells an ambiguous reconcile from a resolved one.
	absent, err := mail.FindMailboxes(ctx, name+"-absent", "")
	if err != nil {
		t.Fatalf("FindMailboxes for a name the account lacks: %v", err)
	}
	if len(absent) != 0 {
		t.Errorf("FindMailboxes for an absent name = %v, want none", absent)
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
