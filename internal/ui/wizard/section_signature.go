package wizard

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/glw907/poplar/internal/catkin"
)

// signatureSection is the wizard's signature-entry step. It hosts a
// catkin.Model below an immutable "-- " chrome row and renders a
// description telling the user that markdown will be rendered as HTML
// on send. The sentinel is not in catkin's buffer; config's decoder
// adds it on the next load (ADR-0177).
type signatureSection struct {
	parent *Model
	editor catkin.Model
}

func newSignatureSection(parent *Model) *signatureSection {
	ed := catkin.New().WithSize(64, 8)
	if parent.State.Signature != "" {
		ed = ed.WithValue(parent.State.Signature)
	}
	return &signatureSection{parent: parent, editor: ed}
}

func (s *signatureSection) Init() tea.Cmd { return s.editor.Init() }

func (s *signatureSection) Update(msg tea.Msg) (*signatureSection, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case k.Mod&tea.ModCtrl != 0 && k.Code == 'x':
			s.parent.State.Signature = s.editor.Value()
			return s, func() tea.Msg { return AdvanceMsg{} }
		case k.Code == tea.KeyEsc:
			s.parent.State.Signature = ""
			return s, func() tea.Msg { return AdvanceMsg{} }
		case k.Mod&tea.ModCtrl != 0 && k.Code == 'p':
			return s, func() tea.Msg { return BackMsg{} }
		}
	}
	var cmd tea.Cmd
	s.editor, cmd = s.editor.Update(msg)
	return s, cmd
}

func (s *signatureSection) View() string {
	st := s.parent.Styles
	var b strings.Builder

	b.WriteString(st.Body.Render("Email signature — optional"))
	b.WriteString("\n\n")
	b.WriteString(st.Help.Render("Markdown is supported and will be rendered as HTML on send."))
	b.WriteString("\n")
	b.WriteString(st.Help.Render("Leave blank to skip."))
	b.WriteString("\n\n")

	// "-- " (two dashes, trailing space) is the RFC 3676 signature
	// boundary; ADR-0177 requires it on every saved signature. The
	// trailing space is load-bearing.
	b.WriteString(st.Help.Render("-- "))
	b.WriteString("\n")
	b.WriteString(s.editor.View())
	b.WriteString("\n\n")

	b.WriteString(st.Help.Render("Markdown  ^B bold · ^I italic · ^K link · ^L list · ^Q quote · ^Space task"))
	b.WriteString("\n")
	b.WriteString(st.Help.Render("Wizard    ^X save · Esc skip · ^P back"))

	return lipgloss.NewStyle().PaddingLeft(2).Render(b.String())
}
