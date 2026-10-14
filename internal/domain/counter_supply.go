package domain

import (
	"errors"
	"fmt"
	"math/rand/v2"
)

// SupplyTick is what a CounterSupply opened in one tick.
type SupplyTick struct {
	BoostersOpened int
	CardsOpened    int
	// FeeCoins is the coin fees paid from the treasury to open those boosters.
	FeeCoins Coins
}

// CounterSupply opens boosters on behalf of the Counter, which then behaves as a special player:
// the cards it draws go on its shelf, subject to its ShelfLife, and it sells them like any other
// stock. It draws on a random stream of its own, so adding it to a run never moves what the
// player or the NPCs draw.
type CounterSupply struct {
	counter *Counter
	sets    []SetCatalogue
	opener  *Opener
	rng     *rand.Rand

	// Fixed mode: perDay is the boosters opened each day. credit accumulates it a tick at a time,
	// so that a day's openings match perDay exactly, as for an NPC.
	perDay int
	credit int

	// Energy mode: the Counter opens a booster whenever its energy pays the cost of the set it has
	// picked next. weights make the pick proportional to 1/cost, so every set gets an equal share
	// of energy and a dear set is opened less often. next stays picked until the booster is
	// opened, so that a dear set is not re-rolled while its energy builds up.
	energy    *Energy
	costs     map[string]int
	journal   *Journal         // set with fees, to pay them from the treasury
	fees      map[string]Coins // coins per booster, by set code
	weights   []int
	weightSum int
	next      int // index into sets, or -1 when none is picked
}

// NewFixedCounterSupply returns a supply that opens boostersPerDay boosters a day, spread over the
// day and with no energy, from sets chosen uniformly at random. rng is the stream the supply
// draws set choices from, and should be the one opener draws from too.
//
// It returns an error when counter, opener or rng is nil, when sets is empty, or when
// boostersPerDay is not positive.
func NewFixedCounterSupply(counter *Counter, sets []SetCatalogue, opener *Opener, rng *rand.Rand, boostersPerDay int) (*CounterSupply, error) {
	switch {
	case counter == nil:
		return nil, errors.New("counter supply: counter must not be nil")
	case opener == nil:
		return nil, errors.New("counter supply: opener must not be nil")
	case rng == nil:
		return nil, errors.New("counter supply: rng must not be nil")
	case len(sets) == 0:
		return nil, errors.New("counter supply: at least one set is required")
	case boostersPerDay <= 0:
		return nil, fmt.Errorf("counter supply: boosters per day must be > 0, got %d", boostersPerDay)
	}
	return &CounterSupply{counter: counter, sets: sets, opener: opener, rng: rng, perDay: boostersPerDay}, nil
}

