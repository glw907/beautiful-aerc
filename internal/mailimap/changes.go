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
// SyncToken layout (12 bytes BE): bytes 0–3 uidvalidity, bytes 4–11
// maxuid.
func (b *Backend) Changes(ctx context.Context, folder string, since mail.SyncToken) (mail.ChangeSet, mail.SyncToken, error) {
	if err := ctx.Err(); err != nil {
		return mail.ChangeSet{}, since, err
	}
	if err := b.OpenFolder(folder); err != nil {
		return mail.ChangeSet{}, since, fmt.Errorf("select %s: %w", folder, err)
	}
	b.mu.Lock()
	uidval := b.currentUIDVal
	b.mu.Unlock()

	prevUIDVal, prevMax := decodeIMAPToken(since)
	if prevUIDVal != 0 && prevUIDVal != uidval {
		return mail.ChangeSet{}, since, mail.ErrCannotCalculateChanges
	}

	cmd, err := b.cmdClient()
	if err != nil {
		return mail.ChangeSet{}, since, fmt.Errorf("changes %s: %w", folder, err)
	}
	all, err := cmd.Search(mail.SearchCriteria{})
	if err != nil {
		wrapped := classifyErr(err)
		b.maybeDropOnConn(cmd, wrapped)
		return mail.ChangeSet{}, since, fmt.Errorf("uid search: %w", wrapped)
	}
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
	return mail.ChangeSet{Added: added}, encodeIMAPToken(uidval, newMax), nil
}

func uidNumeric(u mail.UID) uint64 {
	n, _ := strconv.ParseUint(string(u), 10, 64)
	return n
}

func encodeIMAPToken(uidvalidity uint32, maxuid uint64) mail.SyncToken {
	out := make([]byte, 12)
	binary.BigEndian.PutUint32(out[0:4], uidvalidity)
	binary.BigEndian.PutUint64(out[4:12], maxuid)
	return mail.SyncToken(out)
}

func decodeIMAPToken(t mail.SyncToken) (uint32, uint64) {
	if len(t) != 12 {
		return 0, 0
	}
	return binary.BigEndian.Uint32(t[0:4]), binary.BigEndian.Uint64(t[4:12])
}
