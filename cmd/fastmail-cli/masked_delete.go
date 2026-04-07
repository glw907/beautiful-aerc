package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/glw907/beautiful-aerc/internal/header"
	"github.com/glw907/beautiful-aerc/internal/jmap"
)

func newMaskedDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [address]",
		Short: "Delete a masked email address",
		Long: `Delete a Fastmail masked email address via JMAP.

The address can be provided as a positional argument or extracted from
a piped RFC 2822 message (To/Cc headers). When piped from aerc, the
command matches recipient addresses against your masked emails and
prompts for confirmation before deleting.

After deletion, future mail to the address will bounce.`,
		Example: `  # Delete by address
  fastmail-cli masked delete abc123@fastmail.com

  # From aerc (pipe current message)
  :pipe -m fastmail-cli masked delete`,
		Args: cobra.MaximumNArgs(1),
		RunE: runMaskedDelete,
	}
}

func runMaskedDelete(cmd *cobra.Command, args []string) error {
	var candidates []string

	if len(args) == 1 {
		candidates = []string{args[0]}
	} else {
		msgData, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading message: %w", err)
		}
		if len(msgData) == 0 {
			return fmt.Errorf("no address provided and no message on stdin")
		}
		candidates = header.ExtractTo(bytes.NewReader(msgData))
		if len(candidates) == 0 {
			return fmt.Errorf("no to/cc addresses found in message")
		}
	}

	s, err := newJMAPSession()
	if err != nil {
		return err
	}

	masked, err := jmap.GetMaskedEmails(s)
	if err != nil {
		return err
	}

	// Build lookup set
	maskedByEmail := make(map[string]jmap.MaskedEmail)
	for _, m := range masked {
		maskedByEmail[m.Email] = m
	}

	// Find first candidate that is a masked address
	var match jmap.MaskedEmail
	var found bool
	for _, addr := range candidates {
		if m, ok := maskedByEmail[addr]; ok {
			match = m
			found = true
			break
		}
	}

	if !found {
		fmt.Fprintln(os.Stderr, "No masked email address found in recipients.")
		return nil
	}

	// Confirm via /dev/tty
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return fmt.Errorf("opening terminal: %w", err)
	}
	defer tty.Close()
	scanner := bufio.NewScanner(tty)

	domain := match.ForDomain
	if domain == "" {
		domain = "unknown"
	}
	fmt.Fprintf(os.Stderr, "\nMasked address: %s (domain: %s)\n", match.Email, domain)
	fmt.Fprint(os.Stderr, "Delete this address? Future mail will bounce. [y/N]: ")
	scanner.Scan()
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))

	if answer != "y" && answer != "yes" {
		fmt.Fprintln(os.Stderr, "Cancelled.")
		return nil
	}

	if err := jmap.DeleteMaskedEmail(s, match.ID); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Deleted masked address: %s\n", match.Email)
	fmt.Fprint(os.Stderr, "\nPress Enter to close...")
	scanner.Scan()
	return nil
}
