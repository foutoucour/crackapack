package domain

import "testing"

// A booster's rare slot holds a rare or a mythic, so a foil mythic must not be revealed before a
// non-foil rare.
func TestRarityRankPlacesMythicWithRare(t *testing.T) {
	t.Parallel()

	if got, want := rarityRank(RarityMythic), rarityRank(RarityRare); got != want {
		t.Errorf("rarityRank(mythic) = %d, want the rank of rare, %d", got, want)
	}
}
