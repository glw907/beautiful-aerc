// SPDX-License-Identifier: MIT

// Package mailjmap implements mail.Backend for JMAP servers
// (Fastmail) by calling git.sr.ht/~rockorager/go-jmap directly.
// All RPC methods are synchronous. A single goroutine owned by
// Connect/Disconnect reads JMAP push StateChange events and emits
// mail.Update values onto the channel returned by Updates().
package mailjmap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"git.sr.ht/~rockorager/go-jmap"
	jmapmail "git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"
	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/sync/singleflight"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

// jmapClient is the subset of *jmap.Client poplar uses. The real
// *jmap.Client satisfies it; tests substitute a fake.
type jmapClient interface {
	Do(req *jmap.Request) (*jmap.Response, error)
}

// Backend is one JMAP account. Construct with New, drive lifecycle
// with Connect/Disconnect.
type Backend struct {
	cfg config.AccountConfig

	mu         sync.Mutex
	client     jmapClient
	pushClient *jmap.Client // real client for EventSource; nil in unit tests
	session    *jmap.Session
	current    string
	folders    map[string]folderEntry
	blobIDs    map[mail.UID]string
	states     map[string]string

	bodies             *lru.Cache[string, []byte]
	bodyGroup          singleflight.Group
	downloadBlob       func(blobID string) ([]byte, error) // nil ⇒ set by Connect; tests swap it
	updates            chan mail.Update
	runEventSourceFunc func(ctx context.Context) error // swappable for tests

	pushCancel context.CancelFunc
	pushDone   chan struct{}
}

type folderEntry struct {
	id     string // JMAP mailbox id
	folder mail.Folder
}

// New constructs an unconnected Backend for cfg. cfg.Source is the
// JMAP session URL (e.g. https://api.fastmail.com/jmap/session);
// cfg.Password (post env-var substitution) supplies the bearer token.
func New(cfg config.AccountConfig) *Backend {
	return &Backend{
		cfg:     cfg,
		folders: make(map[string]folderEntry),
		blobIDs: make(map[mail.UID]string),
		states:  make(map[string]string),
	}
}

// NewWithClient is for tests. It bypasses the network handshake and
// installs a pre-built client that already satisfies the session
// contract. The caller is responsible for populating b.session if any
// method under test reads PrimaryAccounts — assign it directly:
//
//	b := NewWithClient(cfg, fake)
//	b.session = &jmap.Session{...}
func NewWithClient(cfg config.AccountConfig, c jmapClient) *Backend {
	b := New(cfg)
	b.client = c
	b.runEventSourceFunc = b.runEventSource
	cache, _ := lru.New[string, []byte](bodyCacheSize)
	b.bodies = cache
	b.updates = make(chan mail.Update, updatesBuffer)
	return b
}

// AccountName satisfies mail.Backend. The cfg.Name fallback only
// surfaces during the pre-Connect window, before the JMAP session
// supplies the email per RFC 8620 §1.6.2.
func (b *Backend) AccountName() string {
	if b.cfg.Display != "" {
		return b.cfg.Display
	}
	if email := b.AccountEmail(); email != "" {
		return email
	}
	return b.cfg.Name
}

