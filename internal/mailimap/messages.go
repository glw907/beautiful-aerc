// SPDX-License-Identifier: MIT

package mailimap

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// QueryFolder satisfies mail.Backend. It selects the folder to get
// the total message count, then issues UID SEARCH ALL to obtain every
// UID, sorts them newest-first (highest UID = most recently arrived),
// and slices the result according to offset and limit.
//
// The lock is held only long enough to snapshot b.cmd; the client call
// runs without the lock so other goroutines are not blocked.
func (b *Backend) QueryFolder(name string, offset, limit int) ([]mail.UID, int, error) {
	b.mu.Lock()
	cmd := b.cmd
	b.mu.Unlock()

	folder, err := cmd.Select(name, true)
	if err != nil {
		return nil, 0, fmt.Errorf("select %q: %w", name, err)
	}
	total := folder.Exists

	if total == 0 {
		return nil, 0, nil
	}

	// UID SEARCH ALL returns all UIDs in the selected folder.
	all, err := cmd.Search(mail.SearchCriteria{})
	if err != nil {
		return nil, total, fmt.Errorf("search all %q: %w", name, err)
	}

	// Sort descending: highest UID first (newest-arrived).
	sort.Slice(all, func(i, j int) bool {
		return uidGreater(all[i], all[j])
	})

	if offset >= len(all) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], total, nil
}

// uidGreater reports whether a is lexicographically greater than b,
// falling back to string comparison when lengths differ. IMAP UIDs are
// unsigned 32-bit integers that increment monotonically, so numeric
// order equals lexicographic order when zero-padded; in practice UIDs
// are never zero-padded in the wild, but a numeric string compare
// (longer string = larger integer when lengths match) is safe here.
//
// We sort by UID as a proxy for "newest-first" because IMAP UIDs are
// assigned in delivery order within a mailbox. This is the same
// heuristic used by most IMAP clients when SORT is not available.
func uidGreater(a, b mail.UID) bool {
	sa, sb := string(a), string(b)
	if len(sa) != len(sb) {
		return len(sa) > len(sb)
	}
	return sa > sb
}

// fetchItems is the set of FETCH data items requested by FetchHeaders.
// BODY.PEEK[...] fetches without setting \Seen.
var fetchItems = []string{
	"UID",
	"ENVELOPE",
	"INTERNALDATE",
	"FLAGS",
	"RFC822.SIZE",
	"BODY.PEEK[HEADER.FIELDS (FROM TO CC BCC SUBJECT DATE IN-REPLY-TO REFERENCES MESSAGE-ID)]",
}

// FetchHeaders satisfies mail.Backend. For an empty uid list it
// returns immediately. Otherwise it calls Fetch with the standard
// header item set and translates each result via infoFromFetch.
func (b *Backend) FetchHeaders(uids []mail.UID) ([]mail.MessageInfo, error) {
	if len(uids) == 0 {
		return nil, nil
	}

	b.mu.Lock()
	cmd := b.cmd
	b.mu.Unlock()

	var results []mail.MessageInfo
	err := cmd.Fetch(uids, fetchItems, func(uid mail.UID, items map[string]any) {
		results = append(results, infoFromFetch(uid, items))
	})
	if err != nil {
		return nil, fmt.Errorf("fetch headers: %w", err)
	}
	return results, nil
}

// infoFromFetch translates one FETCH result map into a mail.MessageInfo.
// The items map is keyed by lowercase field names. Missing keys produce
// zero values — the caller is responsible for providing a complete map.
//
// Threading: InReplyTo is set from the "in-reply-to" field. ThreadID
// is not determined at fetch time (thread grouping is a UI concern);
// it defaults to UID so every standalone message forms a size-1 thread.
func infoFromFetch(uid mail.UID, items map[string]any) mail.MessageInfo {
	info := mail.MessageInfo{
		UID:      uid,
		ThreadID: uid, // fallback: each message is its own thread
	}
	if v, ok := items["subject"]; ok {
		info.Subject, _ = v.(string)
	}
	if v, ok := items["from"]; ok {
		info.From, _ = v.(string)
	}
	if v, ok := items["to"]; ok {
		info.To, _ = v.(string)
	}
	if v, ok := items["cc"]; ok {
		info.Cc, _ = v.(string)
	}
	if v, ok := items["bcc"]; ok {
		info.Bcc, _ = v.(string)
	}
	if v, ok := items["date"]; ok {
		info.Date, _ = v.(string)
	}
	if v, ok := items["sentAt"]; ok {
		if t, ok := v.(time.Time); ok {
			info.SentAt = t
		}
	}
	if v, ok := items["flags"]; ok {
		if f, ok := v.(mail.Flag); ok {
			info.Flags = f
		}
	}
	if v, ok := items["size"]; ok {
		if s, ok := v.(uint32); ok {
			info.Size = s
		}
	}
	if v, ok := items["in-reply-to"]; ok {
		if s, ok := v.(string); ok {
			s = strings.TrimSpace(s)
			if s != "" {
				info.InReplyTo = mail.UID(s)
			}
		}
	}
	return info
}

// FetchBody satisfies mail.Backend. Returns the raw RFC 822 body for
// the given UID as an io.Reader. The caller is responsible for closing
// the reader if it implements io.Closer.
func (b *Backend) FetchBody(uid mail.UID) (io.Reader, error) {
	b.mu.Lock()
	cmd := b.cmd
	b.mu.Unlock()

	rc, err := cmd.FetchBody(uid)
	if err != nil {
		return nil, fmt.Errorf("fetch body %s: %w", uid, err)
	}
	return rc, nil
}

// Search satisfies mail.Backend. It forwards the SearchCriteria to the
// underlying client unchanged; translation from mail.SearchCriteria to
// IMAP SEARCH terms is the responsibility of the realClient adapter
// (realclient.go). The fake client accepts the struct directly.
func (b *Backend) Search(criteria mail.SearchCriteria) ([]mail.UID, error) {
	b.mu.Lock()
	cmd := b.cmd
	b.mu.Unlock()

	uids, err := cmd.Search(criteria)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	return uids, nil
}
