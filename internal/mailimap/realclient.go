// SPDX-License-Identifier: MIT

package mailimap

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	imap "github.com/emersion/go-imap/v2"
	imapclient "github.com/emersion/go-imap/v2/imapclient"

	"github.com/glw907/poplar/internal/mail"
)

// realClient adapts *imapclient.Client to the imapClient interface.
// The unilateralHandler field is set around each Idle call;
// auth.go wires the *imapclient.Client's UnilateralDataHandler to
// dispatch here so EXPUNGE and FETCH FLAGS updates reach the caller.
type realClient struct {
	c *imapclient.Client

	mu                sync.Mutex
	unilateralHandler func(mail.Update) // set during Idle, nil otherwise
	idleCmd           *imapclient.IdleCommand
}

// dispatch is called by the UnilateralDataHandler callbacks registered
// in auth.go. It forwards the update to whatever Idle callback is
// currently active.
func (r *realClient) dispatch(u mail.Update) {
	r.mu.Lock()
	fn := r.unilateralHandler
	r.mu.Unlock()
	if fn != nil {
		fn(u)
	}
}

// imapUID converts imap.UID (uint32) to mail.UID (decimal string).
func imapUID(u imap.UID) mail.UID {
	return mail.UID(strconv.FormatUint(uint64(u), 10))
}

// mailUIDsToSet converts a slice of mail.UID (decimal string) to an imap.UIDSet.
func mailUIDsToSet(uids []mail.UID) imap.UIDSet {
	var set imap.UIDSet
	for _, u := range uids {
		n, err := strconv.ParseUint(string(u), 10, 32)
		if err != nil || n == 0 {
			continue
		}
		set.AddNum(imap.UID(n))
	}
	return set
}

// Logout sends LOGOUT and waits for the server acknowledgement.
func (r *realClient) Logout() error {
	return r.c.Logout().Wait()
}

// Capabilities issues a CAPABILITY command and converts the go-imap v2
// CapSet (map[imap.Cap]struct{}) to the map[string]bool form the
// imapClient interface requires.
func (r *realClient) Capabilities() (map[string]bool, error) {
	caps, err := r.c.Capability().Wait()
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(caps))
	for cap := range caps {
		out[string(cap)] = true
	}
	return out, nil
}

// List issues a LIST command. When specialUse is true, LIST RETURN
// (SPECIAL-USE) is requested so role attributes arrive without a
// separate STATUS round-trip.
func (r *realClient) List(_, pattern string, specialUse bool) ([]listEntry, error) {
	var opts *imap.ListOptions
	if specialUse {
		opts = &imap.ListOptions{ReturnSpecialUse: true}
	}

	mailboxes, err := r.c.List("", pattern, opts).Collect()
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	out := make([]listEntry, 0, len(mailboxes))
	for _, m := range mailboxes {
		out = append(out, listEntry{
			Name:        m.Mailbox,
			Attributes:  attrsToStrings(m.Attrs),
			HasChildren: containsAttr(m.Attrs, imap.MailboxAttrHasChildren),
		})
	}
	return out, nil
}

// attrsToStrings converts a slice of imap.MailboxAttr to plain strings.
func attrsToStrings(attrs []imap.MailboxAttr) []string {
	out := make([]string, len(attrs))
	for i, a := range attrs {
		out[i] = string(a)
	}
	return out
}

// containsAttr reports whether attrs contains target.
func containsAttr(attrs []imap.MailboxAttr, target imap.MailboxAttr) bool {
	for _, a := range attrs {
		if a == target {
			return true
		}
	}
	return false
}

// Select selects (or examines) a folder and returns a summary.
// go-imap v2 SelectData does not carry an Unseen count; callers that
// need it issue UID SEARCH UNSEEN separately.
func (r *realClient) Select(folder string, readOnly bool) (mail.Folder, error) {
	data, err := r.c.Select(folder, &imap.SelectOptions{ReadOnly: readOnly}).Wait()
	if err != nil {
		return mail.Folder{}, fmt.Errorf("select %q: %w", folder, err)
	}
	return mail.Folder{
		Name:   folder,
		Exists: int(data.NumMessages),
	}, nil
}

