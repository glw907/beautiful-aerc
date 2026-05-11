package mailimap

import (
	"errors"
	"fmt"

	"github.com/glw907/poplar/internal/mail"
)

// Move uses UID MOVE (RFC 6851) when the server advertises MOVE,
// otherwise COPY + STORE \Deleted + UID EXPUNGE. Partial failure of
// the fallback surfaces before EXPUNGE fires, so the source folder
// stays in a known state.
func (b *Backend) Move(uids []mail.UID, dest string) error {
	if len(uids) == 0 {
		return nil
	}
	cmd, err := b.cmdClient()
	if err != nil {
		return fmt.Errorf("move: %w", err)
	}
	b.mu.Lock()
	hasMove := b.caps.MOVE
	b.mu.Unlock()

	if hasMove {
		if err := cmd.Move(uids, dest); err != nil {
			wrapped := classifyErr(err)
			b.maybeDropOnConn(cmd, wrapped)
			return fmt.Errorf("uid move: %w", wrapped)
		}
		return nil
	}
	if err := cmd.Copy(uids, dest); err != nil {
		wrapped := classifyErr(err)
		b.maybeDropOnConn(cmd, wrapped)
		return fmt.Errorf("move %s: copy: %w", dest, wrapped)
	}
	if err := cmd.Store(uids, "+FLAGS.SILENT", []string{"\\Deleted"}); err != nil {
		wrapped := classifyErr(err)
		b.maybeDropOnConn(cmd, wrapped)
		return fmt.Errorf("move %s: mark source deleted: %w", dest, wrapped)
	}
	if err := cmd.UIDExpunge(uids); err != nil {
		wrapped := classifyErr(err)
		b.maybeDropOnConn(cmd, wrapped)
		return fmt.Errorf("move %s: expunge source: %w", dest, wrapped)
	}
	return nil
}

// resolveTrashFolder returns the server-side name of the Trash folder,
// caching the result so repeated Deletes don't re-LIST.
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

// Destroy permanently deletes via STORE \Deleted then UID EXPUNGE
// (ADR-0092). Missing UIDs are silent successes.
//
// On Gmail, EXPUNGE outside [Gmail]/Trash only removes labels, so
// Destroy selects [Gmail]/Trash first. The caller must pass UIDs that
// already live in Trash, which both Empty Trash and the retention
// sweep do.
func (b *Backend) Destroy(uids []mail.UID) error {
	if len(uids) == 0 {
		return nil
	}
	cmd, err := b.cmdClient()
	if err != nil {
		return fmt.Errorf("destroy: %w", err)
	}
	b.mu.Lock()
	gmail := b.cfg.GmailQuirks
	b.mu.Unlock()

	if gmail {
		trash, err := b.resolveTrashFolder()
		if err != nil {
			return fmt.Errorf("destroy: %w", classifyErr(err))
		}
		if _, err := cmd.Select(trash, false); err != nil {
			wrapped := classifyErr(err)
			b.maybeDropOnConn(cmd, wrapped)
			return fmt.Errorf("select trash: %w", wrapped)
		}
	}

	if err := cmd.Store(uids, "+FLAGS.SILENT", []string{"\\Deleted"}); err != nil {
		wrapped := classifyErr(err)
		b.maybeDropOnConn(cmd, wrapped)
		return fmt.Errorf("destroy: mark deleted: %w", wrapped)
	}
	if err := cmd.UIDExpunge(uids); err != nil {
		wrapped := classifyErr(err)
		b.maybeDropOnConn(cmd, wrapped)
		return fmt.Errorf("destroy: expunge: %w", wrapped)
	}
	return nil
}

// Flag uses +FLAGS.SILENT when set is true, -FLAGS.SILENT otherwise.
// Unknown flag bits are silently ignored.
func (b *Backend) Flag(uids []mail.UID, f mail.Flag, set bool) error {
	if len(uids) == 0 {
		return nil
	}
	flags := imapFlagsFor(f)
	if len(flags) == 0 {
		return nil
	}
	cmd, err := b.cmdClient()
	if err != nil {
		return fmt.Errorf("flag: %w", err)
	}

	item := "+FLAGS.SILENT"
	if !set {
		item = "-FLAGS.SILENT"
	}
	if err := cmd.Store(uids, item, flags); err != nil {
		wrapped := classifyErr(err)
		b.maybeDropOnConn(cmd, wrapped)
		return fmt.Errorf("store flags: %w", wrapped)
	}
	return nil
}

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
