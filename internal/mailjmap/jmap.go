// Package mailjmap implements mail.Backend for JMAP servers
// (Fastmail) by calling git.sr.ht/~rockorager/go-jmap directly.
// All RPC methods are synchronous. A single goroutine owned by
// Connect/Disconnect reads JMAP push StateChange events and emits
// mail.Update values onto the channel returned by Updates().
package mailjmap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"git.sr.ht/~rockorager/go-jmap"
	jmapmail "git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"
	lru "github.com/hashicorp/golang-lru/v2"

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

	mu      sync.Mutex
	client  jmapClient
	session *jmap.Session
	current string
	folders map[string]folderEntry
	blobIDs map[mail.UID]string
	states  map[string]string

	bodies  *lru.Cache[string, []byte]
	updates chan mail.Update

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
	cache, _ := lru.New[string, []byte](bodyCacheSize)
	b.bodies = cache
	b.updates = make(chan mail.Update, updatesBuffer)
	return b
}

// AccountName satisfies mail.Backend.
func (b *Backend) AccountName() string { return b.cfg.Name }

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
func (b *Backend) Connect(_ context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	cli := &jmap.Client{
		SessionEndpoint: b.cfg.Source,
	}
	cli.WithAccessToken(b.cfg.Password)
	if err := cli.Authenticate(); err != nil {
		return fmt.Errorf("connect: authenticate: %w", err)
	}
	b.client = cli
	b.session = cli.Session

	if err := b.refreshFoldersLocked(); err != nil {
		return fmt.Errorf("connect: list folders: %w", err)
	}

	cache, err := lru.New[string, []byte](bodyCacheSize)
	if err != nil {
		return fmt.Errorf("connect: init body cache: %w", err)
	}
	b.bodies = cache
	b.updates = make(chan mail.Update, updatesBuffer)

	// Push loop wired in Task 13.

	return nil
}

// refreshFoldersLocked issues Mailbox/get, populates b.folders keyed
// by canonical poplar name, and captures the state string into
// b.states["Mailbox"]. Caller must hold b.mu.
func (b *Backend) refreshFoldersLocked() error {
	accountID := b.session.PrimaryAccounts[jmapmail.URI]

	req := &jmap.Request{}
	req.Invoke(&mailbox.Get{Account: accountID})

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("mailbox/get: %w", err)
	}

	for _, inv := range resp.Responses {
		gr, ok := inv.Args.(*mailbox.GetResponse)
		if !ok {
			continue
		}
		b.states["Mailbox"] = gr.State

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
			b.folders[key] = folderEntry{
				id:     string(gr.List[i].ID),
				folder: cf.Folder,
			}
		}
		break
	}
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

// ListFolders satisfies mail.Backend.
func (b *Backend) ListFolders() ([]mail.Folder, error) {
	return nil, errors.New("not implemented")
}

// OpenFolder satisfies mail.Backend.
func (b *Backend) OpenFolder(_ string) error {
	return errors.New("not implemented")
}

// QueryFolder satisfies mail.Backend.
func (b *Backend) QueryFolder(_ string, _, _ int) ([]mail.UID, int, error) {
	return nil, 0, errors.New("not implemented")
}

// FetchHeaders satisfies mail.Backend.
func (b *Backend) FetchHeaders(_ []mail.UID) ([]mail.MessageInfo, error) {
	return nil, errors.New("not implemented")
}

// FetchBody satisfies mail.Backend.
func (b *Backend) FetchBody(_ mail.UID) (io.Reader, error) {
	return nil, errors.New("not implemented")
}

// Search satisfies mail.Backend.
func (b *Backend) Search(_ mail.SearchCriteria) ([]mail.UID, error) {
	return nil, errors.New("not implemented")
}

// Move satisfies mail.Backend.
func (b *Backend) Move(_ []mail.UID, _ string) error {
	return errors.New("not implemented")
}

// Copy satisfies mail.Backend.
func (b *Backend) Copy(_ []mail.UID, _ string) error {
	return errors.New("not implemented")
}

// Delete satisfies mail.Backend.
func (b *Backend) Delete(_ []mail.UID) error {
	return errors.New("not implemented")
}

// Flag satisfies mail.Backend.
func (b *Backend) Flag(_ []mail.UID, _ mail.Flag, _ bool) error {
	return errors.New("not implemented")
}

// MarkRead satisfies mail.Backend.
func (b *Backend) MarkRead(_ []mail.UID) error {
	return errors.New("not implemented")
}

// MarkAnswered satisfies mail.Backend.
func (b *Backend) MarkAnswered(_ []mail.UID) error {
	return errors.New("not implemented")
}

// Send satisfies mail.Backend.
func (b *Backend) Send(_ string, _ []string, _ io.Reader) error {
	return errors.New("not implemented")
}
