package web

import (
	"testing"
	"time"
)

func TestUntilText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"under a minute", 30 * time.Second, "less than a minute"},
		{"one minute", time.Minute, "1 min"},
		{"a part of a minute counts as a whole one", 20*time.Minute + time.Second, "21 mins"},
		{"minutes that fill an hour", 59*time.Minute + time.Second, "1 hour"},
		{"hours and minutes", 3*time.Hour + 20*time.Minute, "3 hours 20 mins"},
		{"a round hour", time.Hour, "1 hour"},
		{"days hours and minutes", 50*time.Hour + 5*time.Minute, "2 days 2 hours 5 mins"},
		{"days only", 48 * time.Hour, "2 days"},
		{"days and minutes", 24*time.Hour + time.Minute, "1 day 1 min"},
	}
	for _, tc := range tests {
		if got := untilText(tc.d); got != tc.want {
			t.Errorf("%s: untilText(%v) = %q, want %q", tc.name, tc.d, got, tc.want)
		}
	}
}
