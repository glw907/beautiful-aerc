//go:build race

package store

// raceEnabled is true when the race detector instruments this build.
// Its instrumentation costs 2-20x time, which is why poplar's perf
// gate runs without it (the build machine design, section 2), and why
// a millisecond-scale wall-clock assertion made under it measures the
// detector rather than the product. A test holding one gates on this.
const raceEnabled = true
