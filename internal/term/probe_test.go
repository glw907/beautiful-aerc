//go:build unix

// SPDX-License-Identifier: MIT
package term

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/creack/pty"
)

// fakeTerminal scripts a slave-side responder that, on seeing the
// glyph or CSI bytes from the probe, replies with CPRs scripted by
// the test. It is a simple byte-pattern reader; not a full terminal
// emulator.
type fakeTerminal struct {
	slave            *os.File
	colBefore, after int
	delay            time.Duration
}

// run consumes bytes from the master side. The probe sequence is:
//
//  1. Write ESC[6n  (initial position request)
//  2. Write the SPUA-A glyph (variable bytes)
//  3. Write ESC[6n  (post-glyph request)
//
// We respond to each ESC[6n with a CPR using the scripted columns.
func (f *fakeTerminal) run(t *testing.T) {
	buf := make([]byte, 64)
	cpr := 0
	for {
		n, err := f.slave.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			return
		}
		// Find ESC [ ... n requests; reply once per request.
		i := 0
		for i < n {
			if i+2 < n && buf[i] == 0x1b && buf[i+1] == '[' {
				// Find the terminator.
				j := i + 2
				for j < n && (buf[j] == ';' || (buf[j] >= '0' && buf[j] <= '9')) {
					j++
				}
				if j < n && buf[j] == 'n' {
					if f.delay > 0 {
						time.Sleep(f.delay)
					}
					col := f.colBefore
					if cpr == 1 {
						col = f.after
					}
					_, _ = f.slave.Write([]byte("\x1b[1;" + intToStr(col) + "R"))
					cpr++
					i = j + 1
					continue
				}
			}
			i++
		}
	}
}

func intToStr(i int) string {
	// avoid strconv import for build-tag isolation
	if i == 0 {
		return "0"
	}
	var b [8]byte
	n := 0
	for i > 0 {
		b[n] = byte('0' + i%10)
		i /= 10
		n++
	}
	out := make([]byte, n)
	for k := 0; k < n; k++ {
		out[k] = b[n-1-k]
	}
	return string(out)
}

func TestMeasureSPUACellsViaPTY(t *testing.T) {
	tests := []struct {
		name      string
		colBefore int
		colAfter  int
		want      int
	}{
		{"narrow glyph (1 cell)", 1, 2, 1},
		{"wide glyph (2 cells)", 1, 3, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			master, slave, err := pty.Open()
			if err != nil {
				t.Fatalf("pty.Open: %v", err)
			}
			defer master.Close()
			defer slave.Close()

			ft := &fakeTerminal{slave: slave, colBefore: tt.colBefore, after: tt.colAfter}
			done := make(chan struct{})
			go func() { ft.run(t); close(done) }()

			got, err := measureSPUACellsOn(master, 200*time.Millisecond)
			if err != nil {
				t.Fatalf("measureSPUACellsOn err = %v", err)
			}
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}

			_ = master.Close()
			_ = slave.Close()
			select {
			case <-done:
			case <-time.After(time.Second):
			}
		})
	}
}

func TestMeasureSPUACellsTimeout(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	defer master.Close()
	defer slave.Close()

	// No goroutine reading the slave — probe should time out.
	got, err := measureSPUACellsOn(master, 50*time.Millisecond)
	if err == nil {
		t.Fatalf("want timeout error, got nil (returned %d)", got)
	}
	if got != 0 {
		t.Errorf("on timeout want 0, got %d", got)
	}
}

func TestMeasureSPUACellsMalformed(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	defer master.Close()
	defer slave.Close()

	go func() {
		buf := make([]byte, 64)
		for {
			n, err := slave.Read(buf)
			if err != nil {
				return
			}
			if bytes.Contains(buf[:n], []byte("\x1b[")) {
				_, _ = slave.Write([]byte("garbage-no-CPR"))
			}
		}
	}()

	_, err = measureSPUACellsOn(master, 100*time.Millisecond)
	if err == nil {
		t.Errorf("want parse error on malformed reply, got nil")
	}
}
