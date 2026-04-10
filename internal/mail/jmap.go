package mail

import (
	"context"
	"fmt"
	"io"

	"github.com/glw907/beautiful-aerc/internal/aercfork/models"
	"github.com/glw907/beautiful-aerc/internal/aercfork/worker"
	"github.com/glw907/beautiful-aerc/internal/aercfork/worker/types"
	"github.com/glw907/beautiful-aerc/internal/poplar"
)

// JMAPAdapter wraps the forked aerc JMAP worker behind the Backend
// interface, bridging async message-passing to synchronous calls.
type JMAPAdapter struct {
	config  *poplar.AccountConfig
	w       *types.Worker
	updates chan Update
	done    chan struct{}
}

// NewJMAPAdapter creates a JMAP backend adapter for the given account.
func NewJMAPAdapter(config *poplar.AccountConfig) (*JMAPAdapter, error) {
	w, err := worker.NewWorker(config.Source, config.Name)
	if err != nil {
		return nil, fmt.Errorf("creating worker: %w", err)
	}
	return &JMAPAdapter{
		config:  config,
		w:       w,
		updates: make(chan Update, 50),
		done:    make(chan struct{}),
	}, nil
}

// Connect configures and connects the JMAP worker to the server.
func (a *JMAPAdapter) Connect(ctx context.Context) error {
	go a.w.Backend.Run()
	go a.pump()

	if err := a.doAction(&types.Configure{Config: a.config}); err != nil {
		return fmt.Errorf("configuring worker: %w", err)
	}
	if err := a.doAction(&types.Connect{}); err != nil {
		return fmt.Errorf("connecting: %w", err)
	}
	return nil
}

// Disconnect stops the message pump.
func (a *JMAPAdapter) Disconnect() error {
	close(a.done)
	return a.doAction(&types.Disconnect{})
}

// ListFolders returns all mail folders from the server.
func (a *JMAPAdapter) ListFolders() ([]Folder, error) {
	var folders []Folder
	err := a.doCollect(&types.ListDirectories{}, func(msg types.WorkerMessage) {
		if d, ok := msg.(*types.Directory); ok {
			folders = append(folders, translateFolder(d.Dir))
		}
	})
	if err != nil {
		return nil, fmt.Errorf("listing folders: %w", err)
	}
	return folders, nil
}

// OpenFolder selects a folder as the current working folder.
func (a *JMAPAdapter) OpenFolder(name string) error {
	return a.doAction(&types.OpenDirectory{Directory: name})
}

// FetchHeaders retrieves header info for the given message UIDs.
func (a *JMAPAdapter) FetchHeaders(uids []UID) ([]MessageInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

// FetchBody retrieves the full body of a single message.
func (a *JMAPAdapter) FetchBody(uid UID) (io.Reader, error) {
	return nil, fmt.Errorf("not implemented")
}

// Search finds messages matching the given criteria.
func (a *JMAPAdapter) Search(criteria SearchCriteria) ([]UID, error) {
	return nil, fmt.Errorf("not implemented")
}

// Move moves messages to the destination folder.
func (a *JMAPAdapter) Move(uids []UID, dest string) error {
	return fmt.Errorf("not implemented")
}

// Copy copies messages to the destination folder.
func (a *JMAPAdapter) Copy(uids []UID, dest string) error {
	return fmt.Errorf("not implemented")
}

// Delete moves messages to trash.
func (a *JMAPAdapter) Delete(uids []UID) error {
	return fmt.Errorf("not implemented")
}

// Flag sets or clears a flag on messages.
func (a *JMAPAdapter) Flag(uids []UID, flag Flag, set bool) error {
	return fmt.Errorf("not implemented")
}

// MarkRead marks messages as read.
func (a *JMAPAdapter) MarkRead(uids []UID) error {
	return fmt.Errorf("not implemented")
}

// MarkAnswered marks messages as answered.
func (a *JMAPAdapter) MarkAnswered(uids []UID) error {
	return fmt.Errorf("not implemented")
}

// Send sends a message.
func (a *JMAPAdapter) Send(from string, rcpts []string, body io.Reader) error {
	return fmt.Errorf("not implemented")
}

// Updates returns a channel of asynchronous backend updates.
func (a *JMAPAdapter) Updates() <-chan Update {
	return a.updates
}

// pump reads worker response messages and dispatches callbacks.
// Runs in its own goroutine, started by Connect.
func (a *JMAPAdapter) pump() {
	for {
		select {
		case <-a.done:
			return
		case msg := <-types.WorkerMessages:
			a.w.ProcessMessage(msg)
		}
	}
}

// doAction sends an action to the worker and blocks until Done or Error.
func (a *JMAPAdapter) doAction(msg types.WorkerMessage) error {
	ch := make(chan error, 1)
	a.w.PostAction(msg, func(resp types.WorkerMessage) {
		switch r := resp.(type) {
		case *types.Done:
			ch <- nil
		case *types.Error:
			ch <- r.Error
		case *types.ConnError:
			ch <- r.Error
		case *types.Unsupported:
			ch <- fmt.Errorf("unsupported operation")
		}
	})
	return <-ch
}

// doCollect sends an action and calls collect for each intermediate
// response before the final Done/Error.
func (a *JMAPAdapter) doCollect(msg types.WorkerMessage, collect func(types.WorkerMessage)) error {
	ch := make(chan error, 1)
	a.w.PostAction(msg, func(resp types.WorkerMessage) {
		switch r := resp.(type) {
		case *types.Done:
			ch <- nil
		case *types.Error:
			ch <- r.Error
		case *types.ConnError:
			ch <- r.Error
		case *types.Unsupported:
			ch <- fmt.Errorf("unsupported operation")
		default:
			collect(resp)
		}
	})
	return <-ch
}

func translateFolder(d *models.Directory) Folder {
	return Folder{
		Name:   d.Name,
		Exists: d.Exists,
		Unseen: d.Unseen,
		Role:   string(d.Role),
	}
}
