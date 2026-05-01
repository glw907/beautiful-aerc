// SPDX-License-Identifier: MIT

package mailjmap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"git.sr.ht/~rockorager/go-jmap"
	jmapmail "git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

func TestNew_AccountName(t *testing.T) {
	b := New(config.AccountConfig{Name: "alice@example.com"})
	if got := b.AccountName(); got != "alice@example.com" {
		t.Errorf("AccountName = %q, want %q", got, "alice@example.com")
	}
}

func TestBackend_DisconnectWithoutConnect(t *testing.T) {
	b := New(config.AccountConfig{Name: "alice"})
	if err := b.Disconnect(); err != nil {
		t.Fatalf("Disconnect on never-connected: %v", err)
	}
}

// newTestBackend returns a Backend wired to fake with the given folders
// pre-populated and session.PrimaryAccounts set to accountID.
func newTestBackend(fake *fakeClient, accountID string, folders map[string]folderEntry) *Backend {
	b := NewWithClient(config.AccountConfig{Name: "test"}, fake)
	b.session = &jmap.Session{
		PrimaryAccounts: map[jmap.URI]jmap.ID{
			jmapmail.URI: jmap.ID(accountID),
		},
	}
	if folders != nil {
		b.folders = folders
	}
	return b
}

// --- ListFolders ---

