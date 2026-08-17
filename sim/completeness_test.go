package sim

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func TestDependencyKindString(t *testing.T) {
	for _, test := range []struct {
		kind DependencyKind
		want string
	}{
		{DependencyBlock, "block"},
		{DependencyEntity, "entity"},
		{DependencyRegistry, "registry"},
		{DependencyKind(9), "DependencyKind(9)"},
	} {
		if got := test.kind.String(); got != test.want {
			t.Errorf("DependencyKind(%d).String() = %q, want %q", test.kind, got, test.want)
		}
	}
}

func TestDependencyStringNamesOnlyWhatMatters(t *testing.T) {
	block := Dependency{Kind: DependencyBlock, Block: geom.BlockPos{X: 1, Y: -2, Z: 3}}
	if got, want := block.String(), "block(1,-2,3)"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}

	body := Dependency{Kind: DependencyEntity, Entity: entity.ID(42)}
	if got, want := body.String(), "entity(42)"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}

	registry := Dependency{Kind: DependencyRegistry, Name: "blocks"}
	if got, want := registry.String(), "registry(blocks)"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}

func TestSortDependenciesDeduplicatesAndOrders(t *testing.T) {
	in := []Dependency{
		{Kind: DependencyEntity, Entity: 5},
		{Kind: DependencyBlock, Block: geom.BlockPos{X: 1}},
		{Kind: DependencyBlock, Block: geom.BlockPos{X: 0, Y: 4}},
		{Kind: DependencyBlock, Block: geom.BlockPos{X: 1}},
		{Kind: DependencyRegistry, Name: "blocks"},
		{Kind: DependencyEntity, Entity: 2},
	}

	got := sortDependencies(in)
	want := []Dependency{
		{Kind: DependencyBlock, Block: geom.BlockPos{X: 0, Y: 4}},
		{Kind: DependencyBlock, Block: geom.BlockPos{X: 1}},
		{Kind: DependencyEntity, Entity: 2},
		{Kind: DependencyEntity, Entity: 5},
		{Kind: DependencyRegistry, Name: "blocks"},
	}
	if len(got) != len(want) {
		t.Fatalf("sortDependencies returned %d entries, want %d: %v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("sortDependencies[%d] = %+v, want %+v", index, got[index], want[index])
		}
	}
}

func TestSortDependenciesDoesNotTouchItsInput(t *testing.T) {
	in := []Dependency{
		{Kind: DependencyEntity, Entity: 5},
		{Kind: DependencyBlock, Block: geom.BlockPos{X: 1}},
	}
	first := in[0]

	sortDependencies(in)
	if in[0] != first {
		t.Fatalf("sortDependencies reordered its argument: %+v", in)
	}
}

func TestAnIncompleteResultNamesWhatWasMissing(t *testing.T) {
	completeness := Completeness{Missing: []Dependency{{Kind: DependencyBlock}}}
	if completeness.Complete {
		t.Error("a completeness with missing dependencies reports itself complete")
	}
}
