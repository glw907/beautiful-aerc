package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/glw907/poplar/internal/contacts"
)

var _ contacts.Store = (*Account)(nil)

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
func (a *Account) LookupContact(ctx context.Context, address string) (contacts.Contact, string, bool) {
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
	if errors.Is(err, sql.ErrNoRows) {
		return contacts.Contact{}, "", false
	}
	if err != nil {
		return contacts.Contact{}, "", false
	}
	if c.Family == "" && c.Given == "" && c.Org != "" {
		c.Kind = contacts.KindOrg
	}
	c.Emails = a.loadEmails(ctx, uid)
	c.Phones = a.loadPhones(ctx, uid)
	return c, uid, true
}

// LoadStoredVCard returns the raw vCard bytes, etag, and href for uid.
func (a *Account) LoadStoredVCard(ctx context.Context, uid string) (raw []byte, etag, href string, err error) {
	err = a.db.QueryRowContext(ctx,
		`SELECT vcard, etag, href FROM contacts WHERE uid = ?`, uid,
	).Scan(&raw, &etag, &href)
	return
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
	return a.tx(ctx, func(tx *sql.Tx) error {
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
		_, err := tx.ExecContext(ctx,
			`UPDATE addressbooks SET sync_token=?, ctag=? WHERE href=?`,
			token, ctag, bookHref)
		return err
	})
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

// contactsFolderID returns the ID of the __contacts__ sentinel folder,
// creating it inside tx if absent. Contact ops are not scoped to a mail
// folder; this sentinel satisfies the outbox.folder NOT NULL constraint
// without polluting the user-visible folder list.
func contactsFolderID(ctx context.Context, tx *sql.Tx) (int64, error) {
	const name = "__contacts__"
	if _, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO folders(name, protocol_name) VALUES (?, '')`, name); err != nil {
		return 0, fmt.Errorf("ensure contacts sentinel folder: %w", err)
	}
	var id int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM folders WHERE name = ?`, name).Scan(&id); err != nil {
		return 0, fmt.Errorf("read contacts sentinel folder: %w", err)
	}
	return id, nil
}

// QueueContactPut writes vcardBytes to the cache and enqueues a CardDAV
// PUT in one transaction. ifMatch is the etag for optimistic concurrency
// (empty for new contacts). The caller assembles vcardBytes via
// contacts.PatchVCard or contacts.BuildVCard.
func (a *Account) QueueContactPut(
	ctx context.Context,
	bookHref string,
	uid string,
	href string,
	ifMatch string,
	c contacts.Contact,
	vcardBytes []byte,
) error {
	args := ContactPutArgs{BookHref: bookHref, Href: href, IfMatch: ifMatch}
	body, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("encode args: %w", err)
	}
	now := time.Now()
	err = a.tx(ctx, func(tx *sql.Tx) error {
		oldEmails, err := loadEmailsTx(ctx, tx, uid)
		if err != nil {
			return err
		}
		stored := contacts.Stored{
			Parsed: contacts.Parsed{
				UID:     uid,
				Rev:     now.UTC().Format(time.RFC3339),
				Raw:     vcardBytes,
				Contact: c,
			},
			Href: href,
			ETag: ifMatch,
		}
		if err := upsertContactTx(ctx, tx, bookHref, stored, now.Unix()); err != nil {
			return err
		}
		if err := reconcileRecipientsTx(ctx, tx, uid, oldEmails, c.Emails); err != nil {
			return err
		}
		folderID, err := contactsFolderID(ctx, tx)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO outbox (folder, message, kind, args, payload, enqueued_at, status, attempts, next_eligible_at)
			VALUES (?, NULL, ?, ?, ?, ?, ?, 0, NULL)`,
			folderID, string(KindContactPut), string(body), vcardBytes, now.UnixNano(), OpPending)
		return err
	})
	if err != nil {
		return err
	}
	a.signalDrainer()
	return nil
}

// QueueContactDelete tombstones the local contact row and enqueues a
// CardDAV DELETE in one transaction. The contact's href and etag are
// read inside the same transaction so the outbox row carries a coherent
// IfMatch guard.
func (a *Account) QueueContactDelete(ctx context.Context, uid string) error {
	now := time.Now()
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var href, etag string
		if err := tx.QueryRowContext(ctx,
			`SELECT href, etag FROM contacts WHERE uid = ?`, uid).Scan(&href, &etag); err != nil {
			return fmt.Errorf("lookup contact %s: %w", uid, err)
		}
		args := ContactDeleteArgs{Href: href, IfMatch: etag}
		body, err := json.Marshal(args)
		if err != nil {
			return fmt.Errorf("encode args: %w", err)
		}
		// FK cascade drops contact_emails and contact_phones.
		if _, err := tx.ExecContext(ctx, `DELETE FROM contacts WHERE uid = ?`, uid); err != nil {
			return err
		}
		folderID, err := contactsFolderID(ctx, tx)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO outbox (folder, message, kind, args, payload, enqueued_at, status, attempts, next_eligible_at)
			VALUES (?, NULL, ?, ?, NULL, ?, ?, 0, NULL)`,
			folderID, string(KindContactDelete), string(body), now.UnixNano(), OpPending)
		return err
	})
	if err != nil {
		return err
	}
	a.signalDrainer()
	return nil
}

// loadEmailsTx returns the stored emails for uid in pref order. Used
// by reconcileRecipientsTx to diff old-vs-new addresses.
func loadEmailsTx(ctx context.Context, tx *sql.Tx, uid string) ([]contacts.Email, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT address, label FROM contact_emails WHERE contact_uid = ? ORDER BY pref ASC`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []contacts.Email
	for rows.Next() {
		var e contacts.Email
		if err := rows.Scan(&e.Address, &e.Label); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// reconcileRecipientsTx is a no-op stub: contact_emails is the only
// address-keyed pivot, and upsertContactTx already replaces it.
// message_recipients rows are keyed by (message_uid, role, address);
// changing a contact's emails does not invalidate any historical row.
// A future pass adding a contact_uid FK to message_recipients will
// populate it here.
func reconcileRecipientsTx(_ context.Context, _ *sql.Tx, _ string, _, _ []contacts.Email) error {
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
