// Package compose is poplar's outbound-mail surface. It defines the
// Editor seam, the Draft value type, MIME assembly, and the
// reply/forward seeders.
//
// The package is UI-free. compose.Model (internal/ui/compose/) wires Editor into
// the tea tree. The cache outbox carries the assembled bytes. The
// mail backends ship them.
package compose

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/glw907/poplar/internal/catkin"
)

// Editor is the swappable text-entry surface for compose.Model. v1
// always uses CatkinEditor. The v1.1 neovim adapter will implement
// the same surface (ADR-0033). Update returns Editor so callers
// chain without a type assertion.
type Editor interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Editor, tea.Cmd)
	View() string

	SetSize(w, h int)
	SetWidth(w int)

	Focus() tea.Cmd
	Blur()
	Focused() bool

	Value() string
	SetValue(s string)

	SetStyles(s catkin.Styles)
	SetTidyHighlights(src string, ranges []catkin.Range)
	RegisterAnnotator(a catkin.Annotator)

	WordCount() int
	CharCount() int
}

// CatkinEditor adapts catkin.Model to the Editor interface. Update
// reassigns the inner model in place so the adapter stays a stable
// pointer for compose.Model to hold.
type CatkinEditor struct {
	inner catkin.Model
}

func NewCatkinEditor() *CatkinEditor {
	return &CatkinEditor{inner: catkin.New()}
}

func (e *CatkinEditor) Init() tea.Cmd { return e.inner.Init() }

func (e *CatkinEditor) Update(msg tea.Msg) (Editor, tea.Cmd) {
	next, cmd := e.inner.Update(msg)
	e.inner = next
	return e, cmd
}

func (e *CatkinEditor) View() string { return e.inner.View() }

func (e *CatkinEditor) SetSize(w, h int) { e.inner.SetSize(w, h) }
func (e *CatkinEditor) SetWidth(w int)   { e.inner.SetWidth(w) }

func (e *CatkinEditor) Focus() tea.Cmd { return e.inner.Focus() }
func (e *CatkinEditor) Blur()          { e.inner.Blur() }
func (e *CatkinEditor) Focused() bool  { return e.inner.Focused() }

func (e *CatkinEditor) Value() string     { return e.inner.Value() }
func (e *CatkinEditor) SetValue(s string) { e.inner.SetValue(s) }

func (e *CatkinEditor) SetStyles(s catkin.Styles) { e.inner.SetStyles(s) }

func (e *CatkinEditor) SetTidyHighlights(src string, ranges []catkin.Range) {
	e.inner.SetTidyHighlights(src, ranges)
}

func (e *CatkinEditor) RegisterAnnotator(a catkin.Annotator) {
	e.inner.RegisterAnnotator(a)
}

func (e *CatkinEditor) WordCount() int { return e.inner.WordCount() }
func (e *CatkinEditor) CharCount() int { return e.inner.CharCount() }
