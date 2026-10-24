package play_test

import (
	"errors"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

func TestCardValue(t *testing.T) {
	t.Parallel()
	g := counterGame(t, &memStore{}, playtest.NewClock(), counterRules(), counterSet())

	tests := []struct {
		name    string
		key     domain.CardKey
		want    domain.Coins
		wantErr error
	}{
		// $2.50 at 100 coins a dollar, whole: the Counter asks 300 and pays 150.
		{name: "a priced card", key: dear(), want: 250},
		{name: "an unknown card", key: domain.CardKey{ID: "nope", Finish: domain.FinishNonFoil}, wantErr: play.ErrUnknownCard},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := g.CardValue(tt.key.ID, tt.key.Finish)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CardValue error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("CardValue = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCardValue_aGameWithoutACounterHasNoCoinsToSayItIn(t *testing.T) {
	t.Parallel()
	g, _, err := newSeededGame(playtest.NewClock(), counterSet())
	if err != nil {
		t.Fatalf("newSeededGame: %v", err)
	}

	_, err = g.CardValue(cheap().ID, cheap().Finish)

	if !errors.Is(err, play.ErrNoCounter) {
		t.Errorf("CardValue error = %v, want ErrNoCounter", err)
	}
}
