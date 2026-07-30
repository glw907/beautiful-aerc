package jmapsource

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/uerr"
	"github.com/glw907/poplar/jmap"
)

// downloadRequest records one Download call's own request, the three
// values downloadBlob supplies for the download URL template's
// {blobId}, {type}, and {name} placeholders. sessionTemplate
// (session_test.go) carries both {type} and {name}, so a value change
// to either reaches this recording rather than going unnoticed the
// way a template without them would.
type downloadRequest struct {
	blobID, contentType, name string
}

// fakeBlobs scripts the /upload/ and /download/ endpoints Submit and
// FetchBodies use, separate from /api's method-call batching.
type fakeBlobs struct {
	uploadResp []byte
	downloads  map[string][]byte

	mu                 sync.Mutex
	uploaded           [][]byte
	uploadContentTypes []string
	downloadRequests   []downloadRequest
}

func (b *fakeBlobs) handleUpload(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	b.mu.Lock()
	b.uploaded = append(b.uploaded, body)
	b.uploadContentTypes = append(b.uploadContentTypes, r.Header.Get("Content-Type"))
	b.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b.uploadResp)
}

// handleDownload parses blobID, type, and name off the request's own
// escaped path rather than through http.ServeMux's wildcard routing:
// FetchBodies's own downloadBlob leaves name empty, and a mux pattern
// of {blobID}/{type}/{name} does not match a URL whose final segment
// is empty (a trailing slash with nothing after it never reaches this
// handler at all). Splitting the escaped path ourselves, and
// unescaping each segment independently, also keeps the {type}
// segment's own "/" (message%2Frfc822) from being read as a fourth
// path separator, which splitting the already-decoded r.URL.Path
// would do.
func (b *fakeBlobs) handleDownload(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.EscapedPath(), "/download/")
	parts := strings.Split(rest, "/")
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}
	blobID, err1 := url.PathUnescape(parts[0])
	contentType, err2 := url.PathUnescape(parts[1])
	name, err3 := url.PathUnescape(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		http.Error(w, "bad path escaping", http.StatusBadRequest)
		return
	}

	b.mu.Lock()
	b.downloadRequests = append(b.downloadRequests, downloadRequest{
		blobID:      blobID,
		contentType: contentType,
		name:        name,
	})
	b.mu.Unlock()
	content, ok := b.downloads[blobID]
	if !ok {
		http.NotFound(w, r)
		return
	}
	_, _ = w.Write(content)
}

func newTestSessionWithBlobs(t *testing.T, blobs *fakeBlobs, responses ...[]byte) (*Session, *fakeAPI) {
	t.Helper()
	api := &fakeAPI{responses: responses}
	mux, srv := newFakeServer(t)
	mux.HandleFunc("/api", api.handle)
	mux.HandleFunc("/upload/", blobs.handleUpload)
	mux.HandleFunc("/download/", blobs.handleDownload)
	return dialTestSession(t, srv), api
}

