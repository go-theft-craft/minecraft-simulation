package navigation

import (
	"context"
	"math/rand/v2"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/terrain"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// seeds are fixed so a failure reproduces exactly. Add to this list rather
// than randomizing it, matching collision/property_test.go.
var seeds = []uint64{1, 2, 3, 5, 8, 13, 21, 34, 55, 89}

// terrainClear is spelled out because the property loops compare against it
// repeatedly and the qualified name buries the assertion.
const terrainClear = terrain.Clear

// maze returns a flat world with pillars knocked into it, deterministic for a
// seed. The start and goal cells are always left open.
func maze(seed uint64) *world.Blocks {
	blocks := flat(-1, -1, 12, 12)
	random := rand.New(rand.NewPCG(seed, 0))

	for x := int32(0); x <= 11; x++ {
		for z := int32(0); z <= 11; z++ {
			if x == 0 && z == 0 {
				continue
			}
			if x == 11 && z == 11 {
				continue
			}
			if random.Float64() < 0.25 {
				blocks.Set(geom.BlockPos{X: x, Y: 0, Z: z}, geom.FullCube())
				blocks.Set(geom.BlockPos{X: x, Y: 1, Z: z}, geom.FullCube())
			}
		}
	}

	return blocks
}

// TestPathsAreContiguous is the first exit property: every edge leaves where
// the previous one arrived. A search that lost a parent link returns a path
// that teleports, and a caller following it walks into a wall.
func TestPathsAreContiguous(t *testing.T) {
	for _, capability := range capabilities() {
		for _, seed := range seeds {
			path := findPathWith(t, maze(seed), capability.value)
			for i := 1; i < len(path.Edges); i++ {
				if path.Edges[i].From != path.Edges[i-1].To {
					t.Fatalf(
						"%s seed %d: edge %d leaves %v but edge %d arrived at %v",
						capability.name, seed, i, path.Edges[i].From, i-1, path.Edges[i-1].To,
					)
				}
			}
		}
	}
}

// TestPathsStartAtTheOrigin guards the partial-path case: a bounded search
// must still hand back something the caller can start walking.
func TestPathsStartAtTheOrigin(t *testing.T) {
	for _, capability := range capabilities() {
		for _, seed := range seeds {
			path := findPathWith(t, maze(seed), capability.value)
			if len(path.Edges) == 0 {
				continue
			}
			if path.Edges[0].From != (geom.BlockPos{X: 0, Y: 0, Z: 0}) {
				t.Fatalf("%s seed %d: path starts at %v, want the origin",
					capability.name, seed, path.Edges[0].From)
			}
		}
	}
}

// TestPathCostIsTheSumOfItsEdges guards against a cost that drifts from the
// route it describes, which would make one path compare wrongly against
// another.
func TestPathCostIsTheSumOfItsEdges(t *testing.T) {
	for _, capability := range capabilities() {
		for _, seed := range seeds {
			path := findPathWith(t, maze(seed), capability.value)
			if !path.Complete {
				continue
			}

			var total float64
			for _, edge := range path.Edges {
				total += edge.Cost
			}
			if total != path.Cost {
				t.Fatalf("%s seed %d: Cost = %v, edges sum to %v",
					capability.name, seed, path.Cost, total)
			}
		}
	}
}

// TestEveryEdgeLandsSomewhereStandable checks the property a caller actually
// depends on: each arrival is a cell the body can occupy.
//
// The check is per edge kind rather than one rule for all of them, because the
// expanded vocabulary arrives in cells that are legal for reasons Passable
// alone does not cover. A climb ends inside a ladder column, where nothing
// holds the body up and nothing needs to; a door ends in a cell that is only
// open because the body opened it.
func TestEveryEdgeLandsSomewhereStandable(t *testing.T) {
	for _, capability := range capabilities() {
		for _, seed := range seeds {
			blocks := maze(seed)
			path := findPathWith(t, blocks, capability.value)
			o := directOracle{
				query:      capability.value.query(blocks, nil),
				capability: capability.value,
				crawlQuery: capability.value.crawling().query(blocks, nil),
			}

			for i, edge := range path.Edges {
				legal, err := arrivalIsLegal(o, edge)
				if err != nil {
					t.Fatalf("%s seed %d: %v", capability.name, seed, err)
				}
				if !legal {
					t.Fatalf("%s seed %d: edge %d (%v) lands somewhere the body cannot be: %v",
						capability.name, seed, i, edge.Kind, edge.To)
				}
			}
		}
	}
}

// arrivalIsLegal re-derives, from the world alone, whether an edge's
// destination is somewhere its kind may end.
func arrivalIsLegal(o directOracle, edge Edge) (bool, error) {
	switch edge.Kind {
	case EdgeClimb:
		// A climb ends inside a climbable column or on the cell it steps off
		// onto. Either way the body has to fit; nothing has to hold it up.
		return o.clear(edge.To)
	case EdgeDoor:
		passable, err := o.passableThroughDoor(edge.To)

		return passable == terrainClear, err
	case EdgePlace, EdgePillar:
		// A mutating edge is legal against the world as its own route leaves
		// it, not against the untouched one this property reads. Find has
		// already validated it forward through an overlay, and
		// TestAReturnedPathIsSelfConsistent is where that is asserted.
		return true, nil
	case EdgeWalk, EdgeStep, EdgeFall, EdgeSwim, EdgeJumpGap, EdgeWaterDrop:
		passable, err := o.passable(edge.To)

		return passable == terrainClear, err
	}

	return false, nil
}

// TestOnlyTheShippedEdgeKindsAppearWithoutOptionalBehaviour is the amendment's
// own criterion: a capability with every optional behaviour off routes exactly
// as it did before any of them existed.
//
// It is a property rather than a single case because the failure it guards
// against is an edge that leaks into a search that never asked for it, and that
// shows up on some worlds and not others.
func TestOnlyTheShippedEdgeKindsAppearWithoutOptionalBehaviour(t *testing.T) {
	shipped := map[EdgeKind]struct{}{
		EdgeWalk: {}, EdgeStep: {}, EdgeFall: {}, EdgeSwim: {},
	}

	for _, seed := range seeds {
		path := findPath(t, maze(seed))
		for i, edge := range path.Edges {
			if _, ok := shipped[edge.Kind]; !ok {
				t.Fatalf("seed %d: edge %d is a %v, and the capability enables none of the new edges",
					seed, i, edge.Kind)
			}
			if edge.Posture != PostureStand && edge.Posture != PostureSwim {
				t.Fatalf("seed %d: edge %d arrives in posture %v, which this capability does not have",
					seed, i, edge.Posture)
			}
		}
	}
}

// TestTheHeuristicNeverOverestimates is the test that catches an inadmissible
// heuristic, which is the failure mode worth the most care here.
//
// The heuristic scales remaining distance by the lowest price the body can pay
// per block of it. If any edge closes distance more cheaply than that floor, the
// heuristic overestimates, A* stops being optimal, and the symptom is a route
// that is merely suboptimal — which is the hardest kind of wrongness to notice
// and the hardest to trace back here.
//
// Every edge the capability can produce has to be checked, which is why this
// runs over random capabilities with random subsets enabled rather than over
// the two named ones.
func TestTheHeuristicNeverOverestimates(t *testing.T) {
	for _, seed := range seeds {
		random := rand.New(rand.NewPCG(seed, 7))

		for range 200 {
			capability := randomCapability(random)
			floor := capability.perBlockFloor()

			for _, rate := range perBlockRates(capability) {
				if floor > rate.perBlock {
					t.Fatalf(
						"seed %d: the floor is %v but %s closes a block for %v; the heuristic overestimates",
						seed, floor, rate.name, rate.perBlock,
					)
				}
			}
		}
	}
}

// rate is one edge's price per block of Manhattan distance it closes.
type rate struct {
	name     string
	perBlock float64
}

// perBlockRates returns the cheapest per-block price each enabled edge can
// reach.
//
// A step closes two blocks — one across and one up — for one step's price, and
// a fall of depth D closes 1+D blocks for FallTicks*D, which is cheapest per
// block at D=1. A jump of distance D closes D blocks for JumpTicks*D, so its
// rate is flat. Getting these wrong in either direction is what the test above
// exists to catch, so they are written out rather than folded into the floor.
func perBlockRates(c Capability) []rate {
	rates := []rate{
		{name: "walk", perBlock: c.WalkTicks},
		{name: "step", perBlock: c.StepTicks / 2},
		{name: "fall", perBlock: c.FallTicks / 2},
	}
	if c.CanSwim {
		rates = append(rates, rate{name: "swim", perBlock: c.SwimTicks})
	}
	if c.JumpReach >= minJump {
		rates = append(rates, rate{name: "jump", perBlock: c.JumpTicks})
	}
	if c.SneakTicks > 0 {
		rates = append(rates, rate{name: "sneak", perBlock: c.SneakTicks})
	}
	if c.CrawlHeight > 0 && c.CrawlHeight < c.Body.Height {
		rates = append(rates, rate{name: "crawl", perBlock: c.CrawlTicks})
	}
	if c.CanClimb {
		rates = append(rates, rate{name: "climb", perBlock: c.ClimbTicks})
	}
	if c.CanOpenDoors {
		rates = append(rates, rate{name: "door", perBlock: c.DoorTicks})
	}
	if c.CanPlace && c.BlockBudget > 0 {
		// A bridge closes one block across and a pillar closes one block up,
		// and both are priced the same: a placement plus what the block is
		// worth.
		placed := c.PlaceTicks + c.BlockTicks
		rates = append(
			rates,
			rate{name: "place", perBlock: placed},
			rate{name: "pillar", perBlock: placed},
		)
	}
	// A water drop is priced at FallTicks per block dropped, which is the
	// ordinary fall's rate and is already above.

	return rates
}

// randomCapability builds a body with a random subset of the optional edges
// enabled and random prices for all of them.
func randomCapability(random *rand.Rand) Capability {
	price := func() float64 { return 1 + random.Float64()*20 }

	capability := Capability{
		Body:      terrain.Body{HalfWidth: 0.3, Height: 1.8, StepHeight: 0.6},
		SafeFall:  3,
		WalkTicks: price(),
		StepTicks: price(),
		FallTicks: price(),
		SwimTicks: price(),
		CanSwim:   random.IntN(2) == 0,
	}
	if random.IntN(2) == 0 {
		capability.JumpReach = 2 + random.Float64()*2
		capability.JumpRise = 1.25
		capability.JumpTicks = price()
	}
	if random.IntN(2) == 0 {
		capability.SneakTicks = price()
	}
	if random.IntN(2) == 0 {
		capability.CrawlHeight = 0.6
		capability.CrawlTicks = price()
	}
	if random.IntN(2) == 0 {
		capability.CanClimb = true
		capability.ClimbTicks = price()
	}
	if random.IntN(2) == 0 {
		capability.CanOpenDoors = true
		capability.DoorTicks = price()
	}
	if random.IntN(2) == 0 {
		// Placement is drawn independently of its price so that the case the
		// floor is wrong for — a placement cheaper per block than a walk — is
		// actually generated rather than assumed impossible.
		capability.CanPlace = true
		capability.PlaceTicks = price()
		capability.BlockTicks = random.Float64() * 5
		capability.BlockBudget = 1 + random.IntN(64)
		capability.MaxPillarHeight = random.IntN(32)
	}

	return capability
}

// named is one capability with a name for a failure message.
type named struct {
	name  string
	value Capability
}

// capabilities returns the bodies every property here is checked against: the
// one that ships and one with every optional behaviour turned on.
//
// Running both is the point. A property that only held for the shipped body
// would say nothing about the edges this amendment added, and one that only
// held for the full body would not prove the shipped search was left alone.
func capabilities() []named {
	full := walker
	full.CanSwim = true
	full.JumpReach = 3.5
	full.JumpRise = 1.25
	full.JumpTicks = 13
	full.SneakTicks = 17
	full.AvoidLedges = true
	full.CrawlHeight = 0.6
	full.CrawlTicks = 19
	full.CanClimb = true
	full.ClimbTicks = 23
	full.CanOpenDoors = true
	full.DoorTicks = 29
	full.WaterLandingDepth = 2
	full.CanPlace = true
	full.PlaceTicks = 31
	full.BlockTicks = 2
	full.BlockBudget = 16
	full.PlacedBlock = refStone
	full.MaxPillarHeight = 8

	return []named{
		{name: "the shipped body", value: walker},
		{name: "every behaviour on", value: full},
	}
}

// TestSearchesAreReproducible is the determinism gate. Go randomizes map
// iteration, so a search that let map order reach an output would return a
// different path on some runs and fail the replay comparison for a reason
// nothing in the recording explains.
func TestSearchesAreReproducible(t *testing.T) {
	for _, capability := range capabilities() {
		for _, seed := range seeds {
			blocks := maze(seed)
			first := findPathWith(t, blocks, capability.value)

			for run := range 100 {
				again := findPathWith(t, blocks, capability.value)
				if again.Cost != first.Cost || again.Reason != first.Reason {
					t.Fatalf("%s seed %d run %d: path summary changed", capability.name, seed, run)
				}
				if len(again.Edges) != len(first.Edges) {
					t.Fatalf("%s seed %d run %d: edge count changed", capability.name, seed, run)
				}
				for i := range first.Edges {
					if again.Edges[i] != first.Edges[i] {
						t.Fatalf("%s seed %d run %d: edge %d changed", capability.name, seed, run, i)
					}
				}
			}
		}
	}
}

// findPath runs one fixed query against a world with the shipped body. It is
// not called search: search is the package-level A* both Find and Planner.Plan
// run.
func findPath(t *testing.T, blocks *world.Blocks) Path {
	t.Helper()

	return findPathWith(t, blocks, walker)
}

// findPathWith runs the same query for a given body.
func findPathWith(t *testing.T, blocks *world.Blocks, capability Capability) Path {
	t.Helper()

	path, err := Find(
		context.Background(), blocks, nil, capability,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 11, Y: 0, Z: 11},
		Budget{Nodes: 5_000, Ceiling: 5_000},
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}

	return path
}
