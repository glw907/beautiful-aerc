// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/compose"
	"github.com/glw907/poplar/internal/mail"
)

func mustEnsureFolder(t *testing.T, a *Account, name string) {
	t.Helper()
	_, err := a.db.Exec(
		`INSERT OR IGNORE INTO folders (name, protocol_name) VALUES (?, ?)`,
		name, name)
	if err != nil {
		t.Fatalf("ensure folder %q: %v", name, err)
	}
}

func TestCreateLoadDraft(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	ctx := context.Background()

	if err := a.CreateDraft(ctx, "d1", []byte("payload-v1")); err != nil {
		t.Fatalf("CreateDraft v1: %v", err)
	}
	got, err := a.LoadDraft(ctx, "d1")
	if err != nil {
		t.Fatalf("LoadDraft: %v", err)
	}
	if string(got) != "payload-v1" {
		t.Errorf("LoadDraft = %q, want %q", got, "payload-v1")
	}

	if err := a.CreateDraft(ctx, "d1", []byte("payload-v2")); err != nil {
		t.Fatalf("CreateDraft v2: %v", err)
	}
	got, err = a.LoadDraft(ctx, "d1")
	if err != nil {
		t.Fatalf("LoadDraft after update: %v", err)
	}
	if string(got) != "payload-v2" {
		t.Errorf("LoadDraft after update = %q, want %q", got, "payload-v2")
	}
}

func TestLoadDraft_NotFound(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	_, err := a.LoadDraft(context.Background(), "nope")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("LoadDraft missing = %v, want sql.ErrNoRows", err)
	}
}

func TestListDrafts(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	ctx := context.Background()

	if err := a.CreateDraft(ctx, "d1", []byte("a")); err != nil {
		t.Fatalf("CreateDraft d1: %v", err)
	}
	if err := a.CreateDraft(ctx, "d2", []byte("b")); err != nil {
		t.Fatalf("CreateDraft d2: %v", err)
	}
	if err := a.MarkDraftPushed(ctx, "d2", mail.UID("server-id-99"), "Drafts"); err != nil {
		t.Fatalf("MarkDraftPushed: %v", err)
	}

	rows, err := a.ListDrafts(ctx)
	if err != nil {
		t.Fatalf("ListDrafts: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("ListDrafts len = %d, want 2", len(rows))
	}
	byID := map[string]DraftRow{rows[0].DraftID: rows[0], rows[1].DraftID: rows[1]}
	if got := byID["d1"]; got.ServerUID != "" || !got.Dirty {
		t.Errorf("d1 = %+v, want ServerUID empty + Dirty", got)
	}
	if got := byID["d2"]; got.ServerUID != "server-id-99" || got.Dirty {
		t.Errorf("d2 = %+v, want ServerUID=server-id-99 + !Dirty", got)
	}
}

func TestMarkDraftPushed_NotFound(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	err := a.MarkDraftPushed(context.Background(), "missing", mail.UID("x"), "Drafts")
	if !errors.Is(err, ErrDraftSuperseded) {
		t.Fatalf("MarkDraftPushed on missing draft = %v, want ErrDraftSuperseded", err)
	}
}

func TestDeleteDraft(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	ctx := context.Background()
	if err := a.CreateDraft(ctx, "d1", []byte("x")); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if err := a.DeleteDraft(ctx, "d1"); err != nil {
		t.Fatalf("DeleteDraft: %v", err)
	}
	_, err := a.LoadDraft(ctx, "d1")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("LoadDraft after delete = %v, want sql.ErrNoRows", err)
	}
}

func TestUpdateDraft_DeletedRow_NoOp(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	ctx := context.Background()

	if err := a.CreateDraft(ctx, "d1", []byte("v1")); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if err := a.DeleteDraft(ctx, "d1"); err != nil {
		t.Fatalf("DeleteDraft: %v", err)
	}
	// In-flight autosave on a deleted row must not resurrect it.
	if err := a.UpdateDraft(ctx, "d1", []byte("v2")); err != nil {
		t.Fatalf("UpdateDraft: %v", err)
	}
	if _, err := a.LoadDraft(ctx, "d1"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("UpdateDraft resurrected the row: err = %v, want sql.ErrNoRows", err)
	}
}

