package main

import (
	"errors"
	"fmt"
	"os"
)

func main() {
	installLogger()
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
