package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/glw907/poplar/internal/compose"
	"github.com/glw907/poplar/internal/mail"
)

// ListFolders returns the cache's folder rows joined with the
// canonical classification (mail.Classify wraps the same alias
// table the UI sidebar uses) and per-folder Exists/Unseen counts
// computed from message_mailboxes ⨝ messages. The cache is the
// source of truth. The syncer keeps it converged with the backend.
func (a *Account) ListFolders() ([]mail.ClassifiedFolder, error) {
	const q = `
        SELECT f.name, f.role, f.exists_total, f.unseen_total,
               COUNT(mm.message) AS local_exists,
               SUM(CASE WHEN (m.flags & ?) = 0 THEN 1 ELSE 0 END) AS local_unseen
        FROM folders f
        LEFT JOIN message_mailboxes mm ON mm.folder = f.id
        LEFT JOIN messages m ON m.id = mm.message AND m.ui_hide = 0
        GROUP BY f.id, f.name, f.role, f.exists_total, f.unseen_total
        ORDER BY f.name`
	rows, err := a.db.Query(q, uint32(mail.FlagSeen))
	if err != nil {
		return nil, fmt.Errorf("list folders: %w", err)
	}
	defer rows.Close()
	var raw []mail.Folder
	for rows.Next() {
		var name string
		var role sql.NullString
		var existsTotal, localExists int
		var unseenTotal int
		var localUnseen sql.NullInt64
		if err := rows.Scan(&name, &role, &existsTotal, &unseenTotal, &localExists, &localUnseen); err != nil {
			return nil, fmt.Errorf("scan folder: %w", err)
		}
		f := mail.Folder{Name: name}
		if role.Valid {
			f.Role = role.String
		}
		// Prefer local counts once any messages are synced. Otherwise
		// fall back to backend-reported totals stored at SyncFolders
		// time so unopened folders still show their unread badges.
		if localExists > 0 {
			f.Exists = localExists
			if localUnseen.Valid {
				f.Unseen = int(localUnseen.Int64)
			}
		} else {
			f.Exists = existsTotal
			f.Unseen = unseenTotal
		}
		raw = append(raw, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return mail.Classify(raw), nil
}

// QueryFolder returns up to limit messages from folder starting at
// offset. Sort is sent_at DESC. Rows with ui_hide = 1 are filtered
// (mid-move source rows).
func (a *Account) QueryFolder(folder string, offset, limit int) ([]mail.MessageInfo, int, error) {
	folderID, err := a.folderID(folder)
	if err != nil {
		return nil, 0, err
	}
	var total int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM message_mailboxes mm JOIN messages m ON m.id = mm.message WHERE mm.folder = ? AND m.ui_hide = 0`, folderID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count: %w", err)
	}
	rows, err := a.db.Query(`
        SELECT m.protocol_id, m.subject, m.from_addr, m.to_addr, m.cc_addr,
               m.bcc_addr, m.date_str, COALESCE(m.sent_at, 0), m.ui_flags,
               COALESCE(m.size, 0), m.thread_id, m.in_reply_to
        FROM message_mailboxes mm
        JOIN messages m ON m.id = mm.message
        WHERE mm.folder = ? AND m.ui_hide = 0
        ORDER BY m.sent_at DESC
        LIMIT ? OFFSET ?`, folderID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	var out []mail.MessageInfo
	for rows.Next() {
		mi, err := scanMessageInfo(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan message: %w", err)
		}
		out = append(out, mi)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	role, _ := a.folderRole(folder)
	if role == "drafts" {
		extras, err := a.draftsAsMessageInfo(context.Background())
		if err != nil {
			return nil, 0, fmt.Errorf("project drafts: %w", err)
		}
		out = append(out, extras...)
		total += len(extras)
	}
	return out, total, nil
}

// folderRole returns the role string stored for folder, or "" when
// the row is absent or the role column is NULL.
func (a *Account) folderRole(folder string) (string, error) {
	var role sql.NullString
	err := a.db.QueryRow(`SELECT role FROM folders WHERE name = ?`, folder).Scan(&role)
	if err != nil {
		return "", err
	}
	if !role.Valid {
		return "", nil
	}
	return role.String, nil
}

// draftsAsMessageInfo projects local-only drafts (server_uid = NULL)
// into the message-list shape with synthetic UIDs "draft:<id>". The
// App's Enter handler keys off the prefix to route through LoadDraft.
// Pushed drafts (server_uid != NULL) reach the list via the normal
// server-message path, so they're excluded here.
func (a *Account) draftsAsMessageInfo(ctx context.Context) ([]mail.MessageInfo, error) {
	rows, err := a.ListDrafts(ctx)
	if err != nil {
		return nil, err
	}
	var out []mail.MessageInfo
	for _, r := range rows {
		if r.ServerUID != "" {
			continue
		}
		d, err := compose.DecodeDraft(r.Payload)
		if err != nil {
			// Skip undecodable rows. Recoverable via the cache CLI.
			continue
		}
		mi := mail.MessageInfo{
			UID:    mail.UID("draft:" + r.DraftID),
			From:   d.From.String(),
			SentAt: r.UpdatedAt,
		}
		if d.Subject != "" {
			mi.Subject = d.Subject
		} else {
			mi.Subject = "(no subject)"
		}
		if len(d.To) > 0 {
			mi.To = d.To[0].String()
		}
		out = append(out, mi)
	}
	return out, nil
}

// scanMessageInfo decodes one row of the canonical 12-column SELECT
// (protocol_id, subject, from, to, cc, bcc, date, sent_at, ui_flags,
// size, thread_id, in_reply_to). Both QueryFolder and FetchHeaders
// share this shape.
func scanMessageInfo(rows *sql.Rows) (mail.MessageInfo, error) {
	var (
		pid, subj, from, to, cc, bcc, date, thread, irt string
		sentNS                                          int64
		flags                                           uint32
		size                                            int64
	)
	if err := rows.Scan(&pid, &subj, &from, &to, &cc, &bcc, &date, &sentNS, &flags, &size, &thread, &irt); err != nil {
		return mail.MessageInfo{}, err
	}
	mi := mail.MessageInfo{
		UID: mail.UID(pid), Subject: subj, From: from, To: to, Cc: cc, Bcc: bcc,
		Date: date, Flags: mail.Flag(flags), Size: uint32(size),
		ThreadID: mail.UID(thread), InReplyTo: mail.UID(irt),
	}
	if sentNS > 0 {
		mi.SentAt = time.Unix(0, sentNS)
	}
	return mi, nil
}

// FetchHeaders returns the cached headers for uids. Cache misses
// fall back to the backend. Results are upserted before returning
// so the next call hits cache. The returned slice order matches
// uids. Missing UIDs are silently skipped.
func (a *Account) FetchHeaders(ctx context.Context, uids []mail.UID) ([]mail.MessageInfo, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	known := make(map[mail.UID]mail.MessageInfo, len(uids))
	placeholders, args := uidsPlaceholders(uids)
	q := `SELECT protocol_id, subject, from_addr, to_addr, cc_addr, bcc_addr, date_str,
              COALESCE(sent_at, 0), ui_flags, COALESCE(size, 0), thread_id, in_reply_to
          FROM messages WHERE protocol_id IN (` + placeholders + `) AND ui_hide = 0`
	rows, err := a.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query messages: %v", err)
	}
	for rows.Next() {
		mi, err := scanMessageInfo(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		known[mi.UID] = mi
	}
	rows.Close()

	var missing []mail.UID
	for _, u := range uids {
		if _, ok := known[u]; !ok {
			missing = append(missing, u)
		}
	}
	if len(missing) > 0 && a.Backend != nil {
		fresh, err := a.Backend.FetchHeaders(missing)
		if err != nil {
			return nil, fmt.Errorf("backend fetch headers: %w", err)
		}
		// folder == "" here is intentional. FetchHeaders is called by
		// callers that already know the folder context, or that don't
		// need membership (e.g. viewer-side body prefetch). The
		// membership row is established by SyncFolder/upsertMessages
		// when the message first lands in a folder.
		if err := a.upsertMessages(ctx, "", fresh); err != nil {
			return nil, err
		}
		for _, m := range fresh {
			known[m.UID] = m
		}
	}
	out := make([]mail.MessageInfo, 0, len(uids))
	for _, u := range uids {
		if mi, ok := known[u]; ok {
			out = append(out, mi)
		}
	}
	return out, nil
}

// FetchBody returns the body bytes for uid. Cache miss: fetch from
// backend, store, return. Cache hit: return without a backend
// round-trip. Store failure is non-fatal. The returned bytes are
// still valid for the caller. Lazy population, no automatic eviction.
// The size backstop in storeBody handles cap pressure inline.
func (a *Account) FetchBody(uid mail.UID) ([]byte, error) {
	ctx := context.Background()
	if buf, ok, err := a.lookupBody(ctx, uid); err != nil {
		return nil, fmt.Errorf("fetch body %s: lookup: %w", uid, err)
	} else if ok {
		return buf, nil
	}
	if a.Backend == nil {
		return nil, errors.New("cache: no backend")
	}
	body, err := a.Backend.FetchBody(uid)
	if err != nil {
		return nil, err
	}
	if storeErr := a.storeBody(ctx, uid, body); storeErr != nil {
		// Store failure is non-fatal. The returned body is valid. The next view re-fetches.
		_ = storeErr
	}
	return body, nil
}

// folderID resolves a canonical folder name to its row id.
func (a *Account) folderID(name string) (int64, error) {
	var id int64
	err := a.db.QueryRow(`SELECT id FROM folders WHERE name = ?`, name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("folder %q: %w", name, err)
	}
	return id, nil
}

// upsertMessages inserts or updates header rows and (when folder is
// non-empty) records membership in message_mailboxes.
func (a *Account) upsertMessages(ctx context.Context, folder string, msgs []mail.MessageInfo) error {
	if len(msgs) == 0 {
		return nil
	}
	var folderID int64
	if folder != "" {
		var err error
		folderID, err = a.folderID(folder)
		if err != nil {
			return err
		}
	}
	return a.tx(ctx, func(tx *sql.Tx) error {
		for _, m := range msgs {
			var sentNS int64
			if !m.SentAt.IsZero() {
				sentNS = m.SentAt.UnixNano()
			}
			res, err := tx.Exec(`
                INSERT INTO messages
                  (protocol_id, thread_id, in_reply_to, subject, from_addr, to_addr, cc_addr, bcc_addr,
                   date_str, sent_at, flags, size, ui_flags, ui_hide)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
                ON CONFLICT(protocol_id) DO UPDATE SET
                  thread_id   = excluded.thread_id,
                  in_reply_to = excluded.in_reply_to,
                  subject     = excluded.subject,
                  from_addr   = excluded.from_addr,
                  to_addr     = excluded.to_addr,
                  cc_addr     = excluded.cc_addr,
                  bcc_addr    = excluded.bcc_addr,
                  date_str    = excluded.date_str,
                  sent_at     = excluded.sent_at,
                  size        = excluded.size,
                  flags       = CASE
                                  WHEN EXISTS (SELECT 1 FROM outbox o WHERE o.message = messages.id AND o.status IN ('pending','executing'))
                                  THEN messages.flags
                                  ELSE excluded.flags
                                END,
                  ui_flags    = CASE
                                  WHEN EXISTS (SELECT 1 FROM outbox o WHERE o.message = messages.id AND o.status IN ('pending','executing'))
                                  THEN messages.ui_flags
                                  ELSE excluded.flags
                                END
            `, string(m.UID), string(m.ThreadID), string(m.InReplyTo), m.Subject, m.From,
				m.To, m.Cc, m.Bcc, m.Date, sentNS, uint32(m.Flags), int64(m.Size), uint32(m.Flags))
			if err != nil {
				return fmt.Errorf("upsert message %s: %w", m.UID, err)
			}
			id, err := res.LastInsertId()
			if err != nil {
				return err
			}
			if id == 0 {
				if err := tx.QueryRow(`SELECT id FROM messages WHERE protocol_id = ?`, string(m.UID)).Scan(&id); err != nil {
					return err
				}
			}
			if folder != "" {
				if _, err := tx.Exec(`INSERT OR IGNORE INTO message_mailboxes (message, folder) VALUES (?, ?)`, id, folderID); err != nil {
					return fmt.Errorf("link message %s ↔ folder: %w", m.UID, err)
				}
			}
		}
		return nil
	})
}

// sqlPlaceholders returns "?,?,...,?" with n question marks comma-separated.
func sqlPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}

// uidsPlaceholders returns "?,?,?" and the matching args slice for
// UIDs in IN clauses.
func uidsPlaceholders(uids []mail.UID) (string, []any) {
	if len(uids) == 0 {
		return "", nil
	}
	args := make([]any, len(uids))
	for i, u := range uids {
		args[i] = string(u)
	}
	return sqlPlaceholders(len(uids)), args
}
