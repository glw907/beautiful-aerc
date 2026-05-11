package compose

import (
	"testing"

	"github.com/glw907/poplar/internal/contacts"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/tidytext"
)

func newTestComposeModel() *Model {
	st := NewStyles(theme.OneDark)
	return New(theme.OneDark, st, "test@example.com", func(string) []contacts.Suggestion { return nil })
}

func TestHandleTidyKey_Disabled(t *testing.T) {
	m := newTestComposeModel()
	m.focus = focusBody
	m.tidyEnabled = false
	if cmd := m.handleTidyKey(); cmd != nil {
		t.Errorf("disabled: cmd != nil")
	}
}

func TestHandleTidyKey_NoAPIKey(t *testing.T) {
	m := newTestComposeModel()
	m.focus = focusBody
	m.tidyEnabled = true
	m.tidyAPIKey = ""
	if cmd := m.handleTidyKey(); cmd != nil {
		t.Errorf("no api key: returned a cmd")
	}
	if m.err == "" {
		t.Errorf("no api key: m.err empty, want explanation")
	}
}

func TestHandleTidyKey_NotBodyFocus(t *testing.T) {
	m := newTestComposeModel()
	m.focus = focusSubject
	m.tidyEnabled = true
	m.tidyAPIKey = "sk-test"
	if cmd := m.handleTidyKey(); cmd != nil {
		t.Errorf("not body focus: cmd != nil")
	}
}

func TestHandleTidyKey_InFlight(t *testing.T) {
	m := newTestComposeModel()
	m.focus = focusBody
	m.tidyEnabled = true
	m.tidyAPIKey = "sk-test"
	m.tidyInFlight = true
	if cmd := m.handleTidyKey(); cmd != nil {
		t.Errorf("in flight: cmd != nil (re-entry should be ignored)")
	}
}

func TestUpdate_TidyResult_Corrected(t *testing.T) {
	m := newTestComposeModel()
	m.editor.SetValue("teh quick brown")
	m.tidyInFlight = true
	msg := tidyResultMsg{
		oldBody: "teh quick brown",
		res: tidytext.Result{
			Status:  tidytext.StatusCorrected,
			Text:    "the quick brown",
			Message: "tidytext: 1 corrections applied",
		},
	}
	next, _ := m.Update(msg)
	if next.editor.Value() != "the quick brown" {
		t.Errorf("body not replaced: got %q", next.editor.Value())
	}
	if next.tidyInFlight {
		t.Errorf("inFlight not cleared")
	}
	if next.info == "" {
		t.Errorf("info empty, want toast text")
	}
}

func TestUpdate_TidyResult_NoChanges(t *testing.T) {
	m := newTestComposeModel()
	m.editor.SetValue("perfect text")
	m.tidyInFlight = true
	msg := tidyResultMsg{
		oldBody: "perfect text",
		res: tidytext.Result{
			Status:  tidytext.StatusNoChanges,
			Text:    "perfect text",
			Message: "tidytext: no changes needed",
		},
	}
	next, _ := m.Update(msg)
	if next.editor.Value() != "perfect text" {
		t.Errorf("body changed on no-changes path")
	}
	if next.info == "" {
		t.Errorf("info empty, want 'no changes' toast")
	}
}

func TestUpdate_TidyResult_Error(t *testing.T) {
	m := newTestComposeModel()
	m.editor.SetValue("anything")
	m.tidyInFlight = true
	msg := tidyResultMsg{
		oldBody: "anything",
		res: tidytext.Result{
			Status:  tidytext.StatusError,
			Text:    "anything",
			Message: "tidytext: API error (500): boom, text unchanged",
		},
	}
	next, _ := m.Update(msg)
	if next.editor.Value() != "anything" {
		t.Errorf("body changed on error path")
	}
	if next.err == "" {
		t.Errorf("err empty, want error text")
	}
}
