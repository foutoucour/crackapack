package domain_test

import (
	"errors"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestMarketListRefusesTheLockedCopy(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	card := fx.keys[0]
	fx.give(t, card, 1)
	if err := fx.player.Lock(card); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	_, err := fx.market.List(card, 100, marketStart())

	if !errors.Is(err, domain.ErrLockedCopy) {
		t.Errorf("List of the only copy, locked = %v, want ErrLockedCopy", err)
	}
}

func TestMarketListAcceptsTheCopyAboveTheLock(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	card := fx.keys[0]
	fx.give(t, card, 2)
	if err := fx.player.Lock(card); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	if _, err := fx.market.List(card, 100, marketStart()); err != nil {
		t.Errorf("List of the spare copy: %v", err)
	}
}
