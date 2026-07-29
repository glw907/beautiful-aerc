// Package store stands in for poplar's internal/store. Every
// exported name here is one poplar's store really exports, so a
// fixture exercises the classifier against the shape it meets in
// the module rather than an invented one.
package store

import (
	"context"
	"database/sql"
	"time"
)

// Writer is the single write connection, run by one goroutine.
type Writer struct{}

// NewWriter starts the writer goroutine over db.
func NewWriter(db *sql.DB, cfg WriterConfig) (*Writer, error) { return &Writer{}, nil }

// Open opens the store at path, migrates it, and starts the writer.
func Open(path string, cfg WriterConfig) (*Writer, error) { return &Writer{}, nil }

// OpenWriteConn opens path as the store's write connection.
func OpenWriteConn(path string) (*sql.DB, error) { return nil, nil }

// Migrate brings db up to the schema version this build knows.
func Migrate(db *sql.DB) error { return nil }

// CheckIntegrity runs SQLite's quick_check against db.
func CheckIntegrity(ctx context.Context, db *sql.DB) error { return nil }

// Recover rebuilds the store at path from its non-rebuildable state.
func Recover(ctx context.Context, path string) error { return nil }

// RebuildIndex regenerates the full-text index through w.
func RebuildIndex(ctx context.Context, w *Writer) error { return nil }

// Apply runs fn as one transaction on the writer's bulk lane.
func (w *Writer) Apply(ctx context.Context, fn func(*sql.Tx) error) error { return fn(nil) }

// ApplyInteractive runs fn as one transaction on the interactive lane.
func (w *Writer) ApplyInteractive(ctx context.Context, fn func(*sql.Tx) error) error {
	return fn(nil)
}

// Close stops the writer goroutine.
func (w *Writer) Close() error { return nil }

// WriterConfig governs the writer's admission and checkpoint timing.
type WriterConfig struct {
	InteractiveQuiet time.Duration
}

// DefaultWriterConfig holds poplar's production writer timings.
func DefaultWriterConfig() WriterConfig { return WriterConfig{} }

// MessageUpsert is one message's writable scalar fields.
type MessageUpsert struct {
	ServerID string
	Subject  string
}

// MailboxUpsert is one mailbox's writable scalar fields.
type MailboxUpsert struct {
	ServerID string
	Name     string
}

// UpsertMessage writes m as accountID's message inside tx.
func UpsertMessage(tx *sql.Tx, accountID int64, m MessageUpsert) error { return nil }

// UpsertMailbox writes m as accountID's mailbox inside tx.
func UpsertMailbox(tx *sql.Tx, accountID int64, m MailboxUpsert) error { return nil }

// DeleteMessage removes accountID's message with server id serverID.
func DeleteMessage(tx *sql.Tx, accountID int64, serverID string) error { return nil }

// DeleteMessageByID removes the message row with the given id.
func DeleteMessageByID(tx *sql.Tx, id int64) error { return nil }

// DeleteMailbox removes accountID's mailbox with server id serverID.
func DeleteMailbox(tx *sql.Tx, accountID int64, serverID string) error { return nil }

// DeleteMailboxByID removes the mailbox row with the given id.
func DeleteMailboxByID(tx *sql.Tx, id int64) error { return nil }

// SyncMessageMailboxes reconciles one message's mailbox associations.
func SyncMessageMailboxes(tx *sql.Tx, accountID, messageID int64, serverIDs []string) error {
	return nil
}

// RepairMailboxAssociations re-associates messages naming a mailbox
// whose local row arrived late.
func RepairMailboxAssociations(tx *sql.Tx, accountID, mailboxID int64, serverID string) error {
	return nil
}

// StaleMessageIDs returns the message rows absent from keep.
func StaleMessageIDs(tx *sql.Tx, accountID int64, keep map[string]bool) ([]int64, error) {
	return nil, nil
}

// StaleMailboxIDs returns the mailbox rows absent from keep.
func StaleMailboxIDs(tx *sql.Tx, accountID int64, keep map[string]bool) ([]int64, error) {
	return nil, nil
}

// ReadPool is the pool of read-only connections, with no Exec method.
type ReadPool struct{}

// NewReadPool opens size read-only connections onto the store file.
func NewReadPool(path string, size int) (*ReadPool, error) { return &ReadPool{}, nil }

// ListMailboxForward returns one page of a mailbox's messages.
func (p *ReadPool) ListMailboxForward(ctx context.Context, mailboxID int64) ([]MessageSummary, error) {
	return nil, nil
}

// MessageSummary is one message's list-painting columns.
type MessageSummary struct {
	MessageID int64
	Subject   string
}

// Flags is the query-relevant subset of a message's keywords.
type Flags uint32

// DecodeFlags reconstructs a message's full keyword set.
func DecodeFlags(bits Flags, overflow []string) []string { return nil }

// MarkCleanShutdown records that the store at dbPath shut down cleanly.
func MarkCleanShutdown(dbPath string) error { return nil }
