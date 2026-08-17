package sim

import (
	"testing"
)

// walkCommand stands in for the movement intents M8.4 will add. A command is an
// interface, so this test needs one implementation to prove the seam works.
type walkCommand struct {
	forward float64
}

func (walkCommand) CommandKind() string { return "movement.walk" }

func TestACommandNamesItsKind(t *testing.T) {
	var command Command = walkCommand{forward: 1}
	if got, want := command.CommandKind(), "movement.walk"; got != want {
		t.Fatalf("CommandKind = %q, want %q", got, want)
	}
}

func TestLimitsFillInDefaults(t *testing.T) {
	got := Limits{}.withDefaults()
	if got.EntitySteps <= 0 || got.BlockUpdates <= 0 ||
		got.CollisionCandidates <= 0 || got.Events <= 0 {
		t.Fatalf("withDefaults left a budget unusable: %+v", got)
	}

	// An explicit budget is never raised to the default.
	explicit := Limits{EntitySteps: 1, BlockUpdates: 2, CollisionCandidates: 3, Events: 4}
	if got := explicit.withDefaults(); got != explicit {
		t.Fatalf("withDefaults changed an explicit budget: %+v", got)
	}
}

func TestRandomStateStreams(t *testing.T) {
	state := RandomState{}.WithStream("world", 42).WithStream("entity", 7)

	if got, ok := state.Stream("world"); !ok || got != 42 {
		t.Fatalf("Stream(world) = (%d, %v), want (42, true)", got, ok)
	}
	if _, ok := state.Stream("absent"); ok {
		t.Error("Stream found a name that was never set")
	}

	// Replacing a stream must not add a second entry with the same name.
	state = state.WithStream("world", 43)
	if got, _ := state.Stream("world"); got != 43 {
		t.Errorf("Stream(world) = %d after replacement, want 43", got)
	}
	if len(state.Streams) != 2 {
		t.Fatalf("state holds %d streams, want 2: %+v", len(state.Streams), state.Streams)
	}
}

func TestRandomStateStreamsAreSortedByName(t *testing.T) {
	// The order is part of the contract: the digest encodes these in order, so
	// two states with the same streams must encode identically however they
	// were built.
	first := RandomState{}.WithStream("b", 1).WithStream("a", 2)
	second := RandomState{}.WithStream("a", 2).WithStream("b", 1)

	if len(first.Streams) != len(second.Streams) {
		t.Fatalf("lengths differ: %d vs %d", len(first.Streams), len(second.Streams))
	}
	for index := range first.Streams {
		if first.Streams[index] != second.Streams[index] {
			t.Fatalf("stream %d differs: %+v vs %+v",
				index, first.Streams[index], second.Streams[index])
		}
	}
}

func TestRandomStateCloneDoesNotAlias(t *testing.T) {
	state := RandomState{}.WithStream("world", 1)
	clone := state.Clone()
	state.Streams[0].State = 99

	if got, _ := clone.Stream("world"); got != 1 {
		t.Fatalf("the clone followed its original: %d", got)
	}
}