func TestApplyBatchUpdateDestroyAndUnsupportedCreate(t *testing.T) {
	session, api := newTestSession(t, readFixture(t, "apply_batch.json"))

	mutations := []backend.Mutation{
		{Op: backend.MutationUpdate, ID: "msg-1", Fields: backend.MessagePatch{SetFlags: backend.FlagSeen, MailboxIDs: []string{"mbx-archive"}}},
		{Op: backend.MutationDestroy, ID: "msg-2"},
		{Op: backend.MutationCreate, CreationID: "c1"},
	}
	result, err := session.Mail().ApplyBatch(context.Background(), mutations)
	if err != nil {
		t.Fatalf("ApplyBatch: %v", err)
	}

	if api.callCount() != 1 {
		t.Fatalf("api calls = %d, want 1", api.callCount())
	}
	_, args, _ := methodCall(t, api.requestAt(0), 0)
	update, ok := args["update"].(map[string]any)
	if !ok {
		t.Fatalf("Email/set update missing: %v", args)
	}
	patch, ok := update["msg-1"].(map[string]any)
	if !ok {
		t.Fatalf("update[msg-1] missing: %v", update)
	}
	if patch["keywords/$seen"] != true {
		t.Errorf("keywords/$seen = %v, want true", patch["keywords/$seen"])
	}
	mailboxIDs, ok := patch["mailboxIds"].(map[string]any)
	if !ok {
		t.Fatalf("mailboxIds missing or malformed: %v", patch["mailboxIds"])
	}
	if _, present := mailboxIDs["mbx-archive"]; !present {
		t.Errorf("mailboxIds missing mbx-archive: %v", mailboxIDs)
	}
	destroy, ok := args["destroy"].([]any)
	if !ok || len(destroy) != 1 || destroy[0] != "msg-2" {
		t.Errorf("destroy = %v", args["destroy"])
	}

	// Every entry in Failed carries a class the outbox dispatcher can
	// branch on, this transport's own refusal of a message create
	// included: a bare error there would reach the dispatcher with no
	// class to read and fall back to ClassServer by accident rather
	// than by decision.
	failures := []struct {
		name string
		key  string
		want uerr.Class
	}{
		{name: "unsupported message create", key: "c1", want: uerr.ClassServer},
		{name: "server notUpdated", key: "msg-3", want: uerr.ClassNotFound},
	}
	for _, f := range failures {
		t.Run(f.name, func(t *testing.T) {
			failure, present := result.Failed[f.key]
			if !present {
				t.Fatalf("result.Failed has no entry for %q", f.key)
			}
			mf, ok := errors.AsType[backend.Failure](failure)
			if !ok {
				t.Fatalf("Failed[%q] = %v, want a backend.Failure", f.key, failure)
			}
			if mf.Class != f.want {
				t.Errorf("Class = %v, want %v", mf.Class, f.want)
			}
		})
	}
}

// TestMessagePatchFlags covers the reverse of the keyword translation
// TestMessageFieldsFlagsFromKeywords covers: a flag the patch sets
// reaches the server as its keyword, one the patch clears as a null,
// one the patch names neither way stays out of the request so the
// server keeps whatever it holds, and one named in both sets
// (MessagePatch's doc comment: "a flag in both resolves to set")
// reaches the server as its keyword rather than a null. The
// membership stays out too, since this patch carries no MailboxIDs.
func TestMessagePatchFlags(t *testing.T) {
	patch := messagePatch(backend.MessagePatch{
		SetFlags:   backend.FlagSeen | backend.FlagAnswered | backend.FlagDraft,
		ClearFlags: backend.FlagFlagged | backend.FlagDraft,
	})
	want := jmap.Patch{
		"keywords/$seen":     true,
		"keywords/$answered": true,
		"keywords/$flagged":  nil,
		"keywords/$draft":    true,
	}
	if !reflect.DeepEqual(patch, want) {
		t.Errorf("messagePatch = %v, want %v", patch, want)
	}
}

// TestMessagePatchMailboxIDs covers messagePatch's nil-vs-empty
// MailboxIDs distinction: a nil slice (MessagePatch's doc comment:
// "leaves the message's folder membership alone") stays out of the
// patch entirely, while a non-nil empty slice ("a non-nil one
// replaces it whole") reaches the wire as an empty mailboxIds map,
// clearing every membership the server holds.
func TestMessagePatchMailboxIDs(t *testing.T) {
	tests := []struct {
		name       string
		mailboxIDs []string
		want       jmap.Patch
	}{
		{"nil leaves membership alone", nil, jmap.Patch{}},
		{"non-nil empty replaces with none", []string{}, jmap.Patch{"mailboxIds": map[string]bool{}}},
		{"non-empty replaces whole", []string{"mbx-1"}, jmap.Patch{"mailboxIds": map[string]bool{"mbx-1": true}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch := messagePatch(backend.MessagePatch{MailboxIDs: tt.mailboxIDs})
			if !reflect.DeepEqual(patch, tt.want) {
				t.Errorf("messagePatch = %v, want %v", patch, tt.want)
			}
		})
	}
}

