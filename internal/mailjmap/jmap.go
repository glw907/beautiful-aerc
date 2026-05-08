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
	"os"
	"strings"
	"sync"
	"time"

	"git.sr.ht/~rockorager/go-jmap"
	jmapmail "git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/emailsubmission"
	"git.sr.ht/~rockorager/go-jmap/mail/identity"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"
	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/sync/singleflight"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

// jmapClient is the subset of *jmap.Client poplar uses. Tests
// substitute a fake.
type jmapClient interface {
	Do(req *jmap.Request) (*jmap.Response, error)
}

// Backend is one JMAP account. Construct with New, drive lifecycle
// with Connect/Disconnect.
type Backend struct {
	cfg config.AccountConfig

	mu         sync.Mutex
	client     jmapClient
	pushClient *jmap.Client // for EventSource, nil in unit tests
	session    *jmap.Session
	password   string
	folders    map[string]folderEntry
	blobIDs    map[mail.UID]string
	// partBlobIDs lets FetchAttachment skip the extra Email/get when
	// Attachments has already populated the per-uid (partID → blobID).
	partBlobIDs map[mail.UID]map[string]string
	states      map[string]string

	bodies       *lru.Cache[string, []byte]
	bodyGroup    singleflight.Group
	downloadBlob func(blobID string) ([]byte, error)
	uploadBlob   func(data []byte) (blobID string, err error)
	// identityIDs caches Identity/get results by lowercased email.
	identityIDs        map[string]jmap.ID
	updates            chan mail.Update
	runEventSourceFunc func(ctx context.Context) error // swappable for tests

	pushCancel context.CancelFunc
	pushDone   chan struct{}
}

type folderEntry struct {
	id     string // JMAP mailbox id
	folder mail.Folder
}

// New constructs an unconnected Backend. cfg.Source is the JMAP
// session URL (e.g. https://api.fastmail.com/jmap/session) and
// cfg.Password supplies the bearer token after env-var substitution.
func New(cfg config.AccountConfig) *Backend {
	return &Backend{
		cfg:         cfg,
		folders:     make(map[string]folderEntry),
		blobIDs:     make(map[mail.UID]string),
		partBlobIDs: make(map[mail.UID]map[string]string),
		states:      make(map[string]string),
		identityIDs: make(map[string]jmap.ID),
	}
}

// NewWithClient bypasses the network handshake for tests. The caller
// must assign b.session directly when the method under test reads
// PrimaryAccounts.
func NewWithClient(cfg config.AccountConfig, c jmapClient) *Backend {
	b := New(cfg)
	b.client = c
	b.runEventSourceFunc = b.runEventSource
	cache, _ := lru.New[string, []byte](bodyCacheSize)
	b.bodies = cache
	b.updates = make(chan mail.Update, updatesBuffer)
	return b
}

// AccountName falls back to cfg.Name only during the pre-Connect
// window, before the JMAP session supplies the email (RFC 8620 §1.6.2).
func (b *Backend) AccountName() string {
	if b.cfg.Display != "" {
		return b.cfg.Display
	}
	if email := b.AccountEmail(); email != "" {
		return email
	}
	return b.cfg.Name
}

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

func (b *Backend) IsJMAP() bool { return true }

// Updates returns a nil channel before Connect succeeds.
func (b *Backend) Updates() <-chan mail.Update { return b.updates }

const (
	bodyCacheSize = 64
	updatesBuffer = 64
)

// resolvedPassword caches the resolved password so reconnects reuse
// the credential without re-running password-cmd.
func (b *Backend) resolvedPassword() (string, error) {
	b.mu.Lock()
	cached := b.password
	b.mu.Unlock()
	if cached != "" {
		return cached, nil
	}
	pw, err := b.cfg.ResolvePassword()
	if err != nil {
		return "", err
	}
	b.mu.Lock()
	b.password = pw
	b.mu.Unlock()
	return pw, nil
}

