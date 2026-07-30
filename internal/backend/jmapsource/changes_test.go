package jmapsource

import (
	"context"
	"errors"
	"os"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/jmap"
)

// stringsFromAny converts a decoded JSON array (v's dynamic type after
// a round trip through encoding/json is []any) into a []string, the
// shape a "properties" argument compares against.
func stringsFromAny(t *testing.T, v any) []string {
	t.Helper()

	items, ok := v.([]any)
	if !ok {
		t.Fatalf("properties = %v (%T), want []any", v, v)
	}
	out := make([]string, len(items))
	for i, item := range items {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("properties[%d] = %v (%T), want string", i, item, item)
		}
		out[i] = s
	}
	return out
}

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
		wantProperties := []string{
			"id", "blobId", "threadId", "mailboxIds", "keywords", "size",
			"receivedAt", "subject", "from", "hasAttachment",
		}
		if got := stringsFromAny(t, args["properties"]); !slices.Equal(got, wantProperties) {
			t.Errorf("methodCalls[%d] properties = %v, want %v", i+1, got, wantProperties)
		}
	}

	if len(cs.Created) != 1 || cs.Created[0].ID != "msg-1" {
		t.Fatalf("Created = %+v", cs.Created)
	}
	created, ok := cs.Created[0].Fields.(backend.MessageFields)
	if !ok {
		t.Fatalf("Created[0].Fields = %T, want backend.MessageFields", cs.Created[0].Fields)
	}
	if want := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC); !created.ReceivedAt.Equal(want) {
		t.Errorf("Created[0] ReceivedAt = %v, want %v", created.ReceivedAt, want)
	}
	// Everything else compares whole, so a fixture property that stops
	// reaching its field fails here without an assertion naming it.
	created.ReceivedAt = time.Time{}
	want := backend.MessageFields{
		BlobID:        "blob-1",
		ThreadKey:     "thread-1",
		Subject:       "Hello",
		From:          []backend.Address{{Name: "Alice", Email: "alice@example.com"}},
		MailboxIDs:    []string{"mbx-1"},
		Size:          1234,
		HasAttachment: true,
		Flags:         backend.FlagSeen,
	}
	if !reflect.DeepEqual(created, want) {
		t.Errorf("Created[0].Fields = %+v, want %+v", created, want)
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
	wantProperties := []string{
		"id", "blobId", "threadId", "mailboxIds", "keywords", "size",
		"receivedAt", "subject", "from", "hasAttachment",
	}
	if got := stringsFromAny(t, getArgs["properties"]); !slices.Equal(got, wantProperties) {
		t.Errorf("Email/get properties = %v, want %v", got, wantProperties)
	}

	if want := baselineTokenPrefix + "1:qs1"; cs.NewToken != want {
		t.Errorf("NewToken = %q, want %s", cs.NewToken, want)
	}
	if !cs.HasMore {
		t.Error("HasMore = false, want true (query total exceeds one page)")
	}
}

// TestChangesBaselineRestartsOnQueryStateChange covers the case a
// destroy shifts positions mid-baseline: the resumed page's
// queryState no longer matches the one the token was issued against,
// so the pull restarts from the top rather than silently skip
// whatever moved past the resume point.
func TestChangesBaselineRestartsOnQueryStateChange(t *testing.T) {
	session, api := newTestSession(t,
		readFixture(t, "baseline_page1.json"),
		readFixture(t, "baseline_page2_state_changed.json"),
		readFixture(t, "baseline_page1.json"),
	)

	page1, err := session.Mail().Changes(context.Background(), backend.ObjectKindMessage, "", 1)
	if err != nil {
		t.Fatalf("Changes page 1: %v", err)
	}
	if want := baselineTokenPrefix + "1:qs1"; page1.NewToken != want {
		t.Fatalf("page1.NewToken = %q, want %s", page1.NewToken, want)
	}

	page2, err := session.Mail().Changes(context.Background(), backend.ObjectKindMessage, page1.NewToken, 1)
	if err != nil {
		t.Fatalf("Changes page 2: %v", err)
	}
	if got := api.callCount(); got != 3 {
		t.Fatalf("api calls = %d, want 3 (page1, the queryState-changed page2, and the restarted pull)", got)
	}
	if len(page2.Created) != 1 || page2.Created[0].ID != "msg-1" {
		t.Fatalf("page2.Created = %+v, want msg-1 from the restarted pull", page2.Created)
	}
	if want := baselineTokenPrefix + "1:qs1"; page2.NewToken != want {
		t.Errorf("page2.NewToken = %q, want %s", page2.NewToken, want)
	}
}

