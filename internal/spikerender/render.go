// Package spikerender is the iterated rendering arm for the renderspike
// tool. It layers go-readability preprocessing on top of the legacy
// filter pipeline and email-specific noise rules developed across rounds
// of comparison against the hand-authored ideal renders.
package spikerender

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/glw907/poplar/internal/filter"
	readability "github.com/go-shiori/go-readability"
)

// readabilityMinLen is the minimum TextContent character count an extracted
// article must reach for go-readability output to replace the source HTML.
const readabilityMinLen = 150

// readabilityMinRatio is the minimum fraction of the source text length
// that readability must extract to be trusted. Most structured email types
// (receipts, calendar invites, threaded replies) fail this check and fall
// back to the original HTML. Newsletters typically pass.
const readabilityMinRatio = 0.50

// Render converts email body content to clean markdown via the iterated
// pipeline. HTML follows: readability extraction, legacy html-to-markdown
// filter, email-specific noise rules. Plain text uses CleanPlain directly.
func Render(content []byte, contentType string) string {
	if contentType != "text/html" {
		return filter.CleanPlain(string(content))
	}
	src := string(content)
	extracted := extractReadable(src)
	md := filter.CleanHTML(extracted)
	md = stripGitHubTracking(md)
	md = stripGitHubFooter(md)
	md = stripGCalFooter(md)
	md = stripGroupsIoFooter(md)
	md = stripViewInBrowser(md)
	md = stripSelfLinks(md)
	return md
}

// extractReadable runs go-readability on src HTML and returns the extracted
// article HTML when the result is both long enough and represents a
// substantial share of the original text content. Short results, errors,
// content that represents less than readabilityMinRatio of the source text,
// and HTML that contains <blockquote> elements all fall back to src.
//
// The ratio guard prevents readability from replacing structured emails
// (receipts, calendar invites) with a single extracted prose block. The
// blockquote guard prevents readability from anchoring on the quoted
// history in threaded replies instead of the new top-level message.
func extractReadable(src string) string {
	if strings.Contains(src, "<blockquote") {
		return src
	}
	u, _ := url.Parse("https://email.local/")
	article, err := readability.FromReader(strings.NewReader(src), u)
	if err != nil || article.Length < readabilityMinLen {
		return src
	}
	srcTextLen := textLen(src)
	if srcTextLen > 0 && float64(article.Length)/float64(srcTextLen) < readabilityMinRatio {
		return src
	}
	return article.Content
}

// textLen returns the approximate character count of visible text in an HTML
// document by skipping everything inside angle brackets.
func textLen(html string) int {
	n := 0
	inTag := false
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			n++
		}
	}
	return n
}

// reGitHubTrackingLink matches a markdown link to github.com that carries
// email_source or email_token tracking parameters.
// Capture group 1: link text. Capture group 2: path before the query string.
var reGitHubTrackingLink = regexp.MustCompile(
	`\[([^\]]*)\]\((https://github\.com[^?)#]*)[?][^)]*email_(?:source|token)[^)]*\)`,
)

// stripGitHubTracking removes email_source and email_token query params
// from github.com markdown links. GitHub injects these into every
// notification URL; the path alone is the stable action destination.
func stripGitHubTracking(text string) string {
	return reGitHubTrackingLink.ReplaceAllString(text, "[$1]($2)")
}

// reGitHubNotifBlock matches the em-dash separator plus "You are receiving
// this because..." footer block that GitHub appends to every notification.
var reGitHubNotifBlock = regexp.MustCompile(
	`(?s)\n+—\s*\nYou are receiving this because [^\n]+\n.*`,
)

// reGitHubNotifLine matches the footer when the em-dash separator is
// absent or has already been stripped.
var reGitHubNotifLine = regexp.MustCompile(
	`(?s)\n+You are receiving this because [^\n]+\n.*`,
)

// reGitHubAddress matches the GitHub corporate postal address line.
var reGitHubAddress = regexp.MustCompile(
	`\n*GitHub, Inc\. [^\n]+$`,
)

// reTrailingEmDash matches a trailing em-dash separator left after footer
// stripping.
var reTrailingEmDash = regexp.MustCompile(`\s*—\s*$`)

// stripGitHubFooter removes the GitHub notification footer and corporate
// address line that appear in every GitHub Actions and notification email.
func stripGitHubFooter(text string) string {
	text = reGitHubNotifBlock.ReplaceAllString(text, "")
	text = reGitHubNotifLine.ReplaceAllString(text, "")
	text = reGitHubAddress.ReplaceAllString(text, "")
	text = reTrailingEmDash.ReplaceAllString(text, "")
	return strings.TrimRight(text, "\n ")
}

// reGCalAttendeeLine matches Google Calendar's "You are receiving this
// email because you are an attendee on the event." footer line and the
// forwarding-risk notice that follows it.
var reGCalAttendeeLine = regexp.MustCompile(
	`(?s)\n+You are receiving this email because you are an attendee[^\n]*\n.*`,
)

// reGCalInvitationFrom matches the "Invitation from Google Calendar" line
// that remains after the attendee block is stripped.
var reGCalInvitationFrom = regexp.MustCompile(
	`\n*Invitation from \[Google Calendar\]\([^)]+\)\s*$`,
)

// stripGCalFooter removes the Google Calendar attendee disclaimer and the
// trailing "Invitation from Google Calendar" attribution line.
func stripGCalFooter(text string) string {
	text = reGCalAttendeeLine.ReplaceAllString(text, "")
	text = reGCalInvitationFrom.ReplaceAllString(text, "")
	return strings.TrimRight(text, "\n ")
}

// reGroupsIoSep matches the Groups.io visual separator line that precedes
// the list-management footer.
var reGroupsIoSep = regexp.MustCompile(
	`(?s)\n+\\_\.\\_,\\_\.\\_,_\n.*`,
)

// reGroupsIoReceive matches the "You receive all messages sent to this
// group." boilerplate, plus everything that follows it to the end.
var reGroupsIoReceive = regexp.MustCompile(
	`(?s)\n*You receive all messages sent to this group\..*`,
)

// stripGroupsIoFooter removes the Groups.io list-management footer.
func stripGroupsIoFooter(text string) string {
	text = reGroupsIoSep.ReplaceAllString(text, "")
	text = reGroupsIoReceive.ReplaceAllString(text, "")
	return strings.TrimRight(text, "\n ")
}

// reViewInBrowser matches a "View in browser" or "View online" markdown
// link that newsletters include at the top as a fallback for email clients
// that cannot render HTML.
var reViewInBrowser = regexp.MustCompile(
	`(?i)\[View (in browser|online|this email( in your browser)?|on the web)\]\([^)]+\)\s*\n*`,
)

// stripViewInBrowser removes "View in browser" boilerplate links.
func stripViewInBrowser(text string) string {
	return strings.TrimLeft(reViewInBrowser.ReplaceAllString(text, ""), "\n ")
}

// reSelfLink matches a markdown link whose display text is exactly equal
// to its href. The two capture groups hold text and href respectively.
var reSelfLink = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// stripSelfLinks converts `[https://example.com](https://example.com)` to
// `https://example.com`. These arise when the html-to-markdown converter
// wraps a plain URL anchor whose text node equals the href.
func stripSelfLinks(text string) string {
	return reSelfLink.ReplaceAllStringFunc(text, func(m string) string {
		sub := reSelfLink.FindStringSubmatch(m)
		if len(sub) == 3 && sub[1] == sub[2] {
			return sub[1]
		}
		return m
	})
}
