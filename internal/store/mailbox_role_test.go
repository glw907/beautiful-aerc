package store

import (
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/uerr/uerrtest"
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
		{"Entwurfe", roleDrafts},
		{"Entwuerfe", roleDrafts},
		{"Entwürfe", roleDrafts},
		{"Sent", roleSent},
		{"Sent Items", roleSent},
		{"[Gmail]/Sent Mail", roleSent},
		{"Gesendet", roleSent},
		{"Enviados", roleSent},
		{"Envoyes", roleSent},
		{"Envoyés", roleSent},
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
		{"Elements supprimes", roleTrash},
		{"Éléments supprimés", roleTrash},
		{"Geloeschte Elemente", roleTrash},
		{"Gelöschte Elemente", roleTrash},
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

// TestServerRoleWins proves a server-declared role, once normalized
// into FO-1's closed six-role set, is never overridden by the name
// heuristic, that the JMAP/Gmail "all" alias normalizes to archive
// the same as the name heuristic's "all mail" does, and that a
// declared role outside the six (inbox, guaranteed by protocol but
// not one of FO-1's special-use roles) classifies to no role rather
// than passing through verbatim.
func TestServerRoleWins(t *testing.T) {
	if got := classifyMailboxRole("archive", "Trash"); got != roleArchive {
		t.Errorf("classifyMailboxRole(archive, Trash) = %q, want %q", got, roleArchive)
	}
	if got := classifyMailboxRole("all", "Team Updates"); got != roleArchive {
		t.Errorf("classifyMailboxRole(all, Team Updates) = %q, want %q", got, roleArchive)
	}
	if got := classifyMailboxRole("inbox", "Team Updates"); got != "" {
		t.Errorf("classifyMailboxRole(inbox, Team Updates) = %q, want no role", got)
	}
}

// TestDuplicateRoleResolution proves that when two mailboxes in the
// same account classify to the same role, resolveMailboxRoles keeps
// the first-created one (the lower ID), drops the role from the rest
// without an error, and logs the collision.
func TestDuplicateRoleResolution(t *testing.T) {
	log := uerrtest.CaptureDefault(t)

	candidates := []mailboxRoleCandidate{
		{ID: 1, AccountID: 1, Name: "Trash"},
		{ID: 2, AccountID: 1, Name: "Papierkorb"},
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

// TestDuplicateRoleResolutionPrefersDeclared proves that when one
// candidate carries a server-declared role and another only matches
// the name heuristic, the declared one wins even when its ID is
// higher, since a server's own classification must never lose to a
// guess about a display name.
func TestDuplicateRoleResolutionPrefersDeclared(t *testing.T) {
	uerrtest.CaptureDefault(t)

	candidates := []mailboxRoleCandidate{
		{ID: 1, AccountID: 1, Name: "Trash"},
		{ID: 2, AccountID: 1, ServerRole: "trash", Name: "Team Updates"},
	}
	resolved := resolveMailboxRoles(candidates)

	if got := resolved[2]; got != roleTrash {
		t.Errorf("resolved[2] = %q, want %q (server-declared wins over the name heuristic)", got, roleTrash)
	}
	if _, ok := resolved[1]; ok {
		t.Errorf("resolved[1] present, want the heuristic-only candidate dropped")
	}
}

// TestDuplicateRoleResolutionScopedToAccount proves that two accounts
// each claiming the trash role keep their own mailbox, since role
// collisions resolve per account, not globally across the store.
func TestDuplicateRoleResolutionScopedToAccount(t *testing.T) {
	uerrtest.CaptureDefault(t)

	candidates := []mailboxRoleCandidate{
		{ID: 1, AccountID: 1, Name: "Trash"},
		{ID: 2, AccountID: 2, Name: "Trash"},
	}
	resolved := resolveMailboxRoles(candidates)

	if got := resolved[1]; got != roleTrash {
		t.Errorf("resolved[1] = %q, want %q", got, roleTrash)
	}
	if got := resolved[2]; got != roleTrash {
		t.Errorf("resolved[2] = %q, want %q (a different account's own trash mailbox)", got, roleTrash)
	}
}
