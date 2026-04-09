package theme

import (
	"fmt"
	"os"
	"strings"
	"text/template"
)

const stylesetTemplate = `#
# aerc styleset -- auto-generated from {{.Name}} theme
# Re-generate with: mailrender themes generate
#

*.default=true
*.normal=true

title.fg={{.C "bg_base"}}
title.bg={{.C "accent_primary"}}
title.bold=true
header.bold=true

*error.bold=true
error.fg={{.C "color_error"}}
warning.fg={{.C "color_warning"}}
success.fg={{.C "color_success"}}

statusline*.default=true
statusline_default.fg={{.C "bg_border"}}
statusline_default.reverse=true
statusline_error.fg={{.C "color_error"}}
statusline_error.reverse=true
statusline_success.fg={{.C "color_success"}}
statusline_success.reverse=true

completion_default.fg={{.C "fg_base"}}
completion_default.bg={{.C "bg_elevated"}}
completion_gutter.bg={{.C "bg_elevated"}}
completion_pill.fg={{.C "bg_base"}}
completion_pill.bg={{.C "accent_primary"}}
completion_description.fg={{.C "fg_dim"}}
completion_description.dim=true

border.fg={{.C "bg_border"}}

spinner.fg={{.C "accent_primary"}}

stack.fg={{.C "fg_base"}}

selector_default.fg={{.C "fg_base"}}
selector_default.bg={{.C "bg_base"}}
selector_focused.fg={{.C "bg_base"}}
selector_focused.bg={{.C "accent_primary"}}
selector_focused.bold=true
selector_chooser.fg={{.C "fg_base"}}
selector_chooser.bold=true

*.selected.bg={{.C "bg_selection"}}

msglist_default.fg={{.C "fg_base"}}
msglist_read.fg={{.C "fg_dim"}}
msglist_unread.fg={{.C "accent_tertiary"}}
msglist_unread.bold=true
msglist_flagged.fg={{.C "color_warning"}}
msglist_flagged.bold=true
msglist_answered.fg={{.C "color_special"}}
msglist_forwarded.fg={{.C "color_special"}}
msglist_forwarded.dim=true
msglist_deleted.fg={{.C "fg_dim"}}
msglist_deleted.dim=true
msglist_marked.bg={{.C "accent_primary"}}
msglist_marked.fg={{.C "bg_base"}}
msglist_result.fg={{.C "color_info"}}
msglist_gutter.fg={{.C "bg_border"}}
msglist_pill.fg={{.C "bg_base"}}
msglist_pill.bg={{.C "accent_primary"}}
msglist_thread_folded.fg={{.C "color_warning"}}
msglist_thread_context.fg={{.C "fg_dim"}}
msglist_thread_context.dim=true
msglist_thread_orphan.fg={{.C "fg_dim"}}

tab.fg={{.C "fg_bright"}}
tab.bg={{.C "bg_border"}}
tab.selected.bg={{.C "accent_secondary"}}
tab.selected.fg={{.C "bg_base"}}

dirlist_default.fg={{.C "fg_base"}}
dirlist_unread.fg={{.C "accent_secondary"}}
dirlist_recent.fg={{.C "accent_secondary"}}

part_*.fg={{.C "fg_brightest"}}
part_mimetype.fg={{.C "bg_selection"}}
part_*.selected.fg={{.C "fg_brightest"}}
part_filename.selected.bold=true

[viewer]
*.default=true
*.normal=true

header.fg={{.C "accent_primary"}}
header.bold=true

quote_1.fg={{.C "accent_tertiary"}}
quote_1.dim=false
quote_2.fg={{.C "fg_dim"}}
quote_2.dim=false
quote_3.fg={{.C "fg_dim"}}
quote_3.dim=true
quote_4.fg={{.C "fg_dim"}}
quote_4.dim=true
quote_x.fg={{.C "fg_dim"}}
quote_x.dim=true

signature.fg={{.C "fg_base"}}
signature.dim=true

url.fg={{.C "accent_tertiary"}}
url.underline=true

diff_meta.bold=true
diff_chunk.dim=true
diff_add.fg={{.C "color_success"}}
diff_del.fg={{.C "color_error"}}
`

// stylesetData wraps a Theme for template execution.
type stylesetData struct {
	Name  string
	theme *Theme
}

// C returns the hex color for a slot name. Used in the template as {{.C "name"}}.
func (d stylesetData) C(name string) string {
	return d.theme.Color(name)
}

var stylesetTmpl = template.Must(template.New("styleset").Parse(stylesetTemplate))

// GenerateStyleset renders the styleset template with the theme's colors.
func GenerateStyleset(t *Theme) (string, error) {
	var buf strings.Builder
	data := stylesetData{Name: t.Name, theme: t}
	if err := stylesetTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing styleset template: %w", err)
	}
	return buf.String(), nil
}

// WriteStyleset generates and writes the styleset to a file.
func WriteStyleset(t *Theme, path string) error {
	content, err := GenerateStyleset(t)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing styleset %s: %w", path, err)
	}
	return nil
}
