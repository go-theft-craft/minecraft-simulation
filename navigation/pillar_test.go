package navigation

import (
	"context"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// pillaring is a builder allowed to stack blocks under itself.
func pillaring() Capability {
	capability := builder()
	capability.MaxPillarHeight = 32

	return capability
}

// wellShaft returns a one-column shaft of the given depth with solid rock all
// around it and open sky above.
//
// Every read-only edge is exhausted in it: a step rises one block, a jump needs
// somewhere to land, and a climb needs a ladder. The only way up is to build
// one.
func wellShaft(depth int32, roofed bool) *world.Blocks {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -2, Y: -depth - 1, Z: -2}, geom.BlockPos{X: 2, Y: depth + 4, Z: 2}, geom.FullCube())
	// The shaft itself, from the floor up to ground level and above.
	blocks.Fill(geom.BlockPos{X: 0, Y: -depth, Z: 0}, geom.BlockPos{X: 0, Y: depth + 4, Z: 0}, geom.EmptyShape())
	if roofed {
		blocks.Set(geom.BlockPos{X: 0, Y: -depth + 2, Z: 0}, geom.FullCube())
	}

	return blocks
}

// TestABodyPillarsOutOfAShaft is the reachability this edge exists for.
func TestABodyPillarsOutOfAShaft(t *testing.T) {
	const depth = 10

	path, err := Find(
		context.Background(), wellShaft(depth, false), nil, pillaring(),
		geom.BlockPos{X: 0, Y: -depth, Z: 0}, GoalBlock{Pos: geom.BlockPos{X: 0, Y: 0, Z: 0}},
		wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("could not pillar out of a shaft: %v", path.Reason)
	}
	if pillared := countKind(path, EdgePillar); pillared != depth {
		t.Fatalf("used %d pillar edges to rise %d blocks: %v", pillared, depth, path.Edges)
	}
}

