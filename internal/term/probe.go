//go:build unix

// SPDX-License-Identifier: MIT
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
// SPUA-A glyph in the current terminal+font, or 0 on any failure.
//
// The probe writes ESC[6n, records the cursor column, writes a test
// SPUA-A glyph, writes ESC[6n again, and computes the delta. Falls
// back to 0 if /dev/tty cannot be opened, the terminal does not
// reply within the timeout, or the response is unparseable.
//
// Adapted from github.com/hymkor/go-cursorposition (MIT) — credit
// retained per the prior-art-with-license discipline. The
// AmbiguousWidth pattern (probe-glyph-probe) is the same shape.
func MeasureSPUACells() int {
	w, err := measureSPUACells(200 * time.Millisecond)
	if err != nil {
		return 0
	}
	return w
}

func measureSPUACells(timeout time.Duration) (int, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return 0, err
	}
	defer tty.Close()
	return measureSPUACellsOn(tty, timeout)
}

// testGlyph is a Nerd Font SPUA-A icon (nf-md-mailbox U+F01EE — chosen
// because it is in the same range as poplar's actual icons; per-glyph
// width is uniform within a Nerd Font).
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
