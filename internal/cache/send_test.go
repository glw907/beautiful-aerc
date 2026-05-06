// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

func TestQueueSendRoundTrip(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	env := mail.Envelope{
		From:  "geoff@907.life",
		Rcpts: []string{"a@example.com", "b@example.com"},
	}
	mime := []byte("From: geoff@907.life\r\n\r\nhello\r\n")

	opID, err := a.QueueSend(context.Background(), "Inbox", env, mime)
	if err != nil {
		t.Fatalf("QueueSend: %v", err)
	}
	if opID == 0 {
		t.Fatal("expected nonzero op id")
	}

	row, err := a.nextOutboxRow(time.Now())
	if err != nil {
		t.Fatalf("nextOutboxRow: %v", err)
	}
	if row.Kind != string(KindSend) {
		t.Errorf("kind = %q, want %q", row.Kind, KindSend)
	}
	if string(row.Payload) != string(mime) {
		t.Errorf("payload mismatch: got %q want %q", row.Payload, mime)
	}
	args, err := decodeArgs(row.Kind, row.ArgsJSON)
	if err != nil {
		t.Fatalf("decodeArgs: %v", err)
	}
	sa, ok := args.(SendArgs)
	if !ok {
		t.Fatalf("args type = %T, want SendArgs", args)
	}
	if sa.Envelope.From != env.From {
		t.Errorf("From = %q, want %q", sa.Envelope.From, env.From)
	}
	if len(sa.Envelope.Rcpts) != 2 {
		t.Errorf("Rcpts len = %d, want 2", len(sa.Envelope.Rcpts))
	}
}

func TestQueueAppendRoundTrip(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	mime := []byte("From: geoff@907.life\r\nSubject: test\r\n\r\nbody\r\n")
	opID, err := a.QueueAppend(context.Background(), "Inbox", mail.FlagSeen, mime)
	if err != nil {
		t.Fatalf("QueueAppend: %v", err)
	}
	if opID == 0 {
		t.Fatal("expected nonzero op id")
	}

	row, err := a.nextOutboxRow(time.Now())
	if err != nil {
		t.Fatalf("nextOutboxRow: %v", err)
	}
	if row.Kind != string(KindAppend) {
		t.Errorf("kind = %q, want %q", row.Kind, KindAppend)
	}
	if row.FolderName != "Inbox" {
		t.Errorf("folder = %q, want Inbox", row.FolderName)
	}
	if string(row.Payload) != string(mime) {
		t.Errorf("payload mismatch")
	}
	args, err := decodeArgs(row.Kind, row.ArgsJSON)
	if err != nil {
		t.Fatalf("decodeArgs: %v", err)
	}
	aa, ok := args.(AppendArgs)
	if !ok {
		t.Fatalf("args type = %T, want AppendArgs", args)
	}
	if aa.Flag != mail.FlagSeen {
		t.Errorf("Flag = %v, want FlagSeen", aa.Flag)
	}
}
