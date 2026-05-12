package config

import (
	"fmt"
	"slices"
	"strconv"
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

// Render emits canonical TOML for the given accounts, UI, and cache
// configuration. Round-trips through Load* are semantic, not
// byte-for-byte. Comments are not preserved and default-valued
// fields are elided.
func Render(accts []AccountConfig, ui UIConfig, cache CacheConfig) []byte {
	var b strings.Builder
	for i, acct := range accts {
		if i > 0 {
			b.WriteString("\n")
		}
		writeAccount(&b, acct)
	}
	if uiBlock := renderUIBlock(ui); uiBlock != "" {
		b.WriteString("\n")
		b.WriteString(uiBlock)
	}
	if cacheBlock := renderCacheBlock(cache); cacheBlock != "" {
		b.WriteString("\n")
		b.WriteString(cacheBlock)
	}
	return []byte(b.String())
}

func writeAccount(b *strings.Builder, a AccountConfig) {
	b.WriteString("[[account]]\n")
	writeKV(b, "name", a.Name)
	switch {
	case a.Preset != "":
		writeKV(b, "provider", a.Preset)
	case a.Backend != "":
		writeKV(b, "provider", a.Backend)
	}
	writeKV(b, "email", a.Email)
	if a.Source != "" {
		writeKV(b, "source", a.Source)
	}
	if a.Host != "" {
		writeKV(b, "host", a.Host)
	}
	if a.Port != 0 {
		writeKVInt(b, "port", a.Port)
	}
	if a.StartTLS {
		writeKVBool(b, "starttls", true)
	}
	if a.InsecureTLS {
		writeKVBool(b, "insecure-tls", true)
	}
	if a.Auth != "" {
		writeKV(b, "auth", a.Auth)
	}
	switch {
	case a.PasswordCmd != "":
		writeKV(b, "password-cmd", a.PasswordCmd)
	case a.Password != "":
		writeKV(b, "password", a.Password)
	}
	if a.OAuthStore != "" {
		writeKV(b, "oauth-store", a.OAuthStore)
	}

	// [account.oauth] and [account.smtp] precede [[account.identity]]
	// blocks. A bare [section] header after array-of-tables entries
	// would be parsed as a sub-table of the last array element instead
	// of the parent.
	if a.OAuth != nil {
		b.WriteString("\n[account.oauth]\n")
		writeKV(b, "client-id", a.OAuth.ClientID)
		writeKV(b, "client-secret", a.OAuth.ClientSecret)
		// Elide endpoint URLs and scopes when they match the preset
		// defaults so round-trips stay clean.
		presetOAuth := oauthPresetsFor(a.Preset)
		if a.OAuth.AuthURL != "" && (presetOAuth == nil || a.OAuth.AuthURL != presetOAuth.AuthURL) {
			writeKV(b, "auth-url", a.OAuth.AuthURL)
		}
		if a.OAuth.TokenURL != "" && (presetOAuth == nil || a.OAuth.TokenURL != presetOAuth.TokenURL) {
			writeKV(b, "token-url", a.OAuth.TokenURL)
		}
		if len(a.OAuth.Scopes) > 0 && (presetOAuth == nil || !slices.Equal(a.OAuth.Scopes, presetOAuth.Scopes)) {
			writeKVStringSlice(b, "scopes", a.OAuth.Scopes)
		}
	}

	if a.SMTP != (SMTPConfig{}) {
		b.WriteString("\n[account.smtp]\n")
		s := a.SMTP
		if s.Host != "" {
			writeKV(b, "host", s.Host)
		}
		if s.Port != 0 {
			writeKVInt(b, "port", s.Port)
		}
		if s.StartTLS {
			writeKVBool(b, "starttls", true)
		}
		if s.InsecureTLS {
			writeKVBool(b, "insecure-tls", true)
		}
		if s.Auth != "" {
			writeKV(b, "auth", s.Auth)
		}
		switch {
		case s.PasswordCmd != "":
			writeKV(b, "password-cmd", s.PasswordCmd)
		case s.Password != "":
			writeKV(b, "password", s.Password)
		}
	}

	for _, id := range a.Identities {
		b.WriteString("\n[[account.identity]]\n")
		writeKV(b, "name", id.Name)
		writeKV(b, "email", id.Email)
		for _, sig := range id.Signatures {
			b.WriteString("\n[[account.identity.signature]]\n")
			writeKV(b, "name", sig.Name)
			if sig.Text != "" {
				fmt.Fprintf(b, "text = %s\n", multilineQuoted(sig.Text))
			}
		}
	}
}

func renderUIBlock(ui UIConfig) string {
	d := DefaultUIConfig()
	type entry struct{ key, val string }
	var rows []entry
	if ui.Threading != d.Threading {
		rows = append(rows, entry{"threading", boolStr(ui.Threading)})
	}
	if !ui.NewMailToast {
		rows = append(rows, entry{"new-mail-toast", "false"})
	}
	if ui.Icons != "" && ui.Icons != d.Icons {
		rows = append(rows, entry{"icons", quoted(ui.Icons)})
	}
	if ui.UndoSeconds != 0 && ui.UndoSeconds != d.UndoSeconds {
		rows = append(rows, entry{"undo_seconds", strconv.Itoa(ui.UndoSeconds)})
	}
	if ui.UndoSendWindow != 0 && ui.UndoSendWindow != d.UndoSendWindow {
		rows = append(rows, entry{"undo-send-window", quoted(ui.UndoSendWindow.String())})
	}
	if ui.TrashRetentionDays != 0 {
		rows = append(rows, entry{"trash_retention_days", strconv.Itoa(ui.TrashRetentionDays)})
	}
	if ui.SpamRetentionDays != 0 {
		rows = append(rows, entry{"spam_retention_days", strconv.Itoa(ui.SpamRetentionDays)})
	}
	if ui.DownloadDir != "" && ui.DownloadDir != d.DownloadDir {
		rows = append(rows, entry{"download_dir", quoted(ui.DownloadDir)})
	}
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("[ui]\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "%s = %s\n", r.key, r.val)
	}
	return b.String()
}

func renderCacheBlock(c CacheConfig) string {
	d := defaultCache()
	var rows []string
	if c.MaxSize != d.MaxSize {
		rows = append(rows, fmt.Sprintf("max-size = %q", formatSize(c.MaxSize)))
	}
	if c.MaxAttachmentSize != d.MaxAttachmentSize {
		rows = append(rows, fmt.Sprintf("max-attachment-size = %q", formatSize(c.MaxAttachmentSize)))
	}
	if len(rows) == 0 {
		return ""
	}
	return "[cache]\n" + strings.Join(rows, "\n") + "\n"
}

// formatSize emits a 1024-based human-readable size mirroring
// parseSize's accepted suffixes. Round trip is exact for power-of-two
// sizes. Non-aligned values fall through as raw bytes.
func formatSize(n int64) string {
	const (
		kb = int64(1024)
		mb = kb * 1024
		gb = mb * 1024
		tb = gb * 1024
	)
	switch {
	case n == 0:
		return "0"
	case n%tb == 0:
		return fmt.Sprintf("%dTB", n/tb)
	case n%gb == 0:
		return fmt.Sprintf("%dGB", n/gb)
	case n%mb == 0:
		return fmt.Sprintf("%dMB", n/mb)
	case n%kb == 0:
		return fmt.Sprintf("%dKB", n/kb)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func oauthPresetsFor(preset string) *OAuthDefaults {
	if p, ok := Providers[preset]; ok {
		return p.OAuth
	}
	return nil
}

func writeKVStringSlice(b *strings.Builder, k string, vals []string) {
	b.WriteString(k)
	b.WriteString(" = [")
	for i, v := range vals {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(quoted(v))
	}
	b.WriteString("]\n")
}

func writeKV(b *strings.Builder, k, v string) {
	if v == "" {
		return
	}
	fmt.Fprintf(b, "%s = %s\n", k, quoted(v))
}

func writeKVInt(b *strings.Builder, k string, v int) {
	fmt.Fprintf(b, "%s = %d\n", k, v)
}

func writeKVBool(b *strings.Builder, k string, v bool) {
	fmt.Fprintf(b, "%s = %s\n", k, boolStr(v))
}

func quoted(s string) string {
	return `"` + tomlEscape(s) + `"`
}

// multilineQuoted renders s as a TOML basic multi-line literal when it
// contains a newline, otherwise as a single-line basic string. Used
// for signature bodies so rendered config reads naturally.
func multilineQuoted(s string) string {
	if !strings.Contains(s, "\n") {
		return quoted(s)
	}
	body := strings.ReplaceAll(s, `"""`, `\"\"\"`)
	return "\"\"\"\n" + body + "\"\"\""
}

func tomlEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`)
	return r.Replace(s)
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
