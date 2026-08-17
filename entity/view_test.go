package entity

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func player() State {
	return State{
		Family:     FamilyPlayer,
		Box:        geom.AABB{MinX: -0.3, MinZ: -0.3, MaxX: 0.3, MaxY: 1.8, MaxZ: 0.3},
		StepHeight: float64(float32(0.6)),
	}
}

func TestBodiesReturnsWhatItWasGiven(t *testing.T) {
	bodies := NewBodies()
	bodies.Set(7, player())

	got, ok := bodies.Entity(7)
	if !ok {
		t.Fatal("Entity reported the body missing")
	}
	if got != player() {
		t.Fatalf("Entity = %+v, want %+v", got, player())
	}
}

func TestBodiesReportsAMissingEntity(t *testing.T) {
	bodies := NewBodies()
	if _, ok := bodies.Entity(1); ok {
		t.Fatal("Entity found a body that was never set")
	}
}

func TestRemoveDropsTheBody(t *testing.T) {
	bodies := NewBodies()
	bodies.Set(3, player())
	bodies.Remove(3)

	if _, ok := bodies.Entity(3); ok {
		t.Fatal("Entity found a removed body")
	}
}

func TestIDsAreSortedAndStable(t *testing.T) {
	bodies := NewBodies()
	for _, id := range []ID{9, -4, 0, 2, 100, -1} {
		bodies.Set(id, player())
	}

	want := []ID{-4, -1, 0, 2, 9, 100}
	for attempt := range 20 {
		got := bodies.IDs()
		if len(got) != len(want) {
			t.Fatalf("attempt %d: IDs returned %d entries, want %d", attempt, len(got), len(want))
		}
		for index := range want {
			if got[index] != want[index] {
				t.Fatalf("attempt %d: IDs = %v, want %v", attempt, got, want)
			}
		}
	}
}

func TestCloneDoesNotFollowItsOrigin(t *testing.T) {
	bodies := NewBodies()
	bodies.Set(1, player())

	clone := bodies.Clone()
	bodies.Set(1, State{Family: FamilyUnknown})
	bodies.Set(2, player())

	got, ok := clone.Entity(1)
	if !ok || got != player() {
		t.Fatalf("the clone followed its origin: (%+v, %v)", got, ok)
	}
	if _, ok := clone.Entity(2); ok {
		t.Fatal("the clone gained a body its origin added afterwards")
	}
}

var _ View = (*Bodies)(nil)
