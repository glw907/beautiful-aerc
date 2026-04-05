// Package picker extracts URLs from text and presents an interactive selection UI.
package picker

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/glw907/beautiful-aerc/internal/palette"
)

var (
	reURL      = regexp.MustCompile(`https?://[^\s>)\]]+`)
	reOSC8Link = regexp.MustCompile(`\x1b\]8;;([^\x1b]*)\x1b\\([^\x1b]*)\x1b\]8;;\x1b\\`)
	reOSC8     = regexp.MustCompile(`\x1b\]8;;[^\x1b]*\x1b\\`)
	reANSI     = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

// ExtractURLs finds all unique URLs in text, preserving order.
// Extracts full URLs from OSC 8 hyperlinks first, then strips all
// escape sequences and finds remaining plain-text URLs.
func ExtractURLs(text string) []string {
	seen := make(map[string]bool)
	var urls []string
	add := func(u string) {
		u = strings.TrimRight(u, ".,;:!?")
		if u == "" || seen[u] {
			return
		}
		seen[u] = true
		urls = append(urls, u)
	}

	// Extract full URLs from OSC 8 hyperlink hrefs.
	for _, m := range reOSC8Link.FindAllStringSubmatch(text, -1) {
		if m[1] != "" {
			add(m[1])
		}
	}

	// Strip full OSC 8 hyperlink spans (open + display + close), then
	// any remaining bare OSC 8 tags, then ANSI codes.
	clean := reOSC8Link.ReplaceAllString(text, "")
	clean = reOSC8.ReplaceAllString(clean, "")
	clean = reANSI.ReplaceAllString(clean, "")
	for _, u := range reURL.FindAllString(clean, -1) {
		add(u)
	}
	return urls
}

// Colors holds ANSI escape sequences for the picker UI.
type Colors struct {
	Number   string // shortcut number (1-9, 0)
	URL      string // URL text
	Selected string // highlighted line (bg + fg)
	Reset    string
}

// FormatLine renders a single picker line with number, URL, and selection state.
func FormatLine(index int, url string, selected bool, colors *Colors) string {
	shortcut := " "
	if index >= 1 && index <= 9 {
		shortcut = fmt.Sprintf("%d", index)
	} else if index == 10 {
		shortcut = "0"
	}

	if selected {
		return fmt.Sprintf("%s %s  %s%s", colors.Selected, shortcut, url, colors.Reset)
	}
	return fmt.Sprintf(" %s%s%s  %s%s%s", colors.Number, shortcut, colors.Reset, colors.URL, url, colors.Reset)
}

// ColorsFromPalette builds picker colors from a loaded palette.
func ColorsFromPalette(p *palette.Palette) *Colors {
	numColor, _ := palette.HexToANSI(p.Get("ACCENT_PRIMARY"))
	urlColor, _ := palette.HexToANSI(p.Get("FG_DIM"))
	selBG, _ := palette.HexToANSI(p.Get("BG_SELECTION"))
	selFG, _ := palette.HexToANSI(p.Get("FG_BRIGHT"))

	c := &Colors{Reset: "\033[0m"}
	if numColor != "" {
		c.Number = "\033[" + numColor + "m"
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

// Run reads stdin, extracts URLs, and runs the interactive picker.
// Returns the selected URL or empty string if cancelled.
func Run(r io.Reader, w io.Writer, colors *Colors) (string, error) {
	input, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("reading input: %w", err)
	}

	urls := ExtractURLs(string(input))
	if len(urls) == 0 {
		return "", nil
	}

	oldState, err := makeRaw(os.Stdin.Fd())
	if err != nil {
		return "", fmt.Errorf("setting raw mode: %w", err)
	}
	defer restore(os.Stdin.Fd(), oldState)

	selected := 0
	render(w, urls, selected, colors)

	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return "", nil
		}

		key := buf[:n]

		// 1-9: instant select
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			idx := int(key[0] - '0' - 1)
			if idx < len(urls) {
				return urls[idx], nil
			}
			continue
		}

		// 0: select 10th
		if len(key) == 1 && key[0] == '0' {
			if len(urls) >= 10 {
				return urls[9], nil
			}
			continue
		}

		// Enter: select current
		if len(key) == 1 && (key[0] == '\r' || key[0] == '\n') {
			return urls[selected], nil
		}

		// q or Escape: cancel
		if len(key) == 1 && (key[0] == 'q' || key[0] == 27) {
			return "", nil
		}

		// j or down arrow: next
		if (len(key) == 1 && key[0] == 'j') || (len(key) == 3 && key[0] == 27 && key[1] == '[' && key[2] == 'B') {
			if selected < len(urls)-1 {
				selected++
				render(w, urls, selected, colors)
			}
			continue
		}

		// k or up arrow: prev
		if (len(key) == 1 && key[0] == 'k') || (len(key) == 3 && key[0] == 27 && key[1] == '[' && key[2] == 'A') {
			if selected > 0 {
				selected--
				render(w, urls, selected, colors)
			}
			continue
		}
	}
}

func render(w io.Writer, urls []string, selected int, colors *Colors) {
	fmt.Fprint(w, "\033[2J\033[H")
	for i, u := range urls {
		fmt.Fprintln(w, FormatLine(i+1, u, i == selected, colors))
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
