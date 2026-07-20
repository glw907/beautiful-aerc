// Package spikerender is the iterated rendering arm for the renderspike
// tool. It layers go-readability preprocessing on top of the legacy
// filter pipeline and email-specific noise rules developed across rounds
// of comparison against the hand-authored ideal renders.
package spikerender

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
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

// MsgHeaders carries email header fields used by the empty-body synthesis rule (R9).
// Both fields are display-form values decoded from MIME headers.
type MsgHeaders struct {
	Subject     string
	FromDisplay string
}

// Render converts email body content to clean markdown via the iterated
// pipeline. HTML follows: readability extraction, legacy html-to-markdown
// filter, then email-specific noise rules. Plain text uses CleanPlain directly.
// hdr is used for R9 empty-body synthesis when the rendered output is blank.
func Render(content []byte, contentType string, hdr MsgHeaders) string {
	if contentType != "text/html" {
		return filter.CleanPlain(string(content))
	}
	src := string(content)
	src = stripHiddenPreheaders(src)
	src = repairWordSplits(src)
	src = promoteStyledHeadings(src)
	ctas := collectButtonCTAs(src)
	src = promoteButtonLinks(src)
	extracted := extractReadable(src)
	extracted = reinjectMissingCTAs(extracted, ctas)
	md := filter.CleanHTML(extracted)
	md = stripGitHubTracking(md)
	md = stripGitHubFooter(md)
	md = stripGCalFooter(md)
	md = stripGroupsIoFooter(md)
	md = stripViewInBrowser(md)
	md = stripTrailingBoilerplate(md)
	md = stripSponsorBlocks(md)
	md = reconstructGitHubJobsTable(md)
	md = unwrapRedirectLinks(md)
	md = stripTrackingParams(md)
	md = collapseDuplicateLinks(md)
	md = stripSelfLinks(md)
	md = dropImageAltResidues(md)
	md = mergeKeyValueParagraphs(md)
	md = stripNavLinkWall(md)
	md = stripLegalDisclaimers(md)
	md = formatReplyForwardBlock(md)
	md = renderGCalTemplate(md, hdr)
	md = fixSerializerArtifacts(md)
	if strings.TrimSpace(md) == "" && hdr.Subject != "" {
		return synthesizeFromHeaders(hdr.Subject, hdr.FromDisplay)
	}
	return md
}

