// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"database/sql"
	"errors"
	"testing"

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

func TestUpsertLoadDraft(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	ctx := context.Background()

	if err := a.UpsertDraft(ctx, "d1", []byte("payload-v1")); err != nil {
		t.Fatalf("UpsertDraft v1: %v", err)
	}
	got, err := a.LoadDraft(ctx, "d1")
	if err != nil {
		t.Fatalf("LoadDraft: %v", err)
	}
	if string(got) != "payload-v1" {
		t.Errorf("LoadDraft = %q, want %q", got, "payload-v1")
	}

	if err := a.UpsertDraft(ctx, "d1", []byte("payload-v2")); err != nil {
		t.Fatalf("UpsertDraft v2: %v", err)
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

	if err := a.UpsertDraft(ctx, "d1", []byte("a")); err != nil {
		t.Fatalf("UpsertDraft d1: %v", err)
	}
	if err := a.UpsertDraft(ctx, "d2", []byte("b")); err != nil {
		t.Fatalf("UpsertDraft d2: %v", err)
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
	if err == nil {
		t.Fatal("MarkDraftPushed on missing draft = nil error, want non-nil")
	}
}

func TestDeleteDraft(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	ctx := context.Background()
	if err := a.UpsertDraft(ctx, "d1", []byte("x")); err != nil {
		t.Fatalf("UpsertDraft: %v", err)
	}
	if err := a.DeleteDraft(ctx, "d1"); err != nil {
		t.Fatalf("DeleteDraft: %v", err)
	}
	_, err := a.LoadDraft(ctx, "d1")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("LoadDraft after delete = %v, want sql.ErrNoRows", err)
	}
}

func TestQueuePushDraft(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	ctx := context.Background()

	mustEnsureFolder(t, a, "Drafts")

	if err := a.UpsertDraft(ctx, "d1", []byte("encoded-draft")); err != nil {
		t.Fatalf("UpsertDraft: %v", err)
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
	if err := a.UpsertDraft(ctx, "d-local", payload); err != nil {
		t.Fatalf("UpsertDraft: %v", err)
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
	if err := a.UpsertDraft(ctx, "d-pushed", payload); err != nil {
		t.Fatalf("UpsertDraft: %v", err)
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