func TestListFolders_ReturnsCachedFolders(t *testing.T) {
	fake := &fakeClient{}
	folders := map[string]folderEntry{
		"Inbox": {id: "mb-1", folder: mail.Folder{Name: "Inbox"}},
		"Sent":  {id: "mb-2", folder: mail.Folder{Name: "Sent"}},
	}
	b := newTestBackend(fake, "acct-1", folders)

	got, err := b.ListFolders()
	if err != nil {
		t.Fatalf("ListFolders: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
	// No server calls should have been made.
	if len(fake.sent) != 0 {
		t.Errorf("unexpected server call with pre-populated folders")
	}
}

// --- OpenFolder ---

func TestOpenFolder(t *testing.T) {
	tests := []struct {
		name      string
		openName  string
		wantErr   bool
		wantCurr  string
	}{
		{
			name:     "known folder sets current",
			openName: "Inbox",
			wantErr:  false,
			wantCurr: "Inbox",
		},
		{
			name:     "unknown folder returns error",
			openName: "Nonexistent",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeClient{}
			folders := map[string]folderEntry{
				"Inbox": {id: "mb-1", folder: mail.Folder{Name: "Inbox"}},
			}
			b := newTestBackend(fake, "acct-1", folders)

			err := b.OpenFolder(tt.openName)
			if (err != nil) != tt.wantErr {
				t.Errorf("OpenFolder(%q) error = %v, wantErr %v", tt.openName, err, tt.wantErr)
			}
			if !tt.wantErr && b.current != tt.wantCurr {
				t.Errorf("current = %q, want %q", b.current, tt.wantCurr)
			}
		})
	}
}

// --- QueryFolder ---

func TestQueryFolder(t *testing.T) {
	tests := []struct {
		name      string
		folder    string
		offset    int
		limit     int
		respond   func(*jmap.Request) (*jmap.Response, error)
		wantUIDs  []mail.UID
		wantTotal int
		wantErr   bool
	}{
		{
			name:   "happy path returns UIDs and total",
			folder: "Inbox",
			offset: 0,
			limit:  50,
			respond: func(_ *jmap.Request) (*jmap.Response, error) {
				return fakeResponse(&jmap.Invocation{
					Name: "Email/query",
					Args: &email.QueryResponse{
						IDs:   []jmap.ID{"e-1", "e-2", "e-3"},
						Total: 42,
					},
				}), nil
			},
			wantUIDs:  []mail.UID{"e-1", "e-2", "e-3"},
			wantTotal: 42,
		},
		{
			name:    "unknown folder returns error",
			folder:  "Nonexistent",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeClient{respond: tt.respond}
			folders := map[string]folderEntry{
				"Inbox": {id: "mb-1", folder: mail.Folder{Name: "Inbox"}},
			}
			b := newTestBackend(fake, "acct-42", folders)

			uids, total, err := b.QueryFolder(tt.folder, tt.offset, tt.limit)
			if (err != nil) != tt.wantErr {
				t.Errorf("QueryFolder error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if total != tt.wantTotal {
				t.Errorf("total = %d, want %d", total, tt.wantTotal)
			}
			if len(uids) != len(tt.wantUIDs) {
				t.Fatalf("len(uids) = %d, want %d", len(uids), len(tt.wantUIDs))
			}
			for i, uid := range uids {
				if uid != tt.wantUIDs[i] {
					t.Errorf("uids[%d] = %q, want %q", i, uid, tt.wantUIDs[i])
				}
			}
		})
	}
}

func TestQueryFolder_RequestShape(t *testing.T) {
	var capturedReq *jmap.Request
	fake := &fakeClient{
		respond: func(req *jmap.Request) (*jmap.Response, error) {
			capturedReq = req
			return fakeResponse(&jmap.Invocation{
				Name: "Email/query",
				Args: &email.QueryResponse{
					IDs:   []jmap.ID{"e-1"},
					Total: 1,
				},
			}), nil
		},
	}
	folders := map[string]folderEntry{
		"Inbox": {id: "mb-1", folder: mail.Folder{Name: "Inbox"}},
	}
	b := newTestBackend(fake, "acct-42", folders)

	if _, _, err := b.QueryFolder("Inbox", 10, 25); err != nil {
		t.Fatalf("QueryFolder: %v", err)
	}

	if capturedReq == nil {
		t.Fatal("no request was sent")
	}
	if len(capturedReq.Calls) != 1 {
		t.Fatalf("calls count = %d, want 1", len(capturedReq.Calls))
	}

	inv := capturedReq.Calls[0]
	q, ok := inv.Args.(*email.Query)
	if !ok {
		t.Fatalf("invocation args type = %T, want *email.Query", inv.Args)
	}
	if q.Account != "acct-42" {
		t.Errorf("account = %q, want %q", q.Account, "acct-42")
	}
	fc, ok := q.Filter.(*email.FilterCondition)
	if !ok {
		t.Fatalf("filter type = %T, want *email.FilterCondition", q.Filter)
	}
	if fc.InMailbox != "mb-1" {
		t.Errorf("InMailbox = %q, want %q", fc.InMailbox, "mb-1")
	}
	if len(q.Sort) != 1 || q.Sort[0].Property != "receivedAt" || q.Sort[0].IsAscending {
		t.Errorf("unexpected sort: %+v", q.Sort)
	}
	if q.Position != 10 {
		t.Errorf("Position = %d, want 10", q.Position)
	}
	if q.Limit != 25 {
		t.Errorf("Limit = %d, want 25", q.Limit)
	}
	if !q.CalculateTotal {
		t.Error("CalculateTotal should be true")
	}
}

// --- FetchHeaders ---

func TestFetchHeaders_EmptySlice(t *testing.T) {
	fake := &fakeClient{}
	b := newTestBackend(fake, "acct-1", nil)

	got, err := b.FetchHeaders(nil)
	if err != nil {
		t.Fatalf("FetchHeaders(nil): %v", err)
	}
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
	if len(fake.sent) != 0 {
		t.Errorf("expected no RPC calls, got %d", len(fake.sent))
	}

	got, err = b.FetchHeaders([]mail.UID{})
	if err != nil {
		t.Fatalf("FetchHeaders([]): %v", err)
	}
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
	if len(fake.sent) != 0 {
		t.Errorf("expected no RPC calls, got %d", len(fake.sent))
	}
}

func TestFetchHeaders_TranslatesEmails(t *testing.T) {
	receivedAt1 := time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC)
	receivedAt2 := time.Date(2024, 3, 14, 8, 30, 0, 0, time.UTC)

	emails := []*email.Email{
		{
			// Email 1: seen + answered, with display name in From.
			ID:         "e-100",
			BlobID:     "blob-100",
			ThreadID:   "t-5",
			Subject:    "Hello world",
			From:       []*jmapmail.Address{{Name: "Alice Smith", Email: "alice@example.com"}},
			ReceivedAt: &receivedAt1,
			Keywords:   map[string]bool{"$seen": true, "$answered": true},
			Size:       2048,
			InReplyTo:  []string{"<parent-msg-id@example.com>"},
		},
		{
			// Email 2: unread + flagged, no display name in From.
			ID:         "e-101",
			BlobID:     "blob-101",
			ThreadID:   "t-5",
			Subject:    "Re: Hello world",
			From:       []*jmapmail.Address{{Email: "bob@example.com"}},
			ReceivedAt: &receivedAt2,
			Keywords:   map[string]bool{"$flagged": true},
			Size:       512,
			InReplyTo:  nil,
		},
	}

	fake := &fakeClient{
		respond: func(_ *jmap.Request) (*jmap.Response, error) {
			return fakeResponse(&jmap.Invocation{
				Name: "Email/get",
				Args: &email.GetResponse{List: emails},
			}), nil
		},
	}
	b := newTestBackend(fake, "acct-1", nil)

	uids := []mail.UID{"e-100", "e-101"}
	got, err := b.FetchHeaders(uids)
	if err != nil {
		t.Fatalf("FetchHeaders: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	// Email 1 assertions.
	m1 := got[0]
	if m1.UID != "e-100" {
		t.Errorf("m1.UID = %q, want %q", m1.UID, "e-100")
	}
	if m1.Subject != "Hello world" {
		t.Errorf("m1.Subject = %q, want %q", m1.Subject, "Hello world")
	}
	if m1.From != "Alice Smith" {
		t.Errorf("m1.From = %q, want %q", m1.From, "Alice Smith")
	}
	if !m1.SentAt.Equal(receivedAt1) {
		t.Errorf("m1.SentAt = %v, want %v", m1.SentAt, receivedAt1)
	}
	wantFlags1 := mail.FlagSeen | mail.FlagAnswered
	if m1.Flags != wantFlags1 {
		t.Errorf("m1.Flags = %b, want %b", m1.Flags, wantFlags1)
	}
	if m1.Size != 2048 {
		t.Errorf("m1.Size = %d, want 2048", m1.Size)
	}
	if m1.ThreadID != "t-5" {
		t.Errorf("m1.ThreadID = %q, want %q", m1.ThreadID, "t-5")
	}
	if m1.InReplyTo != "<parent-msg-id@example.com>" {
		t.Errorf("m1.InReplyTo = %q, want %q", m1.InReplyTo, "<parent-msg-id@example.com>")
	}

	// Email 2 assertions.
	m2 := got[1]
	if m2.UID != "e-101" {
		t.Errorf("m2.UID = %q, want %q", m2.UID, "e-101")
	}
	if m2.From != "bob@example.com" {
		t.Errorf("m2.From = %q, want %q (no display name fallback)", m2.From, "bob@example.com")
	}
	wantFlags2 := mail.FlagFlagged
	if m2.Flags != wantFlags2 {
		t.Errorf("m2.Flags = %b, want %b", m2.Flags, wantFlags2)
	}
	if m2.InReplyTo != "" {
		t.Errorf("m2.InReplyTo = %q, want empty", m2.InReplyTo)
	}

	// BlobIDs must be cached.
	b.mu.Lock()
	blob1 := b.blobIDs["e-100"]
	blob2 := b.blobIDs["e-101"]
	b.mu.Unlock()
	if blob1 != "blob-100" {
		t.Errorf("blobIDs[e-100] = %q, want %q", blob1, "blob-100")
	}
	if blob2 != "blob-101" {
		t.Errorf("blobIDs[e-101] = %q, want %q", blob2, "blob-101")
	}
}

func TestFetchHeaders_RequestShape(t *testing.T) {
	var capturedReq *jmap.Request
	fake := &fakeClient{
		respond: func(req *jmap.Request) (*jmap.Response, error) {
			capturedReq = req
			return fakeResponse(&jmap.Invocation{
				Name: "Email/get",
				Args: &email.GetResponse{
					List: []*email.Email{{ID: "e-1", BlobID: "blob-1"}},
				},
			}), nil
		},
	}
	b := newTestBackend(fake, "acct-42", nil)

	if _, err := b.FetchHeaders([]mail.UID{"e-1"}); err != nil {
		t.Fatalf("FetchHeaders: %v", err)
	}

	if capturedReq == nil {
		t.Fatal("no request was sent")
	}
	if len(capturedReq.Calls) != 1 {
		t.Fatalf("calls count = %d, want 1", len(capturedReq.Calls))
	}
	inv := capturedReq.Calls[0]
	g, ok := inv.Args.(*email.Get)
	if !ok {
		t.Fatalf("invocation args type = %T, want *email.Get", inv.Args)
	}
	if g.Account != "acct-42" {
		t.Errorf("account = %q, want %q", g.Account, "acct-42")
	}
	if len(g.IDs) != 1 || g.IDs[0] != "e-1" {
		t.Errorf("IDs = %v, want [e-1]", g.IDs)
	}
	// Verify all required header properties are present.
	propSet := make(map[string]bool, len(g.Properties))
	for _, p := range g.Properties {
		propSet[p] = true
	}
	for _, want := range headerProperties {
		if !propSet[want] {
			t.Errorf("missing required property %q", want)
		}
	}
}

// --- FetchBody ---

// newBodyTestBackend returns a Backend with a pre-seeded blobID for uid
// and a fake downloader that records call count.
func newBodyTestBackend(uid mail.UID, blobID string, dl func(string) ([]byte, error)) *Backend {
	b := newTestBackend(&fakeClient{}, "acct-1", nil)
	b.blobIDs[uid] = blobID
	b.downloadBlob = dl
	return b
}

func TestFetchBody_CacheMissAndHit(t *testing.T) {
	const uid mail.UID = "e-1"
	const blobID = "blob-1"
	body := []byte("hello world")

	var calls atomic.Int32
	dl := func(id string) ([]byte, error) {
		if id != blobID {
			t.Errorf("unexpected blobID %q", id)
		}
		calls.Add(1)
		return body, nil
	}

	b := newBodyTestBackend(uid, blobID, dl)

	// First call: cache miss → download.
	r1, err := b.FetchBody(uid)
	if err != nil {
		t.Fatalf("FetchBody first call: %v", err)
	}
	got1, _ := io.ReadAll(r1)
	if string(got1) != string(body) {
		t.Errorf("first read = %q, want %q", got1, body)
	}

	// Second call: cache hit → no download.
	r2, err := b.FetchBody(uid)
	if err != nil {
		t.Fatalf("FetchBody second call: %v", err)
	}
	got2, _ := io.ReadAll(r2)
	if string(got2) != string(body) {
		t.Errorf("second read = %q, want %q", got2, body)
	}

	if n := calls.Load(); n != 1 {
		t.Errorf("downloader called %d times, want 1", n)
	}
}

func TestFetchBody_SingleflightCollapse(t *testing.T) {
	const uid mail.UID = "e-2"
	const blobID = "blob-2"
	body := []byte("singleflight body")

	var calls atomic.Int32
	ready := make(chan struct{})
	dl := func(id string) ([]byte, error) {
		<-ready // block until all goroutines have called FetchBody
		calls.Add(1)
		return body, nil
	}

	b := newBodyTestBackend(uid, blobID, dl)

	const n = 10
	var wg sync.WaitGroup
	errs := make([]error, n)
	readers := make([]io.Reader, n)
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			readers[i], errs[i] = b.FetchBody(uid)
		}(i)
	}

	// Give goroutines time to enqueue in singleflight.
	time.Sleep(10 * time.Millisecond)
	close(ready)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}
	for i, r := range readers {
		got, _ := io.ReadAll(r)
		if string(got) != string(body) {
			t.Errorf("goroutine %d: read = %q, want %q", i, got, body)
		}
	}

	if n := calls.Load(); n != 1 {
		t.Errorf("downloader called %d times, want 1", n)
	}
}

