package play_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// emptyCounterRules is counterRules with nothing in stock, so that a card is hunted before it is
// at the Counter.
func emptyCounterRules() play.CounterRules {
	r := counterRules()
	r.Params.InitialStock = map[domain.Rarity]int{}
	return r
}

func mustHunt(t *testing.T, g *play.Game, key domain.CardKey) {
	t.Helper()
	if err := g.Hunt(key.ID, key.Finish); err != nil {
		t.Fatalf("Hunt(%v): %v", key, err)
	}
}

func mustHunted(t *testing.T, g *play.Game) []play.Hunted {
	t.Helper()
	hunted, err := g.Hunted()
	if err != nil {
		t.Fatalf("Hunted: %v", err)
	}
	return hunted
}

func TestHunt_refusals(t *testing.T) {
	t.Parallel()
	big := playtest.TinySet("S", play.MaxHunted+1, 2)
	tests := []struct {
		name string
		play func(t *testing.T) error
		want error
	}{
		{"a card of no catalogue", func(t *testing.T) error {
			g := counterGame(t, &memStore{}, playtest.NewClock(), counterRules(), counterSet())
			return g.Hunt("nope", domain.FinishNonFoil)
		}, play.ErrUnknownCard},
		{"a game without a Counter", func(t *testing.T) error {
			g, _ := newGameWithClock(t, counterSet())
			return g.Hunt(cheap().ID, cheap().Finish)
		}, play.ErrNoCounter},
		{"a card the player holds", func(t *testing.T) error {
			g := counterGame(t, &memStore{}, playtest.NewClock(), counterRules(), counterSet())
			if _, err := g.Open("S"); err != nil {
				t.Fatalf("Open: %v", err)
			}
			return g.Hunt(cheap().ID, cheap().Finish)
		}, play.ErrHeld},
		{"more cards than a hunt list holds", func(t *testing.T) error {
			g := counterGame(t, &memStore{}, playtest.NewClock(), counterRules(), big)
			for i := range play.MaxHunted {
				mustHunt(t, g, domain.CardKey{ID: domain.CardID(fmt.Sprintf("S-%02d", i)), Finish: domain.FinishNonFoil})
			}
			return g.Hunt(domain.CardID(fmt.Sprintf("S-%02d", play.MaxHunted)), domain.FinishNonFoil)
		}, play.ErrTooManyHunted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := tt.play(t); !errors.Is(err, tt.want) {
				t.Errorf("Hunt error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestHunted_aCardAlreadyAtTheCounterIsNewUntilThePlayerHasSeenIt(t *testing.T) {
	t.Parallel()
	g := counterGame(t, &memStore{}, playtest.NewClock(), counterRules(), counterSet())
	mustHunt(t, g, cheap())

	first := mustHunted(t, g)
	if len(first) != 1 || first[0].Card.ID != cheap().ID || !first[0].AtCounter || !first[0].New || first[0].BuyPrice != 120 {
		t.Fatalf("hunted = %+v, want the cheap card at the Counter for 120 coins, new", first)
	}
	if first[0].SetCode != "S" {
		t.Errorf("hunted = %+v, want the set of the binder to buy it from, S", first)
	}
	if err := g.SeeHunted(); err != nil {
		t.Fatalf("SeeHunted: %v", err)
	}

	seen := mustHunted(t, g)
	if !seen[0].AtCounter || seen[0].New {
		t.Errorf("hunted = %+v, want the card still at the Counter but no longer new", seen)
	}
}

func TestHunted_aCardThatComesToTheCounterLaterIsNewThen(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	rules := emptyCounterRules()
	rules.Supply = play.SupplyRules{BoostersPerDay: 2}
	g := counterGame(t, &memStore{}, clock, rules, counterSet())
	mustHunt(t, g, cheap())

	before := mustHunted(t, g)
	clock.Advance(day)
	after := mustHunted(t, g)

	if before[0].AtCounter || before[0].New {
		t.Errorf("before = %+v, want a card not at the Counter yet and nothing new", before)
	}
	if !after[0].AtCounter || !after[0].New {
		t.Errorf("after = %+v, want the card at the Counter, new", after)
	}
}

// A card that leaves the Counter and comes back is new again: the player is told of each copy that
// comes, not only of the first.
func TestHunted_aCardThatRunsOutAndComesBackIsNewAgain(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	rules := emptyCounterRules()
	rules.Params.ShelfLife = domain.VanishAfterDays(1)
	rules.Supply = play.SupplyRules{BoostersPerDay: 1}
	g := counterGame(t, &memStore{}, clock, rules, playtest.TinySet("S", 6, 1))
	mustHunt(t, g, cheap())

	arrivals, told := 0, 0
	wasThere := false
	for range 60 {
		clock.Advance(day)
		hunted := mustHunted(t, g)[0]
		if hunted.AtCounter && !wasThere {
			arrivals++
			if hunted.New {
				told++
			}
		}
		if hunted.AtCounter && hunted.New != !wasThere {
			t.Fatalf("hunted = %+v after being at the Counter before = %v: new must say whether the copy just came", hunted, wasThere)
		}
		wasThere = hunted.AtCounter
		if err := g.SeeHunted(); err != nil {
			t.Fatalf("SeeHunted: %v", err)
		}
	}

	if arrivals < 2 || told != arrivals {
		t.Errorf("the card came %d times and was new %d times, want it to come at least twice and be new each time", arrivals, told)
	}
}

func TestHunted_theHuntEndsWhenThePlayerHoldsTheCard(t *testing.T) {
	t.Parallel()
	g := counterGame(t, &memStore{}, playtest.NewClock(), counterRules(), counterSet())
	mustHunt(t, g, cheap())

	if _, err := g.Buy(cheap().ID, cheap().Finish); err != nil {
		t.Fatalf("Buy: %v", err)
	}

	if hunted := mustHunted(t, g); len(hunted) != 0 {
		t.Errorf("hunted = %+v, want nothing: the player holds the card now", hunted)
	}
}

func TestUnhunt_stopsTheHuntAndIsHarmlessWhenThereIsNone(t *testing.T) {
	t.Parallel()
	g := counterGame(t, &memStore{}, playtest.NewClock(), counterRules(), counterSet())
	mustHunt(t, g, cheap())

	for range 2 {
		if err := g.Unhunt(cheap().ID, cheap().Finish); err != nil {
			t.Fatalf("Unhunt: %v", err)
		}
	}

	if hunted := mustHunted(t, g); len(hunted) != 0 {
		t.Errorf("hunted = %+v, want nothing", hunted)
	}
}

func TestHunted_theNPCsThatHoldTheCardOfferItAndAskAgainAfterADay(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	g := counterGame(t, &memStore{}, clock, marketRules(), counterSet())

	mustHunt(t, g, cheap())
	first := mustMarket(t, g)
	if len(first.Requests) != 1 || len(first.Requests[0].Answers) == 0 {
		t.Fatalf("market = %+v, want the hunt asking around, the fast NPC answered", first)
	}
	clock.Advance(slowDelay)
	for _, a := range mustMarket(t, g).Requests[0].Answers {
		if err := g.DeclineAnswer(a.ID); err != nil {
			t.Fatalf("DeclineAnswer: %v", err)
		}
	}
	mustHunted(t, g)
	sameDay := mustMarket(t, g)

	clock.Advance(day)
	mustHunted(t, g)
	nextDay := mustMarket(t, g)

	if len(sameDay.Requests) != 1 || len(sameDay.Requests[0].Answers) != 0 || sameDay.Requests[0].ID != first.Requests[0].ID {
		t.Errorf("same day = %+v, want the first request, with no answer left, not asked again", sameDay.Requests)
	}
	if len(nextDay.Requests) != 1 || nextDay.Requests[0].ID == first.Requests[0].ID || len(nextDay.Requests[0].Answers) == 0 {
		t.Errorf("next day = %+v, want a new request with answers", nextDay.Requests)
	}
}

func TestHunted_countsTheOffersWaiting(t *testing.T) {
	t.Parallel()
	g := counterGame(t, &memStore{}, playtest.NewClock(), marketRules(), counterSet())

	mustHunt(t, g, cheap())

	if hunted := mustHunted(t, g); hunted[0].Offers != 1 {
		t.Errorf("hunted = %+v, want 1 offer waiting: the fast NPC answered at once", hunted)
	}
}

func TestHunted_isSavedWithTheGame(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	store := &memStore{}
	g := counterGame(t, store, clock, counterRules(), counterSet())
	mustHunt(t, g, cheap())
	mustHunted(t, g)
	if err := g.SeeHunted(); err != nil {
		t.Fatalf("SeeHunted: %v", err)
	}

	again := counterGame(t, store.copied(), clock, counterRules(), counterSet())

	hunted := mustHunted(t, again)
	if len(hunted) != 1 || hunted[0].Card.ID != cheap().ID {
		t.Fatalf("hunted after a reload = %+v, want the cheap card", hunted)
	}
	if hunted[0].New {
		t.Errorf("hunted after a reload = %+v: the player had seen it, it must not be new again", hunted)
	}
}
