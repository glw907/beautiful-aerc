package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/glw907/poplar/internal/tidy"
)

// UIConfig holds the [ui] table values.
type UIConfig struct {
	// Threading is the default threading state for folders without an
	// override. Defaults to true.
	Threading bool

	// Folders maps canonical name (Inbox, Drafts, Sent, Archive, Spam,
	// Trash) or literal provider name to its overrides.
	Folders map[string]FolderConfig

	// Icons is "auto" (default), "simple", or "fancy" (ADR-0084).
	Icons string

	// UndoSeconds clamps to [2, 30] at parse. Default 6.
	UndoSeconds int

	// UndoSendWindow is the hold duration before queued mail is dispatched.
	// Range [0, 5m]. Zero disables the hold (immediate dispatch). Default 10s.
	UndoSendWindow time.Duration

	// TrashRetentionDays and SpamRetentionDays drive per-session
	// retention sweeps. 0 disables. Each clamps to [0, 365].
	TrashRetentionDays int
	SpamRetentionDays  int

	// DownloadDir resolution order: explicit [ui] download_dir,
	// $XDG_DOWNLOAD_DIR, $HOME/Downloads.
	DownloadDir string

	Tidy TidyConfig
}

// TidyConfig carries the [ui.tidy] block. Enabled gates the Ctrl+T
// binding; Config is the package-native settings struct.
type TidyConfig struct {
	Enabled bool
	Config  tidy.Config
}

// FolderConfig holds the overrides from one [ui.folders.<name>]
// subsection. Zero-value fields fall back to the group default.
type FolderConfig struct {
	// Rank is the within-group sort key. Lower sorts first. Ties
	// break on display name. RankSet separates "explicit 0" from
	// "unset".
	Rank    int
	RankSet bool

	// Label overrides the display name.
	Label string

	// ThreadingSet separates "explicit false" from "unset".
	Threading    bool
	ThreadingSet bool

	// Sort is "date-asc", "date-desc", or empty (defaults to
	// date-desc).
	Sort string

	// Hide drops the folder from the sidebar.
	Hide bool
}

// DefaultUIConfig is the fallback for a config.toml with no [ui]
// section.
func DefaultUIConfig() UIConfig {
	return UIConfig{
		Threading:      true,
		Folders:        map[string]FolderConfig{},
		Icons:          "auto",
		UndoSeconds:    6,
		UndoSendWindow: 10 * time.Second,
		DownloadDir:    defaultDownloadDir(),
		Tidy:           TidyConfig{Enabled: false, Config: tidy.DefaultConfig()},
	}
}

// rawUI uses pointers for optional booleans so "unset" reads
// differently from "explicit false".
type rawUI struct {
	Threading          *bool                   `toml:"threading"`
	Folders            map[string]rawFolderCfg `toml:"folders"`
	Icons              string                  `toml:"icons"`
	UndoSeconds        *int                    `toml:"undo_seconds"`
	UndoSendWindow     *string                 `toml:"undo-send-window"`
	TrashRetentionDays *int                    `toml:"trash_retention_days"`
	SpamRetentionDays  *int                    `toml:"spam_retention_days"`
	DownloadDir        string                  `toml:"download_dir"`
	Tidy               *rawTidy                `toml:"tidy"`
}

type rawTidy struct {
	Enabled *bool        `toml:"enabled"`
	API     *rawTidyAPI  `toml:"api"`
	Rules   *rawTidyRule `toml:"rules"`
	Style   *rawTidySty  `toml:"style"`
}

type rawTidyAPI struct {
	Model  *string `toml:"model"`
	APIKey *string `toml:"api_key"`
}

type rawTidyRule struct {
	Spelling           *bool   `toml:"spelling"`
	Grammar            *bool   `toml:"grammar"`
	Punctuation        *bool   `toml:"punctuation"`
	Whitespace         *bool   `toml:"whitespace"`
	Capitalization     *bool   `toml:"capitalization"`
	RepeatedWords      *bool   `toml:"repeated_words"`
	MissingPunctuation *bool   `toml:"missing_punctuation"`
	OxfordComma        *string `toml:"oxford_comma"`
}

