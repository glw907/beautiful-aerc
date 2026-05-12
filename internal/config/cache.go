package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// CacheConfig holds tunables for the local cache, decoded from the
// [cache] table. LoadCache substitutes defaults when the table is
// absent. Body and attachment caps are tracked separately so a flood
// of attachments cannot push out cached bodies and vice-versa.
// MaxAttachmentSize defaults to 2 GB.
type CacheConfig struct {
	// MaxSize caps the body cache in bytes; zero disables the cap.
	MaxSize           int64
	MaxAttachmentSize int64
	// MaxOutboxBytes rejects an outbox INSERT whose payload exceeds
	// this many bytes; zero disables the check.
	MaxOutboxBytes int64
}

// DefaultCacheConfig returns the cache caps applied when no [cache]
// table is present: body cache uncapped, attachment cache capped at
// 2 GB.
func DefaultCacheConfig() CacheConfig {
	return defaultCache()
}

func defaultCache() CacheConfig {
	return CacheConfig{
		MaxSize:           0,
		MaxAttachmentSize: 2 * 1024 * 1024 * 1024,
	}
}

type rawCacheFile struct {
	Cache *rawCache `toml:"cache"`
}

type rawCache struct {
	MaxSize           string `toml:"max-size"`
	MaxAttachmentSize string `toml:"max-attachment-size"`
	MaxOutboxBytes    string `toml:"max-outbox-bytes"`
}

// LoadCache reads the [cache] table. A missing file or missing
// [cache] section returns defaultCache.
func LoadCache(path string) (CacheConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultCache(), nil
		}
		return CacheConfig{}, fmt.Errorf("read %s: %v", path, err)
	}
	var raw rawCacheFile
	if err := strictDecode(data, &raw); err != nil {
		return CacheConfig{}, err
	}
	if raw.Cache == nil {
		return defaultCache(), nil
	}
	out := defaultCache()
	if raw.Cache.MaxSize != "" {
		n, err := parseSize(raw.Cache.MaxSize)
		if err != nil {
			return CacheConfig{}, fmt.Errorf("cache.max-size: %w", err)
		}
		out.MaxSize = n
	}
	if raw.Cache.MaxAttachmentSize != "" {
		n, err := parseSize(raw.Cache.MaxAttachmentSize)
		if err != nil {
			return CacheConfig{}, fmt.Errorf("cache.max-attachment-size: %w", err)
		}
		out.MaxAttachmentSize = n
	}
	if raw.Cache.MaxOutboxBytes != "" {
		n, err := parseSize(raw.Cache.MaxOutboxBytes)
		if err != nil {
			return CacheConfig{}, fmt.Errorf("cache.max-outbox-bytes: %w", err)
		}
		out.MaxOutboxBytes = n
	}
	return out, nil
}

// parseSize accepts a number with an optional 1024-based KB/MB/GB/TB
// suffix. Empty string returns 0. Negative values error.
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
		// Reject "5XB" and friends: a B suffix not preceded by K/M/G/T.
		return 0, fmt.Errorf("invalid size suffix in %q", s)
	}
	s = strings.TrimSpace(s)
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