// extractReadable runs go-readability on src HTML and returns the extracted
// article HTML when the result is both long enough and represents a
// substantial share of the original text content. Short results, errors,
// content that represents less than readabilityMinRatio of the source text,
// HTML that contains <blockquote> elements, and HTML that contains embedded
// reply or forward attribution headers all fall back to src.
//
// The ratio guard prevents readability from replacing structured emails
// (receipts, calendar invites) with a single extracted prose block. The
// blockquote guard prevents readability from anchoring on the quoted
// history in threaded replies. The forward/reply header guard prevents
// readability from stripping the attribution block and lead sentence.
func extractReadable(src string) string {
	if strings.Contains(src, "<blockquote") {
		return src
	}
	if containsForwardReplyHeader(src) {
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

// reGitHubThreadFooter matches the GitHub thread-reply footer: "Reply to this
// email directly ... Message ID:". R3 caught "You are receiving this because"
// but missed this shape which appears in human issue-thread replies.
var reGitHubThreadFooter = regexp.MustCompile(
	`(?s)\n*—?\s*Reply to this email directly[^\n]*\n.*Message ID:[^\n]*`,
)

// stripGitHubFooter removes the GitHub notification footer and corporate
// address line that appear in every GitHub Actions and notification email.
func stripGitHubFooter(text string) string {
	text = reGitHubNotifBlock.ReplaceAllString(text, "")
	text = reGitHubNotifLine.ReplaceAllString(text, "")
	text = reGitHubThreadFooter.ReplaceAllString(text, "")
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
// link, including when it appears inside an H1/H2 heading, that newsletters
// include at the top of their HTML as a fallback for email clients.
// The "in (?:\w+ )?browser" alternative handles "in browser", "in web browser",
// and "in your browser" variants.
var reViewInBrowser = regexp.MustCompile(
	`(?im)^(?:#{1,6}\s*)?\[View (in (?:\w+ )?browser|online|this email(?: in your browser)?|on the web)\]\([^)]+\)\s*$`,
)

// reLeadingHR matches a horizontal rule at the very start of a string.
var reLeadingHR = regexp.MustCompile(`^(\*\s*\*\s*\*|---+)\s*\n+`)

// stripViewInBrowser removes "View in browser" boilerplate links including
// those that have been promoted to headings by the HTML converter. Any
// horizontal rule at the start of the remaining content is also removed,
// since newsletters often use a separator between the chrome and the body.
func stripViewInBrowser(text string) string {
	hadLink := reViewInBrowser.MatchString(text)
	text = reViewInBrowser.ReplaceAllString(text, "")
	text = reEmptyHeading.ReplaceAllString(text, "")
	text = strings.TrimLeft(strings.TrimSpace(text), "\n ")
	if hadLink {
		text = reLeadingHR.ReplaceAllString(text, "")
		text = strings.TrimLeft(strings.TrimSpace(text), "\n ")
	}
	return text
}

// reEmptyHeading matches a heading marker with nothing after it.
var reEmptyHeading = regexp.MustCompile(`(?m)^#{1,6}\s*$`)

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

// R8: Readability guard for embedded reply/forward headers.

// reForwardReplyInHTML detects reply or forward attribution headers in raw
// HTML. The Gmail variant uses the "---------- Forwarded message" text
// inside a div; the Outlook variant uses bold <b>From:</b> field labels
// followed within 200 characters by <b>Date:</b> or <b>Sent:</b>.
var reForwardReplyInHTML = regexp.MustCompile(
	`(?is)(-{3,}\s*Forwarded message|<b>From:\s*</b>.{0,200}<b>(?:Date|Sent):\s*</b>)`,
)

// containsForwardReplyHeader reports whether src HTML contains an embedded
// Outlook or Gmail reply/forward attribution header.
func containsForwardReplyHeader(src string) bool {
	return reForwardReplyInHTML.MatchString(src)
}

// reGmailFwdMarker matches the Gmail forwarded-message horizontal rule.
var reGmailFwdMarker = regexp.MustCompile(`(?i)-{3,}\s*Forwarded message\s*-{3,}`)

// reOutlookHdrLine detects an Outlook/mobile inline reply attribution paragraph:
// **From:** ... **Date:** or **Sent:** on the same line.
var reOutlookHdrLine = regexp.MustCompile(`\*\*From:\*\*[^\n]+\*\*(?:Date:|Sent:)\*\*`)

// reOutlookFieldSplit splits an Outlook inline attribution line at field
// boundaries. The bold markers before field names (except From) are the
// split points.
var reOutlookFieldSplit = regexp.MustCompile(`\s+\*\*(Date|Sent|To|Cc|CC|Subject|BCC|Bcc):\*\*`)

// reGmailFieldSplit splits a Gmail attribution line at plain-text field
// boundaries. Field names in Gmail attribution are not bold.
var reGmailFieldSplit = regexp.MustCompile(`\s+(Date|Sent|To|Cc|CC|Subject|BCC|Bcc):\s+`)

// formatReplyForwardBlock detects embedded Outlook or Gmail reply/forward
// attribution blocks in the markdown and reformats them as blockquotes with
// bolded header lines. Everything from the attribution block to the end of
// the document becomes a blockquote.
func formatReplyForwardBlock(text string) string {
	fwdLoc := reGmailFwdMarker.FindStringIndex(text)
	outlookLoc := reOutlookHdrLine.FindStringIndex(text)

	if fwdLoc == nil && outlookLoc == nil {
		return text
	}

	var startPos int
	var isGmail bool
	switch {
	case fwdLoc != nil && (outlookLoc == nil || fwdLoc[0] <= outlookLoc[0]):
		startPos = fwdLoc[0]
		isGmail = true
	default:
		startPos = outlookLoc[0]
	}

	head := strings.TrimRight(text[:startPos], " \n")
	tail := text[startPos:]

	blocks := strings.Split(tail, "\n\n")
	if len(blocks) == 0 {
		return text
	}

	attrBlock := blocks[0]
	bodyBlocks := blocks[1:]

	var attrLines []string
	if isGmail {
		attrLines = splitGmailAttr(attrBlock)
	} else {
		attrLines = splitOutlookAttr(attrBlock)
	}

	var sb strings.Builder
	for _, line := range attrLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		sb.WriteString("> ")
		sb.WriteString(strings.TrimSpace(line))
		sb.WriteString("\n")
	}

	for _, block := range bodyBlocks {
		if strings.TrimSpace(block) == "" {
			sb.WriteString(">\n")
			continue
		}
		sb.WriteString(">\n")
		for _, line := range strings.Split(strings.TrimRight(block, "\n"), "\n") {
			sb.WriteString("> ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}

	quoted := strings.TrimRight(sb.String(), "\n>")
	if head == "" {
		return quoted
	}
	return head + "\n\n" + quoted
}

// splitGmailAttr splits a Gmail attribution block (after the "Forwarded
// message" marker) into individual header lines with bold field names.
func splitGmailAttr(block string) []string {
	// Remove the forwarded message marker
	block = reGmailFwdMarker.ReplaceAllString(block, "")
	block = strings.TrimSpace(block)
	if block == "" {
		return nil
	}

	// Locate all field name positions in the attribution text.
	// Gmail attribution uses plain "From: X Date: Y Subject: Z To: W" format.
	re := regexp.MustCompile(`\b(From|Date|Sent|To|Cc|CC|Subject|BCC|Bcc):\s*`)
	locs := re.FindAllStringIndex(block, -1)
	if len(locs) == 0 {
		return []string{block}
	}

	var lines []string
	for i, loc := range locs {
		end := len(block)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		field := strings.TrimSpace(block[loc[0]:end])
		// Convert "FieldName: value" to "**FieldName:** value"
		colonIdx := strings.Index(field, ":")
		if colonIdx > 0 {
			name := field[:colonIdx]
			val := strings.TrimSpace(field[colonIdx+1:])
			lines = append(lines, fmt.Sprintf("**%s:** %s", name, val))
		}
	}
	return lines
}

// splitOutlookAttr splits an Outlook inline attribution paragraph into
// individual header lines. The input already uses **bold:** field markers.
func splitOutlookAttr(block string) []string {
	// Split at bold field name boundaries (Date, To, Subject, etc.)
	// keeping the **FieldName:** prefix with each part.
	parts := reOutlookFieldSplit.Split(block, -1)
	names := reOutlookFieldSplit.FindAllStringSubmatch(block, -1)

	if len(parts) == 0 {
		return []string{block}
	}

	// parts[0] is the "**From:** value" portion (before any other field).
	// names[i] is the field name for parts[i+1].
	lines := make([]string, 0, len(parts))
	if v := strings.TrimSpace(parts[0]); v != "" {
		lines = append(lines, v)
	}
	for i, name := range names {
		if i+1 < len(parts) {
			val := strings.TrimSpace(parts[i+1])
			if val != "" {
				lines = append(lines, fmt.Sprintf("**%s:** %s", name[1], val))
			}
		}
	}
	return lines
}

// R9: Empty-body header synthesis.

// reCalendarAccept matches Accepted/Declined/Tentative calendar reply subjects.
// Capture group 1: disposition word. Capture group 2: event title.
var reCalendarAccept = regexp.MustCompile(
	`(?i)^(Accepted|Declined|Tentative(?:ly accepted)?):\s*(.+)$`,
)

// synthesizeFromHeaders returns a human-readable summary synthesized from
// email headers when the body rendered empty. For calendar acceptance replies
// it emits a natural sentence; for all other subjects it emits the subject
// as a single line.
func synthesizeFromHeaders(subject, fromDisplay string) string {
	if m := reCalendarAccept.FindStringSubmatch(subject); m != nil {
		disposition := strings.ToLower(m[1])
		event := m[2]
		var verb string
		switch {
		case strings.HasPrefix(disposition, "accepted"):
			verb = "has accepted"
		case strings.HasPrefix(disposition, "declined"):
			verb = "has declined"
		default:
			verb = "has tentatively accepted"
		}
		sender := fromDisplay
		if sender == "" {
			sender = "The sender"
		}
		return fmt.Sprintf("%s %s the invitation to **%s**.\n\n*(No message included.)*", sender, verb, event)
	}
	out := subject
	if fromDisplay != "" {
		out = subject
	}
	return out + "\n\n*(No message included.)*"
}

// R10: Generalized trailing-boilerplate stripper.

// boilerplatePatterns holds the set of regexes matched against each block
// (working from the bottom of the document) to identify trailing boilerplate.
var boilerplatePatterns = []*regexp.Regexp{
	// Unsubscribe / opt-out / manage-preferences links (as the primary content
	// of a block, often the only line).
	regexp.MustCompile(`(?i)\[(?:Unsubscribe|Opt.?out|Manage (?:preferences|notifications|subscriptions)|Remove me)\]`),
	// Mailing-list membership notices.
	regexp.MustCompile(`(?i)You (?:have received|are receiving) this (?:email|message) (?:because|from the mailing list)`),
	// "Was this email forwarded to you" recruitment line.
	regexp.MustCompile(`(?i)Was this email forwarded to you`),
	// Social-follow and social-find lines.
	regexp.MustCompile(`(?i)^\[?Follow (?:us |[A-Za-z]+ )?on `),
	regexp.MustCompile(`(?i)^Find us on social`),
	// "Please add X to your address book" deliverability notice.
	regexp.MustCompile(`(?i)^Please add .+ to your address book`),
	// Copyright lines: "© YYYY" or "© YYYY-YYYY", with or without "All Rights Reserved".
	regexp.MustCompile(`(?i)©\s*\d{4}[^\n]*All Rights Reserved`),
	regexp.MustCompile(`(?i)©\s*\d{4}-\d{4}\s+[A-Za-z]`),
	// Privacy-policy standalone line.
	regexp.MustCompile(`(?i)\[Privacy Policy\]|\bprivacy policy\b.*\[`),
	// Postal address: single line ending with a country name or US state+zip.
	// The pattern anchors on the terminal token rather than trying to parse the
	// full address structure, which varies widely across senders.
	regexp.MustCompile(`(?i)^[^\n]{5,120}[,\s](?:USA?|Canada|United States?|UK|United Kingdom|Australia)\s*$`),
	regexp.MustCompile(`(?i)^[^\n]{5,80},\s*[A-Z]{2}\s+\d{5}(?:-\d{4})?\s*$`),
	// GitHub thread footer: "Reply to this email directly ... Message ID:"
	regexp.MustCompile(`(?i)Reply to this email directly`),
	// "You are subscribed to a list" or similar list-management lines.
	regexp.MustCompile(`(?i)^you (?:are|were) (?:subscribed|added) to`),
	// Horizontal rule separators left behind after boilerplate stripping.
	regexp.MustCompile(`^\*\s*\*\s*\*\s*$`),
	regexp.MustCompile(`^---+\s*$`),
}

// isBoilerplateBlock reports whether a markdown block matches any of the
// trailing-boilerplate patterns. Blocks over 500 characters are never treated
// as boilerplate: they contain actionable content (calendar invites, news
// digests) that merely happen to embed a footer notice in the same paragraph.
func isBoilerplateBlock(block string) bool {
	trimmed := strings.TrimSpace(block)
	if len(trimmed) > 500 {
		return false
	}
	for _, re := range boilerplatePatterns {
		if re.MatchString(trimmed) {
			return true
		}
	}
	return false
}

// stripTrailingBoilerplate removes contiguous trailing blocks that match
// known boilerplate patterns (unsubscribe links, mailing-list notices,
// social-follow runs, copyright lines, postal-address lines, and the GitHub
// thread-reply footer). Stripping stops at the first block that does not match.
func stripTrailingBoilerplate(text string) string {
	blocks := strings.Split(text, "\n\n")
	end := len(blocks)
	for end > 0 && isBoilerplateBlock(blocks[end-1]) {
		end--
	}
	if end == len(blocks) {
		return text
	}
	result := strings.Join(blocks[:end], "\n\n")
	return strings.TrimRight(result, "\n ")
}

// R11: Generic tracking-query-param strip.

// trackingParams is the set of query parameter names known to be used purely
// for email campaign tracking. Legitimate query parameters are preserved.
var trackingParams = map[string]bool{
	"utm_source":       true,
	"utm_medium":       true,
	"utm_campaign":     true,
	"utm_term":         true,
	"utm_content":      true,
	"mkevt":            true,
	"mkpid":            true,
	"mkcid":            true,
	"emsid":            true,
	"osub":             true,
	"segname":          true,
	"crd":              true,
	"trkId":            true,
	"cnvId":            true,
	"plmtId":           true,
	"mesgId":           true,
	"bu":               true,
	"hs_email_open_id": true,
	"_hsmi":            true,
	"_hsenc":           true,
	// ch and c appear in Constant Contact and eBay redirect URLs.
	"ch": true,
	"c":  true,
	// stream and axios_adlink appear in Axios newsletter links.
	"stream":       true,
	"axios_adlink": true,
	// hs appears in Google Calendar Meet join links.
	"hs": true,
	// LinkedIn email-notification tracking params.
	"lipi":     true,
	"midToken": true,
	"midSig":   true,
	"trk":      true,
	"trkEmail": true,
	"eid":      true,
	"otpToken": true,
}

// reMdLinkForStrip matches markdown links for tracking param stripping.
var reMdLinkForStrip = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)

// stripTrackingParams removes tracking query parameters from markdown link
// URLs. Parameters in trackingParams and any parameter whose name starts
// with "utm_" are removed. The URL is otherwise preserved including any
// non-tracking parameters.
func stripTrackingParams(text string) string {
	return reMdLinkForStrip.ReplaceAllStringFunc(text, func(m string) string {
		sub := reMdLinkForStrip.FindStringSubmatch(m)
		if len(sub) != 3 {
			return m
		}
		linkText, rawURL := sub[1], sub[2]
		u, err := url.Parse(rawURL)
		if err != nil || u.RawQuery == "" {
			return m
		}
		q := u.Query()
		changed := false
		for k := range q {
			if trackingParams[k] || strings.HasPrefix(k, "utm_") {
				q.Del(k)
				changed = true
			}
		}
		if !changed {
			return m
		}
		u.RawQuery = q.Encode()
		if u.RawQuery == "" {
			u.RawQuery = ""
		}
		return fmt.Sprintf("[%s](%s)", linkText, u.String())
	})
}

// R12: Key-value paragraph merge.

// reBoldLabelOnly matches a block that contains only a bold label ending with
// a colon (possibly with trailing whitespace).
var reBoldLabelOnly = regexp.MustCompile(`^\*\*([^*]+):\*\*\s*$`)

// addressLabels is the ordered set of address field labels that get collapsed
// into a single "**Address:** street, city, state zip, country" line.
var addressLabels = []string{"Address", "Street Address", "City", "State", "State/Province", "Province", "ZIP", "ZIP Code", "Postal Code", "Country"}

// isAddressLabel reports whether label is an address-family field.
func isAddressLabel(label string) bool {
	for _, a := range addressLabels {
		if strings.EqualFold(label, a) {
			return true
		}
	}
	return false
}

// mergeKeyValueParagraphs collapses `**Label:**\n\nvalue` paragraph pairs
// into `**Label:** value` single blocks. A run of address-family labels
// (Address, City, ZIP, State, Country) is further collapsed into one
// "**Address:** street, city, state zip, country" line.
func mergeKeyValueParagraphs(text string) string {
	blocks := strings.Split(text, "\n\n")
	out := make([]string, 0, len(blocks))
	i := 0
	for i < len(blocks) {
		m := reBoldLabelOnly.FindStringSubmatch(strings.TrimSpace(blocks[i]))
		if m == nil || i+1 >= len(blocks) {
			out = append(out, blocks[i])
			i++
			continue
		}
		label := m[1]
		value := strings.TrimSpace(blocks[i+1])
		// Only merge if the value is short and plain (no heading, list, table).
		if isShortPlainValue(value) {
			// Collect a run of address-family pairs.
			if isAddressLabel(label) {
				addrPairs := [][2]string{{label, value}}
				j := i + 2
				for j+1 < len(blocks) {
					nm := reBoldLabelOnly.FindStringSubmatch(strings.TrimSpace(blocks[j]))
					if nm == nil || !isAddressLabel(nm[1]) {
						break
					}
					nv := strings.TrimSpace(blocks[j+1])
					if !isShortPlainValue(nv) {
						break
					}
					addrPairs = append(addrPairs, [2]string{nm[1], nv})
					j += 2
				}
				if len(addrPairs) >= 2 {
					out = append(out, collapseAddress(addrPairs))
					i = j
					continue
				}
			}
			out = append(out, fmt.Sprintf("**%s:** %s", label, value))
			i += 2
			continue
		}
		out = append(out, blocks[i])
		i++
	}
	return strings.Join(out, "\n\n")
}

// isShortPlainValue reports whether value is a short plain-text value
// suitable for inlining after a bold label. Headings, list items, tables,
// code blocks, and multi-line blocks are excluded.
func isShortPlainValue(value string) bool {
	if strings.Contains(value, "\n") || len(value) > 80 {
		return false
	}
	if value == "" {
		return false
	}
	if value[0] == '#' || value[0] == '>' || value[0] == '|' ||
		value[0] == '-' || value[0] == '*' || value[0] == '+' ||
		strings.HasPrefix(value, "```") {
		return false
	}
	return true
}

// collapseAddress merges a run of address-family label-value pairs into a
// single "**Address:** street, city, state zip, country" line.
func collapseAddress(pairs [][2]string) string {
	// Build a map for lookup.
	byLabel := make(map[string]string, len(pairs))
	for _, p := range pairs {
		byLabel[strings.ToLower(p[0])] = p[1]
	}

	get := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := byLabel[strings.ToLower(k)]; ok && v != "" {
				return v
			}
		}
		return ""
	}

	var parts []string
	if v := get("address", "street address"); v != "" {
		parts = append(parts, v)
	}
	if v := get("city"); v != "" {
		parts = append(parts, v)
	}
	// State and ZIP are often on the same segment; combine them if both exist.
	state := get("state", "state/province", "province")
	zip := get("zip", "zip code", "postal code")
	if state != "" && zip != "" {
		parts = append(parts, state+" "+zip)
	} else if state != "" {
		parts = append(parts, state)
	} else if zip != "" {
		parts = append(parts, zip)
	}
	if v := get("country"); v != "" {
		parts = append(parts, v)
	}
	if len(parts) == 0 {
		// Fall back: join all values in order.
		vals := make([]string, len(pairs))
		for i, p := range pairs {
			vals[i] = p[1]
		}
		parts = vals
	}
	return "**Address:** " + strings.Join(parts, ", ")
}

// R13(c): Nav link wall strip.

// reShortMdLink matches a single short markdown link (display text under 20 chars).
var reShortMdLink = regexp.MustCompile(`\[[^\]]{1,19}\]\([^)]+\)`)

// reNavLinkBlock matches a block consisting solely of 4 or more short markdown
// links separated by spaces, optionally with mid-dots or pipes between them.
var reNavLinkBlock = regexp.MustCompile(
	`^(?:\[[^\]]{1,19}\]\([^)]+\)[\s·|]*){4,}$`,
)

// stripNavLinkWall removes blocks that consist of 4 or more consecutive short
// navigation links (display text under 20 characters). These are nav menus
// (MEN/SHIRTS/TAILORED etc.) that appear in marketing email headers.
func stripNavLinkWall(text string) string {
	blocks := strings.Split(text, "\n\n")
	out := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if reNavLinkBlock.MatchString(strings.TrimSpace(block)) {
			continue
		}
		out = append(out, block)
	}
	return strings.Join(out, "\n\n")
}

// R14: inline word-split repair.

// reWordSplitBoundary matches adjacent close/open inline tag pairs where
// both surrounding characters are word characters. The filter's
// inlineBoundaryPad spaces every inline tag boundary, so adjacent spans
// with no source whitespace ("S</span><span>unday") would gain a spurious
// space even after removing just the close+open pair. Removing both tags
// and keeping the surrounding word characters eliminates the injection site.
var reWordSplitBoundary = regexp.MustCompile(
	`(?i)(\w)</(span|strong|b|em|i|u)><(?:span|strong|b|em|i|u)(?:\s[^>]*)?>(\w)`,
)

// repairWordSplits collapses adjacent inline close+open tag pairs at
// word-character boundaries before the filter pipeline runs. Both tags are
// removed so inlineBoundaryPad has no tag boundary to space around.
// Runs until stable to handle chains of adjacent splits.
func repairWordSplits(src string) string {
	for {
		next := reWordSplitBoundary.ReplaceAllString(src, "${1}${3}")
		if next == src {
			return src
		}
		src = next
	}
}

// R15: image-alt residue drop.

// reImageAltLink matches a markdown link that occupies its own line,
// with an optional blockquote prefix.
var reImageAltLink = regexp.MustCompile(`(?im)^\[([^\]]+)\]\([^)]+\)\s*$`)

// reAltTextPattern identifies link display texts that are image alt text.
// The "taking a" alternative catches scene-description alt texts such as
// "Brand Girls taking a selfie with mountains behind them" that marketing
// emails use for lifestyle photography.
var reAltTextPattern = regexp.MustCompile(
	`(?i)(?:^image of |^a (?:woman|man|person)\b|^(?:photo|illustration)\s+of\b|\blogo\s*$|^github$|\btaking a\b)`,
)

// reCreditLine matches standalone attribution credit lines
// (e.g. "Illustration: Aida Amer/Axios").
var reCreditLine = regexp.MustCompile(
	`(?im)^(?:>\s*)*(?:photo|illustration|data|chart|image)\s*:[^\n]+$`,
)

// reIllustrationCaption matches standalone plain-text image captions
// that start with "Image of" or "Illustration of".
var reIllustrationCaption = regexp.MustCompile(
	`(?im)^(?:>\s*)*(?:image of |illustration of )[^\n]+$`,
)

// reAIDisclaimer matches Microsoft Outlook's AI-generated content notice.
var reAIDisclaimer = regexp.MustCompile(
	`(?im)^(?:>\s*)*ai-generated content may be incorrect\.\s*$`,
)

// reKnownLogoLine matches standalone platform logo alt-text lines.
// These appear at the top of GitHub notification emails and similar SaaS
// transactional emails where the logo image is not wrapped in an anchor.
var reKnownLogoLine = regexp.MustCompile(
	`(?im)^(?:>\s*)*(?:github|mimestream)\s*$`,
)

// reExcessBlanks collapses three or more consecutive newlines.
var reExcessBlanks = regexp.MustCompile(`\n{3,}`)

// dropImageAltResidues removes standalone image alt-text content from
// rendered markdown. The html-to-markdown converter renders img alt
// attributes as visible text; this function strips the resulting noise:
// logo names, "Image of X" links, illustration captions, credit lines,
// and AI-generated content disclaimers.
func dropImageAltResidues(text string) string {
	text = reCreditLine.ReplaceAllString(text, "")
	text = reIllustrationCaption.ReplaceAllString(text, "")
	text = reAIDisclaimer.ReplaceAllString(text, "")
	text = reKnownLogoLine.ReplaceAllString(text, "")
	text = reImageAltLink.ReplaceAllStringFunc(text, func(m string) string {
		sub := reImageAltLink.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		if reAltTextPattern.MatchString(strings.TrimSpace(sub[1])) {
			return ""
		}
		return m
	})
	text = reExcessBlanks.ReplaceAllString(text, "\n\n")
	return strings.TrimRight(text, "\n ")
}

// R16: hidden preheader/preview-text drop.

// rePreheaderClassID matches opening tags of div/span/p/table elements whose
// id or class attribute value contains a preheader/snippet/preview keyword.
// These elements carry email preview text that should not appear in the body.
var rePreheaderClassID = regexp.MustCompile(
	`(?i)<(div|span|p|table)[^>]*(?:id|class)\s*=\s*["'][^"']*(?:preheader|snippet-container|preview)[^"']*["'][^>]*>`,
)

// reHiddenInlineOpen matches opening span or p tags styled invisible via
// display:none, max-height:0, opacity:0, or font-size:0/1px.
var reHiddenInlineOpen = regexp.MustCompile(
	`(?i)<(span|p)[^>]*\bstyle\s*=\s*"[^"]*(?:display\s*:\s*none|max-height\s*:\s*0|opacity\s*:\s*0|font-size\s*:\s*[01]px)[^"]*"[^>]*>`,
)

// reSmallHeading matches a complete h1-h6 element with an inline font-size
// style. The second capture group holds the font-size value in pixels.
var reSmallHeading = regexp.MustCompile(
	`(?is)<(h[1-6])[^>]*\bstyle\s*=\s*"[^"]*\bfont-size\s*:\s*(\d+)px[^"]*"[^>]*>.*?</h[1-6]>`,
)

// stripHiddenPreheaders removes invisible preheader/preview elements from
// raw HTML before readability extraction. Three cases: (1) elements whose
// id or class names the element as a preheader/snippet/preview container,
// (2) span/p elements with visibility-hiding inline styles, (3) heading
// elements with a cosmetically tiny font-size (<=14px) used as preheader
// text in marketing emails.
func stripHiddenPreheaders(src string) string {
	src = stripHiddenByPattern(src, rePreheaderClassID)
	src = stripHiddenByPattern(src, reHiddenInlineOpen)
	src = stripSmallHeadings(src)
	return src
}

// stripHiddenByPattern removes every element whose opening tag matches
// openRe, including its children, by depth-counting open/close pairs.
// openRe must capture the tag name in submatch group 1.
func stripHiddenByPattern(src string, openRe *regexp.Regexp) string {
	for {
		loc := openRe.FindStringSubmatchIndex(src)
		if loc == nil {
			break
		}
		if loc[2] < 0 {
			break
		}
		tagName := strings.ToLower(src[loc[2]:loc[3]])
		openTag := "<" + tagName
		closeTag := "</" + tagName + ">"

		start := loc[0]
		rest := src[loc[1]:]
		depth := 1
		pos := 0
		for depth > 0 && pos < len(rest) {
			nextOpen := strings.Index(rest[pos:], openTag)
			nextClose := strings.Index(rest[pos:], closeTag)
			if nextClose < 0 {
				pos = len(rest)
				break
			}
			if nextOpen >= 0 && nextOpen < nextClose {
				depth++
				pos += nextOpen + len(openTag)
			} else {
				depth--
				pos += nextClose + len(closeTag)
			}
		}
		end := loc[1] + pos
		if end > len(src) {
			end = len(src)
		}
		src = src[:start] + src[end:]
	}
	return src
}

// stripSmallHeadings removes heading elements (h1-h6) whose inline
// font-size is 14px or smaller. Marketing emails use such headings as
// invisible preheader text rendered at a size below the visible threshold.
func stripSmallHeadings(src string) string {
	return reSmallHeading.ReplaceAllStringFunc(src, func(m string) string {
		sub := reSmallHeading.FindStringSubmatch(m)
		if len(sub) < 3 {
			return m
		}
		size, err := strconv.Atoi(sub[2])
		if err != nil || size > 14 {
			return m
		}
		return ""
	})
}

// R17: redirect-wrapper unwrap.

// unwrapRedirectLinks replaces redirect-wrapper hrefs with their embedded
// target URLs when the target is encoded as a base64url path segment.
// Tracking parameters are stripped from the decoded URL in the same pass.
func unwrapRedirectLinks(text string) string {
	return reMdLinkForStrip.ReplaceAllStringFunc(text, func(m string) string {
		sub := reMdLinkForStrip.FindStringSubmatch(m)
		if len(sub) != 3 {
			return m
		}
		linkText, rawURL := sub[1], sub[2]
		decoded := decodeRedirectURL(rawURL)
		if decoded == "" {
			return m
		}
		u, err := url.Parse(decoded)
		if err != nil {
			return fmt.Sprintf("[%s](%s)", linkText, decoded)
		}
		q := u.Query()
		changed := false
		for k := range q {
			if trackingParams[k] || strings.HasPrefix(k, "utm_") {
				q.Del(k)
				changed = true
			}
		}
		if changed {
			u.RawQuery = q.Encode()
		}
		return fmt.Sprintf("[%s](%s)", linkText, u.String())
	})
}

// decodeRedirectURL looks for a base64url-encoded URL among the path
// segments of rawURL. It returns the decoded URL if a segment of 20 or
// more characters decodes to an http/https URL, and empty string otherwise.
func decodeRedirectURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	for _, seg := range strings.Split(u.Path, "/") {
		if len(seg) < 20 {
			continue
		}
		decoded, err := base64.RawURLEncoding.DecodeString(seg)
		if err != nil {
			continue
		}
		s := string(decoded)
		if strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://") {
			return s
		}
	}
	return ""
}