func TestFetchBody_UnknownUID(t *testing.T) {
	b := newTestBackend(&fakeClient{}, "acct-1", nil)
	b.downloadBlob = func(_ string) ([]byte, error) {
		t.Fatal("downloader should not be called for unknown uid")
		return nil, nil
	}

	_, err := b.FetchBody("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown uid")
	}
}

// --- Push loop and emit tests ---

// TestHandleStateChange_Dedup verifies that calling handleStateChange
// twice with the same state only triggers the dispatcher once.
func TestHandleStateChange_Dedup(t *testing.T) {
	var calls atomic.Int32
	fake := &fakeClient{
		respond: func(req *jmap.Request) (*jmap.Response, error) {
			calls.Add(1)
			return fakeResponse(&jmap.Invocation{
				Name: "Email/changes",
				Args: &email.ChangesResponse{
					OldState: "s1",
					NewState: "s2",
				},
			}), nil
		},
	}
	b := newTestBackend(fake, "acct-1", nil)
	b.states["Email"] = "s1"

	b.handleStateChange("Email", "s2") // dispatches
	b.handleStateChange("Email", "s2") // same new state → dedup

	if n := calls.Load(); n != 1 {
		t.Errorf("dispatcher called %d times, want 1", n)
	}
}

// TestHandleStateChange_StatePreservedOnError verifies that b.states
// is not advanced when the dispatcher returns an error.
func TestHandleStateChange_StatePreservedOnError(t *testing.T) {
	fake := &fakeClient{
		respond: func(_ *jmap.Request) (*jmap.Response, error) {
			return nil, errors.New("rpc error")
		},
	}
	b := newTestBackend(fake, "acct-1", nil)
	b.states["Email"] = "s1"

	b.handleStateChange("Email", "s2")

	b.mu.Lock()
	got := b.states["Email"]
	b.mu.Unlock()
	if got != "s1" {
		t.Errorf("states[Email] = %q after error, want %q", got, "s1")
	}
}

