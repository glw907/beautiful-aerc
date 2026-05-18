package account

import (
	"context"
	"regexp"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/mail"
)

func init() {
	// Fixture expectations were written with uicore.FancyIcons (SPUA-A
	// glyphs). spuaCellWidth must be 2 so ansix.Width measures
	// them correctly in tests run independently of the App-level init.
	_ = ansix.NewMeasurer(2)
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

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

func newTestCache(t *testing.T, backend mail.Backend) *cache.Account {
	t.Helper()
	acct, err := cache.Open("test", t.TempDir(), cache.Config{}, nil)
	if err != nil {
		t.Fatalf("cache.Open: %v", err)
	}
	t.Cleanup(func() { acct.Close() })
	if err := acct.WireBackend(backend, newFullSyncTracker(backend)); err != nil {
		t.Fatalf("cache.WireBackend: %v", err)
	}
	if err := acct.SyncFolders(context.Background()); err != nil {
		t.Fatalf("SyncFolders: %v", err)
	}
	return acct
}
