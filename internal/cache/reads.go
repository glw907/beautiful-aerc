package cache

import (
	"context"
	"database/sql"
	"fmt"
	gomail "net/mail"
	"strings"
	"time"

	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/mailcompose"
)

// ListFolders returns folder rows with classification and per-folder
// Exists/Unseen counts. The cache is the source of truth; the syncer
// keeps it converged with the backend.
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
		// Local counts win once any messages are synced. Otherwise the
		// SyncFolders-cached server totals seed unopened-folder badges.
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

// QueryFolder returns up to limit messages from folder, sent_at DESC,
// from offset. Rows with ui_hide = 1 (mid-move sources) are filtered.
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
               m.bcc_addr, COALESCE(m.sent_at, 0), m.ui_flags,
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

// folderRole returns the role for folder, or "" if absent or NULL.
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

// draftsAsMessageInfo projects local-only drafts as message-list
// rows with synthetic UIDs "draft:<id>". The App's Enter handler
// keys off the prefix to route through LoadDraft. Pushed drafts
// already reach the list via the normal server-message path.
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
		d, err := mailcompose.DecodeDraft(r.Payload)
		if err != nil {
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

// messageInfoCols is the 11-pointer scan target for the canonical
// message-row column order. Callers that join additional columns
// append them after cols() then materialize the message via mi().
type messageInfoCols struct {
	pid, subj, from, to, cc, bcc, thread, irt string
	sentNS                                    int64
	flags                                     uint32
	size                                      int64
}

func (c *messageInfoCols) cols() []any {
	return []any{&c.pid, &c.subj, &c.from, &c.to, &c.cc, &c.bcc, &c.sentNS, &c.flags, &c.size, &c.thread, &c.irt}
}

func (c *messageInfoCols) mi() mail.MessageInfo {
	mi := mail.MessageInfo{
		UID: mail.UID(c.pid), Subject: c.subj, From: c.from, To: c.to, Cc: c.cc, Bcc: c.bcc,
		Flags: mail.Flag(c.flags), Size: uint32(c.size),
		ThreadID: mail.UID(c.thread), InReplyTo: mail.UID(c.irt),
	}
	if c.sentNS > 0 {
		mi.SentAt = time.Unix(0, c.sentNS)
	}
	return mi
}

func scanMessageInfo(rows *sql.Rows) (mail.MessageInfo, error) {
	var c messageInfoCols
	if err := rows.Scan(c.cols()...); err != nil {
		return mail.MessageInfo{}, err
	}
	return c.mi(), nil
}

// FetchHeaders returns cached headers for uids. Misses fetch from
// the backend and upsert. Result order matches uids. UIDs not found
// in either layer are skipped.
func (a *Account) FetchHeaders(ctx context.Context, uids []mail.UID) ([]mail.MessageInfo, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	known := make(map[mail.UID]mail.MessageInfo, len(uids))
	placeholders, args := uidsPlaceholders(uids)
	q := `SELECT protocol_id, subject, from_addr, to_addr, cc_addr, bcc_addr,
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
	if len(missing) > 0 {
		if !a.Connected() {
			return nil, ErrNotConnected
		}
		fresh, err := a.Backend.FetchHeaders(missing)
		if err != nil {
			return nil, fmt.Errorf("backend fetch headers: %w", err)
		}
		// Empty folder skips the membership write. Callers that need
		// it have already paired the message with a folder via SyncFolder.
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

// FetchBody returns body bytes for uid. Cache miss falls through to
// the backend. On success the body is stored under storeBody's size
// backstop. Lazy population, no automatic eviction.
func (a *Account) FetchBody(uid mail.UID) ([]byte, error) {
	ctx := context.Background()
	if buf, ok, err := a.lookupBody(ctx, uid); err != nil {
		return nil, fmt.Errorf("fetch body %s: lookup: %w", uid, err)
	} else if ok {
		return buf, nil
	}
	if !a.Connected() {
		return nil, ErrNotConnected
	}
	body, err := a.Backend.FetchBody(uid)
	if err != nil {
		return nil, err
	}
	if storeErr := a.storeBody(ctx, uid, body); storeErr != nil {
		a.log.Warn("cache: storeBody", "uid", uid, "err", storeErr)
	}
	return body, nil
}

func (a *Account) folderID(name string) (int64, error) {
	var id int64
	err := a.db.QueryRow(`SELECT id FROM folders WHERE name = ?`, name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("folder %q: %w", name, err)
	}
	return id, nil
}

// upsertMessages inserts or updates header rows. A non-empty folder
// also records membership in message_mailboxes.
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
			// RETURNING id over LastInsertId. The latter is the
			// connection-scoped rowid of the most recent real INSERT.
			// On the UPDATE branch of an UPSERT it returns a stale
			// value (often a now-deleted rowid), and the FK link below
			// blows up downstream.
			var id int64
			err := tx.QueryRow(`
                INSERT INTO messages
                  (protocol_id, thread_id, in_reply_to, subject, from_addr, to_addr, cc_addr, bcc_addr,
                   sent_at, flags, size, ui_flags, ui_hide)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
                ON CONFLICT(protocol_id) DO UPDATE SET
                  thread_id   = excluded.thread_id,
                  in_reply_to = excluded.in_reply_to,
                  subject     = excluded.subject,
                  from_addr   = excluded.from_addr,
                  to_addr     = excluded.to_addr,
                  cc_addr     = excluded.cc_addr,
                  bcc_addr    = excluded.bcc_addr,
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
                RETURNING id
            `, string(m.UID), string(m.ThreadID), string(m.InReplyTo), m.Subject, m.From,
				m.To, m.Cc, m.Bcc, sentNS, uint32(m.Flags), int64(m.Size), uint32(m.Flags)).Scan(&id)
			if err != nil {
				return fmt.Errorf("upsert message %s: %w", m.UID, err)
			}
			if err := writeRecipientsTx(ctx, tx, id, &m); err != nil {
				return fmt.Errorf("write recipients %s: %w", m.UID, err)
			}
			if err := writeFTSHeadersTx(ctx, tx, id, &m); err != nil {
				return fmt.Errorf("write fts headers %s: %w", m.UID, err)
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

func writeRecipientsTx(ctx context.Context, tx *sql.Tx, msgID int64, m *mail.MessageInfo) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM message_recipients WHERE message_uid = ?`, msgID); err != nil {
		return err
	}
	sentAt := m.SentAt.Unix()
	for _, role := range []struct {
		name string
		raw  string
	}{
		{"from", m.From},
		{"to", m.To},
		{"cc", m.Cc},
	} {
		if role.raw == "" {
			continue
		}
		addrs, err := gomail.ParseAddressList(role.raw)
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			_, err := tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO message_recipients(message_uid, role, address, name, sent_at) VALUES (?, ?, ?, ?, ?)`,
				msgID, role.name, strings.ToLower(addr.Address), addr.Name, sentAt)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// sqlPlaceholders returns "?,?,...,?" with n question marks.
func sqlPlaceholders(n int) string {
	return strings.Repeat("?,", n-1) + "?"
}

// uidsPlaceholders returns the placeholder string and matching args
// slice for UIDs in IN clauses.
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
