// Package picker extracts URLs from text and presents an interactive selection UI.
package picker

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/glw907/beautiful-aerc/internal/palette"
)

// Colors holds ANSI escape sequences for the picker UI.
type Colors struct {
	Number   string // shortcut number (1-9, 0)
	Label    string // link label text
	URL      string // URL text (dim)
	Selected string // highlighted line (bg + fg)
	Reset    string
}

// FormatLine renders a single picker line with number, label, and truncated URL.
func FormatLine(index int, link filter.FootnoteLink, selected bool, cols int, labelWidth int, colors *Colors) string {
	shortcut := " "
	if index >= 1 && index <= 9 {
		shortcut = fmt.Sprintf("%d", index)
	} else if index == 10 {
		shortcut = "0"
	}

	label := link.Label
	if len(label) > labelWidth {
		label = label[:labelWidth-1] + "…"
	}

	// Fixed columns: " N  " (4) + label + "  " (2) + url
	urlWidth := cols - 4 - labelWidth - 2
	url := link.URL
	if urlWidth > 0 && len(url) > urlWidth {
		if urlWidth > 1 {
			url = url[:urlWidth-1] + "…"
		} else {
			url = "…"
		}
	}

	if selected {
		return fmt.Sprintf("%s %s  %-*s  %s%s",
			colors.Selected, shortcut, labelWidth, label, url, colors.Reset)
	}
	return fmt.Sprintf(" %s%s%s  %s%-*s%s  %s%s%s",
		colors.Number, shortcut, colors.Reset,
		colors.Label, labelWidth, label, colors.Reset,
		colors.URL, url, colors.Reset)
}

// ColorsFromPalette builds picker colors from a loaded palette.
func ColorsFromPalette(p *palette.Palette) *Colors {
	numColor, _ := palette.HexToANSI(p.Get("ACCENT_PRIMARY"))
	labelColor, _ := palette.HexToANSI(p.Get("FG_PRIMARY"))
	urlColor, _ := palette.HexToANSI(p.Get("FG_DIM"))
	selBG, _ := palette.HexToANSI(p.Get("BG_SELECTION"))
	selFG, _ := palette.HexToANSI(p.Get("FG_BRIGHT"))

	c := &Colors{Reset: "\033[0m"}
	if numColor != "" {
		c.Number = "\033[" + numColor + "m"
	}
	if labelColor != "" {
		c.Label = "\033[" + labelColor + "m"
	}
	if urlColor != "" {
		c.URL = "\033[" + urlColor + "m"
	}
	if selBG != "" && selFG != "" {
		bgParam := strings.Replace(selBG, "38;2;", "48;2;", 1)
		c.Selected = "\033[" + bgParam + "m\033[" + selFG + "m"
	}
	return c
}

const maxLabelWidth = 30

// Run reads message content from r, extracts footnoted links, and runs the
// interactive picker. Keyboard input is read from /dev/tty so stdin can be
// a pipe. Returns the selected URL or empty string if cancelled.
func Run(links []filter.FootnoteLink, w io.Writer, cols int, colors *Colors) (string, error) {
	if len(links) == 0 {
		return "", nil
	}

	// Compute label column width from longest label, capped.
	labelWidth := 0
	for _, l := range links {
		if len(l.Label) > labelWidth {
			labelWidth = len(l.Label)
		}
	}
	if labelWidth > maxLabelWidth {
		labelWidth = maxLabelWidth
	}

	tty, err := os.Open("/dev/tty")
	if err != nil {
		return "", fmt.Errorf("opening /dev/tty: %w", err)
	}
	defer tty.Close()

	oldState, err := makeRaw(tty.Fd())
	if err != nil {
		return "", fmt.Errorf("setting raw mode: %w", err)
	}
	defer restore(tty.Fd(), oldState)
	defer fmt.Fprint(w, "\033[?25h") // restore cursor on exit

	// Hide cursor and clear screen for initial draw.
	fmt.Fprint(w, "\033[?25l\033[2J")
	selected := 0
	render(w, links, selected, cols, labelWidth, colors)

	buf := make([]byte, 3)
	for {
		n, err := tty.Read(buf)
		if err != nil {
			return "", nil
		}

		key := buf[:n]

		// 1-9: instant select
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			idx := int(key[0] - '0' - 1)
			if idx < len(links) {
				return links[idx].URL, nil
			}
			continue
		}

		// 0: select 10th
		if len(key) == 1 && key[0] == '0' {
			if len(links) >= 10 {
				return links[9].URL, nil
			}
			continue
		}

		// Enter: select current
		if len(key) == 1 && (key[0] == '\r' || key[0] == '\n') {
			return links[selected].URL, nil
		}

		// q or Escape: cancel
		if len(key) == 1 && (key[0] == 'q' || key[0] == 27) {
			return "", nil
		}

		// j or down arrow: next
		if (len(key) == 1 && key[0] == 'j') || (len(key) == 3 && key[0] == 27 && key[1] == '[' && key[2] == 'B') {
			if selected < len(links)-1 {
				selected++
				render(w, links, selected, cols, labelWidth, colors)
			}
			continue
		}

		// k or up arrow: prev
		if (len(key) == 1 && key[0] == 'k') || (len(key) == 3 && key[0] == 27 && key[1] == '[' && key[2] == 'A') {
			if selected > 0 {
				selected--
				render(w, links, selected, cols, labelWidth, colors)
			}
			continue
		}
	}
}

func render(w io.Writer, links []filter.FootnoteLink, selected, cols, labelWidth int, colors *Colors) {
	// Move cursor home and overwrite in place to avoid flicker.
	fmt.Fprint(w, "\033[H")
	fmt.Fprintln(w)
	for i, l := range links {
		// Clear line before writing to remove stale content.
		fmt.Fprintf(w, "\033[2K%s\n", FormatLine(i+1, l, i == selected, cols, labelWidth, colors))
	}
}

func makeRaw(fd uintptr) (*unix.Termios, error) {
	old, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	if err != nil {
		return nil, err
	}
	raw := *old
	raw.Lflag &^= unix.ECHO | unix.ICANON | unix.ISIG
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(int(fd), unix.TCSETS, &raw); err != nil {
		return nil, err
	}
	return old, nil
}

func restore(fd uintptr, state *unix.Termios) {
	_ = unix.IoctlSetTermios(int(fd), unix.TCSETS, state)
}
