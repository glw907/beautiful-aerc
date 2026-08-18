package sync

import (
	"context"
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/backendtest"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	"github.com/glw907/poplar/internal/uerr/uerrtest"
)

// TestSyncClassifiesMailboxRole is FO-1's wiring gate: a mailbox that
// reaches the store through a real sync cycle comes out carrying the
// classifier's answer, not the backend's role string. The three
// records cover the classification's three outcomes: a name the
// heuristic recognizes with no declared role, a declared role that
// normalizes into the six (JMAP's "all"), and a declared role outside
// the six, which must not survive verbatim.
func TestSyncClassifiesMailboxRole(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	var be backendtest.Fake
	be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
		return backend.ChangeSet{
			NewToken: "state-1",
			Created: []backend.Record{
				{ID: "mb-papierkorb", Fields: backend.MailboxFields{Name: "Papierkorb"}},
				{ID: "mb-all", Fields: backend.MailboxFields{Role: "all", Name: "Everything"}},
				{ID: "mb-inbox", Fields: backend.MailboxFields{Role: "inbox", Name: "Inbox"}},
			},
		}, nil
	}

	worker := NewWorker(accountID, &be, w, testConfig())
	if err := worker.SyncKind(context.Background(), backend.ObjectKindMailbox); err != nil {
		t.Fatalf("SyncKind: %v", err)
	}

	tests := []struct {
		serverID string
		want     string
	}{
		{"mb-papierkorb", "trash"},
		{"mb-all", "archive"},
		{"mb-inbox", ""},
	}
	for _, tt := range tests {
		if got := mailboxRoleForServerID(t, w, accountID, tt.serverID); got != tt.want {
			t.Errorf("role of %s = %q, want %q", tt.serverID, got, tt.want)
		}
	}
}

// TestSyncResolvesDuplicateMailboxRoles proves a server whose mailbox
// names classify to one role twice does not stop sync: the cycle
// succeeds, the first-created mailbox keeps the role, the second
// drops it, and the collision is logged.
func TestSyncResolvesDuplicateMailboxRoles(t *testing.T) {
	log := uerrtest.CaptureDefault(t)

	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	var be backendtest.Fake
	be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
		return backend.ChangeSet{
			NewToken: "state-1",
			Created: []backend.Record{
				{ID: "mb-trash", Fields: backend.MailboxFields{Name: "Trash"}},
				{ID: "mb-papierkorb", Fields: backend.MailboxFields{Name: "Papierkorb"}},
			},
		}, nil
	}

	worker := NewWorker(accountID, &be, w, testConfig())
	if err := worker.SyncKind(context.Background(), backend.ObjectKindMailbox); err != nil {
		t.Fatalf("SyncKind: %v", err)
	}

	if got := mailboxRoleForServerID(t, w, accountID, "mb-trash"); got != "trash" {
		t.Errorf("role of mb-trash = %q, want %q: the first-created claimant keeps the role", got, "trash")
	}
	if got := mailboxRoleForServerID(t, w, accountID, "mb-papierkorb"); got != "" {
		t.Errorf("role of mb-papierkorb = %q, want no role: the duplicate drops it", got)
	}
	if !strings.Contains(log.String(), "duplicate mailbox role") {
		t.Errorf("no log line for the role collision, got: %q", log.String())
	}
}

// mailboxRoleForServerID returns serverID's stored role within
// accountID.
func mailboxRoleForServerID(t *testing.T, w *store.Writer, accountID int64, serverID string) string {
	t.Helper()

	return storetest.ScanValue[string](t, w,
		`SELECT role FROM mailbox WHERE account_id = ? AND server_id = ?`, accountID, serverID)
}
