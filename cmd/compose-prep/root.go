package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/glw907/beautiful-aerc/internal/compose"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type flags struct {
	noCcBcc bool
	debug   bool
}

func newRootCmd() *cobra.Command {
	f := flags{}

	cmd := &cobra.Command{
		Use:          "compose-prep",
		Short:        "Normalize aerc compose buffers for editing",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(f)
		},
	}

	cmd.Flags().BoolVar(&f.noCcBcc, "no-cc-bcc", false, "do not inject empty Cc/Bcc headers")
	cmd.Flags().BoolVar(&f.debug, "debug", false, "write diagnostic messages to stderr")

	return cmd
}

func run(f flags) error {
	if f.debug {
		log.SetPrefix("compose-prep: ")
		log.SetFlags(0)
	} else {
		log.SetOutput(io.Discard)
	}

	if term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("no input (pipe a compose buffer to stdin)")
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	opts := compose.Options{
		InjectCcBcc: !f.noCcBcc,
	}

	output := compose.Prepare(input, opts)

	if _, err := os.Stdout.Write(output); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}
