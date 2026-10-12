package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
)

const (
	// priceProvider is the one provider the snapshot is built from. TCGplayer retail has a
	// price for nearly every card and is quoted in USD; Cardmarket is in EUR.
	priceProvider = "tcgplayer"

	centsPerDollar = 100
)

// cardPrices is the slice of an MTGJSON AllPricesToday entry the snapshot needs:
// paper -> provider -> retail -> finish -> date -> dollars.
type cardPrices struct {
	Paper map[string]struct {
		Retail map[string]map[string]json.Number `json:"retail"`
	} `json:"paper"`
}

// extractPrices streams an MTGJSON AllPricesToday document and returns the USD cent price of
// each card in wanted. Cards without a usable price are left out, so the simulation prices
// them at its fallback price and reports them as unpriced.
//
// The document is decoded one card at a time: it holds over a hundred thousand cards and only
// a few hundred are wanted.
func extractPrices(r io.Reader, wanted map[string]struct{}) (map[string]int64, error) {
	dec := json.NewDecoder(r)
	if err := expectDelim(dec, '{'); err != nil {
		return nil, err
	}
	snapshot := make(map[string]int64, len(wanted))
	for dec.More() {
		key, err := nextKey(dec)
		if err != nil {
			return nil, err
		}
		if key != "data" {
			var skipped json.RawMessage
			if err := dec.Decode(&skipped); err != nil {
				return nil, fmt.Errorf("skip %q: %w", key, err)
			}
			continue
		}
		if err := readCards(dec, wanted, snapshot); err != nil {
			return nil, err
		}
	}
	if err := expectDelim(dec, '}'); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func readCards(dec *json.Decoder, wanted map[string]struct{}, snapshot map[string]int64) error {
	if err := expectDelim(dec, '{'); err != nil {
		return err
	}
	for dec.More() {
		uuid, err := nextKey(dec)
		if err != nil {
			return err
		}
		var prices cardPrices
		if err := dec.Decode(&prices); err != nil {
			return fmt.Errorf("decode card %q: %w", uuid, err)
		}
		if _, ok := wanted[uuid]; !ok {
			continue
		}
		if cents, ok := retailCents(prices); ok {
			snapshot[uuid] = cents
		}
	}
	return expectDelim(dec, '}')
}

// retailCents returns the latest non-foil retail price, or the latest foil one for cards that
// only exist as foils.
func retailCents(p cardPrices) (int64, bool) {
	retail := p.Paper[priceProvider].Retail
	for _, finish := range []string{"normal", "foil"} {
		dollars, ok := latest(retail[finish])
		if !ok {
			continue
		}
		value, err := strconv.ParseFloat(dollars.String(), 64)
		if err != nil || value < 0 {
			continue
		}
		// MTGJSON quotes dollars with two decimals; rounding removes the float64 error
		// (1.15 * 100 is 114.99999999999999).
		return int64(math.Round(value * centsPerDollar)), true
	}
	return 0, false
}

// latest returns the value at the most recent date. ISO dates sort as strings.
func latest(byDate map[string]json.Number) (json.Number, bool) {
	var newest string
	for date := range byDate {
		if date > newest {
			newest = date
		}
	}
	value, ok := byDate[newest]
	return value, ok
}

func expectDelim(dec *json.Decoder, want json.Delim) error {
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("read price document: %w", err)
	}
	if got, ok := tok.(json.Delim); !ok || got != want {
		return fmt.Errorf("read price document: got %v, want %q", tok, want)
	}
	return nil
}

func nextKey(dec *json.Decoder) (string, error) {
	tok, err := dec.Token()
	if err != nil {
		return "", fmt.Errorf("read price document: %w", err)
	}
	key, ok := tok.(string)
	if !ok {
		return "", fmt.Errorf("read price document: got %v, want an object key", tok)
	}
	return key, nil
}
