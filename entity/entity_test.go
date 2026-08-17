package entity

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func TestFamilyString(t *testing.T) {
	for _, test := range []struct {
		family Family
		want   string
	}{
		{FamilyUnknown, "unknown"},
		{FamilyPlayer, "player"},
		{Family(200), "Family(200)"},
	} {
		if got := test.family.String(); got != test.want {
			t.Errorf("Family(%d).String() = %q, want %q", test.family, got, test.want)
		}
	}
}

func TestStateIsComparable(t *testing.T) {
	// State is compared with == throughout the tests below and inside runtime,
	// which requires every field to stay comparable. A slice or a map added to
	// this struct would break that silently, so the check is explicit.
	first := State{Family: FamilyPlayer, Box: geom.AABB{MaxX: 1}, StepHeight: 0.5}
	second := first
	if first != second {
		t.Fatal("a copy of a state does not equal its original")
	}
}
