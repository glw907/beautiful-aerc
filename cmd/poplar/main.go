package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
)

func main() {
	installLogger("")
	defer logPanic()
	cmd := newRootCmd()
	cmd.AddCommand(newThemesCmd())
	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newDiagnoseCmd())
	cmd.AddCommand(newCacheCmd())
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if errors.Is(err, errUnknownReauthAccount) || errors.Is(err, errReauthAccountNotOAuth) {
			os.Exit(78)
		}
		os.Exit(1)
	}
}

// logPanic records an unrecovered panic to the slog file log before
// letting it propagate. Bubbletea recovers panics inside its event
// loop and re-raises after restoring the terminal; without this the
// stack only reaches stderr, which the user sees but the log misses.
func logPanic() {
	r := recover()
	if r == nil {
		return
	}
	slog.Error("poplar panic", "value", fmt.Sprint(r), "stack", string(debug.Stack()))
	panic(r)
}
