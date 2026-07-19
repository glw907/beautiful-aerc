package main

import "strings"

// msgHeaders holds the header fields used to classify a message.
type msgHeaders struct {
	from            string
	subject         string
	listID          string
	listUnsubscribe string
	contentType     string
}

// allClasses is the ordered list of the seven defined corpus classes.
var allClasses = []string{
	"github-ci", "calendar", "list-patch",
	"transactional", "newsletter", "marketing", "personal",
}

// classify assigns one corpus class to a message from its headers.
// Returns one of the seven defined classes, or "unclassified".
func classify(h msgHeaders) string {
	from := strings.ToLower(h.from)
	subj := strings.ToLower(h.subject)
	listID := strings.ToLower(h.listID)
	ct := strings.ToLower(h.contentType)

	if isGitHubCI(from, listID) {
		return "github-ci"
	}
	if isCalendar(ct, subj) {
		return "calendar"
	}
	if listID != "" || strings.Contains(subj, "[patch") {
		return "list-patch"
	}
	if h.listUnsubscribe != "" {
		if isMarketing(subj) {
			return "marketing"
		}
		return "newsletter"
	}
	if isNoReply(from) {
		return "transactional"
	}
	if !isAutomated(from) {
		return "personal"
	}
	return "unclassified"
}

func isGitHubCI(from, listID string) bool {
	if strings.Contains(from, "@github.com") {
		return true
	}
	if strings.Contains(listID, "github.com") {
		return true
	}
	for _, d := range ciSenderPatterns {
		if strings.Contains(from, d) {
			return true
		}
	}
	return false
}

var ciSenderPatterns = []string{
	"travis-ci", "@circleci.com", "drone.io", "@buildkite.com",
	"codecov.io", "coveralls.io", "semaphoreci.com", "dependabot",
	"renovatebot.com", "@snyk.io",
}

func isCalendar(ct, subj string) bool {
	if strings.Contains(ct, "calendar") {
		return true
	}
	for _, kw := range calendarSubjectKeywords {
		if strings.Contains(subj, kw) {
			return true
		}
	}
	return false
}

var calendarSubjectKeywords = []string{
	"invitation:", "meeting invite", "you've been invited",
	"calendar invite", "calendar notification",
	"accepted:", "declined:", "tentative:",
}

var marketingKeywords = []string{
	"% off", "% discount", "sale", "deals", "save big",
	"coupon", "promo", "clearance", "flash sale", "limited time",
	"free shipping", "buy now", "shop now", "order now",
	"expires", "last chance", "hurry",
}

func isMarketing(subj string) bool {
	for _, kw := range marketingKeywords {
		if strings.Contains(subj, kw) {
			return true
		}
	}
	return false
}

var noReplyPatterns = []string{
	"noreply@", "no-reply@", "do-not-reply@", "donotreply@", "no.reply@",
}

func isNoReply(from string) bool {
	for _, p := range noReplyPatterns {
		if strings.Contains(from, p) {
			return true
		}
	}
	return false
}

// automatedPatterns is a superset of noReplyPatterns covering senders that are
// clearly machine-generated but may not use a strict "noreply" prefix.
var automatedPatterns = []string{
	"noreply@", "no-reply@", "do-not-reply@", "donotreply@", "no.reply@",
	"bounce@", "bounces@", "daemon@", "postmaster@", "mailer-daemon@",
	"notifications@", "notification@", "alerts@", "alert@",
	"system@", "automated@", "security-noreply@",
}

func isAutomated(from string) bool {
	for _, p := range automatedPatterns {
		if strings.Contains(from, p) {
			return true
		}
	}
	return false
}
