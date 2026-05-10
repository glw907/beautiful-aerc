package wizard

import (
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)

// AdvanceMsg pushes the wizard to its next section.
type AdvanceMsg struct{}

// BackMsg pops the wizard back into the prior section's last form.
// Used by [e]dit on the probe-failure screen to return to credentials.
type BackMsg struct{}

// ThemeChangedMsg propagates a live-preview theme swap from the theme
// section to the parent wizard model. The parent rebuilds Styles +
// huh.Theme on receipt.
type ThemeChangedMsg struct {
	Theme *theme.CompiledTheme
	Name  string
}

// ProbeDoneMsg signals the probe completed with the given result.
type ProbeDoneMsg struct {
	Result mail.ProbeResult
}

// CancelMsg aborts the wizard. The confirm screen emits it on Cancel
// and the probe-failure screen emits it on [s].
type CancelMsg struct{}

// FinishMsg is emitted by the confirm section after a successful
// write; the parent tears down the wizard on receipt.
type FinishMsg struct {
	Path string
}
