// Forked from aerc (git.sr.ht/~rjarry/aerc) — MIT License
package worker

import (
	"net/url"
	"strings"

	"github.com/glw907/beautiful-aerc/internal/aercfork/worker/handlers"
	"github.com/glw907/beautiful-aerc/internal/aercfork/worker/types"
)

// NewWorker guesses the appropriate worker type based on the given source string.
func NewWorker(source string, name string) (*types.Worker, error) {
	u, err := url.Parse(source)
	if err != nil {
		return nil, err
	}
	worker := types.NewWorker(name)
	scheme := u.Scheme
	if strings.ContainsRune(scheme, '+') {
		scheme = scheme[:strings.IndexRune(scheme, '+')]
	}
	backend, err := handlers.GetHandlerForScheme(scheme, worker)
	if err != nil {
		return nil, err
	}
	worker.Backend = backend
	return worker, nil
}
