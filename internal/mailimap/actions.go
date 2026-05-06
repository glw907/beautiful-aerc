// SPDX-License-Identifier: MIT

package mailimap

import (
	"errors"
	"fmt"

	"github.com/glw907/poplar/internal/mail"
)

// Move uses UID MOVE (RFC 6851) when the
// server advertises MOVE. It falls back to COPY + STORE \Deleted +
// UID EXPUNGE otherwise. The fallback is a single logical
// operation. Partial failure leaves the source folder in a known
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
			return fmt.Errorf("uid move: %w", classifyErr(err))
		}
		return nil
	}
	if err := cmd.Copy(uids, dest); err != nil {
		return fmt.Errorf("move %s: copy: %w", dest, classifyErr(err))
	}
	if err := cmd.Store(uids, "+FLAGS.SILENT", []string{"\\Deleted"}); err != nil {
		return fmt.Errorf("move %s: mark source deleted: %w", dest, classifyErr(err))
	}
	if err := cmd.UIDExpunge(uids); err != nil {
		return fmt.Errorf("move %s: expunge source: %w", dest, classifyErr(err))
	}
	return nil
}

// resolveTrashFolder returns the server-side name of the Trash folder,
// caching the result on the Backend so repeated Deletes don't re-LIST.
// Returns an error if no folder with Canonical == "Trash" is found.
func (b *Backend) resolveTrashFolder() (string, error) {
	b.mu.Lock()
	cached := b.trash
	b.mu.Unlock()
	if cached != "" {
		return cached, nil
	}
	folders, err := b.ListFolders()
	if err != nil {
		return "", fmt.Errorf("list folders: %w", classifyErr(err))
	}
	for _, cf := range mail.Classify(folders) {
		if cf.Canonical == "Trash" {
			b.mu.Lock()
			b.trash = cf.Folder.Name
			b.mu.Unlock()
			return cf.Folder.Name, nil
		}
	}
	return "", errors.New("no Trash folder")
}

// Destroy permanently deletes via STORE \Deleted
// then UID EXPUNGE. Per ADR-0092: empty input is a no-op, the
// operation is irreversible, missing UIDs are treated as success
// (the server silently ignores them).
//
// On Gmail (b.cfg.GmailQuirks), EXPUNGE outside [Gmail]/Trash only
// removes labels. It does not delete. Destroy on a Gmail backend
// therefore selects [Gmail]/Trash before STORE+EXPUNGE. The caller
// must pass UIDs that already live in Trash. Both real callers
// (manual Empty Trash per ADR-0094, retention sweep per ADR-0093)
// satisfy this because they only trigger inside Disposal folders.
func (b *Backend) Destroy(uids []mail.UID) error {
	if len(uids) == 0 {
		return nil
	}
	b.mu.Lock()
	cmd := b.cmd
	gmail := b.cfg.GmailQuirks
	b.mu.Unlock()

	if gmail {
		trash, err := b.resolveTrashFolder()
		if err != nil {
			return fmt.Errorf("destroy: %w", classifyErr(err))
		}
		if _, err := cmd.Select(trash, false); err != nil {
			return fmt.Errorf("select trash: %w", classifyErr(err))
		}
	}

	if err := cmd.Store(uids, "+FLAGS.SILENT", []string{"\\Deleted"}); err != nil {
		return fmt.Errorf("destroy: mark deleted: %w", classifyErr(err))
	}
	if err := cmd.UIDExpunge(uids); err != nil {
		return fmt.Errorf("destroy: expunge: %w", classifyErr(err))
	}
	return nil
}

// Flag item is +FLAGS.SILENT when set is true,
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
		return fmt.Errorf("store flags: %w", classifyErr(err))
	}
	return nil
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
