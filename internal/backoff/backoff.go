// Package backoff is poplar's exponential-backoff helper, shared by
// the JMAP push loop, the IMAP idle loop, and the cache drainer's
// failed-op pickup window.
package backoff

import "time"

// Exponential returns the wait duration for the given attempt count
// using initial * 2^(attempts-1), capped at max. attempts <= 1
// returns initial. Negative or zero initial/max are clamped to a
// non-zero floor so callers can't accidentally produce a 0-duration
// retry storm.
func Exponential(attempts int, initial, max time.Duration) time.Duration {
	if initial <= 0 {
		initial = time.Second
	}
	if max <= 0 {
		max = initial
	}
	if attempts <= 1 {
		return initial
	}
	d := initial << (attempts - 1)
	if d <= 0 || d > max {
		return max
	}
	return d
}
