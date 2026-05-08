package cache

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/glw907/poplar/internal/contacts"
)

const suggestQuery = `
WITH scored AS (
    SELECT mr.address                                                      AS addr,
           MAX(mr.name)                                                    AS name,
           SUM(1.0 / (1 + (julianday('now') - julianday(mr.sent_at, 'unixepoch')))) AS score
      FROM message_recipients mr
     WHERE LOWER(mr.address) LIKE ?
        OR LOWER(mr.name)    LIKE ?
     GROUP BY mr.address
)
SELECT s.addr,
       COALESCE(c.fn, s.name)              AS display,
       COALESCE(c.org, '')                 AS org,
       (c.uid IS NOT NULL AND c.family = '' AND c.given = '' AND c.org != '') AS is_org
  FROM scored s
  LEFT JOIN contact_emails ce ON LOWER(ce.address) = LOWER(s.addr)
  LEFT JOIN contacts c        ON c.uid = ce.contact_uid
 ORDER BY s.score DESC, s.addr ASC
 LIMIT 7;
`

// SuggestAddresses returns up to 7 ranked autocomplete rows. Empty prefix
// returns the most-recently-contacted addresses. Matches on address or
// display name, case-insensitive.
func (a *Account) SuggestAddresses(ctx context.Context, prefix string) ([]contacts.Suggestion, error) {
	pat := strings.ToLower(prefix) + "%"
	rows, err := a.db.QueryContext(ctx, suggestQuery, pat, pat)
	if err != nil {
		return nil, fmt.Errorf("suggest addresses: %w", err)
	}
	defer rows.Close()

	var out []contacts.Suggestion
	for rows.Next() {
		var s contacts.Suggestion
		var isOrg int
		if err := rows.Scan(&s.Email, &s.Name, &s.Org, &isOrg); err != nil {
			return nil, err
		}
		s.IsOrg = isOrg == 1
		if s.Name == "" {
			s.Name = s.Email
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// LookupContact returns the cached contact for an email address.
// ok=false means no match; the caller falls back to the name on the message.
func (a *Account) LookupContact(ctx context.Context, address string) (contacts.Contact, bool) {
	const q = `
SELECT c.uid, c.fn, c.family, c.given, c.org, c.title, c.note
  FROM contact_emails ce
  JOIN contacts c ON c.uid = ce.contact_uid
 WHERE LOWER(ce.address) = LOWER(?)
 LIMIT 1
`
	var uid string
	var c contacts.Contact
	err := a.db.QueryRowContext(ctx, q, address).Scan(
		&uid, &c.Name, &c.Family, &c.Given, &c.Org, &c.Title, &c.Note,
	)
	if err == sql.ErrNoRows {
		return contacts.Contact{}, false
	}
	if err != nil {
		return contacts.Contact{}, false
	}
	if c.Family == "" && c.Given == "" && c.Org != "" {
		c.Kind = contacts.KindOrg
	}
	c.Emails = a.loadEmails(ctx, uid)
	c.Phones = a.loadPhones(ctx, uid)
	return c, true
}

func (a *Account) loadEmails(ctx context.Context, uid string) []contacts.Email {
	rows, err := a.db.QueryContext(ctx,
		`SELECT address, label FROM contact_emails WHERE contact_uid = ? ORDER BY pref ASC`, uid)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []contacts.Email
	for rows.Next() {
		var e contacts.Email
		if rows.Scan(&e.Address, &e.Label) == nil {
			out = append(out, e)
		}
	}
	return out
}

func (a *Account) loadPhones(ctx context.Context, uid string) []contacts.Phone {
	rows, err := a.db.QueryContext(ctx,
		`SELECT number, label FROM contact_phones WHERE contact_uid = ? ORDER BY pref ASC`, uid)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []contacts.Phone
	for rows.Next() {
		var p contacts.Phone
		if rows.Scan(&p.E164, &p.Label) == nil {
			out = append(out, p)
		}
	}
	return out
}

// Books returns every cached address book keyed by href.
func (a *Account) Books(ctx context.Context) (map[string]contacts.BookState, error) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT href, display_name, description, sync_token, ctag, supports_sync, last_synced_at FROM addressbooks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]contacts.BookState{}
	for rows.Next() {
		var b contacts.BookState
		var supports int
		var ts int64
		if err := rows.Scan(&b.Href, &b.DisplayName, &b.Description, &b.SyncToken, &b.CTAG, &supports, &ts); err != nil {
			return nil, err
		}
		b.SupportsSync = supports == 1
		b.LastSyncedAt = time.Unix(ts, 0)
		out[b.Href] = b
	}
	return out, rows.Err()
}

// UpsertBook persists address book metadata. Empty SyncToken and CTAG are valid.
func (a *Account) UpsertBook(ctx context.Context, b contacts.BookState) error {
	supports := 0
	if b.SupportsSync {
		supports = 1
	}
	_, err := a.db.ExecContext(ctx,
		`INSERT INTO addressbooks(href, display_name, description, sync_token, ctag, supports_sync, last_synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(href) DO UPDATE SET
		   display_name=excluded.display_name,
		   description=excluded.description,
		   sync_token=excluded.sync_token,
		   ctag=excluded.ctag,
		   supports_sync=excluded.supports_sync,
		   last_synced_at=excluded.last_synced_at`,
		b.Href, b.DisplayName, b.Description, b.SyncToken, b.CTAG, supports, b.LastSyncedAt.Unix(),
	)
	return err
}

// ApplyChangeset writes added/updated contacts and removes deleted ones
// atomically. token and ctag are stored only on success.
func (a *Account) ApplyChangeset(ctx context.Context, bookHref string, added []contacts.Stored, removed []string, token, ctag string) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, href := range removed {
		if _, err := tx.ExecContext(ctx, `DELETE FROM contacts WHERE href = ?`, href); err != nil {
			return err
		}
	}
	now := time.Now().Unix()
	for _, s := range added {
		if err := upsertContactTx(ctx, tx, bookHref, s, now); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE addressbooks SET sync_token=?, ctag=? WHERE href=?`,
		token, ctag, bookHref); err != nil {
		return err
	}
	return tx.Commit()
}

func upsertContactTx(ctx context.Context, tx *sql.Tx, bookHref string, s contacts.Stored, now int64) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO contacts(uid, addressbook_href, href, etag, vcard, rev, fn, family, given, org, title, note, last_synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(uid) DO UPDATE SET
		   addressbook_href=excluded.addressbook_href,
		   href=excluded.href,
		   etag=excluded.etag,
		   vcard=excluded.vcard,
		   rev=excluded.rev,
		   fn=excluded.fn,
		   family=excluded.family,
		   given=excluded.given,
		   org=excluded.org,
		   title=excluded.title,
		   note=excluded.note,
		   last_synced_at=excluded.last_synced_at`,
		s.UID, bookHref, s.Href, s.ETag, s.Raw, s.Rev,
		s.Contact.Name, s.Contact.Family, s.Contact.Given,
		s.Contact.Org, s.Contact.Title, s.Contact.Note, now,
	)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM contact_emails WHERE contact_uid=?`, s.UID); err != nil {
		return err
	}
	for i, e := range s.Contact.Emails {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO contact_emails(contact_uid, address, label, pref) VALUES (?, ?, ?, ?)`,
			s.UID, e.Address, e.Label, i+1); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM contact_phones WHERE contact_uid=?`, s.UID); err != nil {
		return err
	}
	for i, p := range s.Contact.Phones {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO contact_phones(contact_uid, number, label, pref) VALUES (?, ?, ?, ?)`,
			s.UID, p.E164, p.Label, i+1); err != nil {
			return err
		}
	}
	return nil
}

// SyncContacts runs one CardDAV sync pass. cfg==nil is a no-op (account has no contacts config).
func (a *Account) SyncContacts(ctx context.Context, cfg *contacts.ClientConfig) error {
	if cfg == nil {
		return nil
	}
	cl, err := contacts.NewClient(cfg.URL, cfg.Username, cfg.Password, cfg.InsecureTLS)
	if err != nil {
		return err
	}
	return contacts.Sync(ctx, cl, a)
}
