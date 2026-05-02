// SPDX-License-Identifier: MIT

package mailimap

import (
	"fmt"
	"strings"

	"github.com/glw907/poplar/internal/mail"
)

// ListFolders satisfies mail.Backend. Uses LIST RETURN (SPECIAL-USE)
// when the server advertises it; falls back to plain LIST otherwise.
// The role is derived from RFC 6154 attributes when present; the
// classifier (mail.Classify) handles name-based fallback at the UI
// layer.
func (b *Backend) ListFolders() ([]mail.Folder, error) {
	b.mu.Lock()
	cmd := b.cmd
	useSpecial := b.caps.SpecialUse
	b.mu.Unlock()

	entries, err := cmd.List("", "*", useSpecial)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	out := make([]mail.Folder, 0, len(entries))
	for _, e := range entries {
		f := mail.Folder{Name: e.Name, Role: roleFromAttrs(e.Attributes)}
		// Populate Exists/Unseen via STATUS if needed. For Pass 8
		// initial drop, leave at zero; the UI re-fetches via Select.
		out = append(out, f)
	}
	return out, nil
}

// roleFromAttrs maps RFC 6154 LIST attributes to mail.Folder.Role
// values used by mail.Classify ("inbox", "drafts", "sent", "trash",
// "junk", "archive"). Unknown attributes return "".
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
		case "important", "flagged":
			// Not currently surfaced as a canonical role.
		}
	}
	return ""
}

// OpenFolder satisfies mail.Backend. Selects the folder on the
// command connection and signals the idle goroutine to re-IDLE on
// the new folder.
func (b *Backend) OpenFolder(name string) error {
	b.mu.Lock()
	cmd := b.cmd
	switchCh := b.switchCh
	b.mu.Unlock()

	if _, err := cmd.Select(name, false); err != nil {
		return fmt.Errorf("select %q: %w", name, err)
	}

	b.mu.Lock()
	b.current = name
	b.mu.Unlock()

	if switchCh != nil {
		// Non-blocking — drop earlier pending switches.
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
