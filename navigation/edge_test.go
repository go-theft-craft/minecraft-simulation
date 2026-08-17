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

// The heuristic multiplies distance by the cheapest edge a capability can
// take. Getting this wrong makes the search inadmissible and it stops
// returning shortest paths, which no test of a single path would catch.
func TestCheapestIsTheLowestEnabledEdgeCost(t *testing.T) {
	walker := Capability{WalkTicks: 5, StepTicks: 9, FallTicks: 3, SwimTicks: 1}
	if got := walker.cheapest(); got != 3 {
		t.Fatalf("cheapest = %v, want 3 (swimming disabled)", got)
	}

	swimmer := walker
	swimmer.CanSwim = true
	if got := swimmer.cheapest(); got != 1 {
		t.Fatalf("cheapest = %v, want 1", got)
	}
}
