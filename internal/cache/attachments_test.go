package cache

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// attachBackend extends fakeBackend with Attachments / FetchAttachment fixtures.
type attachBackend struct {
	fakeBackend
	atts        map[mail.UID][]mail.Attachment
	parts       map[string][]byte // key: uid + "::" + partID
	attachCalls int
	fetchCalls  int
	fetchErr    error
}

func (b *attachBackend) Attachments(uid mail.UID) ([]mail.Attachment, error) {
	b.attachCalls++
	return b.atts[uid], nil
}
func (b *attachBackend) FetchAttachment(uid mail.UID, partID string) ([]byte, error) {
	b.fetchCalls++
	if b.fetchErr != nil {
		return nil, b.fetchErr
	}
	return b.parts[string(uid)+"::"+partID], nil
}

func openAttachAccount(t *testing.T, be *attachBackend) *Account {
	t.Helper()
	be.fakeBackend.folders = []mail.Folder{{Name: "INBOX", Role: "inbox"}}
	a, err := Open("Test", t.TempDir(), Config{MaxAttachmentSize: 1 << 30}, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	if err := a.WireBackend(be, &fakeChangeTracker{}); err != nil {
		t.Fatalf("WireBackend: %v", err)
	}
	if err := a.SyncFolders(context.Background()); err != nil {
		t.Fatalf("SyncFolders: %v", err)
	}
	return a
}

func TestAttachments_LazyPopulate(t *testing.T) {
	be := &attachBackend{
		atts: map[mail.UID][]mail.Attachment{
			"u1": {
				{PartID: "2", Filename: "report.pdf", MIMEType: "application/pdf", Size: 1234, Disposition: mail.DispAttachment},
				{PartID: "3", Filename: "logo.png", MIMEType: "image/png", Size: 999, ContentID: "logo@x", Disposition: mail.DispInline},
			},
		},
	}
	a := openAttachAccount(t, be)
	seedMessage(t, a, "u1", time.Now())

	got, err := a.Attachments(context.Background(), "u1")
	if err != nil {
		t.Fatalf("Attachments: %v", err)
	}
	if !reflect.DeepEqual(got, be.atts["u1"]) {
		t.Fatalf("first call: got %+v want %+v", got, be.atts["u1"])
	}
	if be.attachCalls != 1 {
		t.Fatalf("attachCalls = %d, want 1", be.attachCalls)
	}

	got2, err := a.Attachments(context.Background(), "u1")
	if err != nil {
		t.Fatalf("Attachments (2nd): %v", err)
	}
	if !reflect.DeepEqual(got2, got) {
		t.Errorf("second call differs: %+v vs %+v", got2, got)
	}
	if be.attachCalls != 1 {
		t.Errorf("attachCalls = %d after 2nd call, want 1 (cache hit)", be.attachCalls)
	}
}

func TestAttachments_EmptyMessage(t *testing.T) {
	be := &attachBackend{atts: map[mail.UID][]mail.Attachment{"u1": nil}}
	a := openAttachAccount(t, be)
	seedMessage(t, a, "u1", time.Now())

	got, err := a.Attachments(context.Background(), "u1")
	if err != nil {
		t.Fatalf("Attachments: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
	// Second call still hits backend because we cannot distinguish
	// "populated, zero parts" from "not yet populated" without a marker.
	// Document this in the implementation. Test asserts current behavior.
	_, _ = a.Attachments(context.Background(), "u1")
	if be.attachCalls < 1 {
		t.Errorf("attachCalls = %d, want >= 1", be.attachCalls)
	}
}

func TestFetchAttachment_LazyPopulate(t *testing.T) {
	be := &attachBackend{
		atts: map[mail.UID][]mail.Attachment{
			"u1": {{PartID: "2", Filename: "r.pdf", MIMEType: "application/pdf", Size: 4, Disposition: mail.DispAttachment}},
		},
		parts: map[string][]byte{"u1::2": []byte("PDF!")},
	}
	a := openAttachAccount(t, be)
	seedMessage(t, a, "u1", time.Now())
	if _, err := a.Attachments(context.Background(), "u1"); err != nil {
		t.Fatalf("populate metadata: %v", err)
	}

	got, err := a.FetchAttachment(context.Background(), "u1", "2")
	if err != nil {
		t.Fatalf("FetchAttachment: %v", err)
	}
	if string(got) != "PDF!" {
		t.Errorf("got %q, want %q", got, "PDF!")
	}
	if be.fetchCalls != 1 {
		t.Fatalf("fetchCalls = %d, want 1", be.fetchCalls)
	}

	// Second call: cache hit, no backend roundtrip.
	got2, err := a.FetchAttachment(context.Background(), "u1", "2")
	if err != nil {
		t.Fatalf("FetchAttachment 2: %v", err)
	}
	if string(got2) != "PDF!" {
		t.Errorf("got2 %q, want %q", got2, "PDF!")
	}
	if be.fetchCalls != 1 {
		t.Errorf("fetchCalls = %d after 2nd call, want 1", be.fetchCalls)
	}
}

func TestAttachmentDownloadProgress(t *testing.T) {
	a := openTestAccount(t)

	if _, ok := a.AttachmentDownloadProgress(); ok {
		t.Fatal("idle: ok = true, want false")
	}

	a.BeginAttachmentDownload()
	if _, ok := a.AttachmentDownloadProgress(); !ok {
		t.Fatal("in-flight: ok = false, want true")
	}
	a.EndAttachmentDownload()
	if _, ok := a.AttachmentDownloadProgress(); ok {
		t.Fatal("after end: ok = true, want false")
	}
}

func TestSyncProgress(t *testing.T) {
	a := openTestAccount(t)

	if _, ok := a.SyncProgress(); ok {
		t.Fatal("idle: ok = true, want false")
	}
	a.BeginSync()
	if _, ok := a.SyncProgress(); !ok {
		t.Fatal("in-flight: ok = false, want true")
	}
	a.EndSync()
	if _, ok := a.SyncProgress(); ok {
		t.Fatal("after end: ok = true, want false")
	}
}

func TestFetchAttachment_EvictBySize(t *testing.T) {
	be := &attachBackend{
		atts: map[mail.UID][]mail.Attachment{
			"old":   {{PartID: "2", MIMEType: "application/octet-stream", Size: 100, Disposition: mail.DispAttachment}},
			"newer": {{PartID: "2", MIMEType: "application/octet-stream", Size: 100, Disposition: mail.DispAttachment}},
		},
		parts: map[string][]byte{
			"old::2":   make([]byte, 100),
			"newer::2": make([]byte, 100),
		},
	}
	be.fakeBackend.folders = []mail.Folder{{Name: "INBOX", Role: "inbox"}}
	a, err := Open("Test", t.TempDir(), Config{MaxAttachmentSize: 150}, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	if err := a.WireBackend(be, &fakeChangeTracker{}); err != nil {
		t.Fatalf("WireBackend: %v", err)
	}
	if err := a.SyncFolders(context.Background()); err != nil {
		t.Fatalf("SyncFolders: %v", err)
	}
	seedMessage(t, a, "old", time.Now().Add(-2*time.Hour))
	seedMessage(t, a, "newer", time.Now())
	if _, err := a.Attachments(context.Background(), "old"); err != nil {
		t.Fatalf("metadata old: %v", err)
	}
	if _, err := a.Attachments(context.Background(), "newer"); err != nil {
		t.Fatalf("metadata newer: %v", err)
	}

	if _, err := a.FetchAttachment(context.Background(), "old", "2"); err != nil {
		t.Fatalf("fetch old: %v", err)
	}
	if _, err := a.FetchAttachment(context.Background(), "newer", "2"); err != nil {
		t.Fatalf("fetch newer: %v", err)
	}

	// After both fetches, total = 200 > cap 150. Older row (by sent_at)
	// must have been evicted before the second insert. Total should be 100.
	var total int64
	if err := a.db.QueryRow(`SELECT COALESCE(SUM(length(bytes)), 0) FROM attachments WHERE bytes IS NOT NULL`).Scan(&total); err != nil {
		t.Fatalf("sum: %v", err)
	}
	if total != 100 {
		t.Errorf("total cached bytes = %d, want 100", total)
	}
}