// TestABodyWithNoBlocksStaysInTheShaft is the control.
func TestABodyWithNoBlocksStaysInTheShaft(t *testing.T) {
	const depth = 10

	path, err := Find(
		context.Background(), wellShaft(depth, false), nil, walker,
		geom.BlockPos{X: 0, Y: -depth, Z: 0}, GoalBlock{Pos: geom.BlockPos{X: 0, Y: 0, Z: 0}},
		wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if path.Complete {
		t.Fatal("climbed out of a sheer ten-block shaft with nothing to build from")
	}
}

// TestPillaringIntoACeilingIsRefused pins the precondition that the cell above
// has to admit the body.
func TestPillaringIntoACeilingIsRefused(t *testing.T) {
	const depth = 10

	path, err := Find(
		context.Background(), wellShaft(depth, true), nil, pillaring(),
		geom.BlockPos{X: 0, Y: -depth, Z: 0}, GoalBlock{Pos: geom.BlockPos{X: 0, Y: 0, Z: 0}},
		wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if path.Complete {
		t.Fatal("pillared through a ceiling")
	}
}

// TestNoPathPillarsDownward pins that the edge is not treated as reversible.
//
// A pillar cannot be walked back down: coming down is a fall within the safe
// fall, or a dig beneath the body otherwise. A symmetric edge is the natural
// thing to write and it produces routes that strand the body on a tower.
func TestNoPathPillarsDownward(t *testing.T) {
	capability := pillaring()
	view := wellShaft(10, false)
	o := directOracle{
		query:      capability.query(view, nil),
		capability: capability,
		crawlQuery: capability.crawling().query(view, nil),
	}

	edges, err := capability.pillars(o, node{Pos: geom.BlockPos{X: 0, Y: -10, Z: 0}, Posture: PostureStand})
	if err != nil {
		t.Fatalf("pillars: %v", err)
	}
	if len(edges) == 0 {
		t.Fatal("a body in a shaft produced no pillar, so the direction proves nothing")
	}
	for _, edge := range edges {
		if edge.To.Y <= edge.From.Y {
			t.Fatalf("a pillar edge goes from %v to %v; it must only rise", edge.From, edge.To)
		}
	}
}

// TestAnAirborneBodyDoesNotPillar pins that a pillar starts from the ground.
func TestAnAirborneBodyDoesNotPillar(t *testing.T) {
	capability := pillaring()
	view := wellShaft(10, false)
	o := directOracle{
		query:      capability.query(view, nil),
		capability: capability,
		crawlQuery: capability.crawling().query(view, nil),
	}

	edges, err := capability.pillars(o, node{Pos: geom.BlockPos{X: 0, Y: -10, Z: 0}, Posture: PostureFall})
	if err != nil {
		t.Fatalf("pillars: %v", err)
	}
	if len(edges) != 0 {
		t.Fatalf("an airborne body produced %d pillar edges, want none", len(edges))
	}
}

// TestThePerColumnPillarLimitIsRespected pins the bound that stops a search
// building a tower to reach something it could have walked to.
func TestThePerColumnPillarLimitIsRespected(t *testing.T) {
	const depth = 10

	capability := pillaring()
	capability.MaxPillarHeight = 3

	path, err := Find(
		context.Background(), wellShaft(depth, false), nil, capability,
		geom.BlockPos{X: 0, Y: -depth, Z: 0}, GoalBlock{Pos: geom.BlockPos{X: 0, Y: 0, Z: 0}},
		wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}

	column := make(map[geom.BlockPos]int)
	for _, edge := range path.Edges {
		if edge.Kind != EdgePillar {
			continue
		}
		foot := geom.BlockPos{X: edge.From.X, Z: edge.From.Z}
		column[foot]++
		if column[foot] > capability.MaxPillarHeight {
			t.Fatalf("stacked %d pillars in one column with a limit of %d",
				column[foot], capability.MaxPillarHeight)
		}
	}
	if path.Complete {
		t.Fatal("rose ten blocks in a one-column shaft with a three-block pillar limit")
	}
}

// TestAGoalOutsideTheVerticalEnvelopeIsUnreachable pins the other bound.
//
// Placing makes every Y coordinate reachable from every position. The envelope
// is what stops a search spending its whole node budget finding that out.
func TestAGoalOutsideTheVerticalEnvelopeIsUnreachable(t *testing.T) {
	const depth = 10

	capability := pillaring()
	capability.VerticalEnvelope = 4

	path, err := Find(
		context.Background(), wellShaft(depth, false), nil, capability,
		geom.BlockPos{X: 0, Y: -depth, Z: 0}, GoalBlock{Pos: geom.BlockPos{X: 0, Y: 0, Z: 0}},
		wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if path.Complete {
		t.Fatal("reached a goal ten blocks up with a four-block envelope")
	}
	for _, edge := range path.Edges {
		if edge.To.Y+depth > capability.VerticalEnvelope {
			t.Fatalf("expanded to %v, outside a %d-block envelope from y=%d",
				edge.To, capability.VerticalEnvelope, -depth)
		}
	}
}

// TestAZeroEnvelopeIsUnbounded pins the default, which is the shipped search.
func TestAZeroEnvelopeIsUnbounded(t *testing.T) {
	if !walker.withinEnvelope(geom.BlockPos{}, geom.BlockPos{Y: 1_000_000}) {
		t.Fatal("a capability with no stated envelope bounded a search")
	}
}

// TestAPillarCostsAPlacement pins the price.
func TestAPillarCostsAPlacement(t *testing.T) {
	capability := pillaring()

	path, err := Find(
		context.Background(), wellShaft(3, false), nil, capability,
		geom.BlockPos{X: 0, Y: -3, Z: 0}, GoalBlock{Pos: geom.BlockPos{X: 0, Y: 0, Z: 0}},
		wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	want := capability.PlaceTicks + capability.BlockTicks
	for _, edge := range path.Edges {
		if edge.Kind == EdgePillar && edge.Cost != want {
			t.Errorf("a pillar cost %v, want %v", edge.Cost, want)
		}
	}
}
