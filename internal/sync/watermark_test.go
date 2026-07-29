package sync

import (
	"context"
	"database/sql"
	"testing"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
)

// TestWatermarkPerCollection proves sync_state's key (ADR-0005's
// Decision, "poll by collection state") holds a distinct token per
// collection rather than one row per account and object kind: two
// calendars of the same kind each keep their own RFC 6578 sync token.
func TestWatermarkPerCollection(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		if err := saveWatermark(tx, accountID, backend.ObjectKindEvent, "cal-1", watermark{ServerStateToken: "tok-1", LocalRev: 1}); err != nil {
			return err
		}
		return saveWatermark(tx, accountID, backend.ObjectKindEvent, "cal-2", watermark{ServerStateToken: "tok-2", LocalRev: 3})
	})
	if err != nil {
		t.Fatalf("saveWatermark: %v", err)
	}

	wm1, err := loadWatermark(context.Background(), w, accountID, backend.ObjectKindEvent, "cal-1")
	if err != nil {
		t.Fatalf("loadWatermark(cal-1): %v", err)
	}
	wm2, err := loadWatermark(context.Background(), w, accountID, backend.ObjectKindEvent, "cal-2")
	if err != nil {
		t.Fatalf("loadWatermark(cal-2): %v", err)
	}

	if wm1.ServerStateToken != "tok-1" || wm1.LocalRev != 1 {
		t.Errorf("cal-1 watermark = %+v, want token %q rev 1", wm1, "tok-1")
	}
	if wm2.ServerStateToken != "tok-2" || wm2.LocalRev != 3 {
		t.Errorf("cal-2 watermark = %+v, want token %q rev 3", wm2, "tok-2")
	}
}

// TestFirstSyncWatermarkIsNoStoreFailure pins the read every account's
// first sync makes. No sync_state row exists yet, and reporting that
// absence out of the writer's transaction function rolls the
// transaction back and turns it into a uerr.Error at error level,
// claiming poplar could not open its store. ADR-0013 wants one log
// line per outcome, and a first sync is not an outcome anyone needs to
// hear about. The store's revision counter is the observable: it
// advances once per committed transaction and not at all on a
// rollback.
func TestFirstSyncWatermarkIsNoStoreFailure(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	before := w.Revision().Current()
	wm, err := loadWatermark(context.Background(), w, accountID, backend.ObjectKindMessage, mailCollection)
	if err != nil {
		t.Fatalf("loadWatermark on a fresh account: %v", err)
	}
	if wm != (watermark{}) {
		t.Errorf("watermark = %+v, want the zero value", wm)
	}
	if got := w.Revision().Current(); got == before {
		t.Errorf("revision stayed at %d: the missing row rolled the transaction back and surfaced a store failure", got)
	}
}