// AccountEmail satisfies mail.Backend.
func (b *Backend) AccountEmail() string {
	if b.cfg.From != nil && b.cfg.From.Address != "" {
		return b.cfg.From.Address
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.session == nil {
		return ""
	}
	accountID := b.session.PrimaryAccounts[jmapmail.URI]
	if acct, ok := b.session.Accounts[accountID]; ok {
		return acct.Name
	}
	return ""
}

// Updates satisfies mail.Backend. Returns a nil channel before
// Connect succeeds.
func (b *Backend) Updates() <-chan mail.Update { return b.updates }

const (
	bodyCacheSize = 64
	updatesBuffer = 64
)

// Connect satisfies mail.Backend. It authenticates against the JMAP
// session endpoint, populates the folder map, and initialises the
// body cache and updates channel.
//
// All network RPCs run without holding b.mu. The lock is acquired
// once at the end to install the completed state and spawn the push
// goroutine.
func (b *Backend) Connect(_ context.Context) error {
	// --- Phase 1: authenticate (no lock) ---
	cli := &jmap.Client{
		SessionEndpoint: b.cfg.Source,
	}
	cli.WithAccessToken(b.cfg.Password)
	if err := cli.Authenticate(); err != nil {
		return fmt.Errorf("connect: authenticate: %w", err)
	}
	session := cli.Session

	// --- Phase 2: fetch folders (no lock) ---
	folders, mailboxState, err := fetchFolders(cli, session)
	if err != nil {
		return fmt.Errorf("connect: list folders: %w", err)
	}

	// --- Phase 3: seed email state (no lock) ---
	emailState, err := fetchEmailState(cli, session)
	if err != nil {
		return fmt.Errorf("connect: seed email state: %w", err)
	}

	// --- Phase 4: build local values (no lock) ---
	cache, err := lru.New[string, []byte](bodyCacheSize)
	if err != nil {
		return fmt.Errorf("connect: init body cache: %w", err)
	}
	updates := make(chan mail.Update, updatesBuffer)

	accountID := session.PrimaryAccounts[jmapmail.URI]
	dlBlob := func(blobID string) ([]byte, error) {
		rc, err := cli.Download(accountID, jmap.ID(blobID))
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}

	// --- Phase 5: install state under lock, then spawn goroutine ---
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	b.mu.Lock()
	b.client = cli
	b.session = session
	b.folders = folders
	b.states["Mailbox"] = mailboxState
	b.states["Email"] = emailState
	b.bodies = cache
	b.updates = updates
	b.downloadBlob = dlBlob
	b.pushClient = cli
	b.runEventSourceFunc = b.runEventSource
	b.pushCancel = cancel
	b.pushDone = done
	b.mu.Unlock()

	go b.pushLoop(ctx)

	return nil
}

// fetchEmailState issues Email/get with a single sentinel ID to cheaply
// fetch the current Email state string. An empty IDs slice would be
// dropped by omitempty (becoming "fetch all"), which Fastmail rejects on
// large mailboxes; a single non-existent ID lands in `notFound` and the
// server still returns the live state. It does not hold b.mu — callers
// supply the client and session directly.
func fetchEmailState(cli jmapClient, session *jmap.Session) (string, error) {
	accountID := session.PrimaryAccounts[jmapmail.URI]
	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	req.Invoke(&email.Get{
		Account:    accountID,
		IDs:        []jmap.ID{"poplar-state-probe"},
		Properties: []string{"id"},
	})
	resp, err := cli.Do(req)
	if err != nil {
		return "", fmt.Errorf("email/get: %w", err)
	}
	for _, inv := range resp.Responses {
		if me, ok := inv.Args.(*jmap.MethodError); ok {
			return "", fmt.Errorf("email/get: server error: %s", me.Error())
		}
		gr, ok := inv.Args.(*email.GetResponse)
		if !ok {
			continue
		}
		return gr.State, nil
	}
	return "", fmt.Errorf("email/get: no response")
}

// fetchFolders issues Mailbox/get and returns a populated folder map,
// the Mailbox state string, and any error. It does not hold b.mu —
// callers supply the client and session directly.
func fetchFolders(cli jmapClient, session *jmap.Session) (map[string]folderEntry, string, error) {
	accountID := session.PrimaryAccounts[jmapmail.URI]

	req := &jmap.Request{}
	req.Invoke(&mailbox.Get{Account: accountID})

	resp, err := cli.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("mailbox/get: %w", err)
	}

	folders := make(map[string]folderEntry)
	for _, inv := range resp.Responses {
		gr, ok := inv.Args.(*mailbox.GetResponse)
		if !ok {
			continue
		}

		// Build raw mail.Folder slice to run through the classifier.
		raw := make([]mail.Folder, 0, len(gr.List))
		for _, mbox := range gr.List {
			raw = append(raw, mail.Folder{
				Name:   mbox.Name,
				Exists: int(mbox.TotalEmails),
				Unseen: int(mbox.UnreadEmails),
				Role:   string(mbox.Role),
			})
		}

		classified := mail.Classify(raw)
		for i, cf := range classified {
			key := cf.DisplayName
			folders[key] = folderEntry{
				id:     string(gr.List[i].ID),
				folder: cf.Folder,
			}
		}
		return folders, gr.State, nil
	}
	return folders, "", nil
}

