package domain

// Snapshot returns a shallow copy of the collection's quantity map.
// The caller may safely iterate and read the copy; mutations do not affect
// the live collection. Used by the simulation layer to build WorldView.OwnedCards
// without exposing the collection's internal map directly.
func (c *Collection) Snapshot() map[CardKey]int {
	out := make(map[CardKey]int, len(c.qty))
	for k, v := range c.qty {
		out[k] = v
	}
	return out
}
