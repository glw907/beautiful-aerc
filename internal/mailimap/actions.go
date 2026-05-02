// SPDX-License-Identifier: MIT

package mailimap

import (
	"errors"
	"fmt"
	"io"

	"github.com/glw907/poplar/internal/mail"
)

// Move satisfies mail.Backend. Uses UID MOVE (RFC 6851) when the
// server advertises MOVE; falls back to COPY + STORE \Deleted +
// UID EXPUNGE otherwise. The fallback is a single logical
// operation; partial failure leaves the source folder in a known
// state by surfacing the error before the EXPUNGE fires.
func (b *Backend) Move(uids []mail.UID, dest string) error {
	if len(uids) == 0 {
		return nil
	}
	b.mu.Lock()
	cmd := b.cmd
	hasMove := b.caps.MOVE
	b.mu.Unlock()

	if hasMove {
		if err := cmd.Move(uids, dest); err != nil {
			return fmt.Errorf("uid move: %w", err)
		}
		return nil
	}
	if err := cmd.Copy(uids, dest); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	if err := cmd.Store(uids, "+FLAGS.SILENT", []string{"\\Deleted"}); err != nil {
		return fmt.Errorf("store deleted: %w", err)
	}
	if err := cmd.UIDExpunge(uids); err != nil {
		return fmt.Errorf("uid expunge: %w", err)
	}
	return nil
}

// Copy satisfies mail.Backend.
func (b *Backend) Copy(uids []mail.UID, dest string) error {
	if len(uids) == 0 {
		return nil
	}
	b.mu.Lock()
	cmd := b.cmd
	b.mu.Unlock()

	if err := cmd.Copy(uids, dest); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
}

// Delete satisfies mail.Backend. Soft-deletes to Trash. Resolves the
// Trash folder name via ListFolders + Classify; returns an error
// rather than expunging in place when no Trash folder is found.
func (b *Backend) Delete(uids []mail.UID) error {
	trash, err := b.resolveTrashFolder()
	if err != nil {
		return err
	}
	return b.Move(uids, trash)
}

// resolveTrashFolder returns the server-side name of the Trash folder.
// Returns an error if no folder with Canonical == "Trash" is found.
func (b *Backend) resolveTrashFolder() (string, error) {
	folders, err := b.ListFolders()
	if err != nil {
		return "", fmt.Errorf("list folders: %w", err)
	}
	for _, cf := range mail.Classify(folders) {
		if cf.Canonical == "Trash" {
			return cf.Folder.Name, nil
		}
	}
	return "", errors.New("no Trash folder")
}

// Destroy satisfies mail.Backend. Permanently deletes via STORE \Deleted
// then UID EXPUNGE. Per ADR-0092: empty input is a no-op, the
// operation is irreversible, missing UIDs are treated as success
// (the server silently ignores them).
func (b *Backend) Destroy(uids []mail.UID) error {
	if len(uids) == 0 {
		return nil
	}
	b.mu.Lock()
	cmd := b.cmd
	b.mu.Unlock()

	if err := cmd.Store(uids, "+FLAGS.SILENT", []string{"\\Deleted"}); err != nil {
		return fmt.Errorf("store deleted: %w", err)
	}
	if err := cmd.UIDExpunge(uids); err != nil {
		return fmt.Errorf("uid expunge: %w", err)
	}
	return nil
}

// Flag satisfies mail.Backend. item is +FLAGS.SILENT when set is true,
// -FLAGS.SILENT otherwise. Unknown flag bits are silently ignored.
func (b *Backend) Flag(uids []mail.UID, f mail.Flag, set bool) error {
	if len(uids) == 0 {
		return nil
	}
	flags := imapFlagsFor(f)
	if len(flags) == 0 {
		return nil
	}
	b.mu.Lock()
	cmd := b.cmd
	b.mu.Unlock()

	item := "+FLAGS.SILENT"
	if !set {
		item = "-FLAGS.SILENT"
	}
	if err := cmd.Store(uids, item, flags); err != nil {
		return fmt.Errorf("store flags: %w", err)
	}
	return nil
}

// MarkRead satisfies mail.Backend.
func (b *Backend) MarkRead(uids []mail.UID) error { return b.Flag(uids, mail.FlagSeen, true) }

// MarkUnread satisfies mail.Backend.
func (b *Backend) MarkUnread(uids []mail.UID) error { return b.Flag(uids, mail.FlagSeen, false) }

// MarkAnswered satisfies mail.Backend.
func (b *Backend) MarkAnswered(uids []mail.UID) error {
	return b.Flag(uids, mail.FlagAnswered, true)
}

// Send satisfies mail.Backend. Not implemented until Pass 9.
func (b *Backend) Send(_ string, _ []string, _ io.Reader) error {
	return errors.New("send: not implemented (lands in Pass 9)")
}

// imapFlagsFor maps mail.Flag bits to IMAP system flag strings.
func imapFlagsFor(f mail.Flag) []string {
	var out []string
	if f&mail.FlagSeen != 0 {
		out = append(out, "\\Seen")
	}
	if f&mail.FlagAnswered != 0 {
		out = append(out, "\\Answered")
	}
	if f&mail.FlagFlagged != 0 {
		out = append(out, "\\Flagged")
	}
	if f&mail.FlagDeleted != 0 {
		out = append(out, "\\Deleted")
	}
	if f&mail.FlagDraft != 0 {
		out = append(out, "\\Draft")
	}
	return out
}
