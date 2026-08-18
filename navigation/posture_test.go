package navigation

import (
	"context"
	"slices"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// careful is walker told to keep off edges, with a crouch it can cross them on.
func careful() Capability {
	capability := walker
	capability.AvoidLedges = true
	capability.SneakTicks = 17

	return capability
}

// crawler is walker with a prone height, which is what 26.1.2 has and 1.8.9
// does not.
func crawler() Capability {
	capability := walker
	capability.CrawlHeight = 0.6
	capability.CrawlTicks = 19

	return capability
}

// bridge returns a wide ledge, a one-cell-wide walkway, and a wide ledge again,
// over a drop far deeper than the body survives.
//
// The air below the walkway is described rather than left unset, because an
// undescribed cell is refused rather than read as a hole, and this world is
// about holes.
func bridge() *world.Blocks {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -1, Y: -30, Z: -2}, geom.BlockPos{X: 7, Y: 4, Z: 2}, geom.EmptyShape())
	blocks.Fill(geom.BlockPos{X: -1, Y: -31, Z: -2}, geom.BlockPos{X: 7, Y: -31, Z: 2}, geom.FullCube())

	// The wide ends.
	blocks.Fill(geom.BlockPos{X: 0, Y: -1, Z: -1}, geom.BlockPos{X: 1, Y: -1, Z: 1}, geom.FullCube())
	blocks.Fill(geom.BlockPos{X: 4, Y: -1, Z: -1}, geom.BlockPos{X: 5, Y: -1, Z: 1}, geom.FullCube())
	// The walkway between them, one cell wide.
	blocks.Fill(geom.BlockPos{X: 2, Y: -1, Z: 0}, geom.BlockPos{X: 3, Y: -1, Z: 0}, geom.FullCube())

	return blocks
}

// TestABodySneaksAcrossALedgeAndStandsElsewhere is the per-position claim.
//
// Sneaking is a posture rather than a flag because a body sneaks at the cells
// that need it and stands everywhere else. A route that was all one or all the
// other would pass a test that only counted sneaks.
func TestABodySneaksAcrossALedgeAndStandsElsewhere(t *testing.T) {
	path, err := Find(
		context.Background(), bridge(), nil, careful(),
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 5, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("path incomplete, reason %v", path.Reason)
	}

	var sneaked, stood int
	for _, edge := range path.Edges {
		switch edge.Posture {
		case PostureSneak:
			sneaked++
		case PostureStand:
			stood++
		case PostureSwim, PostureFall, PostureCrawl:
		}
	}

	if sneaked == 0 {
		t.Fatalf("crossed the walkway without sneaking: %v", path.Edges)
	}
	if stood == 0 {
		t.Fatalf("sneaked the whole route; sneaking is per position, not per body: %v", path.Edges)
	}
}

