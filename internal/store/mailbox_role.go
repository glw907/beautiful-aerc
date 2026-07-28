package store

import (
	"cmp"
	"log/slog"
	"slices"
	"strings"
)

// mailboxRole is one of FO-1's special-use roles: the folders that
// behave by role everywhere in poplar rather than by display name.
// The empty mailboxRole means no role.
type mailboxRole string

const (
	roleArchive   mailboxRole = "archive"
	roleDrafts    mailboxRole = "drafts"
	roleSent      mailboxRole = "sent"
	roleJunk      mailboxRole = "junk"
	roleTrash     mailboxRole = "trash"
	roleScheduled mailboxRole = "scheduled"
)

// roleByName is FO-1's committed name-heuristic fallback: the display
// names classifyMailboxRole recognizes when a backend declares no
// role of its own. Every key is lowercase; classifyMailboxRole lowers
// a candidate name before the lookup. The set covers a major client's
// default English name for each role plus the non-English and
// punctuated variants poplar's fixture accounts carry, Gmail's
// bracketed IMAP names among them. Every entry stays plain ASCII, the
// styling analyzer's boundary for every package outside internal/theme
// and internal/catkin; the non-English entries are the language's own
// unaccented spelling rather than one that needs a diacritic.
var roleByName = map[string]mailboxRole{
	"archive":          roleArchive,
	"archives":         roleArchive,
	"all mail":         roleArchive,
	"[gmail]/all mail": roleArchive,
	"archiv":           roleArchive, // German

	"drafts":         roleDrafts,
	"draft":          roleDrafts,
	"[gmail]/drafts": roleDrafts,
	"brouillons":     roleDrafts, // French
	"borradores":     roleDrafts, // Spanish

	"sent":              roleSent,
	"sent items":        roleSent,
	"sent messages":     roleSent,
	"[gmail]/sent mail": roleSent,
	"gesendet":          roleSent, // German
	"enviados":          roleSent, // Spanish

	"junk":               roleJunk,
	"junk e-mail":        roleJunk,
	"spam":               roleJunk,
	"[gmail]/spam":       roleJunk,
	"posta indesiderata": roleJunk, // Italian
	"correo no deseado":  roleJunk, // Spanish

	"trash":            roleTrash,
	"deleted items":    roleTrash,
	"deleted messages": roleTrash,
	"bin":              roleTrash,
	"[gmail]/trash":    roleTrash,
	"papierkorb":       roleTrash, // German
	"corbeille":        roleTrash, // French
	"papelera":         roleTrash, // Spanish
	"cestino":          roleTrash, // Italian

	"scheduled":       roleScheduled,
	"scheduled sends": roleScheduled,
	"send later":      roleScheduled,
}

// classifyMailboxRole returns name's role. serverRole wins verbatim
// whenever the backend declared one; the name-heuristic table only
// runs when serverRole is empty, so a server's own classification is
// never second-guessed.
func classifyMailboxRole(serverRole, name string) mailboxRole {
	if serverRole != "" {
		return mailboxRole(serverRole)
	}
	return roleByName[strings.ToLower(strings.TrimSpace(name))]
}

// mailboxRoleCandidate is one mailbox's role inputs to
// resolveMailboxRoles. ID is the mailbox's poplar-minted primary key,
// which resolveMailboxRoles also uses as its creation order.
type mailboxRoleCandidate struct {
	ID         int64
	ServerRole string
	Name       string
}

// resolveMailboxRoles classifies every candidate's role and resolves
// any role two or more candidates claim by keeping the one with the
// lowest ID, the mailbox poplar created first, and dropping the role
// from the rest (their ID is simply absent from the result). FO-1
// requires this degrade rather than a sync refusal: a server that
// (wrongly) declares the same role on two mailboxes must not be able
// to stop sync. Each dropped duplicate logs one line naming the kept
// and dropped mailboxes and the contested role.
func resolveMailboxRoles(candidates []mailboxRoleCandidate) map[int64]mailboxRole {
	byRole := make(map[mailboxRole][]mailboxRoleCandidate, len(candidates))
	for _, c := range candidates {
		if role := classifyMailboxRole(c.ServerRole, c.Name); role != "" {
			byRole[role] = append(byRole[role], c)
		}
	}

	resolved := make(map[int64]mailboxRole, len(byRole))
	for role, group := range byRole {
		winner := slices.MinFunc(group, func(a, b mailboxRoleCandidate) int {
			return cmp.Compare(a.ID, b.ID)
		})
		resolved[winner.ID] = role
		for _, c := range group {
			if c.ID != winner.ID {
				slog.Warn("store: duplicate mailbox role", "role", string(role), "kept", winner.Name, "dropped", c.Name)
			}
		}
	}
	return resolved
}
