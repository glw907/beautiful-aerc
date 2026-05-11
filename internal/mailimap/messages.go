package mailimap

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// QueryFolder selects the folder, runs UID SEARCH ALL, sorts the
// returned UIDs newest-first (highest UID), and slices by offset and
// limit.
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

	all, err := cmd.Search(mail.SearchCriteria{})
	if err != nil {
		return nil, total, fmt.Errorf("search all %q: %w", name, err)
	}

	slices.SortFunc(all, uidCompareDesc)

	if offset >= len(all) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], total, nil
}

// uidCompareDesc orders IMAP UIDs newest-first by their unsigned
// 32-bit decimal value.
func uidCompareDesc(a, b mail.UID) int {
	ai, _ := strconv.ParseUint(string(a), 10, 32)
	bi, _ := strconv.ParseUint(string(b), 10, 32)
	return cmp.Compare(bi, ai)
}

// fetchItems uses BODY.PEEK[...] so reading headers does not set \Seen.
var fetchItems = []string{
	"UID",
	"ENVELOPE",
	"INTERNALDATE",
	"FLAGS",
	"RFC822.SIZE",
	"BODY.PEEK[HEADER.FIELDS (FROM TO CC BCC SUBJECT DATE IN-REPLY-TO REFERENCES MESSAGE-ID)]",
}

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
		return nil, fmt.Errorf("fetch headers: %w", classifyErr(err))
	}
	return results, nil
}

// infoFromFetch translates one FETCH result map (lowercase field
// keys) into a mail.MessageInfo. Threading is a UI concern, so
// ThreadID defaults to UID and InReplyTo is read from "in-reply-to".
func infoFromFetch(uid mail.UID, items map[string]any) mail.MessageInfo {
	info := mail.MessageInfo{
		UID:      uid,
		ThreadID: uid,
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

// FetchBody returns the raw RFC 822 body for uid.
func (b *Backend) FetchBody(uid mail.UID) ([]byte, error) {
	b.mu.Lock()
	cmd := b.cmd
	b.mu.Unlock()

	rc, err := cmd.FetchBody(uid)
	if err != nil {
		return nil, fmt.Errorf("fetch body %s: %w", uid, err)
	}
	defer rc.Close()
	buf, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("fetch body %s: read: %w", uid, err)
	}
	return buf, nil
}
