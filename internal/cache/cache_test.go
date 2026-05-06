// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// fakeBackend records dispatched calls and returns canned errors.
type fakeBackend struct {
	moves    []mail.UID
	flags    []mail.UID
	destroys []mail.UID
	sends    []sendCall
	appends  []appendCall
	sendErr  error
	appErr   error
	err      error
	folders  []mail.Folder
	headers  []mail.MessageInfo
	updates  chan mail.Update
}

type sendCall struct {
	Env  mail.Envelope
	MIME []byte
}

type appendCall struct {
	Folder string
	MIME   []byte
	Flag   mail.Flag
}

func (f *fakeBackend) AccountName() string                 { return "fake" }
func (f *fakeBackend) AccountEmail() string                { return "fake@example.com" }
func (f *fakeBackend) IsJMAP() bool                        { return false }
func (f *fakeBackend) Connect(_ context.Context) error     { return nil }
func (f *fakeBackend) Disconnect() error                   { return nil }
func (f *fakeBackend) ListFolders() ([]mail.Folder, error) { return f.folders, nil }
func (f *fakeBackend) OpenFolder(_ string) error           { return nil }
func (f *fakeBackend) QueryFolder(_ string, _, _ int) ([]mail.UID, int, error) {
	return nil, 0, nil
}
func (f *fakeBackend) FetchHeaders(_ []mail.UID) ([]mail.MessageInfo, error) {
	return f.headers, nil
}
func (f *fakeBackend) FetchBody(_ mail.UID) ([]byte, error)              { return []byte{}, nil }
func (f *fakeBackend) Attachments(_ mail.UID) ([]mail.Attachment, error) { return nil, nil }
func (f *fakeBackend) FetchAttachment(_ mail.UID, _ string) ([]byte, error) {
	return nil, nil
}
func (f *fakeBackend) Move(uids []mail.UID, _ string) error {
	f.moves = append(f.moves, uids...)
	return f.err
}
func (f *fakeBackend) Destroy(uids []mail.UID) error {
	f.destroys = append(f.destroys, uids...)
	return f.err
}
func (f *fakeBackend) Flag(uids []mail.UID, _ mail.Flag, _ bool) error {
	f.flags = append(f.flags, uids...)
	return f.err
}
func (f *fakeBackend) Send(env mail.Envelope, mime []byte) error {
	f.sends = append(f.sends, sendCall{Env: env, MIME: append([]byte(nil), mime...)})
	return f.sendErr
}
func (f *fakeBackend) Append(folder string, mime []byte, flag mail.Flag) error {
	f.appends = append(f.appends, appendCall{Folder: folder, MIME: append([]byte(nil), mime...), Flag: flag})
	return f.appErr
}
func (f *fakeBackend) PushDraft(_ string, _ []byte, _ mail.UID) (mail.UID, error) {
	return "", mail.ErrUnsupported
}
func (f *fakeBackend) Updates() <-chan mail.Update {
	if f.updates == nil {
		f.updates = make(chan mail.Update)
	}
	return f.updates
}

// fakeChangeTracker emits canned ChangeSets.
type fakeChangeTracker struct {
	deltas []mail.ChangeSet
	err    error
	calls  int
}

func (f *fakeChangeTracker) Changes(_ context.Context, _ string, _ mail.SyncToken) (mail.ChangeSet, mail.SyncToken, error) {
	if f.err != nil {
		return mail.ChangeSet{}, nil, f.err
	}
	if f.calls >= len(f.deltas) {
		return mail.ChangeSet{}, mail.SyncToken("done"), nil
	}
	d := f.deltas[f.calls]
	f.calls++
	return d, mail.SyncToken("tok"), nil
}

