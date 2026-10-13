package domain_test

import (
	"errors"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestJournalTradeTopUpIsASinkOfThePlayer(t *testing.T) {
	t.Parallel()
	journal := domain.NewJournal()
	if err := journal.Post(domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 50}); err != nil {
		t.Fatalf("Post welcome bonus: %v", err)
	}

	if err := journal.Post(domain.Entry{From: domain.AccountPlayer, To: domain.AccountTradeTopUp, Amount: 20}); err != nil {
		t.Fatalf("Post top-up: %v", err)
	}

	if got := journal.Balance(domain.AccountPlayer); got != 30 {
		t.Errorf("player balance = %d, want 30", got)
	}
	if got := journal.Balance(domain.AccountTradeTopUp); got != 20 {
		t.Errorf("top-up balance = %d, want 20: the total the player paid", got)
	}
	if got := journal.SumAllBalances(); got != 0 {
		t.Errorf("sum of balances = %d, want 0", got)
	}
}

func TestJournalRefusesATopUpTheWalletCannotPay(t *testing.T) {
	t.Parallel()
	journal := domain.NewJournal()

	err := journal.Post(domain.Entry{From: domain.AccountPlayer, To: domain.AccountTradeTopUp, Amount: 1})

	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Errorf("Post = %v, want ErrInsufficientBalance", err)
	}
}
