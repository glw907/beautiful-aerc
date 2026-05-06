// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/glw907/poplar/internal/mail"
)

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