// Connect resolves the password, authenticates against the JMAP
// session endpoint, populates the folder map, and initialises the
// body cache and updates channel. Network RPCs run without holding
// b.mu so secret-manager prompts and slow calls don't stall readers.
func (b *Backend) Connect(_ context.Context) error {
	pw, err := b.resolvedPassword()
	if err != nil {
		return fmt.Errorf("password: %w", err)
	}

	cli := &jmap.Client{
		SessionEndpoint: b.cfg.Source,
	}
	cli.WithAccessToken(pw)
	if err := cli.Authenticate(); err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}
	session := cli.Session

	folders, mailboxState, err := fetchFolders(cli, session)
	if err != nil {
		return fmt.Errorf("list folders: %w", err)
	}

	emailState, err := fetchEmailState(cli, session)
	if err != nil {
		return fmt.Errorf("seed email state: %w", err)
	}

	cache, err := lru.New[string, []byte](bodyCacheSize)
	if err != nil {
		return fmt.Errorf("body cache: %w", err)
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
	upBlob := func(data []byte) (string, error) {
		resp, err := cli.Upload(accountID, bytes.NewReader(data))
		if err != nil {
			return "", err
		}
		return string(resp.ID), nil
	}

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
	b.uploadBlob = upBlob
	b.pushClient = cli
	b.runEventSourceFunc = b.runEventSource
	b.pushCancel = cancel
	b.pushDone = done
	b.mu.Unlock()

	go b.pushLoop(ctx)

	return nil
}

// fetchEmailState issues Email/get with a single sentinel ID to cheaply
// read the current Email state string. An empty IDs slice would be
// dropped by omitempty (becoming "fetch all"), which Fastmail rejects
// on large mailboxes. A sentinel lands in notFound and the server
// still returns the live state.
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

// fetchFolders issues Mailbox/get and returns a populated folder map
// keyed by canonical poplar name, plus the Mailbox state string.
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

// refreshFoldersLocked refreshes b.folders and b.states["Mailbox"].
// Caller holds b.mu.
func (b *Backend) refreshFoldersLocked() error {
	folders, state, err := fetchFolders(b.client, b.session)
	if err != nil {
		return err
	}
	b.folders = folders
	b.states["Mailbox"] = state
	return nil
}

// Disconnect cancels the push loop and tears down session state.
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
	b.folders = make(map[string]folderEntry)
	b.blobIDs = make(map[mail.UID]string)
	b.partBlobIDs = make(map[mail.UID]map[string]string)
	b.states = make(map[string]string)
	b.bodies = nil
	return nil
}

// ListFolders returns the cached folder map, refreshing from the
// server when empty.
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

// OpenFolder validates name against the cached folder map.
func (b *Backend) OpenFolder(name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.folders[name]; !ok {
		return fmt.Errorf("open folder: unknown folder %q", name)
	}
	return nil
}

// QueryFolder runs Email/query sorted by receivedAt descending.
// offset and limit map directly to JMAP Position and Limit.
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

	resp, err := b.do(req)
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

// headerFetchChunk caps IDs per Email/get to stay under typical
// maxObjectsInGet limits and matches the baseline pull page size.
const headerFetchChunk = 500

// FetchHeaders chunks Email/get at headerFetchChunk and caches each
// blobID so FetchBody can download without a second Email/get.
func (b *Backend) FetchHeaders(uids []mail.UID) ([]mail.MessageInfo, error) {
	if len(uids) == 0 {
		return nil, nil
	}

	b.mu.Lock()
	accountID := b.session.PrimaryAccounts[jmapmail.URI]
	b.mu.Unlock()

	out := make([]mail.MessageInfo, 0, len(uids))
	for start := 0; start < len(uids); start += headerFetchChunk {
		end := start + headerFetchChunk
		if end > len(uids) {
			end = len(uids)
		}
		batch, err := b.fetchHeadersChunk(accountID, uids[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, batch...)
	}
	return out, nil
}

func (b *Backend) fetchHeadersChunk(accountID jmap.ID, uids []mail.UID) ([]mail.MessageInfo, error) {
	ids := make([]jmap.ID, len(uids))
	for i, u := range uids {
		ids[i] = jmap.ID(u)
	}
	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	req.Invoke(&email.Get{
		Account:    accountID,
		IDs:        ids,
		Properties: headerProperties,
	})
	resp, err := b.do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch headers: %w", err)
	}
	for _, inv := range resp.Responses {
		gr, ok := inv.Args.(*email.GetResponse)
		if !ok {
			continue
		}
		out := make([]mail.MessageInfo, 0, len(gr.List))
		for _, e := range gr.List {
			uid := mail.UID(e.ID)
			b.mu.Lock()
			b.blobIDs[uid] = string(e.BlobID)
			b.mu.Unlock()
			out = append(out, translateEmail(e))
		}
		return out, nil
	}
	return nil, fmt.Errorf("fetch headers: no Email/get response")
}