// TestApplyBatchRejectsMismatchedFields covers a mutation whose kind
// and payload disagree. The batch is one request, so a mutation this
// transport cannot translate fails the call before anything reaches
// the server rather than dispatching a request missing part of what
// the caller asked for.
func TestApplyBatchRejectsMismatchedFields(t *testing.T) {
	tests := []struct {
		name string
		mut  backend.Mutation
		want string
	}{
		{
			name: "mailbox create carrying a message patch",
			mut:  backend.Mutation{Op: backend.MutationCreate, Kind: backend.ObjectKindMailbox, CreationID: "c1", Fields: backend.MessagePatch{}},
			want: "backend.MailboxCreate",
		},
		{
			name: "message update carrying a mailbox create",
			mut:  backend.Mutation{Op: backend.MutationUpdate, Kind: backend.ObjectKindMessage, ID: "msg-1", Fields: backend.MailboxCreate{}},
			want: "backend.MessagePatch",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, api := newTestSession(t)

			_, err := session.Mail().ApplyBatch(context.Background(), []backend.Mutation{tt.mut})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ApplyBatch error = %v, want one naming %s", err, tt.want)
			}
			if got := api.callCount(); got != 0 {
				t.Errorf("api calls = %d, want 0", got)
			}
		})
	}
}

// TestApplyBatchCreatesMailboxAndMovesInOneRequest covers the offline
// create-folder-then-move: a mailbox create and the message updates
// filing messages into it reach the server as one request, with the
// updates naming the mailbox by its creation-id back-reference because
// the server has not assigned it an id yet.
func TestApplyBatchCreatesMailboxAndMovesInOneRequest(t *testing.T) {
	session, api := newTestSession(t, readFixture(t, "batch_create_and_move.json"))

	result, err := session.Mail().ApplyBatch(context.Background(), []backend.Mutation{
		{
			Op:         backend.MutationCreate,
			Kind:       backend.ObjectKindMailbox,
			CreationID: "c1",
			Fields:     backend.MailboxCreate{Name: "Projects", ParentID: "mbx-parent"},
		},
		{
			Op:     backend.MutationUpdate,
			Kind:   backend.ObjectKindMessage,
			ID:     "msg-1",
			Fields: backend.MessagePatch{MailboxIDs: []string{"#c1"}},
		},
	})
	if err != nil {
		t.Fatalf("ApplyBatch: %v", err)
	}

	if api.callCount() != 1 {
		t.Fatalf("api calls = %d, want 1 (create and move are one request)", api.callCount())
	}
	name, args, _ := methodCall(t, api.requestAt(0), 0)
	if name != "Mailbox/set" {
		t.Fatalf("methodCalls[0] = %q, want Mailbox/set first, so Email/set can reference it", name)
	}
	create, ok := args["create"].(map[string]any)
	if !ok {
		t.Fatalf("Mailbox/set create missing: %v", args)
	}
	box, ok := create["c1"].(map[string]any)
	if !ok {
		t.Fatalf("create[c1] missing: %v", create)
	}
	if box["name"] != "Projects" {
		t.Errorf("created name = %v, want Projects", box["name"])
	}
	if box["parentId"] != "mbx-parent" {
		t.Errorf("created parentId = %v, want mbx-parent", box["parentId"])
	}

	name, args, _ = methodCall(t, api.requestAt(0), 1)
	if name != "Email/set" {
		t.Fatalf("methodCalls[1] = %q, want Email/set", name)
	}
	update, ok := args["update"].(map[string]any)
	if !ok {
		t.Fatalf("Email/set update missing: %v", args)
	}
	patch, ok := update["msg-1"].(map[string]any)
	if !ok {
		t.Fatalf("update[msg-1] missing: %v", update)
	}
	mailboxIDs, ok := patch["mailboxIds"].(map[string]any)
	if !ok {
		t.Fatalf("mailboxIds missing or malformed: %v", patch["mailboxIds"])
	}
	if _, present := mailboxIDs["#c1"]; !present {
		t.Errorf("mailboxIds = %v, want the #c1 back-reference", mailboxIDs)
	}

	if result.Created["c1"] != "mbx-new" {
		t.Errorf("Created[c1] = %q, want mbx-new", result.Created["c1"])
	}
	if len(result.Failed) != 0 {
		t.Errorf("Failed = %v, want none", result.Failed)
	}
}

