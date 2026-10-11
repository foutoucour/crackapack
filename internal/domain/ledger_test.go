package domain_test

import (
	"errors"
	"math/rand/v2"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestJournalPost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setup       func(j *domain.Journal)
		entry       domain.Entry
		wantErr     error
		wantBalance map[domain.Account]domain.Coins // checked only when wantErr is nil
	}{
		{
			name:  "welcome bonus credits player",
			entry: domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 200, Memo: "welcome"},
			wantBalance: map[domain.Account]domain.Coins{
				domain.AccountWelcomeBonus: -200,
				domain.AccountPlayer:       200,
			},
		},
		{
			name: "player pays counter for a purchase",
			setup: func(j *domain.Journal) {
				mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 100})
			},
			entry: domain.Entry{From: domain.AccountPlayer, To: domain.AccountCounter, Amount: 50},
			wantBalance: map[domain.Account]domain.Coins{
				domain.AccountPlayer:  50,
				domain.AccountCounter: 50,
			},
		},
		{
			name: "daily reward credits player",
			entry: domain.Entry{From: domain.AccountDailyReward, To: domain.AccountPlayer, Amount: 100, Memo: "daily"},
			wantBalance: map[domain.Account]domain.Coins{
				domain.AccountDailyReward: -100,
				domain.AccountPlayer:      100,
			},
		},
		{
			name:    "player overdraft rejected",
			entry:   domain.Entry{From: domain.AccountPlayer, To: domain.AccountCounter, Amount: 1},
			wantErr: domain.ErrInsufficientBalance,
		},
		{
			name:    "self transfer rejected",
			entry:   domain.Entry{From: domain.AccountPlayer, To: domain.AccountPlayer, Amount: 10},
			wantErr: domain.ErrSelfTransfer,
		},
		{
			name:    "zero amount rejected",
			entry:   domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 0},
			wantErr: domain.ErrNonPositiveAmount,
		},
		{
			name:    "negative amount rejected",
			entry:   domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: -1},
			wantErr: domain.ErrNonPositiveAmount,
		},
		{
			// AccountCounter is not guarded by default; it must be allowed to run negative
			// so that a Counter sale credits the player even when the Counter is empty.
			name:  "counter can go negative (not guarded)",
			entry: domain.Entry{From: domain.AccountCounter, To: domain.AccountPlayer, Amount: 100},
			wantBalance: map[domain.Account]domain.Coins{
				domain.AccountCounter: -100,
				domain.AccountPlayer:  100,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			j := domain.NewJournal()
			if tc.setup != nil {
				tc.setup(j)
			}
			err := j.Post(tc.entry)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("Post() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				return
			}
			for acc, want := range tc.wantBalance {
				if got := j.Balance(acc); got != want {
					t.Errorf("Balance(%s) = %d, want %d", acc, got, want)
				}
			}
		})
	}
}

// TestJournalSumZeroInvariant asserts that SumAllBalances() is zero after every Post,
// whether the post succeeds or is rejected. A fixed seed keeps the run deterministic.
func TestJournalSumZeroInvariant(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(42, 0))

	j := domain.NewJournal()
	// Fund player and counter so valid transactions can flow.
	mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 10_000})
	mustPost(t, j, domain.Entry{From: domain.AccountDailyReward, To: domain.AccountCounter, Amount: 10_000})

	allAccounts := []domain.Account{
		domain.AccountPlayer,
		domain.AccountCounter,
		domain.AccountWelcomeBonus,
		domain.AccountDailyReward,
		domain.AccountCounterSale,
		domain.AccountCounterPurchase,
	}

	const iterations = 200
	for range iterations {
		from := allAccounts[rng.IntN(len(allAccounts))]
		to := allAccounts[rng.IntN(len(allAccounts))]
		amount := domain.Coins(rng.Int64N(50) + 1)
		_ = j.Post(domain.Entry{From: from, To: to, Amount: amount})

		if got := j.SumAllBalances(); got != 0 {
			t.Fatalf("SumAllBalances() = %d after post, want 0 (invariant broken)", got)
		}
	}
}

// TestJournalRejectionIsAtomic asserts that a rejected Post leaves all balances unchanged.
func TestJournalRejectionIsAtomic(t *testing.T) {
	t.Parallel()
	j := domain.NewJournal()
	mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 100})

	snapPlayer := j.Balance(domain.AccountPlayer)
	snapCounter := j.Balance(domain.AccountCounter)

	err := j.Post(domain.Entry{From: domain.AccountPlayer, To: domain.AccountCounter, Amount: 200})
	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Fatalf("want ErrInsufficientBalance, got %v", err)
	}
	if got := j.Balance(domain.AccountPlayer); got != snapPlayer {
		t.Errorf("player balance changed on rejected post: got %d, want %d", got, snapPlayer)
	}
	if got := j.Balance(domain.AccountCounter); got != snapCounter {
		t.Errorf("counter balance changed on rejected post: got %d, want %d", got, snapCounter)
	}
	if got := j.SumAllBalances(); got != 0 {
		t.Errorf("SumAllBalances() = %d after rejected post, want 0", got)
	}
}

// TestJournalEntriesReturnsCopy asserts that mutating the slice returned by Entries
// does not affect the journal's internal state.
func TestJournalEntriesReturnsCopy(t *testing.T) {
	t.Parallel()
	j := domain.NewJournal()
	mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 200, Memo: "welcome"})
	mustPost(t, j, domain.Entry{From: domain.AccountDailyReward, To: domain.AccountPlayer, Amount: 100, Memo: "daily"})

	got := j.Entries()
	if len(got) != 2 {
		t.Fatalf("Entries() len = %d, want 2", len(got))
	}
	got[0].Amount = 9_999
	if j.Entries()[0].Amount != 200 {
		t.Error("Entries() returned a reference to the internal slice, not a copy")
	}
}

// mustPost is a test helper that fails the test if Post returns an error.
func mustPost(t *testing.T, j *domain.Journal, e domain.Entry) {
	t.Helper()
	if err := j.Post(e); err != nil {
		t.Fatalf("unexpected Post error: %v", err)
	}
}