func translateEmail(e *email.Email) mail.MessageInfo {
	info := mail.MessageInfo{
		UID:      mail.UID(e.ID),
		Subject:  e.Subject,
		From:     formatFromList(e.From),
		To:       formatFromList(e.To),
		Cc:       formatFromList(e.CC),
		Bcc:      formatFromList(e.BCC),
		Flags:    translateKeywords(e.Keywords),
		Size:     uint32(e.Size),
		ThreadID: mail.UID(e.ThreadID),
	}
	if len(e.InReplyTo) > 0 {
		info.InReplyTo = mail.UID(e.InReplyTo[0])
	}
	if e.ReceivedAt != nil {
		info.SentAt = *e.ReceivedAt
	}
	return info
}

// formatFromList joins addresses with ", " preferring display name
// over bare email.
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

// trimSurroundingQuotes strips the literal single-quote pair some
// mailing-list software (Google Groups) wraps around the leading
// phrase, as in "'DAVE JOHNSON' via Members". The quotes are data,
// not RFC 5322 syntax, but they offset visual alignment. Names that
// don't start with a single quote ("Joe O'Brien") are unchanged.
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

// FetchBody downloads the raw RFC 822 body for uid through an LRU
// cache, collapsing concurrent callers via singleflight. FetchHeaders
// must run first to populate the blobID map.
func (b *Backend) FetchBody(uid mail.UID) ([]byte, error) {
	b.mu.Lock()
	blobID, ok := b.blobIDs[uid]
	cache := b.bodies
	dl := b.downloadBlob
	b.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("fetch body: unknown uid %q (call FetchHeaders first)", uid)
	}

	if buf, hit := cache.Get(blobID); hit {
		return buf, nil
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
	return v.([]byte), nil
}

// Move patches mailboxIds so each uid lives only in destFolder.
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
	resp, err := b.do(req)
	if err != nil {
		return fmt.Errorf("move: %w", err)
	}
	if err := checkEmailSetUpdated(resp, callID); err != nil {
		return fmt.Errorf("move rejected by server: %w", err)
	}
	return nil
}

// Destroy permanently deletes uids via Email/set destroy, bypassing
// Trash. Empty input is a no-op.
func (b *Backend) Destroy(uids []mail.UID) error {
	if len(uids) == 0 {
		return nil
	}
	b.mu.Lock()
	accountID := b.accountIDLocked()
	b.mu.Unlock()

	ids := make([]jmap.ID, 0, len(uids))
	for _, u := range uids {
		ids = append(ids, jmap.ID(u))
	}
	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	callID := req.Invoke(&email.Set{
		Account: accountID,
		Destroy: ids,
	})
	resp, err := b.do(req)
	if err != nil {
		return fmt.Errorf("destroy: %w", err)
	}
	if err := checkEmailSetDestroyed(resp, callID); err != nil {
		return fmt.Errorf("destroy rejected by server: %w", err)
	}
	return nil
}

// checkEmailSetDestroyed treats notFound as success (already gone)
// and returns an error for any other NotDestroyed entry.
func checkEmailSetDestroyed(resp *jmap.Response, callID string) error {
	for _, inv := range resp.Responses {
		if inv.CallID != callID {
			continue
		}
		sr, ok := inv.Args.(*email.SetResponse)
		if !ok {
			continue
		}
		for id, se := range sr.NotDestroyed {
			if se.Type == "notFound" {
				continue
			}
			return fmt.Errorf("not destroyed %s: %s", id, se.Type)
		}
		return nil
	}
	return fmt.Errorf("Email/set: no response for destroy")
}

func (b *Backend) Flag(uids []mail.UID, flag mail.Flag, set bool) error {
	keyword, err := keywordForFlag(flag)
	if err != nil {
		return err
	}
	return b.setKeyword(uids, keyword, set)
}

