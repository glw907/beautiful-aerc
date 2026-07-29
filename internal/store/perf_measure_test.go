//go:build !race

package store

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// perfMeasure runs op exactly count times through testing.B.Loop,
// forcing that exact iteration count with -benchtime's Nx form rather
// than the default calibrated duration: QA-2 and QA-3 name a fixed
// script and query set, not however many iterations fit in a second.
// It captures op's own reported duration per call rather than B's
// mean, since a p95/p99 budget needs the distribution a mean hides.
func perfMeasure(t *testing.T, count int, op func() time.Duration) (samples []time.Duration, benchLine string) {
	t.Helper()

	if err := flag.Set("test.benchtime", fmt.Sprintf("%dx", count)); err != nil {
		t.Fatalf("set benchtime: %v", err)
	}
	result := testing.Benchmark(func(b *testing.B) {
		for b.Loop() {
			samples = append(samples, op())
		}
	})
	return samples, result.String()
}

// perfPercentile returns the p-th percentile (0-100) of samples,
// nearest-rank over the sorted set.
func perfPercentile(samples []time.Duration, p float64) time.Duration {
	sorted := slices.Clone(samples)
	slices.Sort(sorted)
	idx := int(p / 100 * float64(len(sorted)-1))
	return sorted[idx]
}

// perfWriteArtifact writes a baseline summary for name to t's
// ArtifactDir: the go-test-style benchmark line plus this run's
// percentiles, so a later pass can benchstat two such files against
// each other.
func perfWriteArtifact(t *testing.T, name, benchLine string, samples []time.Duration) {
	t.Helper()

	summary := fmt.Sprintf("Benchmark%s %s\nn=%d p50=%s p95=%s p99=%s max=%s\n",
		name, benchLine, len(samples),
		perfPercentile(samples, 50), perfPercentile(samples, 95), perfPercentile(samples, 99),
		perfPercentile(samples, 100))

	path := filepath.Join(t.ArtifactDir(), name+".txt")
	if err := os.WriteFile(path, []byte(summary), 0o644); err != nil { //nolint:gosec // G306: a perf artifact is diagnostic output alongside the test binary, not sensitive data
		t.Fatalf("write artifact %s: %v", path, err)
	}
}
