// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/humanize"
	_ "modernc.org/sqlite"
)

// outboxStatsQ counts pending vs other (executing+failed+conflict) outbox rows.
var outboxStatsQ = fmt.Sprintf(`SELECT
    COALESCE(SUM(CASE WHEN status = '%s' THEN 1 ELSE 0 END), 0),
    COALESCE(SUM(CASE WHEN status IN ('%s','%s','%s') THEN 1 ELSE 0 END), 0)
FROM outbox`,
	cache.OpPending, cache.OpExecuting, cache.OpFailed, cache.OpConflict)

// newCacheCmd assembles the `poplar cache` subcommand tree.
func newCacheCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "cache",
		Short: "Inspect and prune the local mail cache",
	}
	c.AddCommand(newCacheStatsCmd())
	c.AddCommand(newCacheEvictCmd())
	c.AddCommand(newCacheVacuumCmd())
	return c
}

// statsRow is one account's worth of stats output.
type statsRow struct {
	Account       string
	HeadersCount  int64
	BodiesCount   int64
	BodiesBytes   int64
	OutboxPending int64
	OutboxOther   int64 // executing + failed + conflict
	DBBytes       int64
}

func newCacheStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Print per-account cache statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			accts, _, err := loadAccounts()
			if err != nil {
				return err
			}
			writeStatsHeader(cmd.OutOrStdout())
			for _, a := range accts {
				row, err := statsForAccount(cmd.Context(), a.Name)
				if err != nil {
					return fmt.Errorf("stats for %s: %w", a.Name, err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), formatStatsLine(row))
			}
			return nil
		},
	}
}

// statsForAccount opens the per-account SQLite directly (no Backend
// or ChangeTracker — stats works offline).
func statsForAccount(ctx context.Context, name string) (statsRow, error) {
	dbPath, err := cache.DBPath(name, "")
	if err != nil {
		return statsRow{}, err
	}
	db, err := cache.OpenDB(dbPath)
	if err != nil {
		return statsRow{}, fmt.Errorf("open cache db: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return statsRow{}, fmt.Errorf("ping cache db: %w", err)
	}
	defer db.Close()
	row := statsRow{Account: name}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages`).Scan(&row.HeadersCount); err != nil {
		return row, fmt.Errorf("count messages: %w", err)
	}
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(length(bytes)), 0) FROM bodies`).Scan(&row.BodiesCount, &row.BodiesBytes); err != nil {
		return row, fmt.Errorf("count bodies: %w", err)
	}
	if err := db.QueryRowContext(ctx, outboxStatsQ).Scan(&row.OutboxPending, &row.OutboxOther); err != nil {
		return row, fmt.Errorf("count outbox: %w", err)
	}
	if fi, err := os.Stat(dbPath); err == nil {
		row.DBBytes = fi.Size()
	}
	return row, nil
}

// loadAccounts reads account config via the existing config loader.
func loadAccounts() ([]config.AccountConfig, string, error) {
	accts, path, err := config.Load("")
	return accts, path, err
}

func writeStatsHeader(w io.Writer) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ACCOUNT\tHEADERS\tBODIES\tOUTBOX\tDB SIZE")
	tw.Flush()
}

func formatStatsLine(r statsRow) string {
	var sb strings.Builder
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	outbox := fmt.Sprintf("%d pending", r.OutboxPending)
	if r.OutboxOther > 0 {
		outbox = fmt.Sprintf("%d pending, %d other", r.OutboxPending, r.OutboxOther)
	}
	fmt.Fprintf(tw, "%s\t%s\t%s / %s\t%s\t%s\n",
		r.Account,
		formatThousands(r.HeadersCount),
		formatThousands(r.BodiesCount),
		humanize.Bytes(r.BodiesBytes),
		outbox,
		humanize.Bytes(r.DBBytes),
	)
	tw.Flush()
	return strings.TrimRight(sb.String(), "\n")
}

// formatThousands inserts commas as thousands separators.
func formatThousands(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// newCacheEvictCmd assembles the `poplar cache evict` subcommand.
func newCacheEvictCmd() *cobra.Command {
	var olderThan string
	var account string
	c := &cobra.Command{
		Use:          "evict",
		Short:        "Manually evict cached bodies older than a duration",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if olderThan == "" {
				return fmt.Errorf("--older-than is required (e.g. 30d, 2w, 24h)")
			}
			dur, err := parseEvictDuration(olderThan)
			if err != nil {
				return err
			}
			cutoff := time.Now().Add(-dur)
			return runEvict(cmd.Context(), cmd.OutOrStdout(), cutoff, account)
		},
	}
	c.Flags().StringVar(&olderThan, "older-than", "", `Evict bodies fetched longer ago than this (e.g. "30d", "2w", "24h")`)
	c.Flags().StringVar(&account, "account", "", "Limit to one account by name (default: all accounts)")
	return c
}

