// SPDX-License-Identifier: MIT

package account

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/mail"
)

// blockingBackend is a test stub for mail.Backend. FetchBody blocks
// until the release channel is closed or receives a value. All other
// methods are no-ops that return zero values.
type blockingBackend struct {
	release chan struct{}
}

func (b *blockingBackend) AccountName() string  { return "test" }
func (b *blockingBackend) AccountEmail() string { return "test@example.com" }

func (b *blockingBackend) Connect(_ context.Context) error { return nil }
func (b *blockingBackend) Disconnect() error               { return nil }

func (b *blockingBackend) ListFolders() ([]mail.Folder, error) { return nil, nil }
func (b *blockingBackend) OpenFolder(_ string) error           { return nil }
func (b *blockingBackend) QueryFolder(_ string, _, _ int) ([]mail.UID, int, error) {
	return nil, 0, nil
}
func (b *blockingBackend) FetchHeaders(_ []mail.UID) ([]mail.MessageInfo, error) { return nil, nil }

func (b *blockingBackend) FetchBody(_ mail.UID) ([]byte, error) {
	<-b.release
	return []byte("body"), nil
}

func (b *blockingBackend) Attachments(_ mail.UID) ([]mail.Attachment, error) {
	return nil, nil
}
func (b *blockingBackend) FetchAttachment(_ mail.UID, _ string) ([]byte, error) {
	return nil, nil
}

func (b *blockingBackend) Move(_ []mail.UID, _ string) error { return nil }
func (b *blockingBackend) Destroy(_ []mail.UID) error        { return nil }

func (b *blockingBackend) Flag(_ []mail.UID, _ mail.Flag, _ bool) error { return nil }

func (b *blockingBackend) Send(_ mail.Envelope, _ []byte) error                    { return nil }
func (b *blockingBackend) Append(_ string, _ []byte, _ mail.Flag) error            { return nil }
func (b *blockingBackend) PushDraft(_ string, _ []byte, _ mail.UID) (mail.UID, error) {
	return "", mail.ErrUnsupported
}
func (b *blockingBackend) IsJMAP() bool { return false }

func (b *blockingBackend) Updates() <-chan mail.Update {
	ch := make(chan mail.Update)
	return ch
}

func TestLoadBodyCmd_CancelDiscardsResult(t *testing.T) {
	release := make(chan struct{})
	mock := &blockingBackend{release: release}

	acct := newTestCache(t, mock)
	ctx, cancel := context.WithCancel(context.Background())
	cmd := loadBodyCmd(ctx, acct, "uid-1")

	resultCh := make(chan tea.Msg, 1)
	go func() {
		resultCh <- cmd()
	}()

	cancel()

	select {
	case msg := <-resultCh:
		if msg != nil {
			t.Errorf("cancelled cmd returned %T (%v), want nil", msg, msg)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("cancelled cmd did not return within 500ms")
	}

	close(release)
}