func TestQueuePushDraft(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	ctx := context.Background()

	mustEnsureFolder(t, a, "Drafts")

	if err := a.CreateDraft(ctx, "d1", []byte("encoded-draft")); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	opID, err := a.QueuePushDraft(ctx, "d1", "Drafts", []byte("MIME"), mail.UID(""))
	if err != nil {
		t.Fatalf("QueuePushDraft: %v", err)
	}
	if opID == 0 {
		t.Errorf("QueuePushDraft returned zero opID")
	}

	var kind, argsJSON string
	var payload []byte
	err = a.db.QueryRow(
		`SELECT kind, args, payload FROM outbox WHERE id = ?`, opID).
		Scan(&kind, &argsJSON, &payload)
	if err != nil {
		t.Fatalf("read outbox row: %v", err)
	}
	if kind != string(KindPushDraft) {
		t.Errorf("kind = %q, want %q", kind, KindPushDraft)
	}
	if string(payload) != "MIME" {
		t.Errorf("payload = %q, want MIME", payload)
	}
	args, err := decodeArgs(kind, argsJSON)
	if err != nil {
		t.Fatalf("decodeArgs: %v", err)
	}
	pd, ok := args.(PushDraftArgs)
	if !ok {
		t.Fatalf("decoded type = %T, want PushDraftArgs", args)
	}
	if pd.DraftID != "d1" {
		t.Errorf("DraftID = %q, want d1", pd.DraftID)
	}
	if pd.PrevServerUID != "" {
		t.Errorf("PrevServerUID = %q, want empty", pd.PrevServerUID)
	}
}

func TestDrainer_PushDraft_SupersededEmitsNote(t *testing.T) {
	be := &fakeBackend{
		folders:      []mail.Folder{{Name: "Drafts", Role: "drafts"}},
		pushDraftUID: mail.UID("server-99"),
	}
	a := openTestAccountWith(t, be)
	ctx := context.Background()

	// Queue a push for a draftID that has no local row. The drainer
	// will succeed at the backend, then MarkDraftPushed reports
	// ErrDraftSuperseded.
	if _, err := a.QueuePushDraft(ctx, "missing-id", "Drafts", []byte("MIME"), mail.UID("")); err != nil {
		t.Fatalf("QueuePushDraft: %v", err)
	}
	if err := a.StartDrainer(ctx); err != nil {
		t.Fatalf("StartDrainer: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case ev := <-a.Events():
			if ev.Kind != KindPushDraft {
				continue
			}
			if ev.Status != OpDone {
				t.Fatalf("status = %v, want OpDone (superseded counts as success)", ev.Status)
			}
			if ev.Note == "" {
				t.Fatal("Note should be populated for superseded draft")
			}
			return
		case <-deadline:
			t.Fatal("timed out waiting for superseded event")
		}
	}
}

func TestQueryFolder_DraftsLocalOnly(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	ctx := context.Background()

	mustEnsureFolder(t, a, "Drafts")
	if _, err := a.db.Exec(`UPDATE folders SET role = 'drafts' WHERE name = 'Drafts'`); err != nil {
		t.Fatalf("set drafts role: %v", err)
	}

	payload, err := compose.EncodeDraft(compose.Draft{Subject: "local-only"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := a.CreateDraft(ctx, "d-local", payload); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	msgs, total, err := a.QueryFolder("Drafts", 0, 100)
	if err != nil {
		t.Fatalf("QueryFolder: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1", len(msgs))
	}
	if string(msgs[0].UID) != "draft:d-local" {
		t.Errorf("UID = %q, want draft:d-local", msgs[0].UID)
	}
	if msgs[0].Subject != "local-only" {
		t.Errorf("Subject = %q, want local-only", msgs[0].Subject)
	}
}

func TestQueryFolder_DraftsPushedExcluded(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	ctx := context.Background()

	mustEnsureFolder(t, a, "Drafts")
	if _, err := a.db.Exec(`UPDATE folders SET role = 'drafts' WHERE name = 'Drafts'`); err != nil {
		t.Fatalf("set drafts role: %v", err)
	}

	payload, err := compose.EncodeDraft(compose.Draft{Subject: "pushed"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := a.CreateDraft(ctx, "d-pushed", payload); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if err := a.MarkDraftPushed(ctx, "d-pushed", mail.UID("server-99"), "Drafts"); err != nil {
		t.Fatalf("MarkDraftPushed: %v", err)
	}

	msgs, total, err := a.QueryFolder("Drafts", 0, 100)
	if err != nil {
		t.Fatalf("QueryFolder: %v", err)
	}
	if total != 0 || len(msgs) != 0 {
		t.Errorf("pushed draft should be excluded: total=%d len=%d", total, len(msgs))
	}
}