// TestEmit_BufferFullDrop verifies that emit does not block or panic
// when the updates channel is full.
func TestEmit_BufferFullDrop(t *testing.T) {
	b := newTestBackend(&fakeClient{}, "acct-1", nil)
	// Fill the channel to capacity.
	for range updatesBuffer {
		b.updates <- mail.Update{Type: mail.UpdateNewMail}
	}
	// This must not block.
	done := make(chan struct{})
	go func() {
		b.emit(mail.Update{Type: mail.UpdateExpunge})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("emit blocked on full channel")
	}
}

// TestPushLoop_ConnReconnecting verifies that pushLoop emits
// ConnReconnecting when runEventSourceFunc returns an error.
func TestPushLoop_ConnReconnecting(t *testing.T) {
	b := newTestBackend(&fakeClient{}, "acct-1", nil)

	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan struct{}, 1)
	b.runEventSourceFunc = func(ctx context.Context) error {
		select {
		case first <- struct{}{}:
		default:
		}
		return errors.New("stream error")
	}

	b.pushDone = make(chan struct{})
	go b.pushLoop(ctx)

	// Wait for at least one iteration.
	<-first
	cancel()
	<-b.pushDone

	// Drain channel and look for ConnReconnecting.
	close(b.updates)
	var found bool
	for u := range b.updates {
		if u.Type == mail.UpdateConnState && u.ConnState == mail.ConnReconnecting {
			found = true
		}
	}
	if !found {
		t.Error("expected ConnReconnecting update, got none")
	}
}

// TestPushLoop_ConnConnected verifies that a successful
// runEventSourceFunc emits ConnConnected before blocking.
func TestPushLoop_ConnConnected(t *testing.T) {
	b := newTestBackend(&fakeClient{}, "acct-1", nil)

	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan struct{})
	b.runEventSourceFunc = func(ctx context.Context) error {
		b.emit(mail.Update{Type: mail.UpdateConnState, ConnState: mail.ConnConnected})
		close(ready)
		<-ctx.Done()
		return nil
	}

	b.pushDone = make(chan struct{})
	go b.pushLoop(ctx)

	<-ready
	cancel()
	<-b.pushDone

	close(b.updates)
	var found bool
	for u := range b.updates {
		if u.Type == mail.UpdateConnState && u.ConnState == mail.ConnConnected {
			found = true
		}
	}
	if !found {
		t.Error("expected ConnConnected update, got none")
	}
}

