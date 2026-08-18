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

// TestTheFamiliesKeepTheirNumbers pins what a recording's digest depends on. A
// family inserted rather than appended would renumber every family after it,
// and every recording taken before the change would disagree with the build.
func TestTheFamiliesKeepTheirNumbers(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		family Family
		number uint8
		name   string
	}{
		{FamilyUnknown, 0, "unknown"},
		{FamilyPlayer, 1, "player"},
		{FamilyItem, 2, "item"},
		{FamilyArrow, 3, "arrow"},
	} {
		if got := uint8(tc.family); got != tc.number {
			t.Errorf("%s = %d, want %d", tc.name, got, tc.number)
		}
		if got := tc.family.String(); got != tc.name {
			t.Errorf("String() = %q, want %q", got, tc.name)
		}
	}
}
