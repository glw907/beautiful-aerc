package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/glw907/beautiful-aerc/internal/mail"
)

// RenderFolderSubsections renders `[ui.folders.<name>]` subsections
// with commented default hints for each classified folder not already
// present in the existing set. Output is grouped: Primary, then
// Disposal, then Custom, separated by blank lines. Returns "" when
// there is nothing to write.
//
// existing may be nil. Keys are canonical name for classified
// canonicals and provider name for classified custom folders —
// matching the same lookup keys Sidebar and UIConfig.Folders use.
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
		if existing[subsectionKey(cf)] {
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

// subsectionKey returns the lookup key for UIConfig.Folders and for
// detecting existing subsections — canonical name for canonicals,
// provider name for custom.
func subsectionKey(cf mail.ClassifiedFolder) string {
	if cf.Canonical != "" {
		return cf.Canonical
	}
	return cf.Folder.Name
}

// subsectionHeaderKey returns the TOML header key — bare identifier
// when possible, otherwise a quoted string.
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

// ExistingFolderKeys parses an accounts.toml file and returns the set
// of subsection keys already present under [ui.folders.<name>].
func ExistingFolderKeys(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
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

// MergeFolderSubsections appends new rendered subsections to the end
// of the config file at path and returns the merged file contents.
// Existing content is preserved byte-for-byte. If newContent is empty,
// the original contents are returned unchanged.
func MergeFolderSubsections(path, newContent string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading config: %w", err)
	}
	if newContent == "" {
		return string(data), nil
	}
	existing := strings.TrimRight(string(data), "\n")
	return existing + "\n\n" + newContent, nil
}
