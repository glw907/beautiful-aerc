// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// UIConfig holds poplar's UI tuning. Currently scoped to folder/sidebar
// behavior. Populated from the [ui] table in accounts.toml.
type UIConfig struct {
	// Threading is the default threading state for folders that do not
	// specify a per-folder override. Default true.
	Threading bool

	// Folders holds per-folder overrides keyed by canonical name for
	// canonical folders (Inbox, Drafts, Sent, Archive, Spam, Trash) or
	// literal provider name for custom folders.
	Folders map[string]FolderConfig

	// Icons is the iconography mode: "auto" (default), "simple", or
	// "fancy". See ADR-0084.
	Icons string

	// UndoSeconds is the toast/undo timer in seconds. Default 6, clamped to [2, 30] on parse.
	UndoSeconds int

	// TrashRetentionDays is the per-session sweep cutoff for Trash. 0 disables (default).
	// Clamped to [0, 365] on parse.
	TrashRetentionDays int

	// SpamRetentionDays is the per-session sweep cutoff for Spam. 0 disables (default).
	// Clamped to [0, 365] on parse.
	SpamRetentionDays int
}

// FolderConfig holds per-folder overrides from [ui.folders.<name>]
// subsections. Any field left at its zero value is treated as "unset"
// and falls back to the group default.
type FolderConfig struct {
	// Rank is the within-group sort key. Zero means "use group default".
	// Lower values sort first. Ties break on display name.
	Rank int

	// RankSet distinguishes "unset" from "explicit 0".
	RankSet bool

	// Label overrides the display name. Empty = use default
	// (canonical name for canonicals, provider name for custom).
	Label string

	// Threading overrides the global threading default when Set.
	Threading    bool
	ThreadingSet bool

	// Sort is the per-folder sort order. Empty = "date-desc".
	Sort string

	// Hide drops the folder from the sidebar entirely.
	Hide bool
}

// DefaultUIConfig returns an empty UIConfig with sensible defaults.
// Use as a fallback when accounts.toml has no [ui] section.
func DefaultUIConfig() UIConfig {
	return UIConfig{
		Threading:   true,
		Folders:     map[string]FolderConfig{},
		Icons:       "auto",
		UndoSeconds: 6,
	}
}

// rawUI is the on-disk shape of the [ui] table. It uses pointers for
// optional bool fields so we can distinguish "unset" from "explicit false".
type rawUI struct {
	Threading   *bool                   `toml:"threading"`
	Folders     map[string]rawFolderCfg `toml:"folders"`
	Icons              string                  `toml:"icons"`
	UndoSeconds        *int                    `toml:"undo_seconds"`
	TrashRetentionDays *int                    `toml:"trash_retention_days"`
	SpamRetentionDays  *int                    `toml:"spam_retention_days"`
}

type rawFolderCfg struct {
	Rank      *int   `toml:"rank"`
	Label     string `toml:"label"`
	Threading *bool  `toml:"threading"`
	Sort      string `toml:"sort"`
	Hide      bool   `toml:"hide"`
}

type rawUIFile struct {
	UI rawUI `toml:"ui"`
}

// LoadUI reads the [ui] table from an accounts.toml file and returns
// a UIConfig. A missing file is an error; a missing [ui] section
// returns DefaultUIConfig().
func LoadUI(path string) (UIConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return UIConfig{}, fmt.Errorf("reading ui config: %w", err)
	}

	var raw rawUIFile
	if err := toml.Unmarshal(data, &raw); err != nil {
		return UIConfig{}, fmt.Errorf("parsing ui config: %w", err)
	}

	out := DefaultUIConfig()
	if raw.UI.Threading != nil {
		out.Threading = *raw.UI.Threading
	}

	if raw.UI.Icons != "" {
		switch raw.UI.Icons {
		case "auto", "simple", "fancy":
			out.Icons = raw.UI.Icons
		default:
			return UIConfig{}, fmt.Errorf("ui.icons: invalid value %q (want \"auto\", \"simple\", or \"fancy\")", raw.UI.Icons)
		}
	}

	if raw.UI.UndoSeconds != nil {
		v := *raw.UI.UndoSeconds
		if v < 2 {
			v = 2
		} else if v > 30 {
			v = 30
		}
		out.UndoSeconds = v
	}

	if raw.UI.TrashRetentionDays != nil {
		out.TrashRetentionDays = clampRetention(*raw.UI.TrashRetentionDays)
	}
	if raw.UI.SpamRetentionDays != nil {
		out.SpamRetentionDays = clampRetention(*raw.UI.SpamRetentionDays)
	}

	for name, fc := range raw.UI.Folders {
		converted, err := convertFolderCfg(name, fc)
		if err != nil {
			return UIConfig{}, err
		}
		out.Folders[name] = converted
	}

	return out, nil
}

func clampRetention(v int) int {
	if v < 0 {
		return 0
	}
	if v > 365 {
		return 365
	}
	return v
}

func convertFolderCfg(name string, raw rawFolderCfg) (FolderConfig, error) {
	out := FolderConfig{
		Label: raw.Label,
		Sort:  raw.Sort,
		Hide:  raw.Hide,
	}
	if raw.Rank != nil {
		out.Rank = *raw.Rank
		out.RankSet = true
	}
	if raw.Threading != nil {
		out.Threading = *raw.Threading
		out.ThreadingSet = true
	}
	if raw.Sort != "" && raw.Sort != "date-asc" && raw.Sort != "date-desc" {
		return FolderConfig{}, fmt.Errorf(
			"ui.folders.%q: invalid sort %q (want \"date-asc\" or \"date-desc\")",
			name, raw.Sort)
	}
	return out, nil
}