// Send batches Email/import (Sent mailbox) and EmailSubmission/set
// in one request so server-side Sent placement is atomic with
// submission.
func (b *Backend) Send(env mail.Envelope, mime []byte) error {
	b.mu.Lock()
	upload := b.uploadBlob
	sent, hasSent := b.folders["Sent"]
	accountID := b.accountIDLocked()
	b.mu.Unlock()
	if upload == nil {
		return errors.New("jmap: not connected")
	}
	if !hasSent {
		return errors.New("jmap: send: no Sent mailbox on account")
	}

	identityID, err := b.resolveIdentityID(accountID, env.From)
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}

	blobID, err := upload(mime)
	if err != nil {
		return fmt.Errorf("send: upload: %w", classifyErr(err))
	}

	rcpt := make([]*emailsubmission.Address, 0, len(env.Rcpts))
	for _, r := range env.Rcpts {
		rcpt = append(rcpt, &emailsubmission.Address{Email: r})
	}
	now := time.Now().UTC()

	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI, emailsubmission.URI}}
	importCallID := req.Invoke(&email.Import{
		Account: accountID,
		Emails: map[string]*email.EmailImport{
			"k1": {
				BlobID:     jmap.ID(blobID),
				MailboxIDs: map[jmap.ID]bool{jmap.ID(sent.id): true},
				Keywords:   map[string]bool{"$seen": true},
				ReceivedAt: &now,
			},
		},
	})
	subCallID := req.Invoke(&emailsubmission.Set{
		Account: accountID,
		Create: map[jmap.ID]*emailsubmission.EmailSubmission{
			"s1": {
				IdentityID: identityID,
				EmailID:    jmap.ID("#k1"),
				Envelope: &emailsubmission.Envelope{
					MailFrom: &emailsubmission.Address{Email: env.From},
					RcptTo:   rcpt,
				},
			},
		},
	})

	resp, err := b.do(req)
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}
	if err := checkImportCreated(resp, importCallID); err != nil {
		return fmt.Errorf("send: import: %w", err)
	}
	if err := checkSubmissionCreated(resp, subCallID); err != nil {
		return fmt.Errorf("send: submission: %w", err)
	}
	return nil
}

func (b *Backend) Append(folder string, mime []byte, flags mail.Flag) error {
	b.mu.Lock()
	upload := b.uploadBlob
	entry, ok := b.folders[folder]
	accountID := b.accountIDLocked()
	b.mu.Unlock()
	if upload == nil {
		return errors.New("jmap: not connected")
	}
	if !ok {
		return fmt.Errorf("append: unknown folder %q", folder)
	}

	blobID, err := upload(mime)
	if err != nil {
		return fmt.Errorf("append: upload: %w", classifyErr(err))
	}
	now := time.Now().UTC()
	keywords := map[string]bool{}
	for bit, kw := range jmapKeywords {
		if flags&bit != 0 {
			keywords[kw] = true
		}
	}

	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	callID := req.Invoke(&email.Import{
		Account: accountID,
		Emails: map[string]*email.EmailImport{
			"k1": {
				BlobID:     jmap.ID(blobID),
				MailboxIDs: map[jmap.ID]bool{jmap.ID(entry.id): true},
				Keywords:   keywords,
				ReceivedAt: &now,
			},
		},
	})

	resp, err := b.do(req)
	if err != nil {
		return fmt.Errorf("append: %w", err)
	}
	return checkImportCreated(resp, callID)
}

// PushDraft uploads mime as a draft and, when prevUID is non-empty,
// destroys the prior server image in the same request. The destroy
// is best-effort and does not block returning the new UID.
func (b *Backend) PushDraft(folder string, mime []byte, prevUID mail.UID) (mail.UID, error) {
	b.mu.Lock()
	upload := b.uploadBlob
	entry, ok := b.folders[folder]
	accountID := b.accountIDLocked()
	b.mu.Unlock()
	if upload == nil {
		return "", errors.New("jmap: not connected")
	}
	if !ok {
		return "", fmt.Errorf("push-draft: unknown folder %q", folder)
	}

	blobID, err := upload(mime)
	if err != nil {
		return "", fmt.Errorf("push-draft: upload: %w", classifyErr(err))
	}
	now := time.Now().UTC()

	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	importCallID := req.Invoke(&email.Import{
		Account: accountID,
		Emails: map[string]*email.EmailImport{
			"k1": {
				BlobID:     jmap.ID(blobID),
				MailboxIDs: map[jmap.ID]bool{jmap.ID(entry.id): true},
				Keywords:   map[string]bool{"$draft": true},
				ReceivedAt: &now,
			},
		},
	})
	var destroyCallID string
	if prevUID != "" {
		destroyCallID = req.Invoke(&email.Set{
			Account: accountID,
			Destroy: []jmap.ID{jmap.ID(prevUID)},
		})
	}

	resp, err := b.do(req)
	if err != nil {
		return "", fmt.Errorf("push-draft: %w", classifyErr(err))
	}

	newID, err := importedID(resp, importCallID)
	if err != nil {
		return "", fmt.Errorf("push-draft: %w", err)
	}

	if destroyCallID != "" {
		if err := checkEmailSetDestroyed(resp, destroyCallID); err != nil {
			fmt.Fprintln(os.Stderr, "push-draft: destroy prior:", err)
		}
	}

	return mail.UID(newID), nil
}

