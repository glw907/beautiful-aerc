// SPDX-License-Identifier: MIT

package ui

import (
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// FooterContext identifies which keybinding set to display.
type FooterContext int

const (
	// AccountContext is the unified one-pane account view (folders + messages).
	AccountContext FooterContext = iota
	ViewerContext
)

// footerHint is one entry in the footer's keybinding display. dropRank
// controls responsive behavior: when the footer can't fit the full hint
// list, hints with higher dropRank are dropped first. dropRank 0 hints
// are always kept (escape hatch so the user can always reach help/quit).
type footerHint struct {
	key      string
	desc     string
	dropRank int
}

func hint(key, desc string, dropRank int) footerHint {
	return footerHint{key: key, desc: desc, dropRank: dropRank}
}

// triageHints and replyHints are shared by the account and viewer
// footers. Both contexts use identical bindings for these groups.
var (
	triageHints = []footerHint{
		hint("d", "del", 1),
		hint("a", "archive", 1),
		hint("s", "star", 4),
		hint(".", "read", 5),
	}
	replyHints = []footerHint{
		hint("r/R", "reply", 2),
		hint("f", "fwd", 3),
		hint("c", "compose", 2),
	}
)

// accountFooterGroups returns the unified one-pane account footer hint
// groups in display order.
//
// Drop order (highest rank first):
//   - nav entries (10, 9) — vim/arrow users don't need the hint
//   - v select (8), n/N results (7) — niche modes, discoverable via help
//   - F fold all (5), . read (5), s star (4), ␣ fold (4),
//     f fwd (3), / find (3) — secondary actions
//   - r/R reply (2), c compose (2) — primary compose actions
//   - d del (1), a archive (1) — primary triage
//   - ? help (0), q quit (0) — always kept
func accountFooterGroups() [][]footerHint {
	return [][]footerHint{
		{
			hint("j/k/J/K", "nav", 10),
			hint("I/D/S/A", "folders", 9),
		},
		triageHints,
		replyHints,
		{
			hint("/", "find", 3),
			hint("n/N", "results", 7),
			hint("v", "select", 8),
		},
		{
			hint("␣", "fold", 4),
			hint("F", "fold all", 5),
		},
		{
			hint("?", "help", 0),
			hint("q", "quit", 0),
		},
	}
}

// viewerFooterGroups returns the viewer footer hint groups. Reply
// drops before triage (triage is more essential in the viewer); the
// viewer/app group is always kept.
func viewerFooterGroups() [][]footerHint {
	return [][]footerHint{
		triageHints,
		replyHints,
		{
			hint("Tab", "links", 0),
			hint("q", "close", 0),
			hint("?", "help", 0),
		},
	}
}

// Footer renders context-appropriate keybinding hints with group separators.
type Footer struct {
	styles  Styles
	context FooterContext
	counter string
}

// NewFooter creates a Footer with the given styles.
func NewFooter(styles Styles) Footer {
	return Footer{
		styles:  styles,
		context: AccountContext,
	}
}

// SetContext returns a copy of f with the displayed keybinding set switched.
func (f Footer) SetContext(ctx FooterContext) Footer {
	f.context = ctx
	return f
}

// SetCounter returns a copy of f with a dynamic window counter string.
// When non-empty and context is AccountContext, the counter is injected
// as a rank-8 hint group between the fold group and the help/quit group.
// Pass "" to remove the counter hint.
func (f Footer) SetCounter(counter string) Footer {
	f.counter = counter
	return f
}

// View renders the footer at the given width.
//
// Hints are progressively dropped (highest dropRank first) when the
// full hint list would exceed width. Groups that become empty collapse
// along with their preceding separator. dropRank 0 hints are always
// kept — if they alone still exceed width, the line overflows.
func (f Footer) View(width int) string {
	var groups [][]footerHint
	switch f.context {
	case ViewerContext:
		groups = viewerFooterGroups()
	default:
		base := accountFooterGroups()
		if f.counter != "" {
			// Inject the window counter as a one-element group between
			// the fold group (second-to-last) and the help/quit group
			// (last). dropRank 8 keeps it peer to "v select".
			counterGroup := []footerHint{hint("", f.counter, 8)}
			n := len(base)
			groups = make([][]footerHint, 0, n+1)
			groups = append(groups, base[:n-1]...)
			groups = append(groups, counterGroup)
			groups = append(groups, base[n-1])
		} else {
			groups = base
		}
	}

	visible := fitFooterHints(groups, width)

	sep := " " + f.styles.FooterSep.Render("┊") + "  "

	var parts []string
	for _, g := range visible {
		if len(g) == 0 {
			continue
		}
		var rendered []string
		for _, h := range g {
			k := f.styles.FooterKey.Render(h.key)
			d := f.styles.FooterHint.Render(" " + h.desc)
			rendered = append(rendered, k+d)
		}
		parts = append(parts, strings.Join(rendered, "  "))
	}

	line := " " + strings.Join(parts, sep)
	pad := max(0, width-lipgloss.Width(line))
	return line + strings.Repeat(" ", pad)
}

// fitFooterHints returns a trimmed copy of groups that fits within
// width, dropping hints with the highest dropRank first. When a group
// loses all of its hints, it vanishes from the output (taking its
// separator with it).
func fitFooterHints(groups [][]footerHint, width int) [][]footerHint {
	visible := make([][]footerHint, len(groups))
	for i, g := range groups {
		visible[i] = slices.Clone(g)
	}
	for measureFooter(visible) > width {
		gi, hi := highestDropRank(visible)
		if gi < 0 {
			break
		}
		visible[gi] = slices.Delete(visible[gi], hi, hi+1)
	}
	return visible
}

// highestDropRank returns the (group, hint) index of the highest
// dropRank hint across all groups, or (-1, -1) if nothing is droppable
// (everything left is dropRank 0).
func highestDropRank(groups [][]footerHint) (int, int) {
	bestGroup, bestHint, bestRank := -1, -1, 0
	for gi, g := range groups {
		for hi, h := range g {
			if h.dropRank == 0 {
				continue
			}
			if bestGroup < 0 || h.dropRank > bestRank {
				bestGroup, bestHint, bestRank = gi, hi, h.dropRank
			}
		}
	}
	return bestGroup, bestHint
}

// measureFooter returns the visual cell width of the rendered footer:
// leading space, hints joined by two spaces within a group, and
// non-empty groups joined by " ┊  " separators.
func measureFooter(groups [][]footerHint) int {
	total := 1 // leading space
	first := true
	for _, g := range groups {
		if len(g) == 0 {
			continue
		}
		if !first {
			total += 4 // " ┊  " sep
		}
		first = false
		for j, h := range g {
			if j > 0 {
				total += 2 // "  " between hints
			}
			total += lipgloss.Width(h.key) + 1 + lipgloss.Width(h.desc)
		}
	}
	return total
}
