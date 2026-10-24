package domain_test

import (
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestJournalEntriesSince(t *testing.T) {
	t.Parallel()
	j := domain.NewJournal()
	first := domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 200, Memo: "welcome"}
	second := domain.Entry{From: domain.AccountPlayer, To: domain.AccountCounterPurchase, Amount: 120, Memo: "buy"}
	for _, e := range []domain.Entry{first, second} {
		if err := j.Post(e); err != nil {
			t.Fatalf("Post: %v", err)
		}
	}

	tests := []struct {
		name string
		from int
		want []domain.Entry
	}{
		{"from the start", 0, []domain.Entry{first, second}},
		{"from the middle", 1, []domain.Entry{second}},
		{"from the end", 2, nil},
		{"from past the end", 9, nil},
		{"from before the start", -3, []domain.Entry{first, second}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := j.EntriesSince(tc.from); !slices.Equal(got, tc.want) {
				t.Errorf("EntriesSince(%d) = %v, want %v", tc.from, got, tc.want)
			}
		})
	}
	if got := j.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}
}

func TestJournalEntriesSinceReturnsCopy(t *testing.T) {
	t.Parallel()
	j := domain.NewJournal()
	if err := j.Post(domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 200}); err != nil {
		t.Fatalf("Post: %v", err)
	}

	j.EntriesSince(0)[0].Amount = 1

	if got := j.Entries()[0].Amount; got != 200 {
		t.Errorf("the journal's entry changed to %d through the copy, want 200", got)
	}
}