// refreshFoldersLocked issues Mailbox/get, populates b.folders keyed
// by canonical poplar name, and captures the state string into
// b.states["Mailbox"]. Caller must hold b.mu.
func (b *Backend) refreshFoldersLocked() error {
	folders, state, err := fetchFolders(b.client, b.session)
	if err != nil {
		return err
	}
	b.folders = folders
	b.states["Mailbox"] = state
	return nil
}

// Disconnect satisfies mail.Backend. It cancels the push loop (Task
// 13) and tears down all session state.
func (b *Backend) Disconnect() error {
	b.mu.Lock()
	cancel := b.pushCancel
	done := b.pushDone
	b.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.updates != nil {
		close(b.updates)
		b.updates = nil
	}
	b.client = nil
	b.session = nil
	b.current = ""
	b.folders = make(map[string]folderEntry)
	b.blobIDs = make(map[mail.UID]string)
	b.states = make(map[string]string)
	b.bodies = nil
	return nil
}

// ListFolders satisfies mail.Backend. It returns the cached folder
// map, refreshing from the server if the cache is empty.
func (b *Backend) ListFolders() ([]mail.Folder, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.folders) == 0 {
		if err := b.refreshFoldersLocked(); err != nil {
			return nil, fmt.Errorf("list folders: %w", err)
		}
	}
	out := make([]mail.Folder, 0, len(b.folders))
	for _, e := range b.folders {
		out = append(out, e.folder)
	}
	return out, nil
}

// OpenFolder satisfies mail.Backend. It sets the current folder to
// name; returns an error if name is not in the cached folder map.
func (b *Backend) OpenFolder(name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.folders[name]; !ok {
		return fmt.Errorf("open folder: unknown folder %q", name)
	}
	b.current = name
	return nil
}

// QueryFolder satisfies mail.Backend. It issues Email/query against
// the named folder and returns (UIDs, total, error). offset and limit
// map directly to JMAP Position and Limit. Results are sorted by
// receivedAt descending (newest first).
func (b *Backend) QueryFolder(name string, offset, limit int) ([]mail.UID, int, error) {
	b.mu.Lock()
	entry, ok := b.folders[name]
	accountID := b.session.PrimaryAccounts[jmapmail.URI]
	b.mu.Unlock()
	if !ok {
		return nil, 0, fmt.Errorf("query folder: unknown folder %q", name)
	}

	req := &jmap.Request{}
	req.Invoke(&email.Query{
		Account: accountID,
		Filter: &email.FilterCondition{
			InMailbox: jmap.ID(entry.id),
		},
		Sort: []*email.SortComparator{
			{Property: "receivedAt", IsAscending: false},
		},
		Position:       int64(offset),
		Limit:          uint64(limit),
		CalculateTotal: true,
	})

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("query folder: %w", err)
	}

	for _, inv := range resp.Responses {
		qr, ok := inv.Args.(*email.QueryResponse)
		if !ok {
			continue
		}
		uids := make([]mail.UID, len(qr.IDs))
		for i, id := range qr.IDs {
			uids[i] = mail.UID(id)
		}
		return uids, int(qr.Total), nil
	}
	return nil, 0, fmt.Errorf("query folder: no Email/query response")
}

// headerProperties is the minimal Email/get property set for list display.
var headerProperties = []string{
	"id", "blobId", "subject", "from", "to", "cc", "bcc", "receivedAt",
	"keywords", "size", "inReplyTo", "threadId",
}

