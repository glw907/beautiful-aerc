package mailimap

import (
	"context"
	"errors"
	"testing"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

func TestEncodeDecodeIMAPTokenRoundtrip(t *testing.T) {
	tok := encodeIMAPToken(0xDEADBEEF, 0x0102030405060708)
	uidval, maxuid := decodeIMAPToken(tok)
	if uidval != 0xDEADBEEF {
		t.Errorf("uidvalidity round-trip = %x, want DEADBEEF", uidval)
	}
	if maxuid != 0x0102030405060708 {
		t.Errorf("maxuid round-trip = %x, want 0102030405060708", maxuid)
	}
}

func TestChangesEncodesUIDValidityOnInitialSync(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true}
	cmd.folderSummary = map[string]mail.Folder{
		"INBOX": {Name: "INBOX", UIDValidity: 42},
	}
	cmd.searchFn = func(mail.SearchCriteria) ([]mail.UID, error) {
		return []mail.UID{"1", "2", "3"}, nil
	}
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}

	cs, tok, err := b.Changes(context.Background(), "INBOX", nil)
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	if len(cs.Added) != 3 {
		t.Errorf("Added = %d, want 3", len(cs.Added))
	}
	uidval, maxuid := decodeIMAPToken(tok)
	if uidval != 42 {
		t.Errorf("token uidvalidity = %d, want 42", uidval)
	}
	if maxuid != 3 {
		t.Errorf("token maxuid = %d, want 3", maxuid)
	}
}

func TestChangesUIDValidityMismatchReanchors(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true}
	cmd.folderSummary = map[string]mail.Folder{
		"INBOX": {Name: "INBOX", UIDValidity: 100},
	}
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}

	stale := encodeIMAPToken(99, 7)
	_, _, err := b.Changes(context.Background(), "INBOX", stale)
	if !errors.Is(err, mail.ErrCannotCalculateChanges) {
		t.Errorf("Changes on UIDVALIDITY mismatch = %v, want ErrCannotCalculateChanges", err)
	}
}

func TestChangesMatchingUIDValidityProceeds(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true}
	cmd.folderSummary = map[string]mail.Folder{
		"INBOX": {Name: "INBOX", UIDValidity: 100},
	}
	cmd.searchFn = func(mail.SearchCriteria) ([]mail.UID, error) {
		return []mail.UID{"5", "6", "7", "8"}, nil
	}
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}

	prev := encodeIMAPToken(100, 6)
	cs, _, err := b.Changes(context.Background(), "INBOX", prev)
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	if len(cs.Added) != 2 {
		t.Errorf("Added = %d, want 2 (uids 7, 8)", len(cs.Added))
	}
}