// TestDispatchEmailChanges_EmitsCorrectUpdates verifies that
// dispatchEmailChanges emits NewMail, FlagsChanged, and Expunge.
func TestDispatchEmailChanges_EmitsCorrectUpdates(t *testing.T) {
	fake := &fakeClient{
		respond: func(_ *jmap.Request) (*jmap.Response, error) {
			return fakeResponse(&jmap.Invocation{
				Name: "Email/changes",
				Args: &email.ChangesResponse{
					Created:   []jmap.ID{"e-new"},
					Updated:   []jmap.ID{"e-upd"},
					Destroyed: []jmap.ID{"e-del"},
				},
			}), nil
		},
	}
	b := newTestBackend(fake, "acct-1", nil)
	b.states["Email"] = "s0"

	if err := b.dispatchEmailChanges("s0"); err != nil {
		t.Fatalf("dispatchEmailChanges: %v", err)
	}

	// Collect synchronously-emitted updates (Created/Destroyed); Updated
	// also triggers an async refreshBlobIDs but we only check the channel.
	close(b.updates)
	typesSeen := map[mail.UpdateType]bool{}
	for u := range b.updates {
		typesSeen[u.Type] = true
	}
	for _, want := range []mail.UpdateType{mail.UpdateNewMail, mail.UpdateFlagsChanged, mail.UpdateExpunge} {
		if !typesSeen[want] {
			t.Errorf("missing update type %v", want)
		}
	}
}

// TestDispatchMailboxChanges_EmitsFolderInfo verifies that
// dispatchMailboxChanges emits UpdateFolderInfo for each affected mailbox.
func TestDispatchMailboxChanges_EmitsFolderInfo(t *testing.T) {
	fake := &fakeClient{
		respond: func(_ *jmap.Request) (*jmap.Response, error) {
			return fakeResponse(&jmap.Invocation{
				Name: "Mailbox/changes",
				Args: &mailbox.ChangesResponse{
					Updated: []jmap.ID{"mb-1"},
				},
			}), nil
		},
	}
	folders := map[string]folderEntry{
		"Inbox": {id: "mb-1", folder: mail.Folder{Name: "Inbox"}},
	}
	b := newTestBackend(fake, "acct-1", folders)
	b.states["Mailbox"] = "s0"

	if err := b.dispatchMailboxChanges("s0"); err != nil {
		t.Fatalf("dispatchMailboxChanges: %v", err)
	}

	close(b.updates)
	var found bool
	for u := range b.updates {
		if u.Type == mail.UpdateFolderInfo && u.Folder == "Inbox" {
			found = true
		}
	}
	if !found {
		t.Error("expected UpdateFolderInfo for Inbox, got none")
	}
}

// --- setKeyword / MarkRead / MarkAnswered ---

func TestSetKeyword_EmptyUIDs(t *testing.T) {
	fake := &fakeClient{}
	b := newTestBackend(fake, "acct-1", nil)

	if err := b.MarkRead(nil); err != nil {
		t.Errorf("MarkRead(nil): %v", err)
	}
	if err := b.MarkRead([]mail.UID{}); err != nil {
		t.Errorf("MarkRead([]): %v", err)
	}
	if len(fake.sent) != 0 {
		t.Errorf("expected no RPC calls, got %d", len(fake.sent))
	}
}

func TestMarkRead_RequestShape(t *testing.T) {
	var capturedReq *jmap.Request
	fake := &fakeClient{
		respond: func(req *jmap.Request) (*jmap.Response, error) {
			capturedReq = req
			return fakeResponse(&jmap.Invocation{
				Name:   "Email/set",
				CallID: "0",
				Args:   &email.SetResponse{},
			}), nil
		},
	}
	b := newTestBackend(fake, "acct-42", nil)

	uids := []mail.UID{"e-1", "e-2"}
	if err := b.MarkRead(uids); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}

	if capturedReq == nil {
		t.Fatal("no request sent")
	}
	if len(capturedReq.Calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(capturedReq.Calls))
	}
	s, ok := capturedReq.Calls[0].Args.(*email.Set)
	if !ok {
		t.Fatalf("args type = %T, want *email.Set", capturedReq.Calls[0].Args)
	}
	if s.Account != "acct-42" {
		t.Errorf("account = %q, want %q", s.Account, "acct-42")
	}
	if len(s.Update) != 2 {
		t.Fatalf("update len = %d, want 2", len(s.Update))
	}
	for _, uid := range uids {
		patch, ok := s.Update[jmap.ID(uid)]
		if !ok {
			t.Errorf("missing uid %q in Update", uid)
			continue
		}
		val, ok := patch["keywords/$seen"]
		if !ok {
			t.Errorf("uid %q: missing keywords/$seen", uid)
		}
		if val != true {
			t.Errorf("uid %q: keywords/$seen = %v, want true", uid, val)
		}
	}
}

func TestMarkUnread_EmptyUIDs(t *testing.T) {
	fake := &fakeClient{}
	b := newTestBackend(fake, "acct-1", nil)

	if err := b.MarkUnread(nil); err != nil {
		t.Errorf("MarkUnread(nil): %v", err)
	}
	if err := b.MarkUnread([]mail.UID{}); err != nil {
		t.Errorf("MarkUnread([]): %v", err)
	}
	if len(fake.sent) != 0 {
		t.Errorf("expected no RPC calls, got %d", len(fake.sent))
	}
}

