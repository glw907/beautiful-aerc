package jmap

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/glw907/poplar/internal/backend"
)

// fakeBlobs scripts the /upload/ and /download/ endpoints Submit and
// FetchBodies use, separate from /api's method-call batching.
type fakeBlobs struct {
	uploadResp []byte
	downloads  map[string][]byte

	mu       sync.Mutex
	uploaded [][]byte
}

func (b *fakeBlobs) handleUpload(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	b.mu.Lock()
	b.uploaded = append(b.uploaded, body)
	b.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b.uploadResp)
}

func (b *fakeBlobs) handleDownload(w http.ResponseWriter, r *http.Request) {
	blobID := strings.TrimPrefix(r.URL.Path, "/download/")
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
		{Op: backend.MutationUpdate, ID: "msg-1", Fields: map[string]any{"seen": true, "mailbox_ids": []string{"mbx-archive"}}},
		{Op: backend.MutationDestroy, ID: "msg-2"},
		{Op: backend.MutationCreate, CreationID: "c1", Fields: map[string]any{}},
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

	if _, failed := result.Failed["c1"]; !failed {
		t.Error("MutationCreate should fail with c1 in result.Failed")
	}
	if _, failed := result.Failed["msg-3"]; !failed {
		t.Error("server notUpdated msg-3 should surface in result.Failed")
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
	session, api := newTestSessionWithBlobs(t, blobs, readFixture(t, "changes_response.json"))

	// Populate the session's blobId cache via Changes so FetchBodies
	// for msg-1 needs no extra Email/get.
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
	if got := api.callCount(); got != 1 {
		t.Errorf("api calls after FetchBodies = %d, want 1 (blobId came from cache)", got)
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
