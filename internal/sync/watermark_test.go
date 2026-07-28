package sync

import (
	"context"
	"database/sql"
	"testing"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
)

// TestWatermarkPerCollection proves sync_state's key (item 4 of the
// pass-1 audit) holds a distinct token per collection rather than one
// row per account and object kind: two calendars of the same kind
// each keep their own RFC 6578 sync token.
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