// R18: style-driven heading promotion.

// reStyleHeadTD matches a td element whose inline style includes a
// font-size in pixels, capturing the size and the text-only content.
var reStyleHeadTD = regexp.MustCompile(
	`(?i)<td[^>]*\bstyle="[^"]*\bfont-size\s*:\s*(\d+)px[^"]*"[^>]*>([^<]{3,200})</td>`,
)

// reStyleHeadSpan matches a span element whose inline style includes a
// font-size in pixels, capturing the size and the text-only content.
var reStyleHeadSpan = regexp.MustCompile(
	`(?i)<span[^>]*\bstyle="[^"]*\bfont-size\s*:\s*(\d+)px[^"]*"[^>]*>([^<]{3,200})</span>`,
)

// promoteStyledHeadings converts td and span elements with a font-size
// of 20px or larger and short plain-text content to h2 elements before
// the HTML reaches the markdown converter. The converter renders these
// as ## headings. Only elements whose text content contains no child
// HTML tags and is 120 characters or fewer are promoted.
func promoteStyledHeadings(src string) string {
	src = reStyleHeadTD.ReplaceAllStringFunc(src, func(m string) string {
		sub := reStyleHeadTD.FindStringSubmatch(m)
		if len(sub) < 3 {
			return m
		}
		size, _ := strconv.Atoi(sub[1])
		if size < 20 {
			return m
		}
		text := strings.TrimSpace(sub[2])
		if text == "" || len(text) > 120 || strings.ContainsAny(text, "<>") {
			return m
		}
		return "<td><h2>" + text + "</h2></td>"
	})
	src = reStyleHeadSpan.ReplaceAllStringFunc(src, func(m string) string {
		sub := reStyleHeadSpan.FindStringSubmatch(m)
		if len(sub) < 3 {
			return m
		}
		size, _ := strconv.Atoi(sub[1])
		if size < 20 {
			return m
		}
		text := strings.TrimSpace(sub[2])
		if text == "" || len(text) > 120 || strings.ContainsAny(text, "<>") {
			return m
		}
		return "<h2>" + text + "</h2>"
	})
	return src
}

