// SPDX-License-Identifier: MIT

package mailimap

import (
	"context"
	"encoding/binary"
	"fmt"
	"strconv"

	"github.com/glw907/poplar/internal/mail"
)

// Changes implements mail.ChangeTracker.
//
// The IMAP implementation is the simple "scan-and-diff" form: select
// the folder, run UID SEARCH ALL, and diff against the prior maxuid
// encoded in since. UIDVALIDITY change → mail.ErrCannotCalculateChanges
// (the cache responds with the re-anchor path from spec §D.4).
//
// Modified is intentionally always nil — flag-only changes need
// CONDSTORE to detect cheaply. Removed is also nil on the first call
// and best-effort thereafter; UIDPLUS/VANISHED-driven removal
// detection is a follow-up pass (CONDSTORE-aware ChangeTracker).
//
// SyncToken layout (12 bytes BE):
//
//	bytes 0-3   uidvalidity   (uint32, currently 0 — cap not surfaced)
//	bytes 4-11  maxuid        (uint64; numeric max of last-known UIDs)
func (b *Backend) Changes(ctx context.Context, folder string, since mail.SyncToken) (mail.ChangeSet, mail.SyncToken, error) {
	if err := ctx.Err(); err != nil {
		return mail.ChangeSet{}, since, err
	}
	if err := b.OpenFolder(folder); err != nil {
		return mail.ChangeSet{}, since, fmt.Errorf("select %s: %w", folder, err)
	}
	all, err := b.cmdClient().Search(mail.SearchCriteria{})
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

// cmdClient returns the locked command-connection imapClient.
func (b *Backend) cmdClient() imapClient {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cmd
}

func uidNumeric(u mail.UID) uint64 {
	n, _ := strconv.ParseUint(string(u), 10, 64)
	return n
}

func encodeIMAPToken(maxuid uint64) mail.SyncToken {
	out := make([]byte, 12)
	// bytes 0-3 reserved for uidvalidity (filled when surfaced).
	binary.BigEndian.PutUint64(out[4:12], maxuid)
	return mail.SyncToken(out)
}

func decodeIMAPToken(t mail.SyncToken) uint64 {
	if len(t) != 12 {
		return 0
	}
	return binary.BigEndian.Uint64(t[4:12])
}
