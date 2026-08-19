package storetest

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// perfBaselineEnvVar gates WriteBaseline. A baseline is a reference
// point a later pass measures against, so it is worth only as much as
// the machine that produced it was quiet; an ordinary `go test ./...`
// writing one would pin whatever the first noisy run happened to see,
// and leave untracked files behind besides.
const perfBaselineEnvVar = "POPLAR_PERF_BASELINE"

// BaselineName returns the file name for one measurement against env's
// corpus. The corpus scale is part of the name because the gate corpus
// and the full envelope are different reference points, and
// WriteBaseline never overwrites: sharing one name would let whichever
// scale ran first become the record for both.
func (env PerfEnvelope) BaselineName(measurement string) string {
	return fmt.Sprintf("%s_%d", measurement, len(env.MessageIDs))
}

// Measure runs op exactly count times through testing.B.Loop, forcing
// that exact iteration count with -benchtime's Nx form rather than the
// default calibrated duration: a QA harness names a fixed script or
// query set, not however many iterations fit in a second. It captures
// op's own reported duration per call rather than B's mean, since a
// p95/p99 budget needs the distribution a mean hides.
func Measure(t *testing.T, count int, op func() time.Duration) (samples []time.Duration, benchLine string) {
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

// Percentile returns the p-th percentile (0-100) of samples,
// nearest-rank over the sorted set.
func Percentile(samples []time.Duration, p float64) time.Duration {
	sorted := slices.Clone(samples)
	slices.Sort(sorted)
	idx := int(p / 100 * float64(len(sorted)-1))
	return sorted[idx]
}

// WriteBaseline records name's baseline summary under dir, a path
// relative to the calling package's own directory: the go-test-style
// benchmark line plus this run's percentiles. It writes nothing unless
// POPLAR_PERF_BASELINE is set, so recording a baseline is a deliberate
// act on a quiet machine rather than a side effect of running the
// tests.
//
// Unlike t.ArtifactDir(), which Go deletes after the test unless
// -artifacts is passed, dir is a committed testdata directory, so the
// file survives past the test that wrote it for a later pass to
// benchstat a fresh run against. WriteBaseline never overwrites an
// existing file: the baseline is a fixed reference point, not a number
// that moves every time someone runs go test. Deleting the file is how
// a pass deliberately rebases.
func WriteBaseline(t *testing.T, dir, name, benchLine string, samples []time.Duration) {
	t.Helper()

	if os.Getenv(perfBaselineEnvVar) == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("create %s: %v", dir, err)
	}

	path := filepath.Join(dir, name+".txt")
	if _, err := os.Stat(path); err == nil {
		return
	}

	summary := fmt.Sprintf("Benchmark%s %s\nn=%d p50=%s p95=%s p99=%s max=%s\n",
		name, benchLine, len(samples),
		Percentile(samples, 50), Percentile(samples, 95), Percentile(samples, 99),
		Percentile(samples, 100))
	if err := os.WriteFile(path, []byte(summary), 0o644); err != nil { //nolint:gosec // G306: a perf baseline is diagnostic output alongside the test binary, not sensitive data
		t.Fatalf("write baseline %s: %v", path, err)
	}
}