func TestApplyBatchNoOpSkipsRoundTrip(t *testing.T) {
	session, api := newTestSession(t)

	result, err := session.Mail().ApplyBatch(context.Background(), []backend.Mutation{
		{Op: backend.MutationCreate, CreationID: "c1"},
	})
	if err != nil {
		t.Fatalf("ApplyBatch: %v", err)
	}
	if api.callCount() != 0 {
		t.Fatalf("api calls = %d, want 0 (all-create batch skips the round trip)", api.callCount())
	}
	if _, failed := result.Failed["c1"]; !failed {
		t.Error("c1 should be in result.Failed")
	}
}

func TestSubmit(t *testing.T) {
	blobs := &fakeBlobs{uploadResp: []byte(`{"accountId":"u1","blobId":"blob-out","type":"message/rfc822","size":11}`)}
	session, api := newTestSessionWithBlobs(t, blobs,
		readFixture(t, "mailbox_query_sent.json"),
		readFixture(t, "identity_get.json"),
		readFixture(t, "submit_response.json"),
	)

	result, err := session.Mail().Submit(context.Background(), []byte("hello world"))
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if result.ID != "sub-1" {
		t.Errorf("SubmitResult.ID = %q, want sub-1", result.ID)
	}
	if !result.Sent {
		t.Error("SubmitResult.Sent = false, want true")
	}
	if api.callCount() != 3 {
		t.Fatalf("api calls = %d, want 3 (mailbox query, identity get, import+submit)", api.callCount())
	}
	if len(blobs.uploaded) != 1 || string(blobs.uploaded[0]) != "hello world" {
		t.Errorf("uploaded bodies = %v, want [hello world]", blobs.uploaded)
	}
	// Upload requires a real Content-Type (jmap.Client.Upload's
	// signature, unlike go-jmap's own Upload, which sent a hardcoded
	// application/json regardless of what was uploaded): a raw
	// outgoing message is message/rfc822, not the mislabel go-jmap
	// carried forward.
	if len(blobs.uploadContentTypes) != 1 || blobs.uploadContentTypes[0] != "message/rfc822" {
		t.Errorf("upload Content-Type = %v, want [message/rfc822]", blobs.uploadContentTypes)
	}

	importName, importArgs, _ := methodCall(t, api.requestAt(2), 0)
	if importName != "Email/import" {
		t.Fatalf("methodCalls[0] = %q, want Email/import", importName)
	}
	emails, ok := importArgs["emails"].(map[string]any)
	if !ok {
		t.Fatalf("Email/import emails missing: %v", importArgs)
	}
	m1, ok := emails["m1"].(map[string]any)
	if !ok || m1["blobId"] != "blob-out" {
		t.Errorf("emails[m1].blobId = %v, want blob-out", m1["blobId"])
	}
}