func TestMarkUnread_RequestShape(t *testing.T) {
	var capturedReq *jmap.Request
	fake := &fakeClient{
		respond: func(req *jmap.Request) (*jmap.Response, error) {
			capturedReq = req
			return fakeResponse(&jmap.Invocation{
				Name:   "Email/set",
				CallID: "0",
				Args:   &email.SetResponse{},
			}), nil
		},
	}
	b := newTestBackend(fake, "acct-42", nil)

	uids := []mail.UID{"e-1", "e-2"}
	if err := b.MarkUnread(uids); err != nil {
		t.Fatalf("MarkUnread: %v", err)
	}

	if capturedReq == nil {
		t.Fatal("no request sent")
	}
	if len(capturedReq.Calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(capturedReq.Calls))
	}
	s, ok := capturedReq.Calls[0].Args.(*email.Set)
	if !ok {
		t.Fatalf("args type = %T, want *email.Set", capturedReq.Calls[0].Args)
	}
	if s.Account != "acct-42" {
		t.Errorf("account = %q, want %q", s.Account, "acct-42")
	}
	if len(s.Update) != 2 {
		t.Fatalf("update len = %d, want 2", len(s.Update))
	}
	for _, uid := range uids {
		patch, ok := s.Update[jmap.ID(uid)]
		if !ok {
			t.Errorf("missing uid %q in Update", uid)
			continue
		}
		val, ok := patch["keywords/$seen"]
		if !ok {
			t.Errorf("uid %q: missing keywords/$seen", uid)
			continue
		}
		if val != nil {
			t.Errorf("uid %q: keywords/$seen = %v, want nil (unset)", uid, val)
		}
	}
}

func TestSetKeyword_NotUpdatedError(t *testing.T) {
	fake := &fakeClient{
		respond: func(_ *jmap.Request) (*jmap.Response, error) {
			return fakeResponse(&jmap.Invocation{
				Name:   "Email/set",
				CallID: "0",
				Args: &email.SetResponse{
					NotUpdated: map[jmap.ID]*jmap.SetError{
						"e-1": {Type: "notFound"},
					},
				},
			}), nil
		},
	}
	b := newTestBackend(fake, "acct-1", nil)

	err := b.MarkRead([]mail.UID{"e-1"})
	if err == nil {
		t.Fatal("expected error from NotUpdated, got nil")
	}
}

func TestMarkAnswered_RequestShape(t *testing.T) {
	var capturedReq *jmap.Request
	fake := &fakeClient{
		respond: func(req *jmap.Request) (*jmap.Response, error) {
			capturedReq = req
			return fakeResponse(&jmap.Invocation{
				Name:   "Email/set",
				CallID: "0",
				Args:   &email.SetResponse{},
			}), nil
		},
	}
	b := newTestBackend(fake, "acct-1", nil)

	if err := b.MarkAnswered([]mail.UID{"e-5"}); err != nil {
		t.Fatalf("MarkAnswered: %v", err)
	}

	s, ok := capturedReq.Calls[0].Args.(*email.Set)
	if !ok {
		t.Fatalf("args type = %T", capturedReq.Calls[0].Args)
	}
	patch := s.Update[jmap.ID("e-5")]
	if patch["keywords/$answered"] != true {
		t.Errorf("keywords/$answered = %v, want true", patch["keywords/$answered"])
	}
}

// --- Flag ---

func TestFlag_KeywordMapping(t *testing.T) {
	tests := []struct {
		flag            mail.Flag
		set             bool
		wantKeyword     string
		wantVal         interface{}
	}{
		{mail.FlagSeen, true, "keywords/$seen", true},
		{mail.FlagSeen, false, "keywords/$seen", nil},
		{mail.FlagFlagged, true, "keywords/$flagged", true},
		{mail.FlagAnswered, true, "keywords/$answered", true},
		{mail.FlagDraft, true, "keywords/$draft", true},
		{mail.FlagForwarded, true, "keywords/$forwarded", true},
	}

	for _, tt := range tests {
		name := fmt.Sprintf("flag=%d set=%v", tt.flag, tt.set)
		t.Run(name, func(t *testing.T) {
			var capturedReq *jmap.Request
			fake := &fakeClient{
				respond: func(req *jmap.Request) (*jmap.Response, error) {
					capturedReq = req
					return fakeResponse(&jmap.Invocation{
						Name:   "Email/set",
						CallID: "0",
						Args:   &email.SetResponse{},
					}), nil
				},
			}
			b := newTestBackend(fake, "acct-1", nil)
			if err := b.Flag([]mail.UID{"e-1"}, tt.flag, tt.set); err != nil {
				t.Fatalf("Flag: %v", err)
			}
			s, ok := capturedReq.Calls[0].Args.(*email.Set)
			if !ok {
				t.Fatalf("args type = %T", capturedReq.Calls[0].Args)
			}
			patch := s.Update[jmap.ID("e-1")]
			val := patch[tt.wantKeyword]
			if val != tt.wantVal {
				t.Errorf("%s = %v, want %v", tt.wantKeyword, val, tt.wantVal)
			}
		})
	}
}

func TestFlag_UnsupportedFlag(t *testing.T) {
	fake := &fakeClient{}
	b := newTestBackend(fake, "acct-1", nil)

	err := b.Flag([]mail.UID{"e-1"}, mail.FlagRecent, true)
	if err == nil {
		t.Fatal("expected error for unsupported flag")
	}
	if len(fake.sent) != 0 {
		t.Errorf("expected no RPC for unsupported flag, got %d", len(fake.sent))
	}
}

// --- Move ---

func TestMove_EmptyUIDs(t *testing.T) {
	fake := &fakeClient{}
	folders := map[string]folderEntry{
		"Sent": {id: "mb-sent", folder: mail.Folder{Name: "Sent"}},
	}
	b := newTestBackend(fake, "acct-1", folders)

	if err := b.Move(nil, "Sent"); err != nil {
		t.Errorf("Move(nil): %v", err)
	}
	if len(fake.sent) != 0 {
		t.Errorf("expected no RPC, got %d", len(fake.sent))
	}
}

