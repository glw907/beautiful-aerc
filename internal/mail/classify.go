// SPDX-License-Identifier: MIT

package mail

import "strings"

// Group is the visual grouping a classified folder falls into.
type Group int

const (
	GroupPrimary  Group = iota // Inbox, Drafts, Sent, Archive
	GroupDisposal              // Spam, Trash
	GroupCustom                // everything else
)

// ClassifiedFolder wraps a backend Folder with its canonical identity
// (when recognized), display name, and visual group.
type ClassifiedFolder struct {
	Folder      Folder
	Canonical   string // "Inbox", "Drafts", ... or "" for custom
	DisplayName string // canonical name, or provider name for custom
	Group       Group
}

// ConfigKey returns the lookup key used by UIConfig.Folders and the
// TOML writer: canonical name for classified canonicals, provider
// name for custom folders. Callers should prefer this over recomputing
// the rule — it's the single source of truth.
func (cf ClassifiedFolder) ConfigKey() string {
	if cf.Canonical != "" {
		return cf.Canonical
	}
	return cf.Folder.Name
}

// Classify maps raw backend folders into ClassifiedFolders.
// Priority: role attribute, then alias table, then Custom fallback.
// Matching is case-insensitive exact match on the provider name.
// Order of the input is preserved in the output — callers that want
// group ordering (sidebar) do that themselves.
func Classify(folders []Folder) []ClassifiedFolder {
	if len(folders) == 0 {
		return nil
	}
	out := make([]ClassifiedFolder, 0, len(folders))
	for _, f := range folders {
		out = append(out, classifyOne(f))
	}
	return out
}

func classifyOne(f Folder) ClassifiedFolder {
	if canonical := canonicalFromRole(f.Role); canonical != "" {
		return ClassifiedFolder{
			Folder:      f,
			Canonical:   canonical,
			DisplayName: canonical,
			Group:       groupOf(canonical),
		}
	}
	if canonical := canonicalFromAlias(f.Name); canonical != "" {
		return ClassifiedFolder{
			Folder:      f,
			Canonical:   canonical,
			DisplayName: canonical,
			Group:       groupOf(canonical),
		}
	}
	return ClassifiedFolder{
		Folder:      f,
		Canonical:   "",
		DisplayName: f.Name,
		Group:       GroupCustom,
	}
}

func canonicalFromRole(role string) string {
	switch strings.ToLower(role) {
	case "inbox":
		return "Inbox"
	case "drafts":
		return "Drafts"
	case "sent":
		return "Sent"
	case "archive":
		return "Archive"
	case "junk":
		return "Spam"
	case "trash":
		return "Trash"
	}
	return ""
}

// aliasTable maps lowercased provider names to canonical names.
// Verified against Gmail, Fastmail, Outlook/M365, iCloud, Yahoo/AOL,
// Proton Mail Bridge.
var aliasTable = map[string]string{
	"inbox": "Inbox",

	"drafts":         "Drafts",
	"draft":          "Drafts",
	"[gmail]/drafts": "Drafts",

	"sent":              "Sent",
	"sent mail":         "Sent",
	"sent items":        "Sent",
	"sent messages":     "Sent",
	"[gmail]/sent mail": "Sent",

	"archive":          "Archive",
	"all mail":         "Archive",
	"[gmail]/all mail": "Archive",

	"spam":         "Spam",
	"junk":         "Spam",
	"junk email":   "Spam",
	"junk e-mail":  "Spam",
	"bulk mail":    "Spam",
	"[gmail]/spam": "Spam",

	"trash":            "Trash",
	"deleted":          "Trash",
	"deleted items":    "Trash",
	"deleted messages": "Trash",
	"bin":              "Trash",
	"[gmail]/trash":    "Trash",
}

func canonicalFromAlias(name string) string {
	return aliasTable[strings.ToLower(name)]
}

func groupOf(canonical string) Group {
	switch canonical {
	case "Inbox", "Drafts", "Sent", "Archive":
		return GroupPrimary
	case "Spam", "Trash":
		return GroupDisposal
	}
	return GroupCustom
}