// Search runs UID SEARCH, translating mail.SearchCriteria to go-imap v2's
// imap.SearchCriteria. An empty criteria matches all messages (ALL).
func (r *realClient) Search(criteria mail.SearchCriteria) ([]mail.UID, error) {
	data, err := r.c.UIDSearch(translateSearchCriteria(criteria), nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("uid search: %w", err)
	}

	uidSet, ok := data.All.(imap.UIDSet)
	if !ok {
		// Should not happen for UID SEARCH; treat as empty.
		return nil, nil
	}
	nums, ok := uidSet.Nums()
	if !ok {
		// Dynamic set (contains "*") — cannot enumerate.
		return nil, nil
	}
	out := make([]mail.UID, len(nums))
	for i, u := range nums {
		out[i] = imapUID(u)
	}
	return out, nil
}

// translateSearchCriteria converts mail.SearchCriteria to imap.SearchCriteria.
func translateSearchCriteria(in mail.SearchCriteria) *imap.SearchCriteria {
	var sc imap.SearchCriteria
	for k, vals := range in.Header {
		for _, v := range vals {
			sc.Header = append(sc.Header, imap.SearchCriteriaHeaderField{Key: k, Value: v})
		}
	}
	sc.Body = append(sc.Body, in.Body...)
	sc.Text = append(sc.Text, in.Text...)
	return &sc
}

// Fetch runs UID FETCH for the given UIDs, calling resultFn once per
// message with a map of the fetched attributes. Item strings use IMAP
// wire format ("ENVELOPE", "FLAGS", "INTERNALDATE", "RFC822.SIZE",
// "UID", and BODY.PEEK[…] variants).
func (r *realClient) Fetch(uids []mail.UID, items []string, resultFn func(mail.UID, map[string]any)) error {
	if len(uids) == 0 {
		return nil
	}

	msgs, err := r.c.Fetch(mailUIDsToSet(uids), buildFetchOptions(items)).Collect()
	if err != nil {
		return fmt.Errorf("uid fetch: %w", err)
	}

	for _, buf := range msgs {
		resultFn(imapUID(buf.UID), fetchBufToMap(buf))
	}
	return nil
}

// buildFetchOptions translates IMAP item name strings into imap.FetchOptions.
func buildFetchOptions(items []string) *imap.FetchOptions {
	opts := &imap.FetchOptions{UID: true}
	for _, item := range items {
		upper := strings.ToUpper(item)
		switch {
		case upper == "UID":
			// already set
		case upper == "ENVELOPE":
			opts.Envelope = true
		case upper == "FLAGS":
			opts.Flags = true
		case upper == "INTERNALDATE":
			opts.InternalDate = true
		case upper == "RFC822.SIZE":
			opts.RFC822Size = true
		case strings.Contains(upper, "BODY.PEEK[") || strings.Contains(upper, "BODY["):
			opts.BodySection = append(opts.BodySection, parseFetchBodySection(item))
		}
	}
	return opts
}

// parseFetchBodySection parses a BODY[…] or BODY.PEEK[…] item string
// into an imap.FetchItemBodySection. Handles HEADER.FIELDS, HEADER, TEXT,
// and whole-body (empty bracket) sections.
func parseFetchBodySection(item string) *imap.FetchItemBodySection {
	sec := &imap.FetchItemBodySection{Peek: true}

	open := strings.Index(item, "[")
	close := strings.LastIndex(item, "]")
	if open < 0 || close <= open {
		return sec
	}
	inner := strings.TrimSpace(item[open+1 : close])
	innerUpper := strings.ToUpper(inner)

	switch {
	case innerUpper == "":
		// BODY[] — whole body; Specifier stays PartSpecifierNone
	case innerUpper == "TEXT":
		sec.Specifier = imap.PartSpecifierText
	case innerUpper == "HEADER":
		sec.Specifier = imap.PartSpecifierHeader
	case strings.HasPrefix(innerUpper, "HEADER.FIELDS.NOT"):
		sec.Specifier = imap.PartSpecifierHeader
		sec.HeaderFieldsNot = extractFieldList(inner)
	case strings.HasPrefix(innerUpper, "HEADER.FIELDS"):
		sec.Specifier = imap.PartSpecifierHeader
		sec.HeaderFields = extractFieldList(inner)
	}

	return sec
}

