//go:build unix

package term

import (
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

// MeasureSPUACells returns the rendered cell width (1 or 2) of an
// SPUA-A glyph in the current terminal+font, or 0 on failure. The
// probe runs ESC[6n, writes a test glyph, runs ESC[6n again, and
// returns the column delta.
//
// The probe-glyph-probe shape is adapted from
// github.com/hymkor/go-cursorposition (MIT).
func MeasureSPUACells() int {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return 0
	}
	defer tty.Close()
	w, err := measureSPUACellsOn(tty, 200*time.Millisecond)
	if err != nil {
		return 0
	}
	return w
}

// testGlyph is nf-md-mailbox (U+F01EE), in the same SPUA-A range as
// poplar's actual icons. Per-glyph width is uniform within a Nerd
// Font.
const testGlyph = "\U000F01EE"

var probeMu sync.Mutex

func measureSPUACellsOn(tty *os.File, timeout time.Duration) (int, error) {
	probeMu.Lock()
	defer probeMu.Unlock()

	fd := int(tty.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return 0, err
	}
	defer term.Restore(fd, oldState)

	colBefore, err := requestCPR(tty, timeout)
	if err != nil {
		return 0, err
	}
	if _, err := io.WriteString(tty, testGlyph); err != nil {
		return 0, err
	}
	colAfter, err := requestCPR(tty, timeout)
	if err != nil {
		return 0, err
	}
	delta := colAfter - colBefore
	if delta != 1 && delta != 2 {
		return 0, errors.New("term: unexpected cell delta")
	}
	return delta, nil
}

func requestCPR(tty *os.File, timeout time.Duration) (int, error) {
	if _, err := io.WriteString(tty, "\x1b[6n"); err != nil {
		return 0, err
	}
	type result struct {
		col int
		err error
	}
	ch := make(chan result, 1)
	go func() {
		_, col, err := parseCPR(tty)
		ch <- result{col, err}
	}()
	select {
	case r := <-ch:
		return r.col, r.err
	case <-time.After(timeout):
		return 0, errors.New("term: CPR timeout")
	}
}
