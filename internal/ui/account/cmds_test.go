package account

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/mail"
)

// blockingBackend is a test stub for mail.Backend. FetchBody blocks
// until the release channel is closed or receives a value. All other
// methods are no-ops that return zero values.
type blockingBackend struct {
	release chan struct{}

	moveErr    error
	destroyErr error
	flagErr    error
	sendErr    error
	appendErr  error
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

func (b *blockingBackend) Move(_ context.Context, _ []mail.UID, _ string) error { return b.moveErr }
func (b *blockingBackend) Destroy(_ context.Context, _ []mail.UID) error        { return b.destroyErr }

func (b *blockingBackend) Flag(_ context.Context, _ []mail.UID, _ mail.Flag, _ bool) error {
	return b.flagErr
}

func (b *blockingBackend) Send(_ context.Context, _ mail.Envelope, _ []byte) error { return b.sendErr }
func (b *blockingBackend) Append(_ context.Context, _ string, _ []byte, _ mail.Flag) error {
	return b.appendErr
}
func (b *blockingBackend) PushDraft(_ context.Context, _ string, _ []byte, _ mail.UID) (mail.UID, error) {
	return "", mail.ErrUnsupported
}
func (b *blockingBackend) IsJMAP() bool { return false }

func (b *blockingBackend) Updates() <-chan mail.Update {
	ch := make(chan mail.Update)
	return ch
}

func TestWalkBody_ExtractsInvite(t *testing.T) {
	icsData, err := os.ReadFile("../../icalendar/testdata/google_request.ics")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	// Build a minimal multipart/mixed RFC 5322 message with a text/plain
	// part and a text/calendar part.
	boundary := "test-boundary-42"
	raw := fmt.Sprintf("From: alice@example.com\r\nTo: bob@example.com\r\nSubject: Invite\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=%q\r\n\r\n--%s\r\nContent-Type: text/plain\r\n\r\nYou are invited.\r\n--%s\r\nContent-Type: text/calendar\r\n\r\n%s\r\n--%s--\r\n",
		boundary, boundary, boundary, string(icsData), boundary)

	_, _, invite := walkBody([]byte(raw))
	if invite == nil {
		t.Fatal("walkBody: invite is nil, want non-nil")
	}
	if invite.Summary != "Team Sync" {
		t.Errorf("Summary = %q, want %q", invite.Summary, "Team Sync")
	}
	if invite.Method != "REQUEST" {
		t.Errorf("Method = %q, want %q", invite.Method, "REQUEST")
	}
}

func TestWalkBody_NoInviteWhenAbsent(t *testing.T) {
	raw := "From: alice@example.com\r\nTo: bob@example.com\r\nSubject: Plain\r\nContent-Type: text/plain\r\n\r\nHello.\r\n"
	_, _, invite := walkBody([]byte(raw))
	if invite != nil {
		t.Errorf("walkBody: invite = %+v, want nil", invite)
	}
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
