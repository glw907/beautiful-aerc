// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
	"testing"
)

func TestDimANSI(t *testing.T) {
	t.Run("plain string starts with ESC[2m", func(t *testing.T) {
		got := DimANSI("plain")
		if !strings.HasPrefix(got, "\x1b[2m") {
			t.Errorf("plain input: output should start with ESC[2m, got %q", got)
		}
		if !strings.Contains(got, "plain") {
			t.Errorf("plain input: original text missing from output: %q", got)
		}
	})

	t.Run("256-color foreground gets faint injected", func(t *testing.T) {
		input := "\x1b[38;5;120mhello\x1b[0m"
		got := DimANSI(input)
		if !strings.HasPrefix(got, "\x1b[2m") {
			t.Errorf("should start with ESC[2m: %q", got)
		}
		// The 256-color SGR should have faint prepended.
		if !strings.Contains(got, "\x1b[2;38;5;120m") {
			t.Errorf("256-color SGR missing faint prefix: %q", got)
		}
		// Reset should become ESC[0;2m to re-apply faint.
		if !strings.Contains(got, "\x1b[0;2m") {
			t.Errorf("reset not rewritten to ESC[0;2m: %q", got)
		}
		if !strings.Contains(got, "hello") {
			t.Errorf("text content missing: %q", got)
		}
	})

	t.Run("truecolor foreground gets faint injected", func(t *testing.T) {
		input := "\x1b[38;2;100;200;50mhi\x1b[0m"
		got := DimANSI(input)
		if !strings.Contains(got, "\x1b[2;38;2;100;200;50m") {
			t.Errorf("truecolor SGR missing faint prefix: %q", got)
		}
		if !strings.Contains(got, "\x1b[0;2m") {
			t.Errorf("reset not rewritten: %q", got)
		}
	})

	t.Run("nested resets all become ESC[0;2m", func(t *testing.T) {
		input := "\x1b[31ma\x1b[0mb\x1b[32mc\x1b[0m"
		got := DimANSI(input)
		// Both resets should be rewritten.
		resetCount := strings.Count(got, "\x1b[0;2m")
		if resetCount != 2 {
			t.Errorf("expected 2 rewritten resets, got %d: %q", resetCount, got)
		}
		// Bare ESC[0m should not remain.
		if strings.Contains(got, "\x1b[0m") {
			t.Errorf("bare ESC[0m survived (should have been rewritten): %q", got)
		}
	})

	t.Run("bare reset ESC[m becomes ESC[0;2m", func(t *testing.T) {
		// ESC[m (empty params) is equivalent to a reset.
		input := "\x1b[mtext"
		got := DimANSI(input)
		if !strings.Contains(got, "\x1b[0;2m") {
			t.Errorf("bare ESC[m not rewritten to ESC[0;2m: %q", got)
		}
	})
}
