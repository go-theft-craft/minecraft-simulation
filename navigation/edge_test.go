package navigation

import "testing"

func TestEdgeKindStringNamesEveryValue(t *testing.T) {
	cases := map[EdgeKind]string{
		EdgeWalk: "walk",
		EdgeStep: "step",
		EdgeFall: "fall",
		EdgeSwim: "swim",
	}
	for value, want := range cases {
		if got := value.String(); got != want {
			t.Fatalf("EdgeKind(%d).String() = %q, want %q", value, got, want)
		}
	}
}

func TestReasonStringNamesEveryValue(t *testing.T) {
	cases := map[Reason]string{
		ReasonFound:       "found",
		ReasonBudget:      "budget",
		ReasonCeiling:     "ceiling",
		ReasonUnreachable: "unreachable",
	}
	for value, want := range cases {
		if got := value.String(); got != want {
			t.Fatalf("Reason(%d).String() = %q, want %q", value, got, want)
		}
	}
}

// The heuristic multiplies Manhattan distance by this floor. It is the lowest
// cost per block of distance closed, not the lowest edge cost: a step and a
// fall each close two blocks for one edge's price. Getting it wrong makes the
// search inadmissible and it stops returning shortest paths, which no test of a
// single path would catch.
func TestPerBlockFloorIsTheLowestCostPerBlockClosed(t *testing.T) {
	walker := Capability{WalkTicks: 5, StepTicks: 9, FallTicks: 3, SwimTicks: 1}
	// Walk 5 per block, step 9/2 = 4.5, fall 3/2 = 1.5, swim disabled.
	if got := walker.perBlockFloor(); got != 1.5 {
		t.Fatalf("perBlockFloor = %v, want 1.5 (fall, swimming disabled)", got)
	}

	swimmer := walker
	swimmer.CanSwim = true
	// Swim closes one block for 1, which is below the fall's 1.5.
	if got := swimmer.perBlockFloor(); got != 1 {
		t.Fatalf("perBlockFloor = %v, want 1", got)
	}

	// A body whose walk is cheapest per block still floors on the walk.
	walkerFirst := Capability{WalkTicks: 1, StepTicks: 9, FallTicks: 9, SwimTicks: 9}
	if got := walkerFirst.perBlockFloor(); got != 1 {
		t.Fatalf("perBlockFloor = %v, want 1", got)
	}
}
