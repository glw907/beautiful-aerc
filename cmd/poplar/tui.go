package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/colorprofile"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/outbox"
	"github.com/glw907/poplar/internal/platform"
	"github.com/glw907/poplar/internal/store"
	syncengine "github.com/glw907/poplar/internal/sync"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/uerr"
	"github.com/glw907/poplar/internal/ui"
)

// runInteractive is cmd/poplar's TUI entry point (task 11): the same
// startup steps run's own headless preamble takes (the instance lock,
// store preparation, the writer, the orphaned-intent sweep, the log
// health report), then a *tea.Program built over ui.NewApp, the
// sync/outbox bridge wired to its Send, and the program run to
// completion. The instance lock and SY-7's refusal happen before
// anything about the TUI is touched, exactly as they do in run.
//
// A fatal connect failure (a rejected or missing credential) is
// reported and returned before the program is ever constructed, the
// same as run's own headless path: a credential problem is not
// something to surface behind a rendered frame. Any other connect
// failure is SY-3's own case: the program starts anyway, offline, and
// retries in the background (ST-2).
func runInteractive(ctx context.Context, dbPath string, f flags, out, errOut io.Writer, connect backendConnector) error {
	lock, err := platform.AcquireInstanceLock(dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()

	if err := prepareStore(ctx, dbPath, f, out); err != nil {
		return err
	}

	writer, err := store.Open(dbPath, store.DefaultWriterConfig())
	if err != nil {
		return err
	}

	if err := outbox.ReclaimOrphaned(ctx, writer); err != nil {
		_ = writer.Close()
		return err
	}

	if f.rebuildIndex {
		_, _ = fmt.Fprintln(out, "rebuilding full-text index...")
		if err := store.RebuildIndex(ctx, writer); err != nil {
			_ = writer.Close()
			return err
		}
	}

	slog.Info("poplar: store ready", "path", dbPath)
	reportLogHealth(errOut)

	reads, err := store.NewReadPool(dbPath, store.DefaultReadPoolSize, writer.Revision())
	if err != nil {
		_ = writer.Close()
		return err
	}

	be, key, connectErr := connect(ctx)
	if connectErr != nil && isFatalConnect(connectErr) {
		_ = reads.Close()
		_ = writer.Close()
		return surfaceFatalConnect(connectErr)
	}

	profile, _ := ui.ResolveProfile(os.LookupEnv)
	app := ui.NewApp(ui.Deps{Store: reads, Theme: theme.New(ui.DefaultDark, profile), Profile: profile, Account: key})
	program := ui.NewProgram(app, tea.WithColorProfile(mapColorProfile(profile)))

	// Send blocks on program's own unbuffered message channel until
	// its event loop starts reading, which has not happened yet at
	// this point in runInteractive; both sends below run on their own
	// goroutine so constructing them here cannot deadlock program.Run
	// a few lines down.
	if path, ok := uerr.LogFallbackPath(); ok {
		go program.Send(ui.BannerMsg{Message: fmt.Sprintf("state directory unavailable; logging to %s instead", path)})
	}

	engineCtx, cancelEngines := context.WithCancel(context.Background())
	var wg *sync.WaitGroup
	switch connectErr {
	case nil:
		accountID, aerr := ensureAccount(ctx, writer, key)
		if aerr != nil {
			cancelEngines()
			_ = reads.Close()
			_ = writer.Close()
			return aerr
		}
		wg = startEnginesInteractive(engineCtx, accountID, be, writer, reads, program.Send)
	default:
		// sync.State has no case for a connect that has not succeeded
		// even once, since Worker's own loop always retries rather
		// than giving up (bridge.go's bridgeSyncState); ST-2's offline
		// state belongs here instead, one send since nothing about it
		// changes again until retryConnect actually reaches a live
		// stream and bridgeSyncHealth's own observer takes over.
		go program.Send(ui.SyncStateMsg{State: ui.SyncStateOffline})
		wg = startEnginesRetryingInteractive(engineCtx, writer, reads, connect, connectErr, program.Send)
	}

	_, runErr := program.Run()
	if errors.Is(runErr, tea.ErrInterrupted) {
		// A SIGINT program.Run reports through ErrInterrupted (bubbletea's
		// own default signal handler, left enabled) is an ordinary way to
		// stop a terminal program, not a failure to report and exit 1 on.
		runErr = nil
	}

	// The engines' own goroutines (startEnginesInteractive's RunPush and
	// dispatch loop) may still be mid-tick here, calling program.Send
	// after program.Run has already returned: bubbletea v2.0.9's own
	// Send selects on the program's context, already done at this
	// point, and returns immediately rather than blocking on the
	// message channel nothing reads anymore, so this is safe with no
	// guard needed.
	cancelEngines()
	wg.Wait()

	closeErr := reads.Close()
	if err := writer.Close(); err != nil {
		return errors.Join(closeErr, runErr, err)
	}
	reportLogHealth(errOut)
	if runErr != nil {
		return errors.Join(closeErr, runErr)
	}
	return errors.Join(closeErr, store.MarkCleanShutdown(dbPath))
}

// mapColorProfile maps profile, ResolveProfile's own runtime
// capability tier, onto the colorprofile.Profile tea.WithColorProfile
// expects (CARRY 1, task 2's own carried review finding): without it,
// bubbletea's own terminal auto-detection re-downsamples the theme's
// already-resolved values against whatever it independently guesses,
// discarding ResolveProfile's own NO_COLOR/TERM/COLORTERM precedence
// and the config override seam layered on top of it.
func mapColorProfile(profile theme.Profile) colorprofile.Profile {
	switch profile {
	case theme.ProfileTrueColor:
		return colorprofile.TrueColor
	case theme.ProfileNoColor:
		return colorprofile.Ascii
	default:
		return colorprofile.ANSI
	}
}

// startEnginesInteractive is startEngines' own TUI counterpart
// (engine.go): the same sync worker and outbox dispatcher, with
// bridgeSyncHealth installed on the worker before RunPush's goroutine
// starts (its own set-before-run contract) and the dispatch loop's
// own outbox-count bridge riding its existing tick (CARRY 5).
func startEnginesInteractive(ctx context.Context, accountID int64, be backend.Backend, writer *store.Writer, reads *store.ReadPool, send func(tea.Msg)) *sync.WaitGroup {
	worker := syncengine.NewWorker(accountID, be, writer, syncengine.DefaultConfig())
	bridgeSyncHealth(worker, send)
	dispatcher := outbox.NewDispatcher(accountID, be, writer, reads)

	var wg sync.WaitGroup
	wg.Go(func() {
		worker.RunPush(ctx, []backend.ObjectKind{backend.ObjectKindMailbox, backend.ObjectKindMessage})
	})
	wg.Go(func() {
		runDispatchLoopBridged(ctx, dispatcher, reads, send)
	})
	return &wg
}

// startEnginesRetryingInteractive is startEnginesRetrying's own TUI
// counterpart (engine.go): retryConnect's same background retry loop,
// starting the bridged engines once it succeeds.
func startEnginesRetryingInteractive(ctx context.Context, writer *store.Writer, reads *store.ReadPool, connect backendConnector, firstErr error, send func(tea.Msg)) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Go(func() {
		be, key, ok := retryConnect(ctx, connect, firstErr)
		if !ok {
			return
		}
		accountID, err := ensureAccount(ctx, writer, key)
		if err != nil {
			return
		}
		startEnginesInteractive(ctx, accountID, be, writer, reads, send).Wait()
	})
	return &wg
}

// runDispatchLoopBridged is runDispatchLoop's own TUI counterpart
// (engine.go): the same immediate-then-ticked DispatchOnce cadence,
// with bridgeOutboxCount riding it (CARRY 5, task 11) rather than a
// poll of its own, so a triage action's outbox count reaches the
// status line within one dispatchInterval tick and never busier than
// that (QA-8's idle posture).
func runDispatchLoopBridged(ctx context.Context, d *outbox.Dispatcher, reads *store.ReadPool, send func(tea.Msg)) {
	last := bridgeOutboxCount(ctx, reads, 0, send)
	dispatchOnce(ctx, d)
	last = bridgeOutboxCount(ctx, reads, last, send)

	ticker := time.NewTicker(dispatchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			dispatchOnce(ctx, d)
			last = bridgeOutboxCount(ctx, reads, last, send)
		case <-ctx.Done():
			return
		}
	}
}