// Regression fix: Constant Contact button link promotion.

// reButtonContentCell matches a Constant Contact button table cell. The cell
// carries background-color styling that go-readability treats as decorative,
// causing the link inside to be dropped. Converting the cell to a plain
// paragraph before readability runs preserves the action link.
var reButtonContentCell = regexp.MustCompile(
	`(?is)<td[^>]*\bclass="[^"]*button_content-cell[^"]*"[^>]*>(.*?)</td>`,
)

// reButtonLinkAnchor matches the anchor inside a button content cell,
// capturing the href and link text.
var reButtonLinkAnchor = regexp.MustCompile(
	`(?is)<a[^>]*\bhref="([^"]+)"[^>]*>(.*?)</a>`,
)

// reInlineTag matches any HTML tag. Used to strip tags from button link text.
var reInlineTag = regexp.MustCompile(`<[^>]+>`)

// promoteButtonLinks converts Constant Contact button table cells to plain
// paragraph links before readability extraction. go-readability drops table
// cells whose class or style marks them as decorative buttons; promoting
// them to <p><a href="...">text</a></p> preserves the CTA action links.
func promoteButtonLinks(src string) string {
	return reButtonContentCell.ReplaceAllStringFunc(src, func(m string) string {
		inner := reButtonContentCell.FindStringSubmatch(m)
		if len(inner) < 2 {
			return m
		}
		anchors := reButtonLinkAnchor.FindAllStringSubmatch(inner[1], -1)
		if len(anchors) == 0 {
			return m
		}
		var sb strings.Builder
		for _, a := range anchors {
			href := a[1]
			text := reInlineTag.ReplaceAllString(a[2], "")
			text = strings.TrimSpace(text)
			if text == "" || href == "" {
				continue
			}
			sb.WriteString(`<p><a href="`)
			sb.WriteString(href)
			sb.WriteString(`">`)
			sb.WriteString(text)
			sb.WriteString("</a></p>")
		}
		if sb.Len() == 0 {
			return m
		}
		return sb.String()
	})
}

