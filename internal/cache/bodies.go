// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// lookupBody reads a cached body for uid. Returns (bytes, true, nil)
// on hit, (nil, false, nil) on miss, (nil, false, err) on db error.
// No last_accessed update — Cache II is lazy-population only.
func (a *Account) lookupBody(ctx context.Context, uid mail.UID) ([]byte, bool, error) {
	const q = `
        SELECT b.bytes
        FROM bodies b
        JOIN messages m ON m.id = b.message
        WHERE m.protocol_id = ?`
	var buf []byte
	err := a.db.QueryRowContext(ctx, q, string(uid)).Scan(&buf)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("lookup body %s: %w", uid, err)
	}
	return buf, true, nil
}

// unused vars suppressed during phase 3a; remove when storeBody arrives.
var _ = time.Time{}
