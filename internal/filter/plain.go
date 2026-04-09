package filter

import (
	"fmt"
	"html"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/glw907/beautiful-aerc/internal/theme"
)

var (
	htmlTagRe      = regexp.MustCompile(`(?i)<(div|html|body|table|span|br|p[ />])`)
	reTabListItem  = regexp.MustCompile(`(?m)^\t+([-*+] )`)
)

func detectHTML(text string) bool {
	lines := strings.SplitN(text, "\n", 51)
	if len(lines) > 50 {
		lines = lines[:50]
	}
	sample := strings.Join(lines, "\n")
	return htmlTagRe.MatchString(sample)
}

// Plain handles the text/plain filter. If stdin looks like HTML,
// delegates to HTML filter. Otherwise pipes through wrap | colorize.
func Plain(r io.Reader, w io.Writer, t *theme.Theme, cols int) error {
	body, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}
	text := string(body)

	fmt.Fprintln(w) // leading blank line

	if detectHTML(text) {
		return HTML(strings.NewReader(text), w, t, cols)
	}

	// Some senders put HTML entities in text/plain parts.
	text = html.UnescapeString(text)
	// Normalize tab-indented list items to prevent 8-space expansion.
	text = reTabListItem.ReplaceAllString(text, "$1")

	colStr := "80"
	if cols > 0 {
		colStr = fmt.Sprintf("%d", cols)
	}

	wrap := exec.Command("wrap", "-w", colStr, "-r")
	wrap.Stdin = strings.NewReader(text)

	colorize, colorizeErr := findColorize()
	if colorizeErr != nil {
		out, err := wrap.Output()
		if err != nil {
			return fmt.Errorf("running wrap: %w", err)
		}
		_, err = w.Write(out)
		return err
	}

	wrapOut, err := wrap.Output()
	if err != nil {
		return fmt.Errorf("running wrap: %w", err)
	}

	col := exec.Command(colorize)
	col.Stdin = strings.NewReader(string(wrapOut))
	col.Stdout = w
	col.Stderr = os.Stderr
	if err := col.Run(); err != nil {
		return fmt.Errorf("running colorize: %w", err)
	}
	return nil
}