// extractFieldList parses the parenthesised field name list from a
// HEADER.FIELDS or HEADER.FIELDS.NOT section specifier.
// Example input: "HEADER.FIELDS (FROM TO CC SUBJECT)"
func extractFieldList(s string) []string {
	open := strings.Index(s, "(")
	close := strings.LastIndex(s, ")")
	if open < 0 || close <= open {
		return nil
	}
	return strings.Fields(s[open+1 : close])
}

// fetchBufToMap converts a FetchMessageBuffer to the map[string]any
// form consumed by infoFromFetch.
func fetchBufToMap(buf *imapclient.FetchMessageBuffer) map[string]any {
	m := make(map[string]any)

	if buf.Envelope != nil {
		env := buf.Envelope
		m["subject"] = env.Subject
		m["from"] = formatAddresses(env.From)
		m["to"] = formatAddresses(env.To)
		m["cc"] = formatAddresses(env.Cc)
		m["bcc"] = formatAddresses(env.Bcc)
		m["date"] = env.Date.Format("Mon, 02 Jan 2006 15:04:05 -0700")
		m["sentAt"] = env.Date
		if len(env.InReplyTo) > 0 {
			m["in-reply-to"] = env.InReplyTo[0]
		}
	}

	if len(buf.Flags) > 0 {
		m["flags"] = imapFlagsToMailFlags(buf.Flags)
	}

	if !buf.InternalDate.IsZero() {
		if _, ok := m["sentAt"]; !ok {
			m["sentAt"] = buf.InternalDate
		}
	}

	if buf.RFC822Size > 0 {
		m["size"] = uint32(buf.RFC822Size)
	}

	// Merge header fields fetched via BODY[HEADER.FIELDS …] into m;
	// ENVELOPE data already written above takes priority.
	for _, bs := range buf.BodySection {
		if bs.Section != nil && bs.Section.Specifier == imap.PartSpecifierHeader {
			parseHeaderFields(bs.Bytes, m)
		}
	}

	return m
}

// formatAddresses formats a slice of imap.Address into a display string.
func formatAddresses(addrs []imap.Address) string {
	parts := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if a.Name != "" {
			parts = append(parts, a.Name+" <"+a.Mailbox+"@"+a.Host+">")
		} else if a.Mailbox != "" && a.Host != "" {
			parts = append(parts, a.Mailbox+"@"+a.Host)
		}
	}
	return strings.Join(parts, ", ")
}

// imapFlagsToMailFlags maps imap.Flag values to poplar's mail.Flag bitfield.
func imapFlagsToMailFlags(flags []imap.Flag) mail.Flag {
	var out mail.Flag
	for _, f := range flags {
		switch f {
		case imap.FlagSeen:
			out |= mail.FlagSeen
		case imap.FlagAnswered:
			out |= mail.FlagAnswered
		case imap.FlagFlagged:
			out |= mail.FlagFlagged
		case imap.FlagDeleted:
			out |= mail.FlagDeleted
		case imap.FlagDraft:
			out |= mail.FlagDraft
		}
	}
	return out
}

// parseHeaderFields parses a raw RFC 5322 header block (bytes from a
// BODY[HEADER.FIELDS …] fetch) and merges fields into m. Keys already
// present in m are not overwritten, so ENVELOPE data wins.
//
// Folded header lines (RFC 5322 section 2.2.3) are not unfolded here —
// the values are used for display only and folding whitespace is
// acceptable in that context.
func parseHeaderFields(raw []byte, m map[string]any) {
	for len(raw) > 0 {
		idx := bytes.IndexByte(raw, '\n')
		var line []byte
		if idx < 0 {
			line = raw
			raw = nil
		} else {
			line = raw[:idx]
			raw = raw[idx+1:]
		}
		line = bytes.TrimRight(line, "\r")
		if len(line) == 0 {
			break
		}
		colon := bytes.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(string(line[:colon])))
		val := strings.TrimSpace(string(line[colon+1:]))
		if _, exists := m[key]; !exists {
			m[key] = val
		}
	}
}

// FetchBody fetches the complete RFC 822 body for uid and returns a reader.
func (r *realClient) FetchBody(uid mail.UID) (io.ReadCloser, error) {
	opts := &imap.FetchOptions{
		UID:         true,
		BodySection: []*imap.FetchItemBodySection{{Peek: true}}, // BODY.PEEK[]
	}

	msgs, err := r.c.Fetch(mailUIDsToSet([]mail.UID{uid}), opts).Collect()
	if err != nil {
		return nil, fmt.Errorf("uid fetch body: %w", err)
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("uid fetch body: no message for uid %s", uid)
	}

	raw := msgs[0].FindBodySection(&imap.FetchItemBodySection{})
	if raw == nil {
		return nil, fmt.Errorf("uid fetch body: no BODY[] section for uid %s", uid)
	}
	return io.NopCloser(bytes.NewReader(raw)), nil
}

