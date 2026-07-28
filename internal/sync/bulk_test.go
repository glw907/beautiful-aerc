package sync

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
)

// TestBackfillSubordination drives runBulkChunks, the production
// chunk-decision loop ADR-0003 revision 2's backfill subordination
// policy now lives in (retargeted from internal/store's own writer
// tests, which drove the same check from a test-only closure): a
// chunk yields while an interactive commit landed within the quiet
// window, and resumes once that window has passed.
func TestBackfillSubordination(t *testing.T) {
	cfg := store.DefaultWriterConfig()
	cfg.InteractiveQuiet = 30 * time.Millisecond
	w := storetest.OpenWriter(t, cfg)

	ran := make(chan struct{}, 1)
	runChunk := func() {
		go func() {
			_ = runBulkChunks(context.Background(), w, cfg.InteractiveQuiet, func(*sql.Tx) error { return nil })
			ran <- struct{}{}
		}()
	}

	runChunk()
	select {
	case <-ran:
	case <-time.After(time.Second):
		t.Fatal("chunk did not run with no interactive activity yet")
	}

	if err := w.ApplyInteractive(context.Background(), func(*sql.Tx) error { return nil }); err != nil {
		t.Fatalf("interactive submit: %v", err)
	}

	runChunk()
	select {
	case <-ran:
		t.Fatal("chunk ran immediately after interactive activity, want it to yield")
	case <-time.After(cfg.InteractiveQuiet / 2):
	}

	select {
	case <-ran:
	case <-time.After(time.Second):
		t.Fatal("chunk never ran once the quiet window elapsed")
	}
}
