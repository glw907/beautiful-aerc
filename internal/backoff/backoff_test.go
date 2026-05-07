package backoff

import (
	"testing"
	"time"
)

func TestExponential(t *testing.T) {
	tests := []struct {
		name         string
		attempts     int
		initial, max time.Duration
		want         time.Duration
	}{
		{"first attempt returns initial", 1, time.Second, 60 * time.Second, time.Second},
		{"zero attempts treated as first", 0, time.Second, 60 * time.Second, time.Second},
		{"second doubles", 2, time.Second, 60 * time.Second, 2 * time.Second},
		{"third doubles again", 3, time.Second, 60 * time.Second, 4 * time.Second},
		{"caps at max", 20, time.Second, 60 * time.Second, 60 * time.Second},
		{"overflow caps at max", 100, time.Second, 60 * time.Second, 60 * time.Second},
		{"zero initial uses 1s floor", 1, 0, 60 * time.Second, time.Second},
		{"zero max uses initial as max", 5, 2 * time.Second, 0, 2 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Exponential(tt.attempts, tt.initial, tt.max)
			if got != tt.want {
				t.Errorf("Exponential(%d, %v, %v) = %v, want %v", tt.attempts, tt.initial, tt.max, got, tt.want)
			}
		})
	}
}
