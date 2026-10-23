package play

import (
	"fmt"
	"math/rand/v2"
)

// Stream is the random source that the opener draws boosters from. It is a PCG seeded twice with the
// seed, as the opener always was, that a save can write down and pick up again, so that a game
// loaded from a save does not open the boosters it opened before. It is a [rand.Source], and not
// safe for concurrent use: the game uses it under its lock.
type Stream struct {
	seed uint64
	src  *rand.PCG
}

// NewStream returns the stream of seed.
func NewStream(seed uint64) *Stream {
	return &Stream{seed: seed, src: rand.NewPCG(seed, seed)}
}

// Uint64 returns the next number of the stream.
func (s *Stream) Uint64() uint64 { return s.src.Uint64() }

// Seed returns the seed the stream started from.
func (s *Stream) Seed() uint64 { return s.seed }

// State returns where the stream is, to give to Restore.
func (s *Stream) State() ([]byte, error) {
	state, err := s.src.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("stream: write the state: %w", err)
	}
	return state, nil
}

// Restore makes the stream the one of seed that stood at state, which State returned. It returns an
// error, and changes nothing, when state is not one.
func (s *Stream) Restore(seed uint64, state []byte) error {
	src := rand.NewPCG(seed, seed)
	if err := src.UnmarshalBinary(state); err != nil {
		return fmt.Errorf("stream: read the state: %w", err)
	}
	s.seed, s.src = seed, src
	return nil
}