func TestFetchBodies(t *testing.T) {
	blobs := &fakeBlobs{downloads: map[string][]byte{"blob-1": []byte("raw message one")}}
	session, api := newTestSessionWithBlobs(t, blobs,
		readFixture(t, "changes_response.json"),
		readFixture(t, "email_get_blobid.json"),
	)

	if _, err := session.Mail().Changes(context.Background(), backend.ObjectKindMessage, "1", 50); err != nil {
		t.Fatalf("Changes: %v", err)
	}
	if got := api.callCount(); got != 1 {
		t.Fatalf("api calls after Changes = %d, want 1", got)
	}

	seq, err := session.Mail().FetchBodies(context.Background(), []string{"msg-1"})
	if err != nil {
		t.Fatalf("FetchBodies: %v", err)
	}
	var chunks []backend.BodyChunk
	for c := range seq {
		chunks = append(chunks, c)
	}
	if len(chunks) != 1 {
		t.Fatalf("chunks = %d, want 1", len(chunks))
	}
	if chunks[0].Err != nil {
		t.Fatalf("chunk error: %v", chunks[0].Err)
	}
	if string(chunks[0].Raw) != "raw message one" {
		t.Errorf("chunk raw = %q, want %q", chunks[0].Raw, "raw message one")
	}
	// downloadBlob's {type} and {name} both reach the wire (RFC 8620
	// sections 6.1 and 6.2's own template variables): message/rfc822
	// is what a downloaded message source actually is, in place of
	// go-jmap's own hardcoded application/octet-stream, and the empty
	// name is this call site's own choice, in place of go-jmap's
	// hardcoded "filename" placeholder. Neither changes the bytes
	// FetchBodies returns above; both are the Content-Type and
	// Content-Disposition the server would label the response with.
	if len(blobs.downloadRequests) != 1 {
		t.Fatalf("download requests = %d, want 1", len(blobs.downloadRequests))
	}
	if got := blobs.downloadRequests[0]; got.contentType != "message/rfc822" || got.name != "" {
		t.Errorf("download request = %+v, want contentType message/rfc822, name \"\"", got)
	}
	// FetchBodies resolves its own blobId with no session-lifetime
	// cache (SY-5 memory ceiling), so it costs its own Email/get call
	// even though Changes just hydrated the same message.
	if got := api.callCount(); got != 2 {
		t.Errorf("api calls after FetchBodies = %d, want 2", got)
	}
}

func TestCreateRenameDeleteMailbox(t *testing.T) {
	session, api := newTestSession(t,
		readFixture(t, "mailbox_create.json"),
		readFixture(t, "mailbox_rename.json"),
		readFixture(t, "mailbox_delete.json"),
	)

	id, err := session.Mail().CreateMailbox(context.Background(), "Projects", "")
	if err != nil {
		t.Fatalf("CreateMailbox: %v", err)
	}
	if id != "mbx-new" {
		t.Errorf("CreateMailbox id = %q, want mbx-new", id)
	}

	if err := session.Mail().RenameMailbox(context.Background(), "mbx-1", "Archive"); err != nil {
		t.Fatalf("RenameMailbox: %v", err)
	}

	if err := session.Mail().DeleteMailbox(context.Background(), "mbx-1"); err != nil {
		t.Fatalf("DeleteMailbox: %v", err)
	}

	if got := api.callCount(); got != 3 {
		t.Fatalf("api calls = %d, want 3", got)
	}
}

func TestSearch(t *testing.T) {
	session, api := newTestSession(t, readFixture(t, "search_response.json"))

	ids, err := session.Mail().Search(context.Background(), "invoice")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(ids) != 2 || ids[0] != "msg-1" || ids[1] != "msg-2" {
		t.Fatalf("Search ids = %v", ids)
	}
	_, args, _ := methodCall(t, api.requestAt(0), 0)
	filter, ok := args["filter"].(map[string]any)
	if !ok || filter["text"] != "invoice" {
		t.Errorf("Email/query filter = %v, want text=invoice", filter)
	}
}

func TestApplyBatchStateMismatch(t *testing.T) {
	session, _ := newTestSession(t, []byte(`{"methodResponses":[["error",{"type":"stateMismatch"},"0"]],"sessionState":"session-2"}`))

	_, err := session.Mail().ApplyBatch(context.Background(), []backend.Mutation{
		{Op: backend.MutationDestroy, ID: "msg-1"},
	})
	if !errors.Is(err, backend.ErrStateMismatch) {
		t.Fatalf("ApplyBatch error = %v, want backend.ErrStateMismatch", err)
	}
}