// collectButtonCTAs returns (href, text) pairs for all button_content-cell
// anchors in src. Called before promoteButtonLinks so that reinjectMissingCTAs
// can recover any pairs that go-readability later discards.
func collectButtonCTAs(src string) [][2]string {
	var pairs [][2]string
	for _, cell := range reButtonContentCell.FindAllStringSubmatch(src, -1) {
		for _, a := range reButtonLinkAnchor.FindAllStringSubmatch(cell[1], -1) {
			href := a[1]
			text := reInlineTag.ReplaceAllString(a[2], "")
			text = strings.TrimSpace(text)
			if href != "" && text != "" {
				pairs = append(pairs, [2]string{href, text})
			}
		}
	}
	return pairs
}

// reinjectMissingCTAs appends any CTA links from pairs whose hrefs are absent
// from extracted. go-readability drops button cells even after promoteButtonLinks
// promotes them to <p><a>; this step recovers them.
func reinjectMissingCTAs(extracted string, pairs [][2]string) string {
	var missing []string
	for _, p := range pairs {
		if !strings.Contains(extracted, p[0]) {
			missing = append(missing, `<p><a href="`+p[0]+`">`+p[1]+`</a></p>`)
		}
	}
	if len(missing) == 0 {
		return extracted
	}
	return extracted + "\n" + strings.Join(missing, "\n")
}

