package mailimap

import (
	"fmt"
	"strings"

	"github.com/glw907/poplar/internal/mail"
)

// ListFolders uses LIST RETURN (SPECIAL-USE) when advertised, plain
// LIST otherwise. Role comes from RFC 6154 attributes. mail.Classify
// handles name-based fallback at the UI layer.
func (b *Backend) ListFolders() ([]mail.Folder, error) {
	cmd, err := b.cmdClient()
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	b.mu.Lock()
	useSpecial := b.caps.SpecialUse
	b.mu.Unlock()

	entries, err := cmd.List("", "*", useSpecial)
	if err != nil {
		wrapped := classifyErr(err)
		b.maybeDropOnConn(cmd, wrapped)
		return nil, fmt.Errorf("list: %w", wrapped)
	}

	// Exists/Unseen stay zero; OpenFolder returns fresh counts.
	out := make([]mail.Folder, 0, len(entries))
	for _, e := range entries {
		out = append(out, mail.Folder{Name: e.Name, Role: roleFromAttrs(e.Attributes)})
	}
	return out, nil
}

// roleFromAttrs maps RFC 6154 LIST attributes to the role values used
// by mail.Classify. Unknown or non-canonical attributes return "".
func roleFromAttrs(attrs []string) string {
	for _, a := range attrs {
		switch strings.ToLower(strings.TrimPrefix(a, "\\")) {
		case "drafts":
			return "drafts"
		case "sent":
			return "sent"
		case "trash":
			return "trash"
		case "junk":
			return "junk"
		case "archive", "all":
			return "archive"
		}
	}
	return ""
}

// OpenFolder selects on the command connection and signals the idle
// goroutine to re-IDLE on the new folder.
func (b *Backend) OpenFolder(name string) error {
	cmd, err := b.cmdClient()
	if err != nil {
		return fmt.Errorf("open %q: %w", name, err)
	}
	b.mu.Lock()
	switchCh := b.switchCh
	b.mu.Unlock()

	f, err := cmd.Select(name, false)
	if err != nil {
		wrapped := classifyErr(err)
		b.maybeDropOnConn(cmd, wrapped)
		return fmt.Errorf("select %q: %w", name, wrapped)
	}

	b.mu.Lock()
	b.current = name
	b.currentUIDVal = f.UIDValidity
	b.mu.Unlock()

	if switchCh != nil {
		// Drop any earlier pending switch so only the latest wins.
		select {
		case <-switchCh:
		default:
		}
		select {
		case switchCh <- name:
		default:
		}
	}
	return nil
}
