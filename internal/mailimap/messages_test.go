// SPDX-License-Identifier: MIT

package mailimap

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

// connectFake returns a fully-connected Backend backed by cmd.
func connectFake(t *testing.T, cmd *fakeClient) *Backend {
	t.Helper()
	idle := newFakeClient()
	idle.caps = cmd.caps
	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("finishConnect: %v", err)
	}
	return b
}

func baseCaps() map[string]bool {
	return map[string]bool{"IMAP4REV1": true, "UIDPLUS": true}
}

// --- QueryFolder ---

func TestQueryFolderEmptyFolder(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = baseCaps()
	cmd.folderSummary = map[string]mail.Folder{
		"INBOX": {Name: "INBOX", Exists: 0},
	}
	// No search results.
	cmd.searchFn = func(mail.SearchCriteria) ([]mail.UID, error) { return nil, nil }

	b := connectFake(t, cmd)
	uids, total, err := b.QueryFolder("INBOX", 0, 20)
	if err != nil {
		t.Fatalf("QueryFolder: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if len(uids) != 0 {
		t.Errorf("uids = %v, want empty", uids)
	}
}

func TestQueryFolderPaginationNewestFirst(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = baseCaps()
	cmd.folderSummary = map[string]mail.Folder{
		"INBOX": {Name: "INBOX", Exists: 5},
	}
	// Simulate five messages with UIDs 1..5; Search ALL returns them in
	// whatever order — implementation must sort and return newest-first.
	cmd.searchFn = func(mail.SearchCriteria) ([]mail.UID, error) {
		return []mail.UID{"1", "3", "2", "5", "4"}, nil
	}

	b := connectFake(t, cmd)

	// Page 1: first 3 (offset 0).
	uids, total, err := b.QueryFolder("INBOX", 0, 3)
	if err != nil {
		t.Fatalf("QueryFolder page1: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(uids) != 3 {
		t.Fatalf("len(uids) = %d, want 3", len(uids))
	}
	// Newest-first: 5, 4, 3.
	want := []mail.UID{"5", "4", "3"}
	for i, u := range uids {
		if u != want[i] {
			t.Errorf("uids[%d] = %q, want %q", i, u, want[i])
		}
	}

	// Page 2: next 3 (offset 3), only 2 remain.
	uids2, total2, err := b.QueryFolder("INBOX", 3, 3)
	if err != nil {
		t.Fatalf("QueryFolder page2: %v", err)
	}
	if total2 != 5 {
		t.Errorf("total = %d, want 5", total2)
	}
	if len(uids2) != 2 {
		t.Fatalf("len(uids2) = %d, want 2", len(uids2))
	}
	want2 := []mail.UID{"2", "1"}
	for i, u := range uids2 {
		if u != want2[i] {
			t.Errorf("uids2[%d] = %q, want %q", i, u, want2[i])
		}
	}
}

func TestQueryFolderOffsetBeyondEnd(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = baseCaps()
	cmd.folderSummary = map[string]mail.Folder{
		"Box": {Name: "Box", Exists: 2},
	}
	cmd.searchFn = func(mail.SearchCriteria) ([]mail.UID, error) {
		return []mail.UID{"1", "2"}, nil
	}

	b := connectFake(t, cmd)
	uids, total, err := b.QueryFolder("Box", 10, 5)
	if err != nil {
		t.Fatalf("QueryFolder: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(uids) != 0 {
		t.Errorf("uids = %v, want empty", uids)
	}
}

// --- FetchHeaders ---

func TestFetchHeadersPopulatesMessageInfo(t *testing.T) {
	sentAt := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)

	cmd := newFakeClient()
	cmd.caps = baseCaps()
	cmd.fetchFn = func(uids []mail.UID, items []string, fn func(mail.UID, map[string]any)) error {
		for _, uid := range uids {
			fn(uid, map[string]any{
				"subject":    "Test Subject",
				"from":       "Alice <alice@example.com>",
				"to":         "Bob <bob@example.com>",
				"cc":         "",
				"bcc":        "",
				"date":       "Wed, 01 Apr 2026 10:00:00 +0000",
				"sentAt":     sentAt,
				"flags":      mail.FlagSeen,
				"size":       uint32(1024),
				"in-reply-to": "",
				"references": "",
				"message-id": string(uid),
			})
		}
		return nil
	}

	idle := newFakeClient()
	idle.caps = cmd.caps
	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}

	infos, err := b.FetchHeaders([]mail.UID{"42"})
	if err != nil {
		t.Fatalf("FetchHeaders: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("len = %d, want 1", len(infos))
	}
	m := infos[0]
	if m.UID != "42" {
		t.Errorf("UID = %q, want 42", m.UID)
	}
	if m.Subject != "Test Subject" {
		t.Errorf("Subject = %q, want 'Test Subject'", m.Subject)
	}
	if m.From != "Alice <alice@example.com>" {
		t.Errorf("From = %q", m.From)
	}
	if m.To != "Bob <bob@example.com>" {
		t.Errorf("To = %q", m.To)
	}
	if !m.SentAt.Equal(sentAt) {
		t.Errorf("SentAt = %v, want %v", m.SentAt, sentAt)
	}
	if m.Flags&mail.FlagSeen == 0 {
		t.Errorf("FlagSeen not set")
	}
	if m.Size != 1024 {
		t.Errorf("Size = %d, want 1024", m.Size)
	}
}

func TestFetchHeadersEmptyUIDs(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = baseCaps()
	// fetchFn should not be called for an empty uid list.
	called := false
	cmd.fetchFn = func(uids []mail.UID, items []string, fn func(mail.UID, map[string]any)) error {
		called = true
		return nil
	}

	idle := newFakeClient()
	idle.caps = cmd.caps
	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}

	infos, err := b.FetchHeaders(nil)
	if err != nil {
		t.Fatalf("FetchHeaders: %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("want empty slice, got %v", infos)
	}
	if called {
		t.Errorf("Fetch called on empty uid list")
	}
}

func TestFetchHeadersThreadingFields(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = baseCaps()
	cmd.fetchFn = func(uids []mail.UID, items []string, fn func(mail.UID, map[string]any)) error {
		fn("10", map[string]any{
			"subject":     "Re: Hello",
			"from":        "Bob <bob@example.com>",
			"to":          "",
			"cc":          "",
			"bcc":         "",
			"date":        "",
			"sentAt":      time.Time{},
			"flags":       mail.Flag(0),
			"size":        uint32(0),
			"in-reply-to": "5",
			"references":  "3 5",
			"message-id":  "10",
		})
		return nil
	}

	idle := newFakeClient()
	idle.caps = cmd.caps
	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}

	infos, err := b.FetchHeaders([]mail.UID{"10"})
	if err != nil {
		t.Fatalf("FetchHeaders: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("len = %d, want 1", len(infos))
	}
	m := infos[0]
	if m.InReplyTo != "5" {
		t.Errorf("InReplyTo = %q, want '5'", m.InReplyTo)
	}
}

// --- FetchBody ---

func TestFetchBodyReturnsReader(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = baseCaps()
	cmd.bodies[mail.UID("7")] = "Hello, world!"

	idle := newFakeClient()
	idle.caps = cmd.caps
	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}

	r, err := b.FetchBody("7")
	if err != nil {
		t.Fatalf("FetchBody: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(data) != "Hello, world!" {
		t.Errorf("body = %q, want 'Hello, world!'", string(data))
	}
}

func TestFetchBodyMissingUID(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = baseCaps()
	// bodies map is empty.

	idle := newFakeClient()
	idle.caps = cmd.caps
	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}

	_, err := b.FetchBody("99")
	if err == nil {
		t.Fatal("expected error for missing UID, got nil")
	}
}

// --- Search ---

func TestSearchTranslatesCriteria(t *testing.T) {
	var capturedCriteria mail.SearchCriteria

	cmd := newFakeClient()
	cmd.caps = baseCaps()
	cmd.searchFn = func(c mail.SearchCriteria) ([]mail.UID, error) {
		capturedCriteria = c
		return []mail.UID{"3", "7"}, nil
	}

	idle := newFakeClient()
	idle.caps = cmd.caps
	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}

	criteria := mail.SearchCriteria{
		Body: []string{"quarterly report"},
		Text: []string{"Alice"},
	}
	uids, err := b.Search(criteria)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(uids) != 2 {
		t.Errorf("len(uids) = %d, want 2", len(uids))
	}
	if len(capturedCriteria.Body) == 0 || capturedCriteria.Body[0] != "quarterly report" {
		t.Errorf("criteria.Body not forwarded: %v", capturedCriteria.Body)
	}
	if len(capturedCriteria.Text) == 0 || capturedCriteria.Text[0] != "Alice" {
		t.Errorf("criteria.Text not forwarded: %v", capturedCriteria.Text)
	}
}

