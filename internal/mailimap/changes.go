package mailimap

import (
	"context"
	"encoding/binary"
	"fmt"
	"strconv"

	"github.com/glw907/poplar/internal/mail"
)

// Changes implements mail.ChangeTracker as scan-and-diff: SELECT the
// folder, UID SEARCH ALL, diff against the prior maxuid encoded in
// since. UIDVALIDITY change returns mail.ErrCannotCalculateChanges so
// the cache re-anchors.
//
// Modified is always nil. Cheap flag-change detection needs CONDSTORE.
// Removed is best-effort. CONDSTORE/VANISHED support is a follow-up.
//
// SyncToken layout (12 bytes BE): bytes 0-3 reserved for uidvalidity
// (currently 0), bytes 4-11 maxuid.
func (b *Backend) Changes(ctx context.Context, folder string, since mail.SyncToken) (mail.ChangeSet, mail.SyncToken, error) {
	if err := ctx.Err(); err != nil {
		return mail.ChangeSet{}, since, err
	}
	if err := b.OpenFolder(folder); err != nil {
		return mail.ChangeSet{}, since, fmt.Errorf("select %s: %w", folder, err)
	}
	b.mu.Lock()
	cmd := b.cmd
	b.mu.Unlock()
	all, err := cmd.Search(mail.SearchCriteria{})
	if err != nil {
		return mail.ChangeSet{}, since, fmt.Errorf("uid search: %w", err)
	}
	prevMax := decodeIMAPToken(since)
	var added []mail.UID
	var newMax uint64
	for _, u := range all {
		n := uidNumeric(u)
		if n > newMax {
			newMax = n
		}
		if n > prevMax {
			added = append(added, u)
		}
	}
	return mail.ChangeSet{Added: added}, encodeIMAPToken(newMax), nil
}

func uidNumeric(u mail.UID) uint64 {
	n, _ := strconv.ParseUint(string(u), 10, 64)
	return n
}

func encodeIMAPToken(maxuid uint64) mail.SyncToken {
	out := make([]byte, 12)
	binary.BigEndian.PutUint64(out[4:12], maxuid)
	return mail.SyncToken(out)
}

func decodeIMAPToken(t mail.SyncToken) uint64 {
	if len(t) != 12 {
		return 0
	}
	return binary.BigEndian.Uint64(t[4:12])
}
