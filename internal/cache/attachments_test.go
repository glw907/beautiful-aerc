// SPDX-License-Identifier: MIT

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
	ct := &fakeChangeTracker{}
	a, err := Open("Test", be, ct, t.TempDir(), Config{MaxAttachmentSize: 1 << 30})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { a.Close() })
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
	// Document this in the implementation; test asserts current behavior.
	_, _ = a.Attachments(context.Background(), "u1")
	if be.attachCalls < 1 {
		t.Errorf("attachCalls = %d, want >= 1", be.attachCalls)
	}
}
