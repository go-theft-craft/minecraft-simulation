package v26_1

import (
	"errors"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/mining"
	"github.com/go-theft-craft/minecraft-simulation/placement"
)

// placer returns the profile as the optional interface, asserted rather than
// assumed.
func placer(t *testing.T) placement.Placer {
	t.Helper()

	built, err := New(dataset(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	placer, ok := built.(placement.Placer)
	if !ok {
		t.Fatalf("%T cannot place", built)
	}

	return placer
}

func TestEveryPlaceableItemResolvesAState(t *testing.T) {
	t.Parallel()

	// The sweep. A rule that handles stairs, slabs, and logs and misses eight
	// hundred other blocks passes every behavioural case, and the gate against
	// the jar only asks about the families it covers. This asks about all of
	// them.
	//
	// What it does not claim is that the state is *right* for a family this
	// stage does not carry — a door and a torch resolve their default state,
	// which is wrong, and the corpus says which families are checked.
	set := dataset(t)
	place := placer(t)

	var (
		placeable int
		missed    []string
	)
	for _, item := range set.Items().All() {
		if _, ok := set.Blocks().ByName(item.Name); !ok {
			continue
		}
		placeable++

		if _, err := place.PlacedState(item.ID, placement.Target{}, mining.FaceTop, 0, geom.Vec3{}); err != nil {
			missed = append(missed, item.Name)
		}
	}

	if placeable == 0 {
		t.Fatal("this version has no item that places a block, which cannot be right")
	}
	if len(missed) != 0 {
		t.Fatalf("%d of %d placeable items resolved no state: %v",
			len(missed), placeable, missed[:min(len(missed), 20)])
	}
	t.Logf("%d placeable items", placeable)
}

func TestAnItemThatPlacesNoBlockIsRefused(t *testing.T) {
	t.Parallel()

	// A pickaxe places nothing, and answering with a stone would be worse than
	// failing: the caller would place a block the player never chose.
	pickaxe, ok := dataset(t).Items().ByName("diamond_pickaxe")
	if !ok {
		t.Fatal("this version has no diamond pickaxe")
	}

	_, err := placer(t).PlacedState(pickaxe.ID, placement.Target{}, mining.FaceTop, 0, geom.Vec3{})
	if !errors.Is(err, ErrNotPlaceable) {
		t.Fatalf("PlacedState of a pickaxe = %v, want ErrNotPlaceable", err)
	}
}