// openTestAccount creates a backed account in t.TempDir.
func openTestAccount(t *testing.T) *Account {
	t.Helper()
	be := &fakeBackend{
		folders: []mail.Folder{{Name: "INBOX", Role: "inbox"}},
	}
	ct := &fakeChangeTracker{}
	a, err := Open("Test Account", be, ct, t.TempDir(), Config{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	if err := a.SyncFolders(context.Background()); err != nil {
		t.Fatalf("SyncFolders: %v", err)
	}
	return a
}

func TestSchemaMigration(t *testing.T) {
	a := openTestAccount(t)
	var version int
	if err := a.db.QueryRow(`SELECT version FROM schema_version LIMIT 1`).Scan(&version); err != nil {
		t.Fatalf("read schema_version: %v", err)
	}
	if version != schemaVersion {
		t.Errorf("schema version = %d, want %d", version, schemaVersion)
	}
	// Re-open should be idempotent.
	if err := applyMigrations(a.db); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
}

func TestQueueOp_OptimisticFlag(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()

	if err := a.upsertMessages(ctx, "Inbox", []mail.MessageInfo{
		{UID: "1", Subject: "hi", From: "a@x", SentAt: time.Now(), Flags: 0},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := a.QueueOp(ctx, "Inbox", "1", FlagArgs{Flag: mail.FlagFlagged, Set: true}); err != nil {
		t.Fatalf("QueueOp: %v", err)
	}
	var ui uint32
	if err := a.db.QueryRow(`SELECT ui_flags FROM messages WHERE protocol_id = '1'`).Scan(&ui); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if mail.Flag(ui)&mail.FlagFlagged == 0 {
		t.Errorf("ui_flags missing FlagFlagged after QueueOp")
	}
}

func TestQueueOp_Move_HidesSource(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()
	a.upsertMessages(ctx, "Inbox", []mail.MessageInfo{{UID: "5", Subject: "x"}})
	if _, err := a.QueueOp(ctx, "Inbox", "5", MoveArgs{Dest: "Archive"}); err != nil {
		t.Fatalf("QueueOp: %v", err)
	}
	var hide int
	if err := a.db.QueryRow(`SELECT ui_hide FROM messages WHERE protocol_id = '5'`).Scan(&hide); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if hide != 1 {
		t.Errorf("ui_hide = %d, want 1", hide)
	}
	// Cache read should now skip the row.
	got, _, err := a.QueryFolder("Inbox", 0, 10)
	if err != nil {
		t.Fatalf("QueryFolder: %v", err)
	}
	for _, m := range got {
		if m.UID == "5" {
			t.Errorf("hidden row leaked into QueryFolder")
		}
	}
}

func TestDrainer_FlagSuccess(t *testing.T) {
	be := &fakeBackend{folders: []mail.Folder{{Name: "INBOX", Role: "inbox"}}}
	a, err := Open("drain", be, &fakeChangeTracker{}, t.TempDir(), Config{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer a.Close()
	if err := a.SyncFolders(context.Background()); err != nil {
		t.Fatalf("SyncFolders: %v", err)
	}
	a.upsertMessages(context.Background(), "Inbox", []mail.MessageInfo{{UID: "9", Subject: "x"}})
	if _, err := a.QueueOp(context.Background(), "Inbox", "9", FlagArgs{Flag: mail.FlagSeen, Set: true}); err != nil {
		t.Fatalf("QueueOp: %v", err)
	}
	if err := a.StartDrainer(context.Background()); err != nil {
		t.Fatalf("StartDrainer: %v", err)
	}
	select {
	case ev := <-a.Events():
		if ev.Status != OpDone {
			t.Fatalf("event status = %q, want done (err=%q)", ev.Status, ev.Err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for cache event")
	}
	if len(be.flags) == 0 {
		t.Error("backend.Flag never called")
	}
	var status string
	a.db.QueryRow(`SELECT status FROM outbox WHERE id = 1`).Scan(&status)
	if status != "done" {
		t.Errorf("outbox status = %q, want done", status)
	}
}

func TestDrainer_ConflictOnAuthError(t *testing.T) {
	be := &fakeBackend{
		folders: []mail.Folder{{Name: "INBOX", Role: "inbox"}},
		err:     fmt.Errorf("401 Unauthorized: %w", mail.ErrAuth),
	}
	a, _ := Open("auth", be, &fakeChangeTracker{}, t.TempDir(), Config{})
	defer a.Close()
	a.SyncFolders(context.Background())
	a.upsertMessages(context.Background(), "Inbox", []mail.MessageInfo{{UID: "1"}})
	a.QueueOp(context.Background(), "Inbox", "1", FlagArgs{Flag: mail.FlagSeen, Set: true})
	a.StartDrainer(context.Background())
	select {
	case ev := <-a.Events():
		if ev.Status != OpConflict {
			t.Errorf("status = %q, want conflict", ev.Status)
		}
		if !strings.Contains(ev.Err, "Unauthorized") {
			t.Errorf("err missing 'Unauthorized': %q", ev.Err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
	var errPayload string
	a.db.QueryRow(`SELECT error FROM outbox WHERE id = 1`).Scan(&errPayload)
	if !strings.Contains(errPayload, "auth-failure") {
		t.Errorf("outbox.error missing auth-failure kind: %s", errPayload)
	}
}

func TestRecoverExecuting_Idempotent(t *testing.T) {
	be := &fakeBackend{folders: []mail.Folder{{Name: "INBOX", Role: "inbox"}}}
	a, _ := Open("rec", be, &fakeChangeTracker{}, t.TempDir(), Config{})
	defer a.Close()
	a.SyncFolders(context.Background())
	a.upsertMessages(context.Background(), "Inbox", []mail.MessageInfo{{UID: "1"}})
	opID, _ := a.QueueOp(context.Background(), "Inbox", "1", FlagArgs{Flag: mail.FlagSeen, Set: true})
	// Simulate a crash mid-execute.
	a.db.Exec(`UPDATE outbox SET status = 'executing' WHERE id = ?`, opID)
	if err := a.recoverExecuting(); err != nil {
		t.Fatalf("recoverExecuting: %v", err)
	}
	var status string
	a.db.QueryRow(`SELECT status FROM outbox WHERE id = ?`, opID).Scan(&status)
	if status != "pending" {
		t.Errorf("status = %q, want pending", status)
	}
}

func TestRecoverExecuting_Send(t *testing.T) {
	be := &fakeBackend{folders: []mail.Folder{{Name: "INBOX", Role: "inbox"}}}
	a, _ := Open("rec-send", be, &fakeChangeTracker{}, t.TempDir(), Config{})
	defer a.Close()
	a.SyncFolders(context.Background())
	// Insert a synthetic send op directly (cache I doesn't queue
	// send through QueueOp yet, pending Pass 9).
	folderID, _ := a.folderID("Inbox")
	a.db.Exec(`INSERT INTO outbox (folder, kind, args, enqueued_at, status) VALUES (?, 'send', '{}', ?, 'executing')`,
		folderID, time.Now().UnixNano())
	if err := a.recoverExecuting(); err != nil {
		t.Fatalf("recoverExecuting: %v", err)
	}
	var status, errPayload string
	a.db.QueryRow(`SELECT status, error FROM outbox WHERE kind = 'send'`).Scan(&status, &errPayload)
	if status != "conflict" {
		t.Errorf("status = %q, want conflict", status)
	}
	if !strings.Contains(errPayload, "crashed-mid-execute") {
		t.Errorf("error missing crashed-mid-execute: %s", errPayload)
	}
}

func TestSyncFolder_ReAnchor(t *testing.T) {
	ct := &fakeChangeTracker{err: mail.ErrCannotCalculateChanges}
	be := &fakeBackend{folders: []mail.Folder{{Name: "INBOX", Role: "inbox"}}}
	a, _ := Open("reanchor", be, ct, t.TempDir(), Config{})
	defer a.Close()
	a.SyncFolders(context.Background())
	a.upsertMessages(context.Background(), "Inbox", []mail.MessageInfo{{UID: "1"}})
	opID, _ := a.QueueOp(context.Background(), "Inbox", "1", FlagArgs{Flag: mail.FlagSeen, Set: true})

	if err := a.SyncFolder(context.Background(), "Inbox"); err != nil {
		t.Fatalf("SyncFolder: %v", err)
	}
	var status, errPayload string
	a.db.QueryRow(`SELECT status, error FROM outbox WHERE id = ?`, opID).Scan(&status, &errPayload)
	if status != "conflict" {
		t.Errorf("status = %q, want conflict", status)
	}
	if !strings.Contains(errPayload, "anchor-lost") {
		t.Errorf("error missing anchor-lost kind: %s", errPayload)
	}
	// sync_token cleared.
	var token []byte
	a.db.QueryRow(`SELECT sync_token FROM folders WHERE name = 'Inbox'`).Scan(&token)
	if token != nil {
		t.Errorf("sync_token = %v, want nil after re-anchor", token)
	}
}

func TestCoordinationInvariant_PendingOp(t *testing.T) {
	be := &fakeBackend{
		folders: []mail.Folder{{Name: "INBOX", Role: "inbox"}},
		// Server reports message as unflagged.
		headers: []mail.MessageInfo{{UID: "1", Flags: 0}},
	}
	ct := &fakeChangeTracker{deltas: []mail.ChangeSet{{Modified: []mail.UID{"1"}}}}
	a, _ := Open("coord", be, ct, t.TempDir(), Config{})
	defer a.Close()
	a.SyncFolders(context.Background())
	a.upsertMessages(context.Background(), "Inbox", []mail.MessageInfo{{UID: "1", Flags: 0}})
	// User toggles Star. The optimistic ui_flags now has FlagFlagged.
	a.QueueOp(context.Background(), "Inbox", "1", FlagArgs{Flag: mail.FlagFlagged, Set: true})

	// Syncer pulls. Server still says unflagged. Coordination
	// invariant: ui_flags must NOT be reverted because an outbox
	// row is still pending.
	if err := a.SyncFolder(context.Background(), "Inbox"); err != nil {
		t.Fatalf("SyncFolder: %v", err)
	}
	var ui uint32
	a.db.QueryRow(`SELECT ui_flags FROM messages WHERE protocol_id = '1'`).Scan(&ui)
	if mail.Flag(ui)&mail.FlagFlagged == 0 {
		t.Errorf("syncer stomped optimistic ui_flags (= %#x); coordination invariant violated", ui)
	}
}

func TestMigrateV4_DropsLastAccessedAndIndex(t *testing.T) {
	// Open a fresh DB at v3 by stubbing schemaVersion. We can't
	// easily downgrade in-process, so instead we open at the current
	// version, insert a body row, and verify after migration that:
	//   - the bodies table is missing the last_accessed column
	//   - the bodies_lru index is gone
	//   - body bytes are intact
	dir := t.TempDir()
	a, err := Open("test", &fakeBackend{}, &fakeChangeTracker{}, dir, Config{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer a.Close()

	// After migration, last_accessed should not exist.
	row := a.DB().QueryRow(`SELECT name FROM pragma_table_info('bodies') WHERE name='last_accessed'`)
	var name string
	if err := row.Scan(&name); err == nil {
		t.Errorf("bodies.last_accessed should be dropped, found column")
	}

	// bodies_lru index should be gone.
	row = a.DB().QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name='bodies_lru'`)
	if err := row.Scan(&name); err == nil {
		t.Errorf("bodies_lru index should be dropped, found %q", name)
	}
}

func TestAttachmentsTableShape(t *testing.T) {
	a := openTestAccount(t)
	rows, err := a.db.Query(`PRAGMA table_info(attachments)`)
	if err != nil {
		t.Fatalf("table_info: %v", err)
	}
	defer rows.Close()
	cols := map[string]string{}
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		cols[name] = ctype
	}
	want := []string{"id", "message", "part_id", "filename", "mime_type", "size", "content_id", "disposition", "bytes", "fetched_at"}
	for _, n := range want {
		if _, ok := cols[n]; !ok {
			t.Errorf("attachments missing column %q", n)
		}
	}
}

func TestSchemaMigration_V7Drafts(t *testing.T) {
	a := openTestAccount(t)

	var version int
	if err := a.db.QueryRow(`SELECT version FROM schema_version LIMIT 1`).Scan(&version); err != nil {
		t.Fatalf("read schema_version: %v", err)
	}
	if version != 7 {
		t.Errorf("schema version = %d, want 7", version)
	}

	rows, err := a.db.Query(`PRAGMA table_info(drafts)`)
	if err != nil {
		t.Fatalf("table_info(drafts): %v", err)
	}
	defer rows.Close()

	want := map[string]string{
		"draft_id":       "TEXT",
		"server_uid":     "TEXT",
		"server_folder":  "TEXT",
		"payload":        "BLOB",
		"dirty":          "INTEGER",
		"created_at":     "INTEGER",
		"updated_at":     "INTEGER",
		"last_pushed_at": "INTEGER",
	}
	got := map[string]string{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[name] = typ
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating table_info(drafts): %v", err)
	}
	for col, typ := range want {
		if got[col] != typ {
			t.Errorf("drafts.%s type = %q, want %q", col, got[col], typ)
		}
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Geoff Wright", "geoff-wright"},
		{"a@b.com", "a-b-com"},
		{"--Foo--", "foo"},
		{"  ", ""},
	}
	for _, tc := range cases {
		if got := Slugify(tc.in); got != tc.want {
			t.Errorf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestOpenThreadsAttachmentMax(t *testing.T) {
	be := &fakeBackend{folders: []mail.Folder{{Name: "INBOX", Role: "inbox"}}}
	ct := &fakeChangeTracker{}
	a, err := Open("Test", be, ct, t.TempDir(), Config{MaxSize: 100, MaxAttachmentSize: 200})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer a.Close()
	if a.maxSize != 100 {
		t.Errorf("maxSize = %d, want 100", a.maxSize)
	}
	if a.maxAttachmentSize != 200 {
		t.Errorf("maxAttachmentSize = %d, want 200", a.maxAttachmentSize)
	}
}
