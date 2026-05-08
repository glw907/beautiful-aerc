package compose

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/glw907/poplar/internal/catkin"
	"github.com/glw907/poplar/internal/tidy"
)

// tidyResultMsg lands when tidy.Tidy returns. oldBody is the body
// captured at request time so the diff is computed against the
// pre-tidy text regardless of any buffer mutation in flight.
type tidyResultMsg struct {
	oldBody string
	res     tidy.Result
	err     error
}

// handleTidyKey runs when Ctrl+T fires. Returns the cmd that runs
// tidy.Tidy or nil when invocation is inert. Sets c.err for the
// missing-API-key case so the user sees why nothing happened.
func (c *Model) handleTidyKey() tea.Cmd {
	if !c.tidyEnabled || c.focus != focusBody || c.tidyInFlight {
		return nil
	}
	if c.tidyAPIKey == "" {
		c.err = "Tidy: ANTHROPIC_API_KEY not set"
		return nil
	}
	body := c.editor.Value()
	c.tidyInFlight = true
	c.info = "Tidy: running…"
	cfg, key, fn := c.tidyCfg, c.tidyAPIKey, c.tidyFn
	return func() tea.Msg {
		res, err := fn(body, cfg, key, "")
		return tidyResultMsg{oldBody: body, res: res, err: err}
	}
}

// applyTidyResult routes a tidyResultMsg by Status and updates the
// editor + chrome state.
func (c *Model) applyTidyResult(msg tidyResultMsg) tea.Cmd {
	c.tidyInFlight = false
	c.info = ""

	if msg.err != nil {
		c.err = "Tidy: " + msg.err.Error()
		return nil
	}

	switch msg.res.Status {
	case tidy.StatusCorrected:
		c.editor.SetValue(msg.res.Text)
		ranges := byteRangesToCatkin(tidy.DiffRanges(msg.oldBody, msg.res.Text))
		c.editor.SetTidyHighlights(msg.res.Text, ranges)
		c.info = msg.res.Message
		c.err = ""
	case tidy.StatusNoChanges:
		c.info = msg.res.Message
		c.err = ""
	case tidy.StatusNoAuthorText, tidy.StatusError:
		c.err = msg.res.Message
		c.info = ""
	default:
		c.err = fmt.Sprintf("Tidy: unknown status %d", msg.res.Status)
	}
	return nil
}

func byteRangesToCatkin(in []tidy.ByteRange) []catkin.Range {
	if len(in) == 0 {
		return nil
	}
	out := make([]catkin.Range, len(in))
	for i, r := range in {
		out[i] = catkin.Range{Start: r.Start, End: r.End}
	}
	return out
}
