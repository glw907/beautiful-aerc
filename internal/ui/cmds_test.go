// SPDX-License-Identifier: MIT

package ui

import (
	"context"
	"io"
	"strings"
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

func (b *blockingBackend) ListFolders() ([]mail.Folder, error)               { return nil, nil }
func (b *blockingBackend) OpenFolder(_ string) error                         { return nil }
func (b *blockingBackend) QueryFolder(_ string, _, _ int) ([]mail.UID, int, error) {
	return nil, 0, nil
}
func (b *blockingBackend) FetchHeaders(_ []mail.UID) ([]mail.MessageInfo, error) { return nil, nil }

func (b *blockingBackend) FetchBody(_ mail.UID) (io.Reader, error) {
	<-b.release
	return strings.NewReader("body"), nil
}

func (b *blockingBackend) Search(_ mail.SearchCriteria) ([]mail.UID, error) { return nil, nil }

func (b *blockingBackend) Move(_ []mail.UID, _ string) error { return nil }
func (b *blockingBackend) Copy(_ []mail.UID, _ string) error { return nil }
func (b *blockingBackend) Delete(_ []mail.UID) error         { return nil }
func (b *blockingBackend) Destroy(_ []mail.UID) error        { return nil }

func (b *blockingBackend) Flag(_ []mail.UID, _ mail.Flag, _ bool) error { return nil }
func (b *blockingBackend) MarkRead(_ []mail.UID) error                  { return nil }
func (b *blockingBackend) MarkUnread(_ []mail.UID) error                { return nil }
func (b *blockingBackend) MarkAnswered(_ []mail.UID) error              { return nil }

func (b *blockingBackend) Send(_ string, _ []string, _ io.Reader) error { return nil }

func (b *blockingBackend) Updates() <-chan mail.Update {
	ch := make(chan mail.Update)
	return ch
}

func TestLoadBodyCmd_CancelDiscardsResult(t *testing.T) {
	release := make(chan struct{})
	mock := &blockingBackend{release: release}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := loadBodyCmd(ctx, mock, "uid-1")

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