type rawTidySty struct {
	EmDashSpaces       *bool    `toml:"em_dash_spaces"`
	Ellipsis           *string  `toml:"ellipsis"`
	TimeFormat         *string  `toml:"time_format"`
	CustomInstructions []string `toml:"custom_instructions"`
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

// LoadUI reads the [ui] table. A missing file is an error. A missing
// [ui] section returns DefaultUIConfig.
func LoadUI(path string) (UIConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return UIConfig{}, fmt.Errorf("read %s: %v", path, err)
	}

	var raw rawUIFile
	if err := toml.Unmarshal(data, &raw); err != nil {
		return UIConfig{}, fmt.Errorf("decode [ui]: %v", err)
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

	if raw.UI.UndoSendWindow != nil {
		d, err := time.ParseDuration(*raw.UI.UndoSendWindow)
		if err != nil {
			return UIConfig{}, fmt.Errorf("ui.undo-send-window: %v", err)
		}
		if d < 0 || d > 5*time.Minute {
			return UIConfig{}, fmt.Errorf("ui.undo-send-window: %v out of range [0s, 5m]", d)
		}
		out.UndoSendWindow = d
	}

	if raw.UI.TrashRetentionDays != nil {
		out.TrashRetentionDays = clampRetention(*raw.UI.TrashRetentionDays)
	}
	if raw.UI.SpamRetentionDays != nil {
		out.SpamRetentionDays = clampRetention(*raw.UI.SpamRetentionDays)
	}

	if raw.UI.DownloadDir != "" {
		expanded, err := ExpandHome(raw.UI.DownloadDir)
		if err != nil {
			return UIConfig{}, fmt.Errorf("ui.download_dir: %w", err)
		}
		out.DownloadDir = expanded
	}

	if raw.UI.Tidy != nil {
		tcfg := tidy.DefaultConfig()
		if r := raw.UI.Tidy.API; r != nil {
			if r.Model != nil {
				tcfg.API.Model = *r.Model
			}
			if r.APIKey != nil {
				tcfg.API.APIKey = *r.APIKey
			}
		}
		if r := raw.UI.Tidy.Rules; r != nil {
			if r.Spelling != nil {
				tcfg.Rules.Spelling = *r.Spelling
			}
			if r.Grammar != nil {
				tcfg.Rules.Grammar = *r.Grammar
			}
			if r.Punctuation != nil {
				tcfg.Rules.Punctuation = *r.Punctuation
			}
			if r.Whitespace != nil {
				tcfg.Rules.Whitespace = *r.Whitespace
			}
			if r.Capitalization != nil {
				tcfg.Rules.Capitalization = *r.Capitalization
			}
			if r.RepeatedWords != nil {
				tcfg.Rules.RepeatedWords = *r.RepeatedWords
			}
			if r.MissingPunctuation != nil {
				tcfg.Rules.MissingPunctuation = *r.MissingPunctuation
			}
			if r.OxfordComma != nil {
				tcfg.Rules.OxfordComma = *r.OxfordComma
			}
		}
		if r := raw.UI.Tidy.Style; r != nil {
			if r.EmDashSpaces != nil {
				tcfg.Style.EmDashSpaces = *r.EmDashSpaces
			}
			if r.Ellipsis != nil {
				tcfg.Style.Ellipsis = *r.Ellipsis
			}
			if r.TimeFormat != nil {
				tcfg.Style.TimeFormat = *r.TimeFormat
			}
			if r.CustomInstructions != nil {
				tcfg.Style.CustomInstructions = r.CustomInstructions
			}
		}
		if err := tidy.Validate(tcfg); err != nil {
			return UIConfig{}, fmt.Errorf("ui.tidy: %w", err)
		}
		out.Tidy.Config = tcfg
		if raw.UI.Tidy.Enabled != nil {
			out.Tidy.Enabled = *raw.UI.Tidy.Enabled
		}
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

func defaultDownloadDir() string {
	if v := os.Getenv("XDG_DOWNLOAD_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Downloads")
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