// R21: duplicate-destination link collapse.

// normURL returns a normalized form of rawURL with known tracking parameters
// stripped and the query re-encoded in sorted key order. Fragment and scheme
// are preserved. Used to compare two links that point to the same destination.
func normURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if u.RawQuery != "" {
		q := u.Query()
		for k := range q {
			if trackingParams[k] || strings.HasPrefix(k, "utm_") {
				q.Del(k)
			}
		}
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// labelScore returns a score for a markdown link label. Higher scores win
// when collapsing duplicate-destination links: a descriptive text label beats
// a bare URL label beats a short numeric label.
func labelScore(label string) int {
	if label == "" {
		return 0
	}
	// Pure number labels (e.g. "[1]") are least informative.
	allDigits := true
	for _, r := range label {
		if r < '0' || r > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		return 1
	}
	// URL-shaped labels score below descriptive text.
	if strings.HasPrefix(label, "http://") || strings.HasPrefix(label, "https://") ||
		(strings.Contains(label, ".") && !strings.Contains(label, " ")) {
		return 2
	}
	return 3
}

// reMdLink matches any markdown link.
var reMdLink = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)

// reBareURL matches a bare http/https URL not inside a markdown link.
var reBareURL = regexp.MustCompile(`(?:^|(?:[^(]))https?://\S+`)

