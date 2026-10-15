package domain

import "errors"

// Account names a ledger account.
type Account string

const (
	// AccountPlayer is the player's coin wallet. It is guarded by default: Post rejects any
	// transaction that would make its balance negative.
	AccountPlayer Account = "player"

	// AccountWelcomeBonus is the source account for the one-time welcome coin grant.
	// It is unguarded: a negative balance represents coins created by the game.
	AccountWelcomeBonus Account = "welcome_bonus"

	// AccountDailyReward is the source account for daily coin rewards.
	// It is unguarded for the same reason as AccountWelcomeBonus.
	AccountDailyReward Account = "daily_reward"

	// AccountCounterSale is the source account for coins the Counter pays the player when the
	// player sells a card to it: the sale posts from AccountCounterSale to AccountPlayer. It is
	// unguarded, and its negative balance is the total paid out by the Counter.
	AccountCounterSale Account = "counter_sale"

	// AccountCounterPurchase is the sink account for coins the player pays the Counter when the
	// player buys a card from it: the purchase posts from AccountPlayer to AccountCounterPurchase.
	// It is unguarded, and its balance is the total spent at the Counter.
	AccountCounterPurchase Account = "counter_purchase"

	// AccountCounterTreasury is the Counter's coin wallet when the Counter has a budget. It is
	// guarded, so the Counter cannot pay out more than it holds: a sale to the Counter posts from
	// it to AccountPlayer, and a purchase from the Counter posts from AccountPlayer to it.
	AccountCounterTreasury Account = "counter_treasury"

	// AccountCounterFloat is the source account for the coins that fund the Counter's treasury,
	// at the start and each day. It is unguarded like the other sources.
	AccountCounterFloat Account = "counter_float"

	// AccountCounterFee is the sink account for the coin fees the Counter pays to open boosters
	// worth more than the energy cap: the posting goes from the treasury to it.
	AccountCounterFee Account = "counter_fee"

	// AccountBoosterFee is the sink account for the coin fees the player pays to open boosters worth
	// more than the energy cap: the fee posts from AccountPlayer to AccountBoosterFee, and the
	// coins leave the economy. It is unguarded, and its balance is the total of fees paid.
	AccountBoosterFee Account = "booster_fee"

	// AccountTradeTopUp is the sink account for the coins the player adds to a trade with an NPC:
	// the top-up posts from AccountPlayer to AccountTradeTopUp. NPCs hold no coins and have no
	// account, so the coins leave the economy. It is unguarded, and its balance is the total of
	// top-ups paid.
	AccountTradeTopUp Account = "trade_topup"

	// AccountCoinTopup is the sink account for the coins the player pays to substitute coins for
	// missing energy when opening a booster (the optional coin top-up). It posts from
	// AccountPlayer to AccountCoinTopup, and the coins leave the economy. It is unguarded, and
	// its balance is the total of top-up payments made.
	AccountCoinTopup Account = "coin_topup"
)

var (
	// ErrInsufficientBalance is returned when a Post would make a guarded account's balance
	// negative. The journal is not modified.
	ErrInsufficientBalance = errors.New("insufficient balance")

	// ErrSelfTransfer is returned when an Entry's From and To are the same account.
	ErrSelfTransfer = errors.New("from and to account must differ")

	// ErrNonPositiveAmount is returned when an Entry's Amount is not strictly positive.
	ErrNonPositiveAmount = errors.New("amount must be positive")
)

// Entry is one immutable ledger posting. Amount coins move from From to To.
type Entry struct {
	From   Account
	To     Account
	Amount Coins
	Memo   string
}

// Journal is an append-only double-entry ledger. It keeps a running balance map so that
// Balance is O(1). The entries slice is retained for audit; retrieve a copy with Entries.
//
// Guarded accounts (AccountPlayer by default) are rejected on Post if the transaction would
// make their balance negative. All source and sink accounts are not guarded.
//
// The zero value is not useful; use NewJournal.
type Journal struct {
	entries  []Entry
	balances map[Account]Coins
	guarded  map[Account]struct{}
}

// NewJournal returns a Journal that enforces non-negative balances on the given guarded
// accounts. When called with no arguments only AccountPlayer is guarded.
func NewJournal(guarded ...Account) *Journal {
	g := make(map[Account]struct{})
	if len(guarded) == 0 {
		g[AccountPlayer] = struct{}{}
	} else {
		for _, a := range guarded {
			g[a] = struct{}{}
		}
	}
	return &Journal{
		balances: make(map[Account]Coins),
		guarded:  g,
	}
}

// Post appends e to the journal after validating:
//   - Amount > 0
//   - From != To
//   - no guarded account's balance would go below zero
//
// It returns an error and leaves the journal unchanged if any check fails.
func (j *Journal) Post(e Entry) error {
	if e.Amount <= 0 {
		return ErrNonPositiveAmount
	}
	if e.From == e.To {
		return ErrSelfTransfer
	}
	if _, ok := j.guarded[e.From]; ok {
		if j.balances[e.From]-e.Amount < 0 {
			return ErrInsufficientBalance
		}
	}
	j.balances[e.From] -= e.Amount
	j.balances[e.To] += e.Amount
	j.entries = append(j.entries, e)
	return nil
}

// Balance returns the current balance of account a. It is O(1).
func (j *Journal) Balance(a Account) Coins {
	return j.balances[a]
}

// SumAllBalances returns the sum of every account balance. In a correct journal this is
// always zero: every Post moves the same amount from one account to another.
func (j *Journal) SumAllBalances() Coins {
	var sum Coins
	for _, b := range j.balances {
		sum += b
	}
	return sum
}

// Entries returns a copy of the journal's entry slice for audit purposes.
func (j *Journal) Entries() []Entry {
	out := make([]Entry, len(j.entries))
	copy(out, j.entries)
	return out
}
