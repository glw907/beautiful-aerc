package store

import (
	"strings"
	"testing"
)

// TestRoleTable proves classifyMailboxRole's name-heuristic fallback
// against roleByName's committed set, including the non-English and
// punctuated names FO-1 requires fixture coverage for.
func TestRoleTable(t *testing.T) {
	tests := []struct {
		name string
		want mailboxRole
	}{
		{"Archive", roleArchive},
		{"All Mail", roleArchive},
		{"[Gmail]/All Mail", roleArchive},
		{"Archiv", roleArchive},
		{"Drafts", roleDrafts},
		{"[Gmail]/Drafts", roleDrafts},
		{"Brouillons", roleDrafts},
		{"Borradores", roleDrafts},
		{"Sent", roleSent},
		{"Sent Items", roleSent},
		{"[Gmail]/Sent Mail", roleSent},
		{"Gesendet", roleSent},
		{"Enviados", roleSent},
		{"Junk", roleJunk},
		{"Junk E-mail", roleJunk},
		{"Spam", roleJunk},
		{"[Gmail]/Spam", roleJunk},
		{"Posta Indesiderata", roleJunk},
		{"Correo No Deseado", roleJunk},
		{"Trash", roleTrash},
		{"Deleted Items", roleTrash},
		{"Bin", roleTrash},
		{"[Gmail]/Trash", roleTrash},
		{"Papierkorb", roleTrash},
		{"Corbeille", roleTrash},
		{"Papelera", roleTrash},
		{"Cestino", roleTrash},
		{"Scheduled", roleScheduled},
		{"Send Later", roleScheduled},
		{"Team Updates", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyMailboxRole("", tt.name); got != tt.want {
				t.Errorf("classifyMailboxRole(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// TestServerRoleWins proves a server-declared role is never overridden
// by the name heuristic, even when the mailbox's own name would
// classify differently under roleByName.
func TestServerRoleWins(t *testing.T) {
	if got := classifyMailboxRole("archive", "Trash"); got != roleArchive {
		t.Errorf("classifyMailboxRole(archive, Trash) = %q, want %q", got, roleArchive)
	}
	if got := classifyMailboxRole("inbox", "Team Updates"); got != "inbox" {
		t.Errorf("classifyMailboxRole(inbox, Team Updates) = %q, want %q", got, "inbox")
	}
}

// TestDuplicateRoleResolution proves that when two mailboxes classify
// to the same role, resolveMailboxRoles keeps the first-created one
// (the lower ID), drops the role from the rest without an error, and
// logs the collision.
func TestDuplicateRoleResolution(t *testing.T) {
	log := captureSlog(t)

	candidates := []mailboxRoleCandidate{
		{ID: 1, Name: "Trash"},
		{ID: 2, Name: "Papierkorb"},
	}
	resolved := resolveMailboxRoles(candidates)

	if got := resolved[1]; got != roleTrash {
		t.Errorf("resolved[1] = %q, want %q", got, roleTrash)
	}
	if got, ok := resolved[2]; ok {
		t.Errorf("resolved[2] = %q, want no role (duplicate dropped)", got)
	}
	if !strings.Contains(log.String(), "duplicate mailbox role") {
		t.Errorf("no log line for the role collision, got: %q", log.String())
	}
}
