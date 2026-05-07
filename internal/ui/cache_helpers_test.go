package ui

import (
	"context"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/mail"
)

// fullSyncChangeTracker reports every UID currently visible on the
// backend as Added on the first call per folder, then returns empty
// deltas. Adequate for tests that want a one-shot population without
// hand-rolling a per-test ChangeTracker.
type fullSyncChangeTracker struct {
	backend mail.Backend
	mu      sync.Mutex
	seen    map[string]bool
}

func newFullSyncTracker(b mail.Backend) *fullSyncChangeTracker {
	return &fullSyncChangeTracker{backend: b, seen: map[string]bool{}}
}

func (f *fullSyncChangeTracker) Changes(_ context.Context, folder string, _ mail.SyncToken) (mail.ChangeSet, mail.SyncToken, error) {
	f.mu.Lock()
	already := f.seen[folder]
	f.seen[folder] = true
	f.mu.Unlock()
	if already {
		return mail.ChangeSet{}, mail.SyncToken("done"), nil
	}
	if err := f.backend.OpenFolder(folder); err != nil {
		return mail.ChangeSet{}, nil, err
	}
	uids, _, err := f.backend.QueryFolder(folder, 0, 100000)
	if err != nil {
		return mail.ChangeSet{}, nil, err
	}
	return mail.ChangeSet{Added: uids}, mail.SyncToken("done"), nil
}

// drainBatch executes cmd and recursively expands any tea.BatchMsg,
// returning all leaf Msg values. Used across multiple test files.
func drainBatch(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, drainBatch(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

// newTestCache opens a fresh cache.Account in t.TempDir against backend,
// primes the folder list via SyncFolders, and registers Close on cleanup.
// Use this anywhere a test needs a wired cache for AccountTab/App
// construction.
func newTestCache(t *testing.T, backend mail.Backend) *cache.Account {
	t.Helper()
	acct, err := cache.Open("test", backend, newFullSyncTracker(backend), t.TempDir(), cache.Config{})
	if err != nil {
		t.Fatalf("cache.Open: %v", err)
	}
	t.Cleanup(func() { acct.Close() })
	if err := acct.SyncFolders(context.Background()); err != nil {
		t.Fatalf("SyncFolders: %v", err)
	}
	return acct
}