func TestMove_RequestShape(t *testing.T) {
	var capturedReq *jmap.Request
	fake := &fakeClient{
		respond: func(req *jmap.Request) (*jmap.Response, error) {
			capturedReq = req
			return fakeResponse(&jmap.Invocation{
				Name:   "Email/set",
				CallID: "0",
				Args:   &email.SetResponse{},
			}), nil
		},
	}
	folders := map[string]folderEntry{
		"Archive": {id: "mb-arch", folder: mail.Folder{Name: "Archive"}},
	}
	b := newTestBackend(fake, "acct-1", folders)

	if err := b.Move([]mail.UID{"e-1", "e-2"}, "Archive"); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if capturedReq == nil {
		t.Fatal("no request sent")
	}
	s, ok := capturedReq.Calls[0].Args.(*email.Set)
	if !ok {
		t.Fatalf("args type = %T", capturedReq.Calls[0].Args)
	}
	if len(s.Update) != 2 {
		t.Errorf("update len = %d, want 2", len(s.Update))
	}
	patch := s.Update[jmap.ID("e-1")]
	mboxIDs, ok := patch["mailboxIds"].(map[string]bool)
	if !ok {
		t.Fatalf("mailboxIds type = %T", patch["mailboxIds"])
	}
	if !mboxIDs["mb-arch"] {
		t.Errorf("mailboxIds[mb-arch] = false, want true")
	}
}

func TestMove_UnknownFolder(t *testing.T) {
	fake := &fakeClient{}
	b := newTestBackend(fake, "acct-1", nil)

	err := b.Move([]mail.UID{"e-1"}, "Nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown folder")
	}
}

// --- Delete ---

func TestDelete_EmptyUIDs(t *testing.T) {
	fake := &fakeClient{}
	folders := map[string]folderEntry{
		"Trash": {id: "mb-trash", folder: mail.Folder{Name: "Trash"}},
	}
	b := newTestBackend(fake, "acct-1", folders)
	if err := b.Delete(nil); err != nil {
		t.Errorf("Delete(nil): %v", err)
	}
	if len(fake.sent) != 0 {
		t.Errorf("expected no RPC, got %d", len(fake.sent))
	}
}

