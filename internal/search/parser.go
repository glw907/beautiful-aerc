// Package search parses poplar's search query language and exposes
// the cache-side search query shape. The grammar is small: bare
// terms, double-quoted phrases, and a fixed operator set
// (from:/to:/cc:/subject:/in:/has:attachment). Unknown key:value
// tokens fall through as bare terms. Operator typos shouldn't
// silently shrink the result set.
package search

import "strings"

// Query is the parsed query. Zero value matches everything (caller
// decides; this package doesn't run searches).
type Query struct {
	// Terms are bare words and quoted phrases not bound to a field.
	// The cache-side builder MATCHes these against subject + body +
	// from + to.
	Terms []string

	// Field-scoped terms. Empty slice = no constraint on that field.
	From    []string
	To      []string
	Cc      []string
	Subject []string

	// In names a folder. Empty = no folder filter.
	In []string

	// HasAttachment requires the message to carry at least one
	// attachment row.
	HasAttachment bool
}

// IsZero reports whether q has no constraints. Useful for the empty-
// query short-circuit on the caller side.
func (q Query) IsZero() bool {
	return len(q.Terms) == 0 && len(q.From) == 0 && len(q.To) == 0 &&
		len(q.Cc) == 0 && len(q.Subject) == 0 && len(q.In) == 0 &&
		!q.HasAttachment
}

// Parse turns input into a Query. Whitespace separates tokens;
// double quotes group a phrase. Operators are case-insensitive on
// the key side; values keep their case for the FTS5 matcher to
// fold per its tokenizer.
func Parse(input string) Query {
	var q Query
	for _, tok := range tokenize(input) {
		key, val, ok := splitOperator(tok)
		if !ok {
			q.Terms = append(q.Terms, tok)
			continue
		}
		switch key {
		case "from":
			q.From = append(q.From, val)
		case "to":
			q.To = append(q.To, val)
		case "cc":
			q.Cc = append(q.Cc, val)
		case "subject":
			q.Subject = append(q.Subject, val)
		case "in":
			q.In = append(q.In, val)
		case "has":
			if strings.EqualFold(val, "attachment") || strings.EqualFold(val, "attachments") {
				q.HasAttachment = true
			} else {
				q.Terms = append(q.Terms, tok)
			}
		default:
			q.Terms = append(q.Terms, tok)
		}
	}
	return q
}

// tokenize splits input on whitespace, honoring double quotes.
// Empty tokens drop out.
func tokenize(input string) []string {
	var out []string
	var cur strings.Builder
	inQuote := false
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		out = append(out, cur.String())
		cur.Reset()
	}
	for _, r := range input {
		switch {
		case r == '"':
			inQuote = !inQuote
		case !inQuote && (r == ' ' || r == '\t' || r == '\n'):
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

// splitOperator returns (key, value, true) when tok is a recognized
// "key:value" form. Both halves must be non-empty; the key is
// lowercased.
func splitOperator(tok string) (key, val string, ok bool) {
	colon := strings.IndexByte(tok, ':')
	if colon <= 0 || colon == len(tok)-1 {
		return "", "", false
	}
	return strings.ToLower(tok[:colon]), tok[colon+1:], true
}