// Store runs UID STORE. item uses the wire format ("+FLAGS.SILENT",
// "-FLAGS.SILENT", "FLAGS.SILENT"). value must be []string of IMAP
// flag literals (e.g. "\\Seen").
func (r *realClient) Store(uids []mail.UID, item string, value any) error {
	if len(uids) == 0 {
		return nil
	}

	flags, err := toFlagSlice(value)
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}

	sf := &imap.StoreFlags{
		Op:     parseStoreFlagsOp(item),
		Silent: strings.Contains(strings.ToUpper(item), ".SILENT"),
		Flags:  flags,
	}
	return r.c.Store(mailUIDsToSet(uids), sf, nil).Close()
}

// parseStoreFlagsOp maps a STORE item prefix to a StoreFlagsOp value.
func parseStoreFlagsOp(item string) imap.StoreFlagsOp {
	switch {
	case strings.HasPrefix(item, "+"):
		return imap.StoreFlagsAdd
	case strings.HasPrefix(item, "-"):
		return imap.StoreFlagsDel
	default:
		return imap.StoreFlagsSet
	}
}

// toFlagSlice coerces value to []imap.Flag. value must be []string.
func toFlagSlice(value any) ([]imap.Flag, error) {
	ss, ok := value.([]string)
	if !ok {
		return nil, fmt.Errorf("store value must be []string, got %T", value)
	}
	flags := make([]imap.Flag, len(ss))
	for i, s := range ss {
		flags[i] = imap.Flag(s)
	}
	return flags, nil
}

// Copy runs UID COPY.
func (r *realClient) Copy(uids []mail.UID, dest string) error {
	if len(uids) == 0 {
		return nil
	}
	if _, err := r.c.Copy(mailUIDsToSet(uids), dest).Wait(); err != nil {
		return fmt.Errorf("uid copy to %q: %w", dest, err)
	}
	return nil
}

// Move runs UID MOVE. go-imap v2 falls back to COPY+STORE+EXPUNGE when
// the server does not advertise MOVE.
func (r *realClient) Move(uids []mail.UID, dest string) error {
	if len(uids) == 0 {
		return nil
	}
	if _, err := r.c.Move(mailUIDsToSet(uids), dest).Wait(); err != nil {
		return fmt.Errorf("uid move to %q: %w", dest, err)
	}
	return nil
}

// UIDExpunge runs UID EXPUNGE (UIDPLUS / IMAP4rev2).
func (r *realClient) UIDExpunge(uids []mail.UID) error {
	if len(uids) == 0 {
		return nil
	}
	return r.c.UIDExpunge(mailUIDsToSet(uids)).Close()
}

// Idle starts IDLE and blocks until IdleStop is called or the server
// disconnects. Unilateral updates (EXISTS → UpdateNewMail, EXPUNGE →
// UpdateExpunge, FETCH FLAGS → UpdateFlagsChanged) are forwarded to
// onUpdate via the UnilateralDataHandler wired in auth.go.
func (r *realClient) Idle(onUpdate func(mail.Update)) error {
	r.mu.Lock()
	r.unilateralHandler = onUpdate
	r.mu.Unlock()

	cmd, err := r.c.Idle()
	if err != nil {
		r.mu.Lock()
		r.unilateralHandler = nil
		r.mu.Unlock()
		return fmt.Errorf("idle: %w", err)
	}

	r.mu.Lock()
	r.idleCmd = cmd
	r.mu.Unlock()

	waitErr := cmd.Wait()

	r.mu.Lock()
	r.idleCmd = nil
	r.unilateralHandler = nil
	r.mu.Unlock()

	if waitErr != nil {
		return fmt.Errorf("idle wait: %w", waitErr)
	}
	return nil
}

// IdleStop sends DONE to stop the running IDLE command.
func (r *realClient) IdleStop() {
	r.mu.Lock()
	cmd := r.idleCmd
	r.mu.Unlock()

	if cmd != nil {
		_ = cmd.Close()
	}
}