// collapseDuplicateLinks keeps the most informative label for links that
// share the same normalized destination URL and removes subsequent occurrences.
// Bare URLs that duplicate a nearby markdown link are also removed.
func collapseDuplicateLinks(text string) string {
	// Collect all markdown links grouped by normalized URL.
	type linkInfo struct {
		full  string
		label string
		score int
	}
	byURL := map[string][]linkInfo{}
	var order []string
	matches := reMdLink.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		label, href := m[1], m[2]
		norm := normURL(href)
		if _, seen := byURL[norm]; !seen {
			order = append(order, norm)
		}
		byURL[norm] = append(byURL[norm], linkInfo{
			full:  m[0],
			label: label,
			score: labelScore(label),
		})
	}

	// For each URL group with more than one link, pick the best label and
	// build a replacement set. For equal-score groups, subsumption guards
	// the collapse: a link is only removed when the best label contains its
	// label as a substring (case-insensitive). This prevents CTA links like
	// "Start Learning for $199" from being silently dropped in favour of an
	// unrelated image-description link that happens to share the redirect URL.
	// When subsumption does collapse a score-3 (descriptive) label, the anchor
	// text is kept in place to avoid blank gaps in prose sentences.
	replacements := map[string]string{}
	for _, norm := range order {
		group := byURL[norm]
		if len(group) < 2 {
			continue
		}
		best := group[0]
		for _, li := range group[1:] {
			switch {
			case li.score > best.score:
				best = li
			case li.score == best.score && len(li.label) > len(best.label):
				// Among equal-score labels prefer the longer one: it is more
				// informative and is more likely to subsume the shorter ones.
				best = li
			}
		}
		for _, li := range group {
			if li.full == best.full {
				continue
			}
			if li.score < best.score {
				// Clear score advantage: remove the weaker label entirely.
				replacements[li.full] = ""
				continue
			}
			// Equal score: only collapse when best's label contains li's label
			// as a substring. No subsumption means both links carry distinct
			// meaning and must be preserved.
			if strings.Contains(strings.ToLower(best.label), strings.ToLower(li.label)) {
				if li.score == 3 {
					// Preserve anchor text so mid-sentence prose stays readable.
					replacements[li.full] = li.label
				} else {
					replacements[li.full] = ""
				}
			}
		}
	}

	if len(replacements) > 0 {
		text = reMdLink.ReplaceAllStringFunc(text, func(m string) string {
			if r, ok := replacements[m]; ok {
				return r
			}
			return m
		})
	}

	// Remove bare URLs whose normalized form matches a markdown link in the text.
	knownURLs := map[string]bool{}
	for _, m := range reMdLink.FindAllStringSubmatch(text, -1) {
		knownURLs[normURL(m[2])] = true
	}
	text = reBareURL.ReplaceAllStringFunc(text, func(m string) string {
		// reBareURL may include a leading character; extract the URL part.
		urlStart := strings.Index(m, "http")
		prefix := m[:urlStart]
		rawURL := strings.TrimRight(m[urlStart:], ",;.!?)")
		if knownURLs[normURL(rawURL)] {
			return prefix
		}
		return m
	})

	text = reExcessBlanks.ReplaceAllString(text, "\n\n")
	return strings.TrimRight(text, "\n ")
}

// R22: Google Calendar template rendering.

// reGCalRespond detects a Google Calendar RESPOND action link in markdown.
var reGCalRespond = regexp.MustCompile(
	`\[[^\]]+\]\(https://calendar\.google\.com/calendar/event\?action=RESPOND[^)]*\)`,
)

// reGCalSubject extracts the event title from a Google Calendar invitation
// or updated-invitation subject line. Google Calendar subjects follow the
// pattern "Invitation: TITLE @ DATE (ATTENDEE)" or "Updated invitation: TITLE @ ...".
var reGCalSubject = regexp.MustCompile(
	`(?i)^(?:(?:Updated|Cancelled|Canceled)\s+)?[Ii]nvitation:\s*(.+?)\s*@`,
)

// reGCalDualDatetime matches two consecutive Google Calendar datetime lines
// in the format "Weekday Mon DD, YYYY ⋅ H:MMpm – H:MMpm (Timezone)".
// The first line is the new time; the second is the old time.
var reGCalDualDatetime = regexp.MustCompile(
	`((?:Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday) \w+ \d+, \d{4} ⋅[^\n]+)\s+((?:Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday) \w+ \d+, \d{4} ⋅[^\n]+)`,
)

// reGCalChangedBadge matches the standalone "CHANGED" badge text that Google
// Calendar injects into updated-invitation emails.
var reGCalChangedBadge = regexp.MustCompile(`(?m)^CHANGED\s*$`)

// reExistingHeading matches an H1 or H2 heading line.
var reExistingHeading = regexp.MustCompile(`(?m)^#{1,2}\s+\S`)

// renderGCalTemplate applies Google Calendar-specific rendering to md when
// the content looks like a calendar invitation (contains a RESPOND action
// link). It: injects the event title as an H1 from the subject header (when
// no heading is already present), drops the "CHANGED" badge text, and
// reformats dual datetime lines (new-time old-time) as "new (was: old)".
func renderGCalTemplate(md string, hdr MsgHeaders) string {
	if !reGCalRespond.MatchString(md) {
		return md
	}

	// Drop CHANGED badge.
	md = reGCalChangedBadge.ReplaceAllString(md, "")

	// Reformat dual datetimes: new\nwas-old -> "new\n\n(was: old)"
	md = reGCalDualDatetime.ReplaceAllString(md, "$1\n\n(was: $2)")

	// Inject H1 from subject when no heading is already present.
	if !reExistingHeading.MatchString(md) && hdr.Subject != "" {
		if m := reGCalSubject.FindStringSubmatch(hdr.Subject); m != nil {
			eventTitle := strings.TrimSpace(m[1])
			if eventTitle != "" {
				md = "# " + eventTitle + "\n\n" + strings.TrimLeft(md, "\n ")
			}
		}
	}

	md = reExcessBlanks.ReplaceAllString(md, "\n\n")
	return strings.TrimRight(md, "\n ")
}

// R23: sponsor block excision.

// reSponsorBlockStart matches the opening line of a sponsor advertisement
// block. Both "A MESSAGE FROM" and "PRESENTED BY" patterns are used by
// newsletter publishers to label sponsored content sections.
var reSponsorBlockStart = regexp.MustCompile(
	`(?m)^(?:A MESSAGE FROM|PRESENTED BY)\b[^\n]*$`,
)

// reSponsorBlockCTA matches a call-to-action link that ends a sponsor block.
// Publisher convention is a single-sentence CTA on its own line.
var reSponsorBlockCTA = regexp.MustCompile(
	`(?m)\[[^\]]{3,80}\]\(https?://[^)]+\)\s*$`,
)

// stripSponsorBlocks removes "A MESSAGE FROM" and "PRESENTED BY" sponsor
// advertisement blocks from newsletter markdown. Each block starts at the
// marker line and ends at the first standalone CTA link that follows it.
func stripSponsorBlocks(text string) string {
	for {
		loc := reSponsorBlockStart.FindStringIndex(text)
		if loc == nil {
			break
		}
		before := text[:loc[0]]
		after := text[loc[1]:]

		// Find the CTA link that closes this block.
		ctaLoc := reSponsorBlockCTA.FindStringIndex(after)
		if ctaLoc == nil {
			// No CTA found; remove from marker to end of next paragraph.
			nextPara := strings.Index(after, "\n\n")
			if nextPara < 0 {
				text = strings.TrimRight(before, "\n ")
				break
			}
			text = strings.TrimRight(before, "\n ") + after[nextPara:]
			continue
		}

		// Remove from marker through end of the CTA line.
		ctaEnd := ctaLoc[1]
		// Advance past the trailing newline.
		if ctaEnd < len(after) && after[ctaEnd] == '\n' {
			ctaEnd++
		}
		text = strings.TrimRight(before, "\n ") + "\n\n" + strings.TrimLeft(after[ctaEnd:], "\n ")
	}
	text = reExcessBlanks.ReplaceAllString(text, "\n\n")
	return strings.TrimRight(text, "\n ")
}

// R24: GitHub workflow-run table reconstruction.

// reGitHubJobsTableHeader matches the "Status Job Annotations" text that
// GitHub Actions emails emit as the collapsed header of their jobs table.
var reGitHubJobsTableHeader = regexp.MustCompile(
	`(?m)^Status Job Annotations\s*$`,
)

