package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/search"
)

// SearchScope picks folder-local vs cross-folder.
type SearchScope struct {
	// Folder, when non-empty, restricts the query to that folder.
	// Empty means cross-folder (every folder the account knows about).
	Folder string
}

// SearchHit pairs a message header row with the folder it lives in.
// Folder is populated only on cross-folder searches; folder-scope
// hits leave it empty (the caller already knows the folder).
type SearchHit struct {
	mail.MessageInfo
	Folder string
}

// Search runs q against the FTS5 index, scoped per scope, and returns
// hits sorted sent_at DESC up to limit. An empty Query returns nil
// without touching the database.
func (a *Account) Search(ctx context.Context, q search.Query, scope SearchScope, limit int) ([]SearchHit, error) {
	if q.IsZero() {
		return nil, nil
	}
	if limit <= 0 {
		limit = 200
	}
	matchExpr := buildMatch(q)

	var sb strings.Builder
	args := []any{matchExpr}
	sb.WriteString(`
        SELECT m.protocol_id, m.subject, m.from_addr, m.to_addr, m.cc_addr,
               m.bcc_addr, m.date_str, COALESCE(m.sent_at, 0), m.ui_flags,
               COALESCE(m.size, 0), m.thread_id, m.in_reply_to,
               COALESCE(f.name, '')
        FROM messages_fts fts
        JOIN messages m ON m.id = fts.rowid
        LEFT JOIN message_mailboxes mm ON mm.message = m.id
        LEFT JOIN folders f ON f.id = mm.folder
        WHERE messages_fts MATCH ? AND m.ui_hide = 0`)

	if scope.Folder != "" {
		sb.WriteString(` AND f.name = ?`)
		args = append(args, scope.Folder)
	}
	for _, in := range q.In {
		sb.WriteString(` AND f.name = ?`)
		args = append(args, in)
	}
	if q.HasAttachment {
		sb.WriteString(` AND EXISTS (SELECT 1 FROM attachments a WHERE a.message = m.id)`)
	}
	sb.WriteString(` ORDER BY m.sent_at DESC LIMIT ?`)
	args = append(args, limit)

	rows, err := a.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()

	var out []SearchHit
	seen := map[mail.UID]struct{}{}
	for rows.Next() {
		var (
			pid, subj, from, to, cc, bcc, date, thread, irt, folder string
			sentNS                                                  int64
			flags                                                   uint32
			size                                                    int64
		)
		if err := rows.Scan(&pid, &subj, &from, &to, &cc, &bcc, &date, &sentNS, &flags, &size, &thread, &irt, &folder); err != nil {
			return nil, fmt.Errorf("search scan: %w", err)
		}
		uid := mail.UID(pid)
		// LEFT JOIN admits rows without a mailbox link. Dedup on UID.
		if _, dup := seen[uid]; dup {
			continue
		}
		seen[uid] = struct{}{}
		hit := SearchHit{
			MessageInfo: mail.MessageInfo{
				UID: uid, Subject: subj, From: from, To: to, Cc: cc, Bcc: bcc,
				Date: date, Flags: mail.Flag(flags), Size: uint32(size),
				ThreadID: mail.UID(thread), InReplyTo: mail.UID(irt),
			},
			Folder: folder,
		}
		if sentNS != 0 {
			hit.MessageInfo.SentAt = time.Unix(0, sentNS)
		}
		out = append(out, hit)
	}
	return out, rows.Err()
}

// buildMatch turns a parsed Query into an FTS5 MATCH expression.
// Field-scoped clauses use FTS5's column filter; bare terms span
// {subject from_addr to_addr body}. Tokens are wrapped as phrases
// so user input never trips FTS5 syntax characters.
func buildMatch(q search.Query) string {
	var parts []string
	for _, t := range q.Terms {
		parts = append(parts, `{subject from_addr to_addr body}:`+ftsPhrase(t))
	}
	for _, t := range q.From {
		parts = append(parts, `from_addr:`+ftsPhrase(t))
	}
	for _, t := range q.To {
		parts = append(parts, `to_addr:`+ftsPhrase(t))
	}
	for _, t := range q.Cc {
		parts = append(parts, `cc_addr:`+ftsPhrase(t))
	}
	for _, t := range q.Subject {
		parts = append(parts, `subject:`+ftsPhrase(t))
	}
	if len(parts) == 0 {
		// HasAttachment-only or In-only: the SQL filters carry the
		// constraint, but FTS5 MATCH requires *something*. A no-op
		// phrase that matches every row would be ideal; SQLite has
		// no such primitive. Use a wildcard match across columns
		// with a permissive prefix.
		return `body:* OR subject:* OR from_addr:* OR to_addr:*`
	}
	return strings.Join(parts, " ")
}

// ftsPhrase wraps a user term as an FTS5 double-quoted phrase,
// escaping any embedded quote per FTS5 rules ("" escapes ").
func ftsPhrase(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