func TestDelete_MovesToTrash(t *testing.T) {
	var capturedReq *jmap.Request
	fake := &fakeClient{
		respond: func(req *jmap.Request) (*jmap.Response, error) {
			capturedReq = req
			return fakeResponse(&jmap.Invocation{
				Name:   "Email/set",
				CallID: "0",
				Args:   &email.SetResponse{},
			}), nil
		},
	}
	folders := map[string]folderEntry{
		"Trash": {id: "mb-trash", folder: mail.Folder{Name: "Trash"}},
	}
	b := newTestBackend(fake, "acct-1", folders)

	if err := b.Delete([]mail.UID{"e-10"}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if capturedReq == nil {
		t.Fatal("no request sent")
	}
	s, ok := capturedReq.Calls[0].Args.(*email.Set)
	if !ok {
		t.Fatalf("args type = %T", capturedReq.Calls[0].Args)
	}
	patch := s.Update[jmap.ID("e-10")]
	mboxIDs, ok := patch["mailboxIds"].(map[string]bool)
	if !ok {
		t.Fatalf("mailboxIds type = %T, want map[string]bool", patch["mailboxIds"])
	}
	if !mboxIDs["mb-trash"] {
		t.Errorf("expected e-10 moved to mb-trash")
	}
}

func TestDelete_NoTrashFolder(t *testing.T) {
	fake := &fakeClient{}
	b := newTestBackend(fake, "acct-1", nil) // no Trash folder
	err := b.Delete([]mail.UID{"e-1"})
	if err == nil {
		t.Fatal("expected error when Trash not found")
	}
}

// --- Copy ---

func TestCopy_EmptyUIDs(t *testing.T) {
	fake := &fakeClient{}
	folders := map[string]folderEntry{
		"Archive": {id: "mb-arch", folder: mail.Folder{Name: "Archive"}},
	}
	b := newTestBackend(fake, "acct-1", folders)
	if err := b.Copy(nil, "Archive"); err != nil {
		t.Errorf("Copy(nil): %v", err)
	}
	if len(fake.sent) != 0 {
		t.Errorf("expected no RPC, got %d", len(fake.sent))
	}
}

func TestCopy_RequestShape(t *testing.T) {
	var capturedReq *jmap.Request
	fake := &fakeClient{
		respond: func(req *jmap.Request) (*jmap.Response, error) {
			capturedReq = req
			return fakeResponse(&jmap.Invocation{
				Name:   "Email/set",
				CallID: "0",
				Args:   &email.SetResponse{},
			}), nil
		},
	}
	folders := map[string]folderEntry{
		"Archive": {id: "mb-arch", folder: mail.Folder{Name: "Archive"}},
	}
	b := newTestBackend(fake, "acct-1", folders)
	b.blobIDs["e-1"] = "blob-e1"

	if err := b.Copy([]mail.UID{"e-1"}, "Archive"); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if capturedReq == nil {
		t.Fatal("no request sent")
	}
	s, ok := capturedReq.Calls[0].Args.(*email.Set)
	if !ok {
		t.Fatalf("args type = %T", capturedReq.Calls[0].Args)
	}
	if len(s.Create) != 1 {
		t.Fatalf("create len = %d, want 1", len(s.Create))
	}
	for _, e := range s.Create {
		if e.BlobID != "blob-e1" {
			t.Errorf("BlobID = %q, want %q", e.BlobID, "blob-e1")
		}
		if !e.MailboxIDs["mb-arch"] {
			t.Errorf("MailboxIDs[mb-arch] = false, want true")
		}
	}
}

func TestCopy_UnknownBlobID(t *testing.T) {
	fake := &fakeClient{}
	folders := map[string]folderEntry{
		"Archive": {id: "mb-arch", folder: mail.Folder{Name: "Archive"}},
	}
	b := newTestBackend(fake, "acct-1", folders)
	// blobIDs not populated

	err := b.Copy([]mail.UID{"e-missing"}, "Archive")
	if err == nil {
		t.Fatal("expected error for missing blob")
	}
	if len(fake.sent) != 0 {
		t.Errorf("expected no RPC for missing blob, got %d", len(fake.sent))
	}
}

// --- Send ---

func TestSend_ReturnsNotImplemented(t *testing.T) {
	b := newTestBackend(&fakeClient{}, "acct-1", nil)
	err := b.Send("from@example.com", []string{"to@example.com"}, nil)
	if err == nil {
		t.Fatal("expected error from Send stub")
	}
}

// --- Search ---

func TestSearch_ReturnsNilNil(t *testing.T) {
	b := newTestBackend(&fakeClient{}, "acct-1", nil)
	uids, err := b.Search(mail.SearchCriteria{})
	if err != nil {
		t.Errorf("Search: unexpected error: %v", err)
	}
	if uids != nil {
		t.Errorf("Search: got %v, want nil", uids)
	}
}

// --- Destroy ---

func TestDestroy_EmptyUIDs(t *testing.T) {
	fake := &fakeClient{}
	b := newTestBackend(fake, "acct-1", nil)

	if err := b.Destroy(nil); err != nil {
		t.Errorf("Destroy(nil): %v", err)
	}
	if err := b.Destroy([]mail.UID{}); err != nil {
		t.Errorf("Destroy([]): %v", err)
	}
	if len(fake.sent) != 0 {
		t.Errorf("expected no RPC calls for empty input, got %d", len(fake.sent))
	}
}

func TestDestroy_RequestShape(t *testing.T) {
	var capturedReq *jmap.Request
	fake := &fakeClient{
		respond: func(req *jmap.Request) (*jmap.Response, error) {
			capturedReq = req
			return fakeResponse(&jmap.Invocation{
				Name:   "Email/set",
				CallID: "0",
				Args:   &email.SetResponse{},
			}), nil
		},
	}
	b := newTestBackend(fake, "acct-42", nil)

	uids := []mail.UID{"id-1", "id-2"}
	if err := b.Destroy(uids); err != nil {
		t.Fatalf("Destroy: %v", err)
	}

	if capturedReq == nil {
		t.Fatal("no request sent")
	}
	if len(capturedReq.Calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(capturedReq.Calls))
	}
	s, ok := capturedReq.Calls[0].Args.(*email.Set)
	if !ok {
		t.Fatalf("args type = %T, want *email.Set", capturedReq.Calls[0].Args)
	}
	if s.Account != "acct-42" {
		t.Errorf("account = %q, want %q", s.Account, "acct-42")
	}
	if len(s.Destroy) != 2 {
		t.Fatalf("Destroy len = %d, want 2", len(s.Destroy))
	}
	wantIDs := map[jmap.ID]bool{"id-1": true, "id-2": true}
	for _, id := range s.Destroy {
		if !wantIDs[id] {
			t.Errorf("unexpected ID in Destroy: %q", id)
		}
	}
}

func TestDestroy_NotDestroyedError(t *testing.T) {
	fake := &fakeClient{
		respond: func(_ *jmap.Request) (*jmap.Response, error) {
			return fakeResponse(&jmap.Invocation{
				Name:   "Email/set",
				CallID: "0",
				Args: &email.SetResponse{
					NotDestroyed: map[jmap.ID]*jmap.SetError{
						"id-1": {Type: "serverFail"},
					},
				},
			}), nil
		},
	}
	b := newTestBackend(fake, "acct-1", nil)

	err := b.Destroy([]mail.UID{"id-1"})
	if err == nil {
		t.Fatal("expected error from NotDestroyed, got nil")
	}
}

func TestDestroy_NotFoundIsSuccess(t *testing.T) {
	// notFound in NotDestroyed is idempotent — message is already gone.
	fake := &fakeClient{
		respond: func(_ *jmap.Request) (*jmap.Response, error) {
			return fakeResponse(&jmap.Invocation{
				Name:   "Email/set",
				CallID: "0",
				Args: &email.SetResponse{
					NotDestroyed: map[jmap.ID]*jmap.SetError{
						"id-gone": {Type: "notFound"},
					},
				},
			}), nil
		},
	}
	b := newTestBackend(fake, "acct-1", nil)

	if err := b.Destroy([]mail.UID{"id-gone"}); err != nil {
		t.Errorf("Destroy with notFound: expected success, got %v", err)
	}
}