// reGitHubJobRow matches a GitHub Actions job entry: a bold job name on the
// first line followed by a result line. GitHub formats these as
// `**Name** / sub-job\nResult text`, so we capture the bold name, any
// remainder on the first line (e.g. " / ci"), and the result line separately.
var reGitHubJobRow = regexp.MustCompile(
	`(?m)^\*\*([^*]+)\*\*([^\n]*)\n([^\n]+)`,
)

// reconstructGitHubJobsTable replaces the GitHub Actions "Status Job
// Annotations" flat text dump with a markdown table. The table has three
// columns: Job, Result, and Annotations. Annotation links (if present) are
// placed in the third column; the result text goes in the second.
func reconstructGitHubJobsTable(text string) string {
	loc := reGitHubJobsTableHeader.FindStringIndex(text)
	if loc == nil {
		return text
	}

	before := text[:loc[0]]
	after := text[loc[1]:]

	// Collect job rows from the text that follows the header.
	rows := reGitHubJobRow.FindAllStringSubmatch(after, -1)
	if len(rows) == 0 {
		return text
	}

	var sb strings.Builder
	sb.WriteString("| Job | Result | Annotations |\n")
	sb.WriteString("| --- | --- | --- |\n")
	for _, row := range rows {
		// row[1] is the bold name, row[2] is the rest of the first line (e.g. " / ci"),
		// row[3] is the result/duration line.
		job := strings.TrimSpace(row[1] + row[2])
		result := strings.TrimSpace(row[3])
		sb.WriteString("| ")
		sb.WriteString(job)
		sb.WriteString(" | ")
		sb.WriteString(result)
		sb.WriteString(" | |\n")
	}

	// Remove the job rows from the after text.
	remaining := reGitHubJobRow.ReplaceAllString(after, "")
	remaining = reExcessBlanks.ReplaceAllString(remaining, "\n\n")
	remaining = strings.TrimLeft(remaining, "\n ")

	result := strings.TrimRight(before, "\n ") + "\n\n" + sb.String()
	if remaining != "" {
		result += "\n" + remaining
	}
	return strings.TrimRight(result, "\n ")
}

// R25: legal-disclaimer and signature-address drop.

// legalPatterns holds patterns that identify legal disclaimer and boilerplate
// signature blocks in quoted reply email content.
var legalPatterns = []*regexp.Regexp{
	// Confidentiality and forwarding notices.
	regexp.MustCompile(`(?i)may include confidential`),
	regexp.MustCompile(`(?i)if you are not the intended recipient`),
	regexp.MustCompile(`(?i)\bIMPORTANT NOTICE\s*:`),
	// Insurance coverage disclaimers.
	regexp.MustCompile(`(?i)Coverage may not be issued`),
	// Google Maps address links in signatures.
	regexp.MustCompile(`google\.com/maps/search/`),
	// Quoted footer separator lines "--- Organization name".
	regexp.MustCompile(`(?m)^> --- [A-Z]`),
}

// reBlockquoteEmptyLine matches an empty blockquote line used as a
// paragraph separator within a quoted block.
var reBlockquoteEmptyLine = regexp.MustCompile(`\n>[ \t]*\n`)

// stripLegalDisclaimers removes legal disclaimer paragraphs and signature
// address noise from markdown. Both regular \n\n-separated paragraphs and
// blockquote-internal paragraphs (separated by empty > lines) are processed.
// Paragraphs matching any legalPatterns entry are dropped. Blocks over 500
// characters are skipped: a large block containing a maps link or a
// confidentiality phrase is actionable content (a calendar invite body, a
// full email thread) and must not be silently erased.
func stripLegalDisclaimers(text string) string {
	// Treat empty blockquote lines as paragraph boundaries so we can
	// process blockquote sub-paragraphs alongside normal paragraphs.
	text = reBlockquoteEmptyLine.ReplaceAllString(text, "\n\n")

	blocks := strings.Split(text, "\n\n")
	out := make([]string, 0, len(blocks))
	for _, block := range blocks {
		trimmed := strings.TrimSpace(block)
		if len(trimmed) > 500 {
			out = append(out, block)
			continue
		}
		drop := false
		for _, re := range legalPatterns {
			if re.MatchString(trimmed) {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, block)
		}
	}
	result := strings.Join(out, "\n\n")
	result = reExcessBlanks.ReplaceAllString(result, "\n\n")
	return strings.TrimRight(result, "\n ")
}

// R19: serializer cleanup.

// reSpaceBeforePunct matches a closing markdown character (backtick,
// closing paren, double-star, or word-char+star) followed by a single
// space and then a punctuation character. The space is an artifact from
// inlineBoundaryPad in the html filter.
var reSpaceBeforePunct = regexp.MustCompile("([\x60)]|\\*\\*|\\w\\*) ([,.;:!?])")

// reEscapeUnderscore matches an unnecessary backslash before an underscore
// between two word characters. The html-to-markdown converter escapes
// underscores to prevent markdown italic interpretation, but underscores
// inside identifiers (e.g. transaction IDs) do not need escaping.
var reEscapeUnderscore = regexp.MustCompile(`(\w)\\_(\w)`)

// reEscapeStarDigit matches a backslash-escaped asterisk before a digit.
// The converter escapes * to avoid markdown bold/italic, but *digit is
// never emphasis in practice (e.g. Visa *9111).
var reEscapeStarDigit = regexp.MustCompile(`\\\*(\d)`)

// reGlyphBullet matches a glyph bullet character (● • ·) at the start
// of a line, with an optional blockquote prefix.
var reGlyphBullet = regexp.MustCompile("(?m)^((?:> ?)*)([●•·])\\s+")

// reHeadingEscapedBracket matches a backslash-escaped `\[` or `\]` inside
// a markdown heading line. The serializer escapes brackets in heading text to
// prevent link parsing, but `[org/repo]` in a subject heading reads cleaner
// without the backslashes.
var reHeadingEscapedBracket = regexp.MustCompile(`(?m)^(#{1,6}\s+.*)\\([\[\]])`)

// fixSerializerArtifacts cleans up noise introduced by the html-to-markdown
// serializer: spaces before punctuation after inline code or link closers,
// unnecessary backslash escapes before underscores and asterisks, glyph
// bullets that should be markdown list items, and escaped brackets inside
// heading lines.
func fixSerializerArtifacts(text string) string {
	text = reSpaceBeforePunct.ReplaceAllString(text, "${1}${2}")
	text = reEscapeUnderscore.ReplaceAllString(text, "${1}_${2}")
	text = reEscapeStarDigit.ReplaceAllString(text, "*${1}")
	text = reGlyphBullet.ReplaceAllString(text, "${1}- ")
	// Unescape \[ and \] inside heading lines (run until stable for `\[a\]`).
	for {
		next := reHeadingEscapedBracket.ReplaceAllString(text, "${1}${2}")
		if next == text {
			break
		}
		text = next
	}
	return text
}
