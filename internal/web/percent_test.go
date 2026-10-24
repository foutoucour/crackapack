package web

import "testing"

func TestPercentOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		owned, total int
		want         int
	}{
		{"nothing held", 0, 10, 0},
		{"a third rounds down, so that 100 means complete", 1, 3, 33},
		{"two thirds rounds down", 2, 3, 66},
		{"all held", 3, 3, 100},
		{"an empty binder is not a division by zero", 0, 0, 0},
	}
	for _, tc := range tests {
		if got := percentOf(tc.owned, tc.total); got != tc.want {
			t.Errorf("%s: percentOf(%d, %d) = %d, want %d", tc.name, tc.owned, tc.total, got, tc.want)
		}
	}
}
