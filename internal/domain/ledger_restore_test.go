package domain_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestJournalRestore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		balances map[domain.Account]domain.Coins
		wantErr  string
	}{
		{name: "balances that sum to zero", balances: map[domain.Account]domain.Coins{
			domain.AccountWelcomeBonus: -200, domain.AccountPlayer: 150, domain.AccountCounterPurchase: 50,
		}},
		{name: "no balance at all", balances: map[domain.Account]domain.Coins{}},
		{name: "balances that do not sum to zero", balances: map[domain.Account]domain.Coins{
			domain.AccountWelcomeBonus: -200, domain.AccountPlayer: 199,
		}, wantErr: "sum"},
		{name: "a guarded account below zero", balances: map[domain.Account]domain.Coins{
			domain.AccountWelcomeBonus: 10, domain.AccountPlayer: -10,
		}, wantErr: "player"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			j := domain.NewJournal()
			err := j.Restore(tt.balances)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Restore error = %v, want one mentioning %q", err, tt.wantErr)
				}
				if got := j.Balance(domain.AccountPlayer); got != 0 {
					t.Errorf("a refused restore changed the player's balance to %d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Restore: %v", err)
			}
			for a, want := range tt.balances {
				if got := j.Balance(a); got != want {
					t.Errorf("Balance(%s) = %d, want %d", a, got, want)
				}
			}
		})
	}
}

// A restored journal goes on guarding its accounts from the balances it was given.
func TestJournalRestore_keepsGuardingThePlayer(t *testing.T) {
	t.Parallel()
	j := domain.NewJournal()
	if err := j.Restore(map[domain.Account]domain.Coins{domain.AccountWelcomeBonus: -30, domain.AccountPlayer: 30}); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if err := j.Post(domain.Entry{From: domain.AccountPlayer, To: domain.AccountCounterPurchase, Amount: 31}); err == nil {
		t.Error("a purchase of 31 from a restored balance of 30 went through")
	}
	mustPost(t, j, domain.Entry{From: domain.AccountPlayer, To: domain.AccountCounterPurchase, Amount: 30})
	if got := j.SumAllBalances(); got != 0 {
		t.Errorf("SumAllBalances = %d, want 0", got)
	}
}

// Only an empty journal can be restored, so that a restore never mixes two histories.
func TestJournalRestore_refusesAJournalThatHasEntries(t *testing.T) {
	t.Parallel()
	j := domain.NewJournal()
	mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 5})
	if err := j.Restore(map[domain.Account]domain.Coins{}); err == nil {
		t.Error("Restore on a journal with entries went through")
	}
}

func TestFundTreasury(t *testing.T) {
	t.Parallel()
	j := domain.NewJournal(domain.AccountPlayer, domain.AccountCounterTreasury)
	if err := domain.FundTreasury(j, 0, "nothing"); err != nil {
		t.Fatalf("FundTreasury of 0: %v", err)
	}
	if err := domain.FundTreasury(j, 500, "day 1"); err != nil {
		t.Fatalf("FundTreasury: %v", err)
	}
	if got := j.Balance(domain.AccountCounterTreasury); got != 500 {
		t.Errorf("treasury = %d, want 500", got)
	}
	if got := len(j.Entries()); got != 1 {
		t.Errorf("entries = %d, want 1: a zero amount posts nothing", got)
	}
}

// Balances are what Restore takes back.
func TestJournalBalances_restoreAnotherJournal(t *testing.T) {
	t.Parallel()
	j := domain.NewJournal()
	mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 200})
	mustPost(t, j, domain.Entry{From: domain.AccountPlayer, To: domain.AccountCounterPurchase, Amount: 50})
	balances := j.Balances()
	balances[domain.AccountPlayer] = 0 // a copy: changing it changes nothing in j

	restored := domain.NewJournal()
	if err := restored.Restore(j.Balances()); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	for _, a := range []domain.Account{domain.AccountWelcomeBonus, domain.AccountPlayer, domain.AccountCounterPurchase} {
		if restored.Balance(a) != j.Balance(a) {
			t.Errorf("Balance(%s) = %d restored, %d original", a, restored.Balance(a), j.Balance(a))
		}
	}
}