// FetchHeaders satisfies mail.Backend. It issues a single Email/get
// request for the supplied UIDs and translates each response email
// into mail.MessageInfo. BlobIDs are cached in b.blobIDs for Task 12.
func (b *Backend) FetchHeaders(uids []mail.UID) ([]mail.MessageInfo, error) {
	if len(uids) == 0 {
		return nil, nil
	}

	b.mu.Lock()
	accountID := b.session.PrimaryAccounts[jmapmail.URI]
	b.mu.Unlock()

	ids := make([]jmap.ID, 0, len(uids))
	for _, u := range uids {
		ids = append(ids, jmap.ID(u))
	}

	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	req.Invoke(&email.Get{
		Account:    accountID,
		IDs:        ids,
		Properties: headerProperties,
	})

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch headers: %w", err)
	}

	var out []mail.MessageInfo
	for _, inv := range resp.Responses {
		gr, ok := inv.Args.(*email.GetResponse)
		if !ok {
			continue
		}
		out = make([]mail.MessageInfo, 0, len(gr.List))
		for _, e := range gr.List {
			uid := mail.UID(e.ID)
			b.mu.Lock()
			b.blobIDs[uid] = string(e.BlobID)
			b.mu.Unlock()
			out = append(out, translateEmail(e))
		}
		break
	}
	if out == nil {
		return nil, fmt.Errorf("fetch headers: no Email/get response")
	}
	return out, nil
}

// translateEmail converts a JMAP *email.Email into mail.MessageInfo.
func translateEmail(e *email.Email) mail.MessageInfo {
	info := mail.MessageInfo{
		UID:       mail.UID(e.ID),
		Subject:   e.Subject,
		From:      formatFromList(e.From),
		To:        formatFromList(e.To),
		Cc:        formatFromList(e.CC),
		Bcc:       formatFromList(e.BCC),
		Flags:     translateKeywords(e.Keywords),
		Size:      uint32(e.Size),
		ThreadID:  mail.UID(e.ThreadID),
		InReplyTo: mail.UID(firstInReplyTo(e.InReplyTo)),
	}
	if e.ReceivedAt != nil {
		info.SentAt = *e.ReceivedAt
	}
	return info
}

// formatFromList formats a list of JMAP addresses into a display string.
// Multiple senders are joined with ", ". Each address uses the display
// name if present, otherwise falls back to the email address.
func formatFromList(addrs []*jmapmail.Address) string {
	if len(addrs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if a == nil {
			continue
		}
		if a.Name != "" {
			parts = append(parts, trimSurroundingQuotes(a.Name))
		} else {
			parts = append(parts, a.Email)
		}
	}
	return strings.Join(parts, ", ")
}

// trimSurroundingQuotes strips a leading single-quote-wrapped phrase
// from a display name. Some mailing-list software (Google Groups,
// etc.) emits names like "'DAVE JOHNSON' via Members" — the literal
// single quotes are data, not RFC 5322 syntax, but they offset
// visual alignment. Names that don't start with a single quote (e.g.
// "Joe O'Brien") are returned unchanged.
func trimSurroundingQuotes(s string) string {
	if !strings.HasPrefix(s, "'") {
		return s
	}
	rest := s[1:]
	closeIdx := strings.Index(rest, "'")
	if closeIdx < 0 {
		return s
	}
	return rest[:closeIdx] + rest[closeIdx+1:]
}

// translateKeywords maps JMAP keyword strings to mail.Flag bits.
func translateKeywords(kw map[string]bool) mail.Flag {
	var f mail.Flag
	if kw["$seen"] {
		f |= mail.FlagSeen
	}
	if kw["$flagged"] {
		f |= mail.FlagFlagged
	}
	if kw["$answered"] {
		f |= mail.FlagAnswered
	}
	if kw["$draft"] {
		f |= mail.FlagDraft
	}
	if kw["$forwarded"] {
		f |= mail.FlagForwarded
	}
	return f
}