func importedID(resp *jmap.Response, callID string) (jmap.ID, error) {
	for _, inv := range resp.Responses {
		if inv.CallID != callID {
			continue
		}
		ir, ok := inv.Args.(*email.ImportResponse)
		if !ok {
			continue
		}
		if se, bad := ir.NotCreated["k1"]; bad {
			return "", fmt.Errorf("import rejected: %s", se.Type)
		}
		e, ok := ir.Created["k1"]
		if !ok {
			return "", errors.New("import: no Created entry")
		}
		return e.ID, nil
	}
	return "", errors.New("Email/import: no response")
}

// jmapKeywords maps the mail.Flag bits JMAP recognizes to their
// $-prefixed keyword names.
var jmapKeywords = map[mail.Flag]string{
	mail.FlagSeen:      "$seen",
	mail.FlagFlagged:   "$flagged",
	mail.FlagAnswered:  "$answered",
	mail.FlagDraft:     "$draft",
	mail.FlagForwarded: "$forwarded",
}

// resolveIdentityID returns the JMAP identity ID for the given email,
// consulting the per-email cache first. On a miss it issues Identity/get
// once and populates the cache for every identity the server returns, so
// subsequent sends from any identity on this account skip the probe.
func (b *Backend) resolveIdentityID(accountID jmap.ID, email string) (jmap.ID, error) {
	key := strings.ToLower(email)

	b.mu.Lock()
	if id, ok := b.identityIDs[key]; ok {
		b.mu.Unlock()
		return id, nil
	}
	b.mu.Unlock()

	req := &jmap.Request{Using: []jmap.URI{emailsubmission.URI}}
	callID := req.Invoke(&identity.Get{Account: accountID})
	resp, err := b.do(req)
	if err != nil {
		return "", fmt.Errorf("identity/get: %w", err)
	}
	for _, inv := range resp.Responses {
		if inv.CallID != callID {
			continue
		}
		gr, ok := inv.Args.(*identity.GetResponse)
		if !ok {
			continue
		}
		b.mu.Lock()
		for _, id := range gr.List {
			b.identityIDs[strings.ToLower(id.Email)] = id.ID
		}
		got, ok := b.identityIDs[key]
		b.mu.Unlock()
		if ok {
			return got, nil
		}
		return "", fmt.Errorf("identity/get: no identity for %q", email)
	}
	return "", fmt.Errorf("identity/get: empty response")
}

func checkImportCreated(resp *jmap.Response, callID string) error {
	for _, inv := range resp.Responses {
		if inv.CallID != callID {
			continue
		}
		ir, ok := inv.Args.(*email.ImportResponse)
		if !ok {
			continue
		}
		if se, bad := ir.NotCreated["k1"]; bad {
			return fmt.Errorf("import rejected: %s", se.Type)
		}
		if _, ok := ir.Created["k1"]; !ok {
			return errors.New("import: no Created entry")
		}
		return nil
	}
	return errors.New("Email/import: no response")
}

func checkSubmissionCreated(resp *jmap.Response, callID string) error {
	for _, inv := range resp.Responses {
		if inv.CallID != callID {
			continue
		}
		sr, ok := inv.Args.(*emailsubmission.SetResponse)
		if !ok {
			continue
		}
		if se, bad := sr.NotCreated["s1"]; bad {
			return fmt.Errorf("submission rejected: %s", se.Type)
		}
		if _, ok := sr.Created["s1"]; !ok {
			return errors.New("submission: no Created entry")
		}
		return nil
	}
	return errors.New("EmailSubmission/set: no response")
}

// accountIDLocked returns the primary JMAP account ID. Caller holds b.mu.
func (b *Backend) accountIDLocked() jmap.ID {
	return b.session.PrimaryAccounts[jmapmail.URI]
}

func keywordForFlag(flag mail.Flag) (string, error) {
	if kw, ok := jmapKeywords[flag]; ok {
		return kw, nil
	}
	return "", fmt.Errorf("unsupported flag for JMAP: %v", flag)
}

// setKeyword patches keyword to true (set) or nil (unset) on all
// uids in one Email/set request.
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
	resp, err := b.do(req)
	if err != nil {
		return fmt.Errorf("set keyword %s: %w", keyword, err)
	}
	if err := checkEmailSetUpdated(resp, callID); err != nil {
		return fmt.Errorf("keyword %s rejected: %w", keyword, err)
	}
	return nil
}

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
	return fmt.Errorf("Email/set: no response for update")
}
