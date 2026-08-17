package bot

import (
	"testing"
	"time"
)

func TestStickyDebounceDuration(t *testing.T) {
	cases := []struct {
		name    string
		seconds int
		want    time.Duration
	}{
		{"zero_falls_back_to_default", 0, defaultStickyDebounce},
		{"negative_falls_back_to_default", -5, defaultStickyDebounce},
		{"configured_value_used", 10, 10 * time.Second},
		{"one_second_allowed", 1, time.Second},
		{"above_maximum_clamped", 1000, 300 * time.Second},
		{"exactly_maximum", 300, 300 * time.Second},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stickyDebounceDuration(c.seconds)
			if got != c.want {
				t.Errorf("stickyDebounceDuration(%d) = %v, want %v", c.seconds, got, c.want)
			}
			// A zero or negative window would repost on every single message
			// and immediately hit Discord's rate limits.
			if got <= 0 {
				t.Errorf("stickyDebounceDuration(%d) returned non-positive duration %v", c.seconds, got)
			}
		})
	}
}