// TestMessageFieldsFlagsFromKeywords covers messageFields translating
// the server's keyword set into the seam's flag set: each keyword
// reaches its own flag and nothing else, and a keyword the server
// omits or carries as false leaves its flag clear.
func TestMessageFieldsFlagsFromKeywords(t *testing.T) {
	tests := []struct {
		name     string
		keywords map[string]bool
		want     backend.MessageFlags
	}{
		{"no keywords", nil, 0},
		{"seen", map[string]bool{"$seen": true}, backend.FlagSeen},
		{"flagged", map[string]bool{"$flagged": true}, backend.FlagFlagged},
		{"answered", map[string]bool{"$answered": true}, backend.FlagAnswered},
		{"draft", map[string]bool{"$draft": true}, backend.FlagDraft},
		{"forwarded", map[string]bool{"$forwarded": true}, backend.FlagForwarded},
		{"a false keyword stays clear", map[string]bool{"$seen": false, "$draft": true}, backend.FlagDraft},
		{"a keyword poplar does not track", map[string]bool{"$phishing": true}, 0},
		{
			"all five",
			map[string]bool{"$seen": true, "$flagged": true, "$answered": true, "$draft": true, "$forwarded": true},
			backend.FlagSeen | backend.FlagFlagged | backend.FlagAnswered | backend.FlagDraft | backend.FlagForwarded,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := messageFields(&jmap.Email{ID: "msg-1", Keywords: tt.keywords}).Flags
			if got != tt.want {
				t.Errorf("Flags = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestMailboxChanges covers the Mailbox/changes path
// messageChanges's sibling shares changesRoundTrip with.
func TestMailboxChanges(t *testing.T) {
	session, api := newTestSession(t, readFixture(t, "mailbox_changes_response.json"))

	cs, err := session.Mail().Changes(context.Background(), backend.ObjectKindMailbox, "1", 50)
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	if got := api.callCount(); got != 1 {
		t.Fatalf("api calls = %d, want 1", got)
	}
	wantProperties := []string{"id", "name", "role", "sortOrder", "totalEmails", "unreadEmails"}
	_, createdArgs, _ := methodCall(t, api.requestAt(0), 1)
	if got := stringsFromAny(t, createdArgs["properties"]); !slices.Equal(got, wantProperties) {
		t.Errorf("Mailbox/get created properties = %v, want %v", got, wantProperties)
	}
	_, updatedArgs, _ := methodCall(t, api.requestAt(0), 2)
	if got := stringsFromAny(t, updatedArgs["properties"]); !slices.Equal(got, wantProperties) {
		t.Errorf("Mailbox/get updated properties = %v, want %v", got, wantProperties)
	}
	if len(cs.Created) != 1 || cs.Created[0].ID != "mbx-new" {
		t.Fatalf("Created = %+v", cs.Created)
	}
	box, ok := cs.Created[0].Fields.(backend.MailboxFields)
	if !ok {
		t.Fatalf("Created[0].Fields = %T, want backend.MailboxFields", cs.Created[0].Fields)
	}
	want := backend.MailboxFields{Role: "archive", Name: "Projects", SortOrder: 3, TotalCount: 42, UnreadCount: 5}
	if box != want {
		t.Errorf("Created[0].Fields = %+v, want %+v", box, want)
	}
	if len(cs.Destroyed) != 1 || cs.Destroyed[0] != "mbx-old" {
		t.Fatalf("Destroyed = %+v", cs.Destroyed)
	}
	if cs.NewToken != "2" {
		t.Errorf("NewToken = %q, want 2", cs.NewToken)
	}
}

// TestChangesBaselineMailboxes covers the empty-token mailbox path:
// one Mailbox/get call fetching every mailbox, mailboxChanges's
// baseline counterpart.
func TestChangesBaselineMailboxes(t *testing.T) {
	session, api := newTestSession(t, readFixture(t, "baseline_mailboxes.json"))

	cs, err := session.Mail().Changes(context.Background(), backend.ObjectKindMailbox, "", 50)
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	if got := api.callCount(); got != 1 {
		t.Fatalf("api calls = %d, want 1", got)
	}
	name, args, _ := methodCall(t, api.requestAt(0), 0)
	if name != "Mailbox/get" {
		t.Fatalf("methodCalls[0] = %q, want Mailbox/get", name)
	}
	wantProperties := []string{"id", "name", "role", "sortOrder", "totalEmails", "unreadEmails"}
	if got := stringsFromAny(t, args["properties"]); !slices.Equal(got, wantProperties) {
		t.Errorf("Mailbox/get properties = %v, want %v", got, wantProperties)
	}

	if len(cs.Created) != 1 || cs.Created[0].ID != "mbx-1" {
		t.Fatalf("Created = %+v", cs.Created)
	}
	if cs.NewToken != "1" {
		t.Errorf("NewToken = %q, want 1", cs.NewToken)
	}
}
