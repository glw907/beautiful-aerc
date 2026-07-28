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
// role of its own. Every key is lowercase and diacritic-folded;
// classifyMailboxRole lowers and folds a candidate name before the
// lookup. The set covers a major client's default English name for
// each role plus the non-English and punctuated variants poplar's
// fixture accounts carry, Gmail's bracketed IMAP names among them.
// Every entry stays plain ASCII, the styling analyzer's boundary for
// every package outside internal/theme and internal/catkin.
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
	"entwurfe":       roleDrafts, // German, diacritic-folded
	"entwuerfe":      roleDrafts, // German, "ue" transliteration

	"sent":              roleSent,
	"sent items":        roleSent,
	"sent messages":     roleSent,
	"[gmail]/sent mail": roleSent,
	"gesendet":          roleSent, // German
	"enviados":          roleSent, // Spanish
	"envoyes":           roleSent, // French, diacritic-folded

	"junk":               roleJunk,
	"junk e-mail":        roleJunk,
	"spam":               roleJunk,
	"[gmail]/spam":       roleJunk,
	"posta indesiderata": roleJunk, // Italian
	"correo no deseado":  roleJunk, // Spanish

	"trash":               roleTrash,
	"deleted items":       roleTrash,
	"deleted messages":    roleTrash,
	"bin":                 roleTrash,
	"[gmail]/trash":       roleTrash,
	"papierkorb":          roleTrash, // German
	"corbeille":           roleTrash, // French
	"papelera":            roleTrash, // Spanish
	"cestino":             roleTrash, // Italian
	"elements supprimes":  roleTrash, // French, diacritic-folded
	"geloschte elemente":  roleTrash, // German, diacritic-folded
	"geloeschte elemente": roleTrash, // German, "oe" transliteration

	"scheduled":       roleScheduled,
	"scheduled sends": roleScheduled,
	"send later":      roleScheduled,
}

// serverRoleToMailboxRole normalizes a backend-declared role string
// into FO-1's closed six-role set. It covers the JMAP (RFC 8621) and
// IMAP special-use (RFC 6154) vocabulary that maps onto one of the
// six: "all" is Gmail's All Mail concept, the same as the name
// heuristic's "all mail". A declared role outside this set (inbox,
// flagged, important, subscribed, snoozed, or anything else) returns
// the empty mailboxRole: it is a real role, just not one of FO-1's
// six that behave by role everywhere.
var serverRoleToMailboxRole = map[string]mailboxRole{
	"archive":   roleArchive,
	"all":       roleArchive,
	"drafts":    roleDrafts,
	"sent":      roleSent,
	"junk":      roleJunk,
	"trash":     roleTrash,
	"scheduled": roleScheduled,
}

// classifyMailboxRole returns name's role. serverRole, normalized
// through serverRoleToMailboxRole, wins whenever the backend declared
// one; the name-heuristic table only runs when serverRole is empty,
// so a server's own classification is never second-guessed by the
// heuristic.
func classifyMailboxRole(serverRole, name string) mailboxRole {
	if serverRole != "" {
		return serverRoleToMailboxRole[serverRole]
	}
	return roleByName[foldDiacritics(strings.ToLower(strings.TrimSpace(name)))]
}

// diacriticFold maps a lowercase Latin-1 accented code point to its
// unaccented ASCII base letter, or letters for a multigraph like the
// German sharp s. Keys are the code point value rather than a rune
// literal, since the styling analyzer's boundary forbids a non-ASCII
// literal outside internal/theme and internal/catkin.
var diacriticFold = map[rune]string{
	0x00e0: "a",  // a-grave
	0x00e1: "a",  // a-acute
	0x00e2: "a",  // a-circumflex
	0x00e3: "a",  // a-tilde
	0x00e4: "a",  // a-diaeresis
	0x00e5: "a",  // a-ring
	0x00e7: "c",  // c-cedilla
	0x00e8: "e",  // e-grave
	0x00e9: "e",  // e-acute
	0x00ea: "e",  // e-circumflex
	0x00eb: "e",  // e-diaeresis
	0x00ec: "i",  // i-grave
	0x00ed: "i",  // i-acute
	0x00ee: "i",  // i-circumflex
	0x00ef: "i",  // i-diaeresis
	0x00f1: "n",  // n-tilde
	0x00f2: "o",  // o-grave
	0x00f3: "o",  // o-acute
	0x00f4: "o",  // o-circumflex
	0x00f5: "o",  // o-tilde
	0x00f6: "o",  // o-diaeresis
	0x00f8: "o",  // o-stroke
	0x00f9: "u",  // u-grave
	0x00fa: "u",  // u-acute
	0x00fb: "u",  // u-circumflex
	0x00fc: "u",  // u-diaeresis
	0x00fd: "y",  // y-acute
	0x00ff: "y",  // y-diaeresis
	0x00df: "ss", // sharp s
}

// foldDiacritics replaces every accented rune in s with its unaccented
// ASCII base letter, using diacriticFold, and leaves every other rune
// untouched.
func foldDiacritics(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if folded, ok := diacriticFold[r]; ok {
			b.WriteString(folded)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// mailboxRoleCandidate is one mailbox's role inputs to
// resolveMailboxRoles. ID is the mailbox's poplar-minted primary key,
// which resolveMailboxRoles also uses as its creation order.
// AccountID scopes duplicate-role resolution to one account, since
// mailbox.account_id is the schema's own account key and two
// accounts' mailboxes must never contend for the same role.
type mailboxRoleCandidate struct {
	ID         int64
	AccountID  int64
	ServerRole string
	Name       string
}

// resolveMailboxRoles classifies every candidate's role and resolves
// any role two or more candidates in the same account claim: a
// candidate with a server-declared role wins over one classified by
// the name heuristic, and among candidates tied on that, the lowest
// ID (the mailbox poplar created first) wins. The rest drop the role
// (their ID is simply absent from the result). FO-1 requires this
// degrade rather than a sync refusal: a server that (wrongly)
// declares the same role on two mailboxes must not be able to stop
// sync. Each dropped duplicate logs one line naming the kept and
// dropped mailboxes and the contested role.
func resolveMailboxRoles(candidates []mailboxRoleCandidate) map[int64]mailboxRole {
	type accountRole struct {
		accountID int64
		role      mailboxRole
	}
	byRole := make(map[accountRole][]mailboxRoleCandidate, len(candidates))
	for _, c := range candidates {
		if role := classifyMailboxRole(c.ServerRole, c.Name); role != "" {
			key := accountRole{c.AccountID, role}
			byRole[key] = append(byRole[key], c)
		}
	}

	resolved := make(map[int64]mailboxRole, len(byRole))
	for key, group := range byRole {
		winner := slices.MinFunc(group, func(a, b mailboxRoleCandidate) int {
			return cmp.Or(
				cmp.Compare(boolRank(a.ServerRole != ""), boolRank(b.ServerRole != "")),
				cmp.Compare(a.ID, b.ID),
			)
		})
		resolved[winner.ID] = key.role
		for _, c := range group {
			if c.ID != winner.ID {
				slog.Warn("store: duplicate mailbox role", "role", string(key.role), "kept", winner.Name, "dropped", c.Name)
			}
		}
	}
	return resolved
}

// boolRank orders true before false for a slices.MinFunc comparison.
func boolRank(b bool) int {
	if b {
		return 0
	}
	return 1
}