// firstInReplyTo returns the first value from the InReplyTo header list,
// or empty string if there are none.
func firstInReplyTo(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

// FetchBody satisfies mail.Backend. It returns the raw message blob for
// uid, using an LRU cache to avoid redundant downloads. Concurrent callers
// for the same blobID are collapsed via singleflight so only one download
// runs. FetchHeaders must be called first to populate the blobID map.
func (b *Backend) FetchBody(uid mail.UID) (io.Reader, error) {
	b.mu.Lock()
	blobID, ok := b.blobIDs[uid]
	cache := b.bodies
	dl := b.downloadBlob
	b.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("fetch body: unknown uid %q (call FetchHeaders first)", uid)
	}

	if buf, hit := cache.Get(blobID); hit {
		return bytes.NewReader(buf), nil
	}

	v, err, _ := b.bodyGroup.Do(blobID, func() (any, error) {
		if buf, hit := cache.Get(blobID); hit {
			return buf, nil
		}
		buf, err := dl(blobID)
		if err != nil {
			return nil, err
		}
		cache.Add(blobID, buf)
		return buf, nil
	})
	if err != nil {
		return nil, fmt.Errorf("fetch body: %w", err)
	}
	return bytes.NewReader(v.([]byte)), nil
}

// Search satisfies mail.Backend. Sidebar search filters in-memory in
// Pass 2.5b-7; backend Search is unused. Pass 6 may wire server-side search.
func (b *Backend) Search(_ mail.SearchCriteria) ([]mail.UID, error) {
	return nil, nil
}

// Move satisfies mail.Backend. It moves uids into destFolder by patching
// mailboxIds to contain only the destination mailbox.
func (b *Backend) Move(uids []mail.UID, destFolder string) error {
	if len(uids) == 0 {
		return nil
	}
	b.mu.Lock()
	entry, ok := b.folders[destFolder]
	accountID := b.accountIDLocked()
	b.mu.Unlock()
	if !ok {
		return fmt.Errorf("move: unknown folder %q", destFolder)
	}
	update := make(map[jmap.ID]jmap.Patch, len(uids))
	for _, u := range uids {
		update[jmap.ID(u)] = jmap.Patch{
			"mailboxIds": map[string]bool{entry.id: true},
		}
	}
	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	callID := req.Invoke(&email.Set{
		Account: accountID,
		Update:  update,
	})
	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("move: %w", err)
	}
	if err := checkEmailSetUpdated(resp, callID); err != nil {
		return fmt.Errorf("move: %w", err)
	}
	return nil
}

// Copy satisfies mail.Backend. It copies uids into destFolder by creating
// new messages with the same blob and the destination mailbox. This is a
// same-account copy using Email/set Create.
func (b *Backend) Copy(uids []mail.UID, destFolder string) error {
	if len(uids) == 0 {
		return nil
	}
	b.mu.Lock()
	destEntry, ok := b.folders[destFolder]
	accountID := b.accountIDLocked()
	blobIDs := make(map[mail.UID]string, len(uids))
	for _, u := range uids {
		blobIDs[u] = b.blobIDs[u]
	}
	b.mu.Unlock()
	if !ok {
		return fmt.Errorf("copy: unknown folder %q", destFolder)
	}

	create := make(map[jmap.ID]*email.Email, len(uids))
	for i, u := range uids {
		blobID := blobIDs[u]
		if blobID == "" {
			return fmt.Errorf("copy: unknown blob for uid %q (call FetchHeaders first)", u)
		}
		key := jmap.ID(fmt.Sprintf("copy-%d", i))
		create[key] = &email.Email{
			BlobID:     jmap.ID(blobID),
			MailboxIDs: map[jmap.ID]bool{jmap.ID(destEntry.id): true},
		}
	}
	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	callID := req.Invoke(&email.Set{
		Account: accountID,
		Create:  create,
	})
	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	if err := checkEmailSetCreated(resp, callID); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
}

// Delete satisfies mail.Backend. It soft-deletes by moving uids to Trash.
func (b *Backend) Delete(uids []mail.UID) error {
	if len(uids) == 0 {
		return nil
	}
	b.mu.Lock()
	_, ok := b.folders["Trash"]
	b.mu.Unlock()
	if !ok {
		return fmt.Errorf("delete: Trash folder not found")
	}
	if err := b.Move(uids, "Trash"); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}

// Destroy permanently deletes uids via JMAP Email/set destroy, bypassing
// Trash. Irreversible. Empty input is a no-op.
// Full implementation is deferred to Task 3 (Pass 6.6).
func (b *Backend) Destroy(uids []mail.UID) error {
	if len(uids) == 0 {
		return nil
	}
	return fmt.Errorf("destroy: not yet implemented")
}

