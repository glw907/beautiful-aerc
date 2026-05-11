package cache

import (
	"testing"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

func TestBackfillProgress(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	for i := range 5 {
		seedMessage(t, a, mail.UID(string(rune('a'+i))), time.Now())
	}
	for i := range 2 {
		var msgID int64
		a.db.QueryRow(`SELECT id FROM messages WHERE protocol_id = ?`, string(rune('a'+i))).Scan(&msgID)
		a.db.Exec(`INSERT INTO bodies (message, bytes, fetched_at) VALUES (?, ?, ?)`,
			msgID, []byte("b"), time.Now().UnixNano())
	}

	done, total, warn, err := a.BackfillProgress()
	if err != nil {
		t.Fatalf("BackfillProgress: %v", err)
	}
	if done != 2 || total != 5 {
		t.Errorf("BackfillProgress = (%d, %d), want (2, 5)", done, total)
	}
	if warn {
		t.Errorf("warn = true on a fresh account; expected false")
	}
}

func TestBackfillProgressWarnReflectsThrottle(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	a.backfiller.throttling.Store(true)
	_, _, warn, err := a.BackfillProgress()
	if err != nil {
		t.Fatalf("BackfillProgress: %v", err)
	}
	if !warn {
		t.Errorf("warn = false while throttling; expected true")
	}
}
