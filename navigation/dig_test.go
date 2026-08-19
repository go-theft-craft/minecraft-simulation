package navigation

import (
	"context"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/terrain"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

func TestEdgeDigIsNamedAndAppendedAfterPillar(t *testing.T) {
	if got := EdgeDig.String(); got != "dig" {
		t.Fatalf("EdgeDig.String() = %q, want %q", got, "dig")
	}
	// The kinds reach recorded paths as numbers, so a new one is appended and
	// never inserted. Pinning the adjacency is what makes an insertion fail
	// here rather than in a recording taken a month ago.
	if EdgeDig != EdgePillar+1 {
		t.Fatalf("EdgeDig = %d, want %d (one past EdgePillar)", EdgeDig, EdgePillar+1)
	}
}

// refBedrock is a handle no tool touches. refStone, the breakable one, is
// declared beside the overlay tests.
const refBedrock world.BlockRef = 7

// tableBreaker prices the handles it knows and refuses every other, which is
// how a real breaker reports bedrock: mining calls it ErrUnbreakable, a fact
// about the block rather than a failure.
type tableBreaker map[world.BlockRef]float64

func (t tableBreaker) BreakTicks(ref world.BlockRef) (float64, bool) {
	ticks, ok := t[ref]

	return ticks, ok
}

// digger is walker with a tool: stone in 20 ticks, bedrock never.
func digger() Capability {
	c := walker
	c.Breaker = tableBreaker{refStone: 20}

	return c
}

// walledCorridor is a one-lane floor from x=-1 to x=5 with a wall of ref
// filling both of the body's cells at x=2, so the only way east is through it.
// The lane is one cell wide on purpose: a corridor three wide is a corridor
// with a way around the wall, and every dig assertion below would pass without
// a dig happening.
func walledCorridor(ref world.BlockRef) *world.Blocks {
	blocks := flat(-1, 0, 5, 0)
	for y := int32(0); y < 2; y++ {
		blocks.SetBlock(geom.BlockPos{X: 2, Y: y}, ref, geom.FullCube())
	}

	return blocks
}

func TestABodyWithAToolRoutesThroughAWall(t *testing.T) {
	path, err := Find(context.Background(), walledCorridor(refStone), nil, digger(),
		geom.BlockPos{}, GoalBlock{Pos: geom.BlockPos{X: 4}}, wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("path incomplete, reason %v", path.Reason)
	}

	var dug []geom.BlockPos
	for _, edge := range path.Edges {
		if edge.Kind == EdgeDig {
			dug = append(dug, edge.To)
		}
	}
	if len(dug) != 1 {
		t.Fatalf("dug %v, want exactly the wall cell", dug)
	}
	if dug[0] != (geom.BlockPos{X: 2}) {
		t.Fatalf("dug %v, want the wall at x=2", dug[0])
	}
}

func TestABodyWithNoToolIsStoppedByTheSameWall(t *testing.T) {
	path, err := Find(context.Background(), walledCorridor(refStone), nil, walker,
		geom.BlockPos{}, GoalBlock{Pos: geom.BlockPos{X: 4}}, wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if path.Complete {
		t.Fatal("a body with no breaker walked through a wall")
	}
}

func TestAnUnbreakableWallProducesNoDigEdge(t *testing.T) {
	path, err := Find(context.Background(), walledCorridor(refBedrock), nil, digger(),
		geom.BlockPos{}, GoalBlock{Pos: geom.BlockPos{X: 4}}, wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if path.Complete {
		t.Fatal("a body dug through bedrock its breaker refused")
	}
	for _, edge := range path.Edges {
		if edge.Kind == EdgeDig {
			t.Fatalf("produced a dig edge into %v, which nothing can break", edge.To)
		}
	}
}

func TestADigCostsTheWalkPlusEveryCellItClears(t *testing.T) {
	// The wall fills both of the body's cells, so the break is charged twice.
	path, err := Find(context.Background(), walledCorridor(refStone), nil, digger(),
		geom.BlockPos{}, GoalBlock{Pos: geom.BlockPos{X: 4}}, wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}

	want := walker.WalkTicks + 20 + 20
	for _, edge := range path.Edges {
		if edge.Kind != EdgeDig {
			continue
		}
		if edge.Cost != want {
			t.Fatalf("dig cost = %v, want %v (walk %v plus two stone breaks)",
				edge.Cost, want, walker.WalkTicks)
		}
	}
}

func TestADigBudgetSmallerThanTheWallRefusesTheRoute(t *testing.T) {
	tight := digger()
	tight.DigBudget = 1

	path, err := Find(context.Background(), walledCorridor(refStone), nil, tight,
		geom.BlockPos{}, GoalBlock{Pos: geom.BlockPos{X: 4}}, wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if path.Complete {
		t.Fatal("a body with one break in it cleared a wall two cells tall")
	}
}

func TestSpanOfCoversEveryCellABodyStandsIn(t *testing.T) {
	cell := geom.BlockPos{X: 3, Y: 64, Z: -1}

	short := spanOf(terrain.Body{HalfWidth: 0.3, Height: 1.8}, cell)
	if len(short) != 2 {
		t.Fatalf("a 1.8-tall body spans %d cells, want 2", len(short))
	}
	if short[0] != cell || short[1] != (geom.BlockPos{X: 3, Y: 65, Z: -1}) {
		t.Fatalf("span = %v, want the cell and the one above it", short)
	}

	tall := spanOf(terrain.Body{HalfWidth: 0.3, Height: 2.5}, cell)
	if len(tall) != 3 {
		t.Fatalf("a 2.5-tall body spans %d cells, want 3", len(tall))
	}
}

func TestADugViewAnswersAirWithoutInventingOne(t *testing.T) {
	blocks := walledCorridor(refStone)
	wall := geom.BlockPos{X: 2}
	view := dugView{view: blocks, span: []geom.BlockPos{wall}}

	if _, lookup := view.CollisionShape(wall); lookup != world.LookupAir {
		t.Fatalf("a dug cell reports %v, want air", lookup)
	}
	if ref, _ := view.BlockState(wall); ref != refStone {
		t.Fatal("a dug view changed which block a cell holds; only its shape is masked")
	}
	// Nothing described the sky above the corridor.
	far := geom.BlockPos{X: 2, Y: 200}
	if _, lookup := view.CollisionShape(far); lookup != world.LookupUnknown {
		t.Fatal("a dug view described a cell nobody had described")
	}
	// A cell outside the span is deferred, wall or not.
	if _, lookup := view.CollisionShape(geom.BlockPos{X: 2, Y: 1}); lookup == world.LookupAir {
		t.Fatal("a dug view cleared a cell outside its span")
	}
}

func TestABreakerAloneMakesABodyMutating(t *testing.T) {
	if walker.mutates() {
		t.Fatal("a body that can neither place nor break is mutating")
	}
	if !digger().mutates() {
		t.Fatal("a body with a breaker is not mutating; its route would never be validated")
	}
}

func TestAHoleHidesTheBaseBlockWithoutInventingOne(t *testing.T) {
	at := geom.BlockPos{X: 0, Y: 64}

	base := world.NewBlocks()
	base.SetBlock(at, refStone, geom.FullCube())

	overlay := NewOverlay(base)
	overlay.Break(at)

	if _, lookup := overlay.CollisionShape(at); lookup != world.LookupAir {
		t.Fatalf("a dug cell reports %v through the overlay, want air", lookup)
	}
	if ref, _ := overlay.BlockState(at); ref != 0 {
		t.Fatalf("a dug cell still holds handle %v", ref)
	}
	if _, lookup := base.CollisionShape(at); lookup == world.LookupAir {
		t.Fatal("the break reached the caller's snapshot")
	}

	// Nobody described the cell above, and digging one does not describe it.
	undescribed := geom.BlockPos{X: 0, Y: 65}
	overlay.Break(undescribed)
	if _, lookup := overlay.CollisionShape(undescribed); lookup != world.LookupUnknown {
		t.Fatal("breaking an undescribed cell described it")
	}

	overlay.Reset()
	if _, lookup := overlay.CollisionShape(at); lookup == world.LookupAir {
		t.Fatal("Reset left the hole behind")
	}
}

func TestBreakingACellThisRouteFilledLeavesAHole(t *testing.T) {
	at := geom.BlockPos{X: 0, Y: 64}

	base := world.NewBlocks()
	base.Set(at, geom.EmptyShape())

	overlay := NewOverlay(base)
	overlay.Place(at, refStone, geom.FullCube())
	overlay.Break(at)

	if _, lookup := overlay.CollisionShape(at); lookup != world.LookupAir {
		t.Fatal("a cell placed and then broken still holds the block")
	}
}
