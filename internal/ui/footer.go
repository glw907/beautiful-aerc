package ui

import (
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// FooterContext picks which keybinding set the footer displays.
type FooterContext int

const (
	// AccountContext is the unified one-pane account view (folders + messages).
	AccountContext FooterContext = iota
	ViewerContext
	ContactsContext
)

// footerHint is one entry in the footer keybinding display. dropRank
// controls responsive behavior: when the footer can't fit the full hint
// list, higher dropRanks are dropped first. dropRank 0 hints are always
// kept so the user can always reach help/quit.
type footerHint struct {
	key      string
	desc     string
	dropRank int
}

func hint(key, desc string, dropRank int) footerHint {
	return footerHint{key: key, desc: desc, dropRank: dropRank}
}

// triageHints and replyHints are shared by the account and viewer
// footers; both contexts bind these groups identically.
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
// groups in display order. Drop order (highest rank first):
//   - nav entries (10, 9): vim/arrow users don't need the hint
//   - v select (8), n/N results (7): niche, discoverable via help
//   - secondary actions (5, 4, 3): F fold all, . read, s star, ␣ fold,
//     f fwd, / find
//   - r/R reply (2), c compose (2): primary compose actions
//   - d del (1), a archive (1): primary triage
//   - ? help (0), q quit (0): always kept
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

// viewerFooterGroups returns the viewer footer hint groups. Reply drops
// before triage because triage is more essential in the viewer. The
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

func contactsFooterGroups() [][]footerHint {
	return [][]footerHint{
		{
			hint("j/k", "cursor", 10),
			hint("J/K", "group", 9),
			hint("a–z", "jump", 8),
		},
		{
			hint("n", "new", 3),
			hint("e", "edit", 3),
		},
		{
			hint("M", "mail", 0),
			hint("q", "quit", 0),
		},
	}
}

func composeFooterGroups(hasSig, isFocusFrom, tidyVisible bool) [][]footerHint {
	core := []footerHint{
		hint("Ctrl+X", "send", 0),
		hint("Ctrl+C", "cancel", 0),
		hint("Tab", "field", 4),
		hint("Ctrl+O", "attach", 6),
	}
	if hasSig {
		core = append(core, hint("Ctrl+G", "sig", 5))
	}
	core = append(core, hint("Ctrl+L", "later", 6))
	if tidyVisible {
		core = append(core, hint("Ctrl+T", "tidy", 6))
	}
	groups := [][]footerHint{core}
	if isFocusFrom {
		groups = append(groups, []footerHint{
			hint("Space/←→", "identity", 6),
		})
	}
	return groups
}

// Footer renders context-appropriate keybinding hints with group
// separators.
type Footer struct {
	styles  Styles
	context FooterContext
	counter string
}

func NewFooter(styles Styles) Footer {
	return Footer{
		styles:  styles,
		context: AccountContext,
	}
}

func (f Footer) SetContext(ctx FooterContext) Footer {
	f.context = ctx
	return f
}

// SetCounter sets a dynamic window-counter string. When non-empty and
// context is AccountContext, the counter renders as a rank-8 hint group
// between the fold group and the help/quit group. Pass "" to remove it.
func (f Footer) SetCounter(counter string) Footer {
	f.counter = counter
	return f
}

// View renders the footer at the given width. Hints are dropped (highest
// dropRank first) when the full list would overflow; groups that lose
// every hint vanish along with their preceding separator. dropRank 0
// hints are always kept, so the line can still overflow if those alone
// exceed width.
func (f Footer) View(width int) string {
	var groups [][]footerHint
	switch f.context {
	case ViewerContext:
		groups = viewerFooterGroups()
	case ContactsContext:
		groups = contactsFooterGroups()
	default:
		base := accountFooterGroups()
		if f.counter != "" {
			// Inject the window counter as a one-element group between the
			// fold group (second-to-last) and the help/quit group (last).
			// dropRank 8 makes it peer to "v select".
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
	return f.renderGroups(groups, width)
}

// ViewGroups renders the footer using caller-supplied hint groups.
func (f Footer) ViewGroups(groups [][]footerHint, width int) string {
	return f.renderGroups(groups, width)
}

func (f Footer) renderGroups(groups [][]footerHint, width int) string {
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
// width, dropping the highest-dropRank hint first. A group that loses
// every hint vanishes along with its preceding separator.
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
// dropRank across all groups, or (-1, -1) when only dropRank 0 remains.
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

// measureFooter returns the rendered footer's cell width: leading space,
// hints joined by "  " within a group, non-empty groups joined by " ┊  ".
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
