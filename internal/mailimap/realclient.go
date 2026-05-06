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

func imapUID(u imap.UID) mail.UID {
	return mail.UID(strconv.FormatUint(uint64(u), 10))
}

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
			Name:       m.Mailbox,
			Attributes: attrsToStrings(m.Attrs),
		})
	}
	return out, nil
}

func attrsToStrings(attrs []imap.MailboxAttr) []string {
	out := make([]string, len(attrs))
	for i, a := range attrs {
		out[i] = string(a)
	}
	return out
}

// Select selects (or examines) a folder and returns a summary.
// go-imap v2 SelectData does not carry an Unseen count. Callers that
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
		// Should not happen for UID SEARCH. Treat as empty.
		return nil, nil
	}
	nums, ok := uidSet.Nums()
	if !ok {
		// Dynamic set (contains "*"): cannot enumerate.
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
		// BODY[]: whole body. Specifier stays PartSpecifierNone
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
// Folded header lines (RFC 5322 section 2.2.3) are not unfolded here.
// The values are used for display only and folding whitespace is
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

func (r *realClient) FetchBody(uid mail.UID) (io.ReadCloser, error) {
	opts := &imap.FetchOptions{
		UID:         true,
		BodySection: []*imap.FetchItemBodySection{{Peek: true}}, // BODY.PEEK[]
	}

	msgs, err := r.c.Fetch(mailUIDsToSet([]mail.UID{uid}), opts).Collect()
	if err != nil {
		return nil, fmt.Errorf("fetch body uid %s: %v", uid, err)
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("uid %s not on server", uid)
	}

	raw := msgs[0].FindBodySection(&imap.FetchItemBodySection{})
	if raw == nil {
		return nil, fmt.Errorf("uid %s: no body returned", uid)
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

func (r *realClient) Append(folder string, mime []byte, flags []string) (mail.UID, error) {
	opts := &imap.AppendOptions{}
	for _, f := range flags {
		opts.Flags = append(opts.Flags, imap.Flag(f))
	}
	cmd := r.c.Append(folder, int64(len(mime)), opts)
	if _, err := cmd.Write(mime); err != nil {
		_ = cmd.Close()
		return "", fmt.Errorf("append %q: write: %w", folder, err)
	}
	if err := cmd.Close(); err != nil {
		return "", fmt.Errorf("append %q: close: %w", folder, err)
	}
	data, err := cmd.Wait()
	if err != nil {
		return "", fmt.Errorf("append %q: %w", folder, err)
	}
	if data == nil || data.UID == 0 {
		return "", nil
	}
	return imapUID(data.UID), nil
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

// Extended structure is requested so Disposition and filename params populate.
func (r *realClient) FetchBodyStructure(uid mail.UID) (BodyStructure, error) {
	opts := &imap.FetchOptions{
		UID:           true,
		BodyStructure: &imap.FetchItemBodyStructure{Extended: true},
	}

	msgs, err := r.c.Fetch(mailUIDsToSet([]mail.UID{uid}), opts).Collect()
	if err != nil {
		return BodyStructure{}, fmt.Errorf("BODYSTRUCTURE uid %s: %v", uid, err)
	}
	if len(msgs) == 0 {
		return BodyStructure{}, fmt.Errorf("uid %s not on server", uid)
	}

	if msgs[0].BodyStructure == nil {
		return BodyStructure{}, fmt.Errorf("uid %s: server omitted BODYSTRUCTURE", uid)
	}

	return convertBodyStructure(msgs[0].BodyStructure, nil), nil
}

// convertBodyStructure recursively converts an imap.BodyStructure (go-imap
// v2 type) to the protocol-agnostic BodyStructure. path is the integer
// path from the Walk tree. nil means multipart root.
func convertBodyStructure(bs imap.BodyStructure, path []int) BodyStructure {
	sec := sectionString(path)
	disp := ""
	if d := bs.Disposition(); d != nil {
		disp = strings.ToLower(d.Value)
	}

	switch v := bs.(type) {
	case *imap.BodyStructureSinglePart:
		return BodyStructure{
			Section:     sec,
			MIMEType:    v.MediaType(),
			Filename:    v.Filename(),
			SizeBytes:   v.Size,
			ContentID:   v.ID,
			Disposition: disp,
		}

	case *imap.BodyStructureMultiPart:
		children := make([]BodyStructure, len(v.Children))
		pathBuf := make([]int, len(path)+1)
		copy(pathBuf, path)
		for i, child := range v.Children {
			pathBuf[len(path)] = i + 1
			childPath := make([]int, len(pathBuf))
			copy(childPath, pathBuf)
			children[i] = convertBodyStructure(child, childPath)
		}
		return BodyStructure{
			Section:     sec,
			MIMEType:    v.MediaType(),
			Disposition: disp,
			Children:    children,
		}
	}

	// Unreachable: go-imap only produces SinglePart and MultiPart.
	return BodyStructure{Section: sec}
}

// section is the dot-joined path (e.g. "2", "2.1"). Fetched as BODY.PEEK[<section>].
func (r *realClient) FetchBodyPart(uid mail.UID, section string) ([]byte, error) {
	parts, err := parseSectionPath(section)
	if err != nil {
		return nil, fmt.Errorf("parse section %q: %v", section, err)
	}

	fetchSec := &imap.FetchItemBodySection{
		Part:      parts,
		Specifier: imap.PartSpecifierNone,
		Peek:      true,
	}
	opts := &imap.FetchOptions{
		UID:         true,
		BodySection: []*imap.FetchItemBodySection{fetchSec},
	}

	msgs, err := r.c.Fetch(mailUIDsToSet([]mail.UID{uid}), opts).Collect()
	if err != nil {
		return nil, fmt.Errorf("fetch part %s of uid %s: %v", section, uid, err)
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("uid %s not on server", uid)
	}

	raw := msgs[0].FindBodySection(fetchSec)
	if raw == nil {
		return nil, fmt.Errorf("uid %s part %s: server omitted bytes", uid, section)
	}
	return raw, nil
}

// parseSectionPath converts a dot-joined section string ("2.1") to the
// []int slice expected by imap.FetchItemBodySection.Part.
func parseSectionPath(section string) ([]int, error) {
	if section == "" {
		return nil, nil
	}
	parts := strings.Split(section, ".")
	nums := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid section path component %q in %q", p, section)
		}
		nums[i] = n
	}
	return nums, nil
}