// NewEnergyCounterSupply returns a supply that opens a booster whenever energy pays the energy cost
// of the set it has picked, from costs keyed by set code. A set is picked with a weight of
// 1/cost, so each set gets an equal share of energy.
//
// It returns an error when counter, opener, rng or energy is nil, when sets is empty, or when a
// set has no positive cost. A cost above what energy can ever hold is not checked here: the supply
// would then never open that set.
func NewEnergyCounterSupply(counter *Counter, sets []SetCatalogue, opener *Opener, rng *rand.Rand, energy *Energy, costs map[string]int, opts ...SupplyOption) (*CounterSupply, error) {
	switch {
	case counter == nil:
		return nil, errors.New("counter supply: counter must not be nil")
	case opener == nil:
		return nil, errors.New("counter supply: opener must not be nil")
	case rng == nil:
		return nil, errors.New("counter supply: rng must not be nil")
	case energy == nil:
		return nil, errors.New("counter supply: energy must not be nil")
	case len(sets) == 0:
		return nil, errors.New("counter supply: at least one set is required")
	}
	costList := make([]int, len(sets))
	for i, cat := range sets {
		cost, ok := costs[cat.Code]
		if !ok || cost <= 0 {
			return nil, fmt.Errorf("counter supply: set %q needs a positive energy cost, got %d", cat.Code, cost)
		}
		costList[i] = cost
	}
	scale := 1
	for _, cost := range costList {
		scale = scale / gcd(scale, cost) * cost
	}
	weights := make([]int, len(sets))
	var sum int
	for i, cost := range costList {
		weights[i] = scale / cost
		sum += weights[i]
	}
	s := &CounterSupply{
		counter: counter, sets: sets, opener: opener, rng: rng,
		energy: energy, costs: costs, weights: weights, weightSum: sum, next: -1,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// Tick advances the supply by one tick and opens the boosters it has come due for.
func (s *CounterSupply) Tick() (SupplyTick, error) {
	if s.energy != nil {
		return s.tickEnergy()
	}
	var total SupplyTick
	s.credit += s.perDay
	for s.credit >= TicksPerDay {
		s.credit -= TicksPerDay
		cards, err := s.openBooster(s.sets[s.rng.IntN(len(s.sets))])
		if err != nil {
			return total, err
		}
		total.BoostersOpened++
		total.CardsOpened += cards
	}
	return total, nil
}

// tickEnergy recharges the Counter's energy and opens every booster it can now pay for.
func (s *CounterSupply) tickEnergy() (SupplyTick, error) {
	var total SupplyTick
	if err := s.energy.Tick(1); err != nil {
		return total, fmt.Errorf("counter supply: energy: %w", err)
	}
	for {
		if s.next < 0 {
			s.next = s.pickWeighted()
		}
		cat := s.sets[s.next]
		cost := s.costs[cat.Code]
		if s.energy.Reserve()+s.energy.Savings() < cost {
			return total, nil
		}
		paid, afford, err := s.payFee(cat)
		if err != nil {
			return total, err
		}
		if !afford {
			s.next = -1
			return total, nil
		}
		total.FeeCoins += paid
		if err := s.energy.Spend(cost); err != nil {
			return total, fmt.Errorf("counter supply: spend energy for %q: %w", cat.Code, err)
		}
		cards, err := s.openBooster(cat)
		if err != nil {
			return total, err
		}
		s.next = -1
		total.BoostersOpened++
		total.CardsOpened += cards
	}
}

// payFee pays the coin fee of cat from the treasury. It returns the coins paid, and false when the
// treasury cannot pay, in which case nothing moves.
func (s *CounterSupply) payFee(cat SetCatalogue) (paid Coins, afford bool, err error) {
	fee := s.fees[cat.Code]
	if fee <= 0 || s.journal == nil {
		return 0, true, nil
	}
	err = s.journal.Post(Entry{From: AccountCounterTreasury, To: AccountCounterFee, Amount: fee, Memo: "booster fee: set " + cat.Code})
	switch {
	case errors.Is(err, ErrInsufficientBalance):
		return 0, false, nil
	case err != nil:
		return 0, false, fmt.Errorf("counter supply: pay fee for %q: %w", cat.Code, err)
	}
	return fee, true, nil
}

// pickWeighted returns the index of a set chosen with a probability proportional to its weight.
func (s *CounterSupply) pickWeighted() int {
	r := s.rng.IntN(s.weightSum)
	for i, w := range s.weights {
		if r < w {
			return i
		}
		r -= w
	}
	return len(s.sets) - 1
}

// openBooster opens one booster of cat and shelves every card it holds.
func (s *CounterSupply) openBooster(cat SetCatalogue) (int, error) {
	booster, err := s.opener.Open(BoosterLayoutClassic, cat)
	if err != nil {
		return 0, fmt.Errorf("counter supply: open booster of %q: %w", cat.Code, err)
	}
	for _, key := range booster.Cards {
		if err := s.counter.AcceptSurplus(key, 1); err != nil {
			return 0, fmt.Errorf("counter supply: shelve %q: %w", key.ID, err)
		}
	}
	return len(booster.Cards), nil
}

// SupplyOption tunes NewEnergyCounterSupply.
type SupplyOption func(*CounterSupply)

// WithCoinFees makes the supply pay fees[set code] coins from AccountCounterTreasury to
// AccountCounterFee before it opens a booster of that set. A fee the treasury cannot pay means the
// booster is not opened that tick and the supply picks another set the next. A set with no fee
// costs no coins.
func WithCoinFees(journal *Journal, fees map[string]Coins) SupplyOption {
	return func(s *CounterSupply) {
		s.journal = journal
		s.fees = fees
	}
}
