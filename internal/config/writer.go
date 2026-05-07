package config

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/glw907/poplar/internal/mail"
)

// RenderFolderSubsections renders [ui.folders.<name>] stubs with
// commented hints for each classified folder not already in existing.
// Output is grouped Primary, Disposal, Custom and returns "" when
// nothing needs writing. existing keys match Sidebar/UIConfig.Folders
// lookup (canonical for classified canonicals, provider name for
// custom).
func RenderFolderSubsections(classified []mail.ClassifiedFolder, existing map[string]bool) string {
	primary, disposal, custom := splitByGroup(classified, existing)

	var parts []string
	if block := renderGroup(primary); block != "" {
		parts = append(parts, block)
	}
	if block := renderGroup(disposal); block != "" {
		parts = append(parts, block)
	}
	if block := renderGroup(custom); block != "" {
		parts = append(parts, block)
	}
	return strings.Join(parts, "\n")
}

func splitByGroup(classified []mail.ClassifiedFolder, existing map[string]bool) (primary, disposal, custom []mail.ClassifiedFolder) {
	for _, cf := range classified {
		if existing[cf.ConfigKey()] {
			continue
		}
		switch cf.Group {
		case mail.GroupPrimary:
			primary = append(primary, cf)
		case mail.GroupDisposal:
			disposal = append(disposal, cf)
		default:
			custom = append(custom, cf)
		}
	}
	return
}

func renderGroup(folders []mail.ClassifiedFolder) string {
	if len(folders) == 0 {
		return ""
	}
	var b strings.Builder
	for _, cf := range folders {
		b.WriteString(renderSubsection(cf))
	}
	b.WriteString("\n")
	return b.String()
}

func renderSubsection(cf mail.ClassifiedFolder) string {
	var b strings.Builder
	b.WriteString("[ui.folders.")
	b.WriteString(subsectionHeaderKey(cf))
	b.WriteString("]\n")
	b.WriteString("# label = \"\"\n")
	b.WriteString("# rank = 0\n")
	b.WriteString("# threading = true\n")
	b.WriteString("# sort = \"date-desc\"\n")
	b.WriteString("# hide = false\n")
	return b.String()
}

// subsectionHeaderKey prefers a bare identifier and falls back to a
// quoted string.
func subsectionHeaderKey(cf mail.ClassifiedFolder) string {
	if cf.Canonical != "" {
		return cf.Canonical
	}
	if isBareKey(cf.Folder.Name) {
		return cf.Folder.Name
	}
	return `"` + cf.Folder.Name + `"`
}

func isBareKey(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z',
			r >= 'a' && r <= 'z',
			r >= '0' && r <= '9',
			r == '-', r == '_':
			continue
		default:
			return false
		}
	}
	return true
}

// FolderKeys returns the set of subsection keys present under
// [ui.folders.<name>] in data.
func FolderKeys(data []byte) (map[string]bool, error) {
	var raw rawUIFile
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	out := make(map[string]bool, len(raw.UI.Folders))
	for k := range raw.UI.Folders {
		out[k] = true
	}
	return out, nil
}

// MergeFolderSubsections appends newContent to existing config bytes,
// preserving the original byte-for-byte. Empty newContent passes
// through unchanged.
func MergeFolderSubsections(existing []byte, newContent string) string {
	if newContent == "" {
		return string(existing)
	}
	return strings.TrimRight(string(existing), "\n") + "\n\n" + newContent
}
