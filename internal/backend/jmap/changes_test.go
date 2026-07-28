package jmap

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/glw907/poplar/internal/backend"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

// TestChangesOneRoundTrip asserts that Changes carries Email/changes
// plus its created and updated Email/get calls, back-referenced to
// the changes call, in a single HTTP request, and that the response
// hydrates into the ChangeSet the recorded fixture describes.
func TestChangesOneRoundTrip(t *testing.T) {
	session, api := newTestSession(t, readFixture(t, "changes_response.json"))

	cs, err := session.Mail().Changes(context.Background(), backend.ObjectKindMessage, "1", 50)
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}

	if got := api.callCount(); got != 1 {
		t.Fatalf("api calls = %d, want 1", got)
	}
	req := api.requestAt(0)
	changesName, changesArgs, changesCallID := methodCall(t, req, 0)
	if changesName != "Email/changes" {
		t.Fatalf("methodCalls[0] = %q, want Email/changes", changesName)
	}
	if got := changesArgs["sinceState"]; got != "1" {
		t.Fatalf("Email/changes sinceState = %v, want 1", got)
	}

	for i, wantPath := range []string{"/created", "/updated"} {
		name, args, _ := methodCall(t, req, i+1)
		if name != "Email/get" {
			t.Fatalf("methodCalls[%d] = %q, want Email/get", i+1, name)
		}
		ref, ok := args["#ids"].(map[string]any)
		if !ok {
			t.Fatalf("methodCalls[%d] has no #ids back-reference", i+1)
		}
		if ref["resultOf"] != changesCallID {
			t.Errorf("methodCalls[%d] resultOf = %v, want %v", i+1, ref["resultOf"], changesCallID)
		}
		if ref["path"] != wantPath {
			t.Errorf("methodCalls[%d] path = %v, want %v", i+1, ref["path"], wantPath)
		}
	}

	if len(cs.Created) != 1 || cs.Created[0].ID != "msg-1" {
		t.Fatalf("Created = %+v", cs.Created)
	}
	if got := cs.Created[0].Fields["subject"]; got != "Hello" {
		t.Errorf("Created[0].Fields[subject] = %v, want Hello", got)
	}
	if got := cs.Created[0].Fields["seen"]; got != true {
		t.Errorf("Created[0].Fields[seen] = %v, want true", got)
	}
	if got, ok := cs.Created[0].Fields["mailbox_ids"].([]string); !ok || len(got) != 1 || got[0] != "mbx-1" {
		t.Errorf("Created[0].Fields[mailbox_ids] = %v", cs.Created[0].Fields["mailbox_ids"])
	}

	if len(cs.Updated) != 1 || cs.Updated[0].ID != "msg-2" {
		t.Fatalf("Updated = %+v", cs.Updated)
	}
	if len(cs.Destroyed) != 1 || cs.Destroyed[0] != "msg-3" {
		t.Fatalf("Destroyed = %+v", cs.Destroyed)
	}
	if cs.NewToken != "2" {
		t.Errorf("NewToken = %q, want 2", cs.NewToken)
	}
	if cs.HasMore {
		t.Error("HasMore = true, want false")
	}
}

// TestChangesPaging covers HasMore and token advance across two
// successive Changes calls, the second passing back the first's
// NewToken as sinceState.
func TestChangesPaging(t *testing.T) {
	session, api := newTestSession(t,
		readFixture(t, "changes_page1.json"),
		readFixture(t, "changes_page2.json"),
	)

	page1, err := session.Mail().Changes(context.Background(), backend.ObjectKindMessage, "1", 1)
	if err != nil {
		t.Fatalf("Changes page 1: %v", err)
	}
	if !page1.HasMore {
		t.Error("page1.HasMore = false, want true")
	}
	if page1.NewToken != "2" {
		t.Fatalf("page1.NewToken = %q, want 2", page1.NewToken)
	}
	if len(page1.Created) != 1 || page1.Created[0].ID != "msg-1" {
		t.Fatalf("page1.Created = %+v", page1.Created)
	}

	page2, err := session.Mail().Changes(context.Background(), backend.ObjectKindMessage, page1.NewToken, 1)
	if err != nil {
		t.Fatalf("Changes page 2: %v", err)
	}
	if page2.HasMore {
		t.Error("page2.HasMore = true, want false")
	}
	if page2.NewToken != "3" {
		t.Fatalf("page2.NewToken = %q, want 3", page2.NewToken)
	}
	if len(page2.Created) != 1 || page2.Created[0].ID != "msg-2" {
		t.Fatalf("page2.Created = %+v", page2.Created)
	}

	if got := api.callCount(); got != 2 {
		t.Fatalf("api calls = %d, want 2", got)
	}
	_, secondArgs, _ := methodCall(t, api.requestAt(1), 0)
	if secondArgs["sinceState"] != "2" {
		t.Errorf("second call sinceState = %v, want 2", secondArgs["sinceState"])
	}
}

// TestCannotCalculateChanges asserts that a cannotCalculateChanges
// method error becomes backend.ErrStateReset, the typed signal the
// sync engine turns into a full resync.
func TestCannotCalculateChanges(t *testing.T) {
	session, _ := newTestSession(t, readFixture(t, "changes_cannot_calculate.json"))

	_, err := session.Mail().Changes(context.Background(), backend.ObjectKindMessage, "stale", 50)
	if !errors.Is(err, backend.ErrStateReset) {
		t.Fatalf("Changes error = %v, want backend.ErrStateReset", err)
	}
}

// TestChangesBaselinePull covers the empty-token path: Email/query
// paged by position plus a back-referenced Email/get, with NewToken
// carrying the resume position until the last page switches to a
// real JMAP state.
func TestChangesBaselinePull(t *testing.T) {
	session, api := newTestSession(t, readFixture(t, "baseline_page1.json"))

	cs, err := session.Mail().Changes(context.Background(), backend.ObjectKindMessage, "", 1)
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}

	req := api.requestAt(0)
	name, args, callID := methodCall(t, req, 0)
	if name != "Email/query" {
		t.Fatalf("methodCalls[0] = %q, want Email/query", name)
	}
	if args["position"] != nil {
		t.Errorf("Email/query position = %v, want omitted (0)", args["position"])
	}
	getName, getArgs, _ := methodCall(t, req, 1)
	if getName != "Email/get" {
		t.Fatalf("methodCalls[1] = %q, want Email/get", getName)
	}
	ref, ok := getArgs["#ids"].(map[string]any)
	if !ok {
		t.Fatalf("Email/get has no #ids back-reference: %v", getArgs)
	}
	if ref["resultOf"] != callID || ref["path"] != "/ids" {
		t.Errorf("Email/get back-reference = %v", ref)
	}

	if cs.NewToken != baselineTokenPrefix+"1" {
		t.Errorf("NewToken = %q, want %s1", cs.NewToken, baselineTokenPrefix)
	}
	if !cs.HasMore {
		t.Error("HasMore = false, want true (query total exceeds one page)")
	}
}