// parseEvictDuration extends time.ParseDuration with day (d) and
// week (w) suffixes since cache eviction operates at coarser
// granularity than time.ParseDuration's hour ceiling.
func parseEvictDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if strings.HasPrefix(s, "-") {
		return 0, fmt.Errorf("duration must be positive")
	}
	switch s[len(s)-1] {
	case 'd':
		days, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil {
			return 0, fmt.Errorf("parse %q: %w", s, err)
		}
		return time.Duration(days * 24 * float64(time.Hour)), nil
	case 'w':
		weeks, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil {
			return 0, fmt.Errorf("parse %q: %w", s, err)
		}
		return time.Duration(weeks * 7 * 24 * float64(time.Hour)), nil
	}
	return time.ParseDuration(s)
}

// runEvict opens each account (or one if scoped) and runs EvictByAge.
// Passing nil backend/tracker is safe — Evict only touches the bodies
// table and performs no backend I/O.
func runEvict(ctx context.Context, w io.Writer, cutoff time.Time, scope string) error {
	accts, _, err := loadAccounts()
	if err != nil {
		return err
	}
	matched := false
	for _, a := range accts {
		if scope != "" && a.Name != scope {
			continue
		}
		matched = true
		acct, err := cache.Open(a.Name, nil, nil, "", cache.Config{})
		if err != nil {
			return fmt.Errorf("open %s: %w", a.Name, err)
		}
		rows, freed, evictErr := acct.EvictByAge(ctx, cutoff)
		acct.Close()
		if evictErr != nil {
			return fmt.Errorf("evict %s: %w", a.Name, evictErr)
		}
		fmt.Fprintf(w, "evicted %d bodies (%s freed) from %s\n", rows, humanize.Bytes(freed), a.Name)
	}
	if scope != "" && !matched {
		return fmt.Errorf("account %q not found", scope)
	}
	return nil
}

// newCacheVacuumCmd assembles the `poplar cache vacuum` subcommand.
func newCacheVacuumCmd() *cobra.Command {
	var account string
	c := &cobra.Command{
		Use:          "vacuum",
		Short:        "VACUUM the per-account SQLite cache (reclaim free pages after eviction)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVacuum(cmd.Context(), cmd.OutOrStdout(), account)
		},
	}
	c.Flags().StringVar(&account, "account", "", "Limit to one account by name (default: all accounts)")
	return c
}

// runVacuum opens each account's database and runs VACUUM, reporting
// before/after file sizes.
func runVacuum(ctx context.Context, w io.Writer, scope string) error {
	accts, _, err := loadAccounts()
	if err != nil {
		return err
	}
	matched := false
	for _, a := range accts {
		if scope != "" && a.Name != scope {
			continue
		}
		matched = true
		dbPath, err := cache.DBPath(a.Name, "")
		if err != nil {
			return err
		}
		var before int64
		if fi, statErr := os.Stat(dbPath); statErr == nil {
			before = fi.Size()
		}
		// VACUUM cannot run inside a transaction or with concurrent
		// writers. Use a short-lived dedicated connection with no
		// pool — single-connection bypass.
		db, err := cache.OpenDB(dbPath)
		if err != nil {
			return fmt.Errorf("open cache db: %w", err)
		}
		if err := db.Ping(); err != nil {
			db.Close()
			return fmt.Errorf("ping cache db: %w", err)
		}
		db.SetMaxOpenConns(1)
		if _, err := db.ExecContext(ctx, `VACUUM`); err != nil {
			db.Close()
			return fmt.Errorf("vacuum %s: %w", a.Name, err)
		}
		db.Close()
		var after int64
		if fi, statErr := os.Stat(dbPath); statErr == nil {
			after = fi.Size()
		}
		fmt.Fprintf(w, "vacuumed %s: %s → %s\n", a.Name, humanize.Bytes(before), humanize.Bytes(after))
	}
	if scope != "" && !matched {
		return fmt.Errorf("account %q not found", scope)
	}
	return nil
}

