package play_test

import (
	"math/rand/v2"
	"testing"

	"github.com/foutoucour/crackapack/internal/play"
)

func TestNewStream_drawsTheSequenceOfAPCGSeededTwiceWithTheSeed(t *testing.T) {
	t.Parallel()
	// A seed must keep opening the boosters it opened before the stream existed.
	stream := play.NewStream(42)
	want := rand.NewPCG(42, 42)

	for i := range 5 {
		if got := stream.Uint64(); got != want.Uint64() {
			t.Fatalf("draw %d differs from rand.NewPCG(42, 42)", i)
		}
	}
	if stream.Seed() != 42 {
		t.Errorf("Seed() = %d, want 42", stream.Seed())
	}
}

func TestStream_restoreContinuesWhereTheSavedStreamStopped(t *testing.T) {
	t.Parallel()
	saved := play.NewStream(42)
	for range 7 {
		saved.Uint64()
	}
	state, err := saved.State()
	if err != nil {
		t.Fatalf("State: %v", err)
	}

	restored := play.NewStream(1)
	if err := restored.Restore(saved.Seed(), state); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if restored.Seed() != 42 {
		t.Errorf("restored Seed() = %d, want 42", restored.Seed())
	}
	for i := range 5 {
		if got, want := restored.Uint64(), saved.Uint64(); got != want {
			t.Fatalf("draw %d after the restore = %d, want %d", i, got, want)
		}
	}
}

func TestStream_restoreRefusesAStateItCannotRead(t *testing.T) {
	t.Parallel()
	stream := play.NewStream(42)
	want := play.NewStream(42)

	if err := stream.Restore(9, []byte("not a pcg state")); err == nil {
		t.Fatal("Restore with a garbage state returned nil error")
	}

	if stream.Seed() != 42 || stream.Uint64() != want.Uint64() {
		t.Error("a refused Restore changed the stream")
	}
}
