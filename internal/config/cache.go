// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// CacheConfig holds tunables for the local cache. Populated from
// the [cache] table in config.toml. The zero value is "no body-cache
// size cap"; LoadCache substitutes 2GB when [cache] is absent.
type CacheConfig struct {
	// MaxSize is the body-cache size cap in bytes. 0 disables.
	MaxSize int64
}

// DefaultCacheConfig returns the defaults applied when [cache] is
// missing from config.toml. Currently 2GB body-cache cap.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{MaxSize: 2 * 1024 * 1024 * 1024}
}

type rawCacheFile struct {
	Cache *rawCache `toml:"cache"`
}

type rawCache struct {
	MaxSize string `toml:"max-size"`
}

// LoadCache reads the [cache] table from a config.toml file. A
// missing file or missing [cache] section returns DefaultCacheConfig.
func LoadCache(path string) (CacheConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultCacheConfig(), nil
		}
		return CacheConfig{}, fmt.Errorf("reading cache config: %w", err)
	}
	var raw rawCacheFile
	if err := toml.Unmarshal(data, &raw); err != nil {
		return CacheConfig{}, fmt.Errorf("parsing cache config: %w", err)
	}
	if raw.Cache == nil {
		return DefaultCacheConfig(), nil
	}
	out := DefaultCacheConfig()
	if raw.Cache.MaxSize != "" {
		n, err := parseSize(raw.Cache.MaxSize)
		if err != nil {
			return CacheConfig{}, fmt.Errorf("cache.max-size: %w", err)
		}
		out.MaxSize = n
	}
	return out, nil
}

// parseSize parses a size string with optional KB/MB/GB/TB suffix
// (1024-based). An empty string returns 0. Negative values error.
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	upper := strings.ToUpper(s)
	mult := int64(1)
	switch {
	case strings.HasSuffix(upper, "TB"):
		mult = 1024 * 1024 * 1024 * 1024
		s = s[:len(s)-2]
	case strings.HasSuffix(upper, "GB"):
		mult = 1024 * 1024 * 1024
		s = s[:len(s)-2]
	case strings.HasSuffix(upper, "MB"):
		mult = 1024 * 1024
		s = s[:len(s)-2]
	case strings.HasSuffix(upper, "KB"):
		mult = 1024
		s = s[:len(s)-2]
	case len(upper) >= 1 && upper[len(upper)-1] == 'B' && (len(upper) < 2 || (upper[len(upper)-2] < '0' || upper[len(upper)-2] > '9')):
		// Reject single suffix letter not in {K,M,G,T}: "5XB" → bad.
		return 0, fmt.Errorf("invalid size suffix in %q", s)
	}
	s = strings.TrimSpace(s)
	// Try integer first, then float for "1.5GB" cases.
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		if v < 0 {
			return 0, fmt.Errorf("size cannot be negative: %q", s)
		}
		return v * mult, nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		if f < 0 {
			return 0, fmt.Errorf("size cannot be negative: %q", s)
		}
		return int64(f * float64(mult)), nil
	}
	return 0, fmt.Errorf("cannot parse size %q (want NUMBER, NUMBER KB, MB, GB, or TB)", s)
}
