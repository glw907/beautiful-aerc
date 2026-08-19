-- Schema version 1, the store's founding migration. Technical
-- design section 3 is the model; ADR-0001 and ADR-0002 (both
-- revision 2) settle one file for every account, poplar-minted
-- primary keys, and a single FTS5 content source.

CREATE TABLE account (
    id           INTEGER PRIMARY KEY,
    slug         TEXT NOT NULL UNIQUE,
    backend_kind TEXT NOT NULL,
    address      TEXT NOT NULL,
    data         TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE mailbox (
    id           INTEGER PRIMARY KEY,
    account_id   INTEGER NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    server_id    TEXT,
    role         TEXT NOT NULL DEFAULT '',
    name         TEXT NOT NULL,
    sort_order   INTEGER NOT NULL DEFAULT 0,
    visible      INTEGER NOT NULL DEFAULT 1,
    unread_count INTEGER NOT NULL DEFAULT 0,
    total_count  INTEGER NOT NULL DEFAULT 0,
    data         TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_mailbox_account ON mailbox(account_id);

CREATE UNIQUE INDEX idx_mailbox_account_server ON mailbox(account_id, server_id) WHERE server_id IS NOT NULL;

CREATE TABLE thread (
    id               INTEGER PRIMARY KEY,
    account_id       INTEGER NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    server_thread_id TEXT,
    muted            INTEGER NOT NULL DEFAULT 0,
    data             TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_thread_account ON thread(account_id);

CREATE TABLE message (
    id             INTEGER PRIMARY KEY,
    account_id     INTEGER NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    server_id      TEXT,
    blob_id        TEXT,
    thread_key     TEXT NOT NULL DEFAULT '',
    received_at    INTEGER NOT NULL,
    subject        TEXT NOT NULL DEFAULT '',
    from_addr      TEXT NOT NULL DEFAULT '',
    flags          INTEGER NOT NULL DEFAULT 0,
    size           INTEGER NOT NULL DEFAULT 0,
    has_attachment INTEGER NOT NULL DEFAULT 0,
    origin         TEXT NOT NULL DEFAULT 'server' CHECK (origin IN ('server', 'local')),
    hidden_until   INTEGER,
    search_text    TEXT NOT NULL DEFAULT '',
    data           TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_message_thread ON message(account_id, thread_key, received_at);

-- UpsertMessage and UpsertMailbox each look up their row by
-- (account_id, server_id) on every sync page; with no index that was
-- a full table scan, quadratic over a 100k-message baseline. UNIQUE
-- because two rows sharing an account and a server id is corruption,
-- not a valid state. Partial rather than plain: an origin = 'local'
-- draft (message) or a mailbox mid-create carries no server_id yet,
-- and a server-identity lookup never probes those rows anyway.
CREATE UNIQUE INDEX idx_message_account_server ON message(account_id, server_id) WHERE server_id IS NOT NULL;

-- A single content source (message itself), not the two-table
-- external-content shape ADR-0001 revision 2 retracts. Every write
-- to message.subject or message.search_text must delete the old
-- message_fts row (by prior column values) before writing the new
-- one; FTS5 external content cannot look that up on its own.
--
-- prefix='2 3 4' joined length 4 after measurement: an unindexed
-- 4-character keystroke cost 60-980ms against ~170µs indexed, a
-- trade worth the resulting file's 4% growth.
CREATE VIRTUAL TABLE message_fts USING fts5(
    subject, search_text,
    content='message', content_rowid='id',
    prefix='2 3 4'
);

-- message_fts mirrors message's subject and search_text through
-- three triggers, matching FTS5's own documented shape for an
-- external-content table (fts5.html, "External Content Tables").
-- Insert and targeted update keep the index current with the row's
-- own writes, using SQLite's NEW/OLD values directly rather than a
-- Go helper reading message back inside the transaction.
--
-- trg_message_fts_delete predates the other two and covers a case
-- they cannot: an account row's ON DELETE CASCADE reaches message
-- directly (not through mailbox) inside SQLite itself, below any
-- trigger scoped to a specific column update. It fires for every
-- deletion of a message row regardless of cause, cascade or direct.
--
-- Together the three triggers make message_fts's reciprocal
-- invariant structural: every message row carries a message_fts
-- entry. TestUnindexedMessageRowMustBeIndexed pins what happens if
-- that invariant is ever broken by hand: trg_message_fts_delete's
-- own delete command fails with SQLite's disk-image-malformed error
-- instead of a clean no-op.
CREATE TRIGGER trg_message_fts_insert AFTER INSERT ON message BEGIN
    INSERT INTO message_fts(rowid, subject, search_text)
    VALUES (NEW.id, NEW.subject, NEW.search_text);
END;

CREATE TRIGGER trg_message_fts_update AFTER UPDATE OF subject, search_text ON message BEGIN
    INSERT INTO message_fts(message_fts, rowid, subject, search_text)
    VALUES ('delete', OLD.id, OLD.subject, OLD.search_text);
    INSERT INTO message_fts(rowid, subject, search_text)
    VALUES (NEW.id, NEW.subject, NEW.search_text);
END;

CREATE TRIGGER trg_message_fts_delete AFTER DELETE ON message BEGIN
    INSERT INTO message_fts(message_fts, rowid, subject, search_text)
    VALUES ('delete', OLD.id, OLD.subject, OLD.search_text);
END;

CREATE TABLE message_mailbox (
    message_id  INTEGER NOT NULL REFERENCES message(id) ON DELETE CASCADE,
    mailbox_id  INTEGER NOT NULL REFERENCES mailbox(id) ON DELETE CASCADE,
    received_at INTEGER NOT NULL,
    unread      INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (message_id, mailbox_id)
);

-- received_at and unread are denormalized from message so the list
-- and unread-by-mailbox queries are covering index scans that never
-- touch the message table (ADR-0002's index-set revision).
CREATE INDEX idx_message_mailbox_list ON message_mailbox(mailbox_id, received_at DESC, message_id);
CREATE INDEX idx_message_mailbox_unread ON message_mailbox(mailbox_id, received_at DESC, message_id) WHERE unread = 1;

CREATE TABLE body (
    message_id INTEGER PRIMARY KEY REFERENCES message(id) ON DELETE CASCADE,
    content    BLOB NOT NULL,
    fetched_at INTEGER NOT NULL
);

CREATE TABLE calendar (
    id         INTEGER PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    href       TEXT,
    name       TEXT NOT NULL,
    color_slot INTEGER NOT NULL DEFAULT 0,
    visible    INTEGER NOT NULL DEFAULT 1,
    is_default INTEGER NOT NULL DEFAULT 0,
    data       TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_calendar_account ON calendar(account_id);

CREATE TABLE event (
    id            INTEGER PRIMARY KEY,
    account_id    INTEGER NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    calendar_id   INTEGER NOT NULL REFERENCES calendar(id) ON DELETE CASCADE,
    uid           TEXT NOT NULL,
    href          TEXT,
    etag          TEXT,
    raw_ics       BLOB,
    summary       TEXT NOT NULL DEFAULT '',
    start_local   INTEGER NOT NULL,
    tzid          TEXT NOT NULL DEFAULT '',
    duration_secs INTEGER NOT NULL DEFAULT 0,
    is_all_day    INTEGER NOT NULL DEFAULT 0,
    is_floating   INTEGER NOT NULL DEFAULT 0,
    is_recurring  INTEGER NOT NULL DEFAULT 0,
    transparency  TEXT NOT NULL DEFAULT 'opaque',
    sequence      INTEGER NOT NULL DEFAULT 0,
    data          TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_event_calendar ON event(calendar_id);

CREATE TABLE occurrence (
    event_id      INTEGER NOT NULL REFERENCES event(id) ON DELETE CASCADE,
    recurrence_id TEXT NOT NULL DEFAULT '',
    start_utc     INTEGER NOT NULL,
    end_utc       INTEGER NOT NULL,
    start_local   INTEGER NOT NULL,
    local_date    TEXT NOT NULL,
    PRIMARY KEY (event_id, recurrence_id)
);

CREATE INDEX idx_occurrence_start_utc ON occurrence(start_utc);
CREATE INDEX idx_occurrence_local_date ON occurrence(local_date);

CREATE TABLE contact_card (
    id         INTEGER PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    server_id  TEXT,
    uid        TEXT,
    full_name  TEXT NOT NULL DEFAULT '',
    data       TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_contact_card_account ON contact_card(account_id);

CREATE UNIQUE INDEX idx_contact_card_account_server ON contact_card(account_id, server_id) WHERE server_id IS NOT NULL;

CREATE TABLE contact_email (
    contact_card_id INTEGER NOT NULL REFERENCES contact_card(id) ON DELETE CASCADE,
    address         TEXT NOT NULL,
    rank_hint       INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (contact_card_id, address)
);

-- Not reachable by foreign key from any scoped parent: account_id
-- is this table's only link to its account, which is the case
-- ADR-0002's account-scoping rule exists to catch.
CREATE TABLE sent_history (
    account_id   INTEGER NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    address      TEXT NOT NULL,
    name         TEXT NOT NULL DEFAULT '',
    last_used_at INTEGER NOT NULL,
    use_count    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, address)
);

CREATE TABLE outbox (
    id              INTEGER PRIMARY KEY,
    account_id      INTEGER NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    kind            TEXT NOT NULL,
    payload         TEXT NOT NULL,
    -- 'queued' and 'dispatching' are ADR-0006 revision 2's claim-
    -- discipline states; a CHECK here refuses anything the dispatcher
    -- (task 10) does not itself write.
    state           TEXT NOT NULL DEFAULT 'queued' CHECK (state IN ('queued', 'dispatching')),
    undo_group      TEXT,
    chunk_seq       INTEGER NOT NULL DEFAULT 0,
    attempt_count   INTEGER NOT NULL DEFAULT 0,
    next_attempt_at INTEGER NOT NULL DEFAULT 0,
    failure_class   TEXT,
    failure_detail  TEXT,
    created_at      INTEGER NOT NULL
);

CREATE INDEX idx_outbox_dispatch ON outbox(state, next_attempt_at);

CREATE TABLE draft_meta (
    message_id   INTEGER PRIMARY KEY REFERENCES message(id) ON DELETE CASCADE,
    local_rev    INTEGER NOT NULL DEFAULT 0,
    pushed_rev   INTEGER NOT NULL DEFAULT 0,
    anchor_msgid TEXT
);

-- collection_id lets calendar and contacts hold a token per
-- collection (RFC 6578; ADR-0005's "poll by collection state"), while
-- mail keeps its single row per object kind under the '' sentinel.
CREATE TABLE sync_state (
    account_id         INTEGER NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    object_kind        TEXT NOT NULL,
    collection_id      TEXT NOT NULL DEFAULT '',
    server_state_token TEXT,
    local_rev          INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, object_kind, collection_id)
);