// Flag satisfies mail.Backend. It sets or clears a JMAP keyword for each uid.
func (b *Backend) Flag(uids []mail.UID, flag mail.Flag, set bool) error {
	keyword, err := keywordForFlag(flag)
	if err != nil {
		return err
	}
	return b.setKeyword(uids, keyword, set)
}

// MarkRead satisfies mail.Backend.
func (b *Backend) MarkRead(uids []mail.UID) error {
	return b.setKeyword(uids, "$seen", true)
}

// MarkUnread satisfies mail.Backend.
func (b *Backend) MarkUnread(uids []mail.UID) error {
	return b.setKeyword(uids, "$seen", false)
}

// MarkAnswered satisfies mail.Backend.
func (b *Backend) MarkAnswered(uids []mail.UID) error {
	return b.setKeyword(uids, "$answered", true)
}

// Send satisfies mail.Backend. Compose is planned for Pass 9.
func (b *Backend) Send(_ string, _ []string, _ io.Reader) error {
	return errors.New("send not implemented in pass 3 — see pass 9")
}

// accountIDLocked returns the primary JMAP account ID. Caller must hold b.mu.
func (b *Backend) accountIDLocked() jmap.ID {
	return b.session.PrimaryAccounts[jmapmail.URI]
}

// keywordForFlag maps a mail.Flag to its JMAP keyword string.
func keywordForFlag(flag mail.Flag) (string, error) {
	switch flag {
	case mail.FlagSeen:
		return "$seen", nil
	case mail.FlagFlagged:
		return "$flagged", nil
	case mail.FlagAnswered:
		return "$answered", nil
	case mail.FlagDraft:
		return "$draft", nil
	case mail.FlagForwarded:
		return "$forwarded", nil
	default:
		return "", fmt.Errorf("unsupported flag for JMAP: %v", flag)
	}
}

// setKeyword patches the given JMAP keyword to set (true) or unset (nil) for
// all uids in a single Email/set request.
func (b *Backend) setKeyword(uids []mail.UID, keyword string, set bool) error {
	if len(uids) == 0 {
		return nil
	}
	b.mu.Lock()
	accountID := b.accountIDLocked()
	b.mu.Unlock()

	var val interface{}
	if set {
		val = true
	}
	update := make(map[jmap.ID]jmap.Patch, len(uids))
	for _, u := range uids {
		update[jmap.ID(u)] = jmap.Patch{
			"keywords/" + keyword: val,
		}
	}
	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	callID := req.Invoke(&email.Set{
		Account: accountID,
		Update:  update,
	})
	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("set keyword %s: %w", keyword, err)
	}
	if err := checkEmailSetUpdated(resp, callID); err != nil {
		return fmt.Errorf("set keyword %s: %w", keyword, err)
	}
	return nil
}

// checkEmailSetUpdated finds the Email/setResponse matching callID and returns
// an error if any ids appear in NotUpdated.
func checkEmailSetUpdated(resp *jmap.Response, callID string) error {
	for _, inv := range resp.Responses {
		if inv.CallID != callID {
			continue
		}
		sr, ok := inv.Args.(*email.SetResponse)
		if !ok {
			continue
		}
		for id, se := range sr.NotUpdated {
			return fmt.Errorf("not updated %s: %s", id, se.Type)
		}
		return nil
	}
	return fmt.Errorf("no Email/set response")
}

// checkEmailSetCreated finds the Email/setResponse matching callID and returns
// an error if any ids appear in NotCreated.
func checkEmailSetCreated(resp *jmap.Response, callID string) error {
	for _, inv := range resp.Responses {
		if inv.CallID != callID {
			continue
		}
		sr, ok := inv.Args.(*email.SetResponse)
		if !ok {
			continue
		}
		for id, se := range sr.NotCreated {
			return fmt.Errorf("not created %s: %s", id, se.Type)
		}
		return nil
	}
	return fmt.Errorf("no Email/set response")
}