func TestSearchEmptyResult(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = baseCaps()
	cmd.searchFn = func(mail.SearchCriteria) ([]mail.UID, error) { return nil, nil }

	idle := newFakeClient()
	idle.caps = cmd.caps
	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}

	uids, err := b.Search(mail.SearchCriteria{Body: []string{"no-match"}})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(uids) != 0 {
		t.Errorf("uids = %v, want empty", uids)
	}
}

func TestSearchHeaderCriteria(t *testing.T) {
	var capturedCriteria mail.SearchCriteria

	cmd := newFakeClient()
	cmd.caps = baseCaps()
	cmd.searchFn = func(c mail.SearchCriteria) ([]mail.UID, error) {
		capturedCriteria = c
		return []mail.UID{"1"}, nil
	}

	idle := newFakeClient()
	idle.caps = cmd.caps
	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}

	criteria := mail.SearchCriteria{
		Header: map[string][]string{
			"From": {"alice@example.com"},
		},
	}
	_, err := b.Search(criteria)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if froms, ok := capturedCriteria.Header["From"]; !ok || len(froms) == 0 {
		t.Errorf("Header criteria not forwarded: %v", capturedCriteria.Header)
	}
}

// infoFromFetch white-box tests.

func TestInfoFromFetchDefaults(t *testing.T) {
	// A completely empty items map should produce an info with UID set
	// and zero values for everything else — no panic.
	info := infoFromFetch("99", map[string]any{})
	if info.UID != "99" {
		t.Errorf("UID = %q, want 99", info.UID)
	}
	// ThreadID falls back to UID when in-reply-to is empty.
	if info.ThreadID != "99" {
		t.Errorf("ThreadID = %q, want 99 (UID fallback)", info.ThreadID)
	}
	// No flags set.
	if info.Flags != 0 {
		t.Errorf("Flags = %v, want 0", info.Flags)
	}
}

func TestInfoFromFetchMultipleFrom(t *testing.T) {
	// from field containing multiple addresses — passed through verbatim.
	info := infoFromFetch("1", map[string]any{
		"from": "Alice <a@example.com>, Bob <b@example.com>",
	})
	if !strings.Contains(info.From, "Alice") {
		t.Errorf("From = %q, expected Alice", info.From)
	}
}