// TestABodyThatCannotSneakRefusesTheLedge pins the other half.
//
// A caller that asked to be kept off edges and gave the body no careful way
// across one gets no route rather than a route along the edge.
func TestABodyThatCannotSneakRefusesTheLedge(t *testing.T) {
	capability := careful()
	capability.SneakTicks = 0

	path, err := Find(
		context.Background(), bridge(), nil, capability,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 5, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if path.Complete {
		t.Fatal("walked a one-cell walkway over a killing drop with edge avoidance on")
	}
}

// TestLedgeAvoidanceIsOffByDefault pins that the shipped search is untouched.
//
// This is the criterion the amendment states as a whole: a capability with
// every optional behaviour off routes exactly as it did before any of them
// existed. Here that means walking the walkway, standing, at the walk price.
func TestLedgeAvoidanceIsOffByDefault(t *testing.T) {
	path, err := Find(
		context.Background(), bridge(), nil, walker,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 5, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("path incomplete, reason %v", path.Reason)
	}
	for _, edge := range path.Edges {
		if edge.Posture != PostureStand {
			t.Fatalf("a capability with no ledge policy produced posture %v", edge.Posture)
		}
		if edge.Cost != walker.WalkTicks {
			t.Fatalf("edge cost %v, want the walk price %v", edge.Cost, walker.WalkTicks)
		}
	}
}

// gapOneBlockTall returns a corridor whose ceiling drops to a single block of
// headroom over two columns.
func gapOneBlockTall() *world.Blocks {
	blocks := flat(-1, -1, 6, 1)
	blocks.Fill(geom.BlockPos{X: 2, Y: 1, Z: -1}, geom.BlockPos{X: 3, Y: 3, Z: 1}, geom.FullCube())

	return blocks
}

// TestABodyCrawlsThroughAOneBlockGap pins the posture that reaches what no
// other one does.
//
// A block grid is why this is crawl and not sneak: 1.8 and 1.5 both need two
// cells, so crouching gets a body through nothing a standing body cannot
// already pass. Going prone is about half a block tall and gets through one.
func TestABodyCrawlsThroughAOneBlockGap(t *testing.T) {
	path, err := Find(
		context.Background(), gapOneBlockTall(), nil, crawler(),
		geom.BlockPos{X: 1, Y: 0, Z: 0}, geom.BlockPos{X: 4, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("path incomplete, reason %v", path.Reason)
	}

	var crawled int
	for _, edge := range path.Edges {
		if edge.Posture == PostureCrawl {
			crawled++
			if edge.Cost != crawler().CrawlTicks {
				t.Errorf("a crawl cost %v, want %v", edge.Cost, crawler().CrawlTicks)
			}
		}
	}
	if crawled != 2 {
		t.Fatalf("crawled %d cells through a two-cell gap: %v", crawled, path.Edges)
	}
}

// TestABodyWithNoCrawlIsStoppedByAOneBlockGap is the same world with the
// posture taken away.
func TestABodyWithNoCrawlIsStoppedByAOneBlockGap(t *testing.T) {
	path, err := Find(
		context.Background(), gapOneBlockTall(), nil, walker,
		geom.BlockPos{X: 1, Y: 0, Z: 0}, geom.BlockPos{X: 4, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if path.Complete {
		t.Fatal("a body 1.8 tall walked through one block of headroom")
	}
}

// TestACrawlHeightNoShorterThanTheBodyProducesNothing pins the boundary.
func TestACrawlHeightNoShorterThanTheBodyProducesNothing(t *testing.T) {
	capability := crawler()
	capability.CrawlHeight = capability.Body.Height

	path, err := Find(
		context.Background(), gapOneBlockTall(), nil, capability,
		geom.BlockPos{X: 1, Y: 0, Z: 0}, geom.BlockPos{X: 4, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if path.Complete {
		t.Fatal("a crawl that shortens nothing got through one block of headroom")
	}
}

// TestTheCrawlAsymmetryIsAssertedInBothDirections is the version gate running
// backwards.
//
// The project's usual rule is that a scenario passing on 1.8.9 and failing on
// 26.1.2 is a failure. Crawl is the first behaviour that goes the other way, and
// the gate's job is to record the absence with a reason rather than skip
// quietly: a future version that gained the posture, or lost it, must fail here
// rather than land silently.
func TestTheCrawlAsymmetryIsAssertedInBothDirections(t *testing.T) {
	view := gapOneBlockTall()
	start, goal := geom.BlockPos{X: 1, Y: 0, Z: 0}, geom.BlockPos{X: 4, Y: 0, Z: 0}

	modern, err := Find(context.Background(), view, nil, crawler(), start, goal, wideBudget)
	if err != nil {
		t.Fatalf("Find with a crawl: %v", err)
	}
	if !modern.Complete {
		t.Fatalf("a body with a crawl could not cross a one-block gap: %v", modern.Reason)
	}

	// 1.8.9 has no crawl. This is not a body that happens to lack the field —
	// it is the version, and the capability says so by carrying no crawl
	// height.
	old, err := Find(context.Background(), view, nil, walker, start, goal, wideBudget)
	if err != nil {
		t.Fatalf("Find without a crawl: %v", err)
	}
	if old.Complete {
		t.Fatal("a body with no crawl crossed a one-block gap; 1.8.9 has no crawl")
	}
	if old.Reason != ReasonUnreachable {
		t.Fatalf("Reason = %v, want ReasonUnreachable — the gap is closed, not merely expensive", old.Reason)
	}
}

// TestACapabilityDeclaresItsPostures pins that the asymmetry is readable off
// the value rather than inferred from which version built it.
func TestACapabilityDeclaresItsPostures(t *testing.T) {
	if slices.Contains(walker.Postures(), PostureCrawl) {
		t.Error("a capability with no crawl height declares a crawl posture")
	}
	if !slices.Contains(crawler().Postures(), PostureCrawl) {
		t.Error("a capability with a crawl height does not declare a crawl posture")
	}
	if !slices.Contains(careful().Postures(), PostureSneak) {
		t.Error("a capability that can sneak does not declare a sneak posture")
	}
	if slices.Contains(walker.Postures(), PostureSneak) {
		t.Error("a capability with no sneak price declares a sneak posture")
	}
	// Every capability stands, including one that can do nothing else.
	if !slices.Contains(Capability{}.Postures(), PostureStand) {
		t.Error("the empty capability does not declare standing")
	}
}
