package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/uerr"
	"github.com/glw907/poplar/internal/ui"
)

// runInteractive is cmd/poplar's TUI entry point (task 11): startup's
// shared preamble, then a *tea.Program built over ui.NewApp, the
// sync/outbox bridge wired to its Send, and the program run to
// completion. The instance lock and SY-7's refusal happen before
// anything about the TUI is touched, exactly as they do in run.
//
// A fatal connect failure (a rejected or missing credential) is
// reported and returned before the program is ever constructed, the
// same as run's headless path: a credential problem is not
// something to surface behind a rendered frame. Any other connect
// failure is SY-3's case: the program starts anyway, offline, and
// retries in the background (ST-2).
func runInteractive(ctx context.Context, dbPath string, f flags, out, errOut io.Writer, connect backendConnector) error {
	st, err := startup(ctx, dbPath, f, out, errOut)
	if err != nil {
		return err
	}
	defer func() { _ = st.lock.Release() }()
	writer := st.writer

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

	profile, isDark := ui.ResolveProfile(os.LookupEnv)
	app := ui.NewApp(ui.Deps{Store: reads, Theme: theme.New(isDark, profile), Profile: profile, Account: key})
	program := ui.NewProgram(app)

	engineCtx, cancelEngines := context.WithCancel(context.Background())
	bridge := &engineBridge{send: program.Send}
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
		wg = startEngines(engineCtx, accountID, be, writer, reads, bridge)
	default:
		wg = startEnginesRetrying(engineCtx, writer, reads, connect, connectErr, bridge)
	}

	// Both sends below run on their goroutine because program's
	// message channel is unbuffered and unread until program.Run's
	// event loop starts a few lines down; both are placed here, after
	// every pre-Run error path above has already returned, so a
	// goroutine that blocks until Run starts reading can never outlive
	// a runInteractive that returned without ever calling Run: v2.0.9's
	// Run holds the only p.cancel that would otherwise unblock Send.
	if msg, ok := logFallbackBanner(); ok {
		go program.Send(msg)
	}
	if msg := initialSyncMsg(connectErr); msg != nil {
		go program.Send(msg)
	}

	_, runErr := program.Run()
	if errors.Is(runErr, tea.ErrInterrupted) {
		// A SIGINT program.Run reports through ErrInterrupted (bubbletea's
		// default signal handler, left enabled) is an ordinary way to
		// stop a terminal program, not a failure to report and exit 1 on.
		runErr = nil
	}
	var loggedRunErr error
	if runErr != nil {
		// A program.Run failure is the binary's highest-visibility
		// crash; wrapping it here is what reaches the log before
		// reportStartupFailure's post-exit stderr report (BACKLOG #64's
		// class, extended to the TUI's case).
		loggedRunErr = uerr.New("main.tui", nil, uerr.ClassLocalIO, runErr)
	}

	// The engines' goroutines (startEngines' RunPush and dispatch
	// loop) may still be mid-tick here, calling program.Send after
	// program.Run has already returned: bubbletea v2.0.9's Send selects
	// on the program's context, already done at this point, and returns
	// immediately rather than blocking on the message channel nothing
	// reads anymore, so this is safe with no guard needed.
	cancelEngines()
	wg.Wait()

	closeErr := reads.Close()
	if err := writer.Close(); err != nil {
		return errors.Join(closeErr, loggedRunErr, err)
	}
	reportLogHealth(errOut)
	if loggedRunErr != nil {
		return errors.Join(closeErr, loggedRunErr)
	}
	return errors.Join(closeErr, store.MarkCleanShutdown(dbPath))
}

// logFallbackBanner returns the ER-3 banner runInteractive sends once
// at startup when uerr's log destination is not its normal state-dir
// home, and whether one is owed at all (dispositions row 24). A
// working fallback names the path it engaged; a fallback that failed
// its trial write too says logging is degraded rather than naming
// a path nothing reaches.
func logFallbackBanner() (ui.BannerMsg, bool) {
	if path, ok := uerr.LogFallbackPath(); ok {
		return ui.BannerMsg{Message: fmt.Sprintf("state directory unavailable; logging to %s instead", path)}, true
	}
	if uerr.LogDegraded() {
		return ui.BannerMsg{Message: "state directory unavailable and logging is degraded; some lines may be lost"}, true
	}
	return ui.BannerMsg{}, false
}

// initialSyncMsg reports the ui.Msg runInteractive sends immediately
// before program.Run reflecting connectErr's outcome: nil means
// the first connect succeeded, and bridgeSyncHealth's observer
// (installed on the worker before RunPush starts) takes over from
// here with nothing to send; any other error is ST-2's offline
// case, the state sync.State itself has no room for since Worker's
// loop always retries rather than giving up (bridge.go's
// bridgeSyncState), reported once since nothing about it changes
// again until retryConnect actually reaches a live stream.
func initialSyncMsg(connectErr error) tea.Msg {
	if connectErr == nil {
		return nil
	}
	return ui.SyncStateMsg{State: ui.SyncStateOffline}
}
