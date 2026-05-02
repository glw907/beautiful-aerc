//go:build integration

// SPDX-License-Identifier: MIT

package mailimap

import (
	"context"
	"os"
	"testing"

	"github.com/glw907/poplar/internal/config"
)

// TestLiveIMAPLifecycle requires a running IMAP server reachable at
// $POPLAR_TEST_IMAP_HOST (default 127.0.0.1) on port 1143 with a
// test user. See README.md for Dovecot setup instructions.
func TestLiveIMAPLifecycle(t *testing.T) {
	host := os.Getenv("POPLAR_TEST_IMAP_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	user := os.Getenv("POPLAR_TEST_IMAP_USER")
	pass := os.Getenv("POPLAR_TEST_IMAP_PASS")
	if user == "" || pass == "" {
		t.Skip("set POPLAR_TEST_IMAP_USER and POPLAR_TEST_IMAP_PASS")
	}

	cfg := config.AccountConfig{
		Name:        "test",
		Email:       user,
		Host:        host,
		Port:        1143,
		StartTLS:    true,
		InsecureTLS: true,
		Auth:        "plain",
		Password:    pass,
	}
	b := New(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := b.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer b.Disconnect()

	folders, err := b.ListFolders()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	hasInbox := false
	for _, f := range folders {
		if f.Name == "INBOX" {
			hasInbox = true
		}
	}
	if !hasInbox {
		t.Errorf("INBOX not in folders: %v", folders)
	}
}
