// Package navigation searches a route through a world and reports it as typed
// edges.
//
// The route is a value rather than a hidden state machine. A caller can print
// it, test it, and compare it against a recording, which a navigator that only
// answered "what do I press this tick" could not.
//
// Every cost is in ticks. Break time is in ticks and movement is in ticks, so
// a version that adds digging can compare "mine through" against "walk around"
// in one unit rather than through a weighting nobody can justify.
//
// Nothing here imports sim. A rule that needs a version's number receives it
// on Capability, which is what lets 1.8.9 and 26.1.2 share this search.
//
// # Where the two supported versions disagree
//
// Crawling is the first behaviour this package carries that one supported
// version has and the other does not. 26.1.2 has a prone posture that fits
// through a one-block gap; 1.8.9 has no crawl at all, so a 1.8.9 capability
// supplies no crawl height and the search produces no crawl edge for it.
//
// That runs backwards through the two-version rule the project otherwise
// applies, which reads that a scenario passing on 1.8.9 and not on 26.1.2 is a
// failure. The resolution is the one the master plan already states: a
// per-version gate is allowed to say that a behaviour does not exist in a
// version and record why. So the absence is asserted in both directions rather
// than skipped — the modern capability crosses the gap, the older one does not,
// and both are tests. A future version that gained or lost a posture would fail
// one of them rather than land in silence.
//
// The asymmetry is a property of the capability, not a branch in the search.
// Capability.Postures reports what a body can do, and nothing here asks which
// version it came from.
package navigation

import (
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/terrain"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// Posture is how a body occupies a position.
//
// Two postures at one position are distinct nodes, because they differ in the
// box the body needs and in which edges leave them.
type Posture uint8

const (
	// PostureStand is a body standing on ground.
	PostureStand Posture = iota
	// PostureSwim is a body in a fluid it can swim.
	PostureSwim
	// PostureFall is a body in the air, holding no ground.
	//
	// It is a posture rather than a flag because the edges leaving it are
	// different: a body already airborne cannot start a jump, which is the
	// constraint this encodes and the reason the jump expansion refuses it.
	//
	// No edge arrives in it, and that is not an omission. Every node in this
	// search is a place the body comes to rest, and a resting body is standing
	// or swimming; being in the air is what happens between two nodes. The
	// pillar edge was expected to produce it, on the argument that the body is
	// airborne at the moment it places the block beneath itself — but it comes
	// to rest on that block, so its node stands.
	//
	// It earns its place as the jump guard and as vocabulary. jumps refuses a
	// body in this posture, which is the rule that a jump starts from the
	// ground, and a follower reporting what the body is doing mid-arc has a
	// name for it.
	PostureFall
	// PostureSneak is a body crouched, which is what crosses a ledge without
	// being steered off it.
	//
	// It is a posture rather than a flag on Capability because it is a
	// per-position decision. A flag would make a body sneak for a whole route
	// or none of it, and the value of sneaking is doing it at the one cell
	// that needs it.
	PostureSneak
	// PostureCrawl is a body prone, which fits through a one-block gap that
	// stops a standing and a crouched body alike.
	//
	// Only 26.1.2 has it. 1.8.9 has no crawl at all, which is the first
	// behaviour in this package that one supported version has and the other
	// does not.
	PostureCrawl
)

// Postures are appended, never inserted, for the reason EdgeKind values are: a
// posture's number reaches a recorded path.

// String returns the posture's name.
func (p Posture) String() string {
	switch p {
	case PostureStand:
		return "stand"
	case PostureSwim:
		return "swim"
	case PostureFall:
		return "fall"
	case PostureSneak:
		return "sneak"
	case PostureCrawl:
		return "crawl"
	default:
		return fmt.Sprintf("Posture(%d)", uint8(p))
	}
}

// Breaker answers how long a body takes to break one block.
//
// It is an interface this package declares rather than a dependency on mining,
// because mining imports sim and nothing in navigation does — that separation
// is what lets the search be tested without a tick loop. A caller holds the
// three things the answer depends on (the held tool, the effects, and the
// version profile), closes over all of them, and answers per block handle.
//
// The bool is "can this be broken at all". mining reports that as
// ErrUnbreakable, which is a fact about bedrock rather than a failure, so the
// seam takes it as an answer and leaves the error for things that really went
// wrong.
type Breaker interface {
	BreakTicks(ref world.BlockRef) (float64, bool)
}

// Capability is what one body can do and what each thing costs it.
//
// A mob is this value with CanSwim false; it gets a ground navigator out of
// the same search. Every duration is in ticks and every one is supplied by the
// caller, because 1.8.9 and 26.1.2 disagree about all of them.
type Capability struct {
	// Body is the box the search routes.
	Body terrain.Body
	// SafeFall is how far the body drops without harm, in blocks.
	SafeFall float64
	// CanSwim allows swim edges.
	CanSwim bool
	// WalkTicks is the cost of crossing one block on the level.
	WalkTicks float64
	// StepTicks is the cost of rising one block.
	StepTicks float64
	// FallTicks is the cost of descending one block.
	FallTicks float64
	// SwimTicks is the cost of crossing one block in fluid.
	SwimTicks float64
	// JumpTicks is the cost of crossing one block by jump, in ticks.
	JumpTicks float64
	// JumpReach is how far the body's jump carries it horizontally, in blocks.
	// Zero produces no jump edges, which is how a mob keeps getting a ground
	// navigator out of the same search.
	//
	// It comes from navigation/reach, which measures it by running the
	// profile's own movement kernel over a flat world. A hand-written value
	// here is a number this repository cannot verify, and the 2026-08-17
	// navigation plan deferred this edge rather than ship one.
	//
	// It is the distance between cell centres the arc covers, so a jump to the
	// cell two away needs a reach of two. Measured from a standing start both
	// supported profiles clear a little over two, which crosses a one-block
	// hole; a body already running clears more, and a caller that measured its
	// own running start may say so.
	JumpReach float64
	// JumpRise is how high the arc's peak is above the take-off, in blocks.
	// The clearance check needs it: a jump passes over the cells between, and
	// what it passes through is what stops it.
	JumpRise float64
	// AvoidLedges refuses to stand in a cell beside a drop the body could not
	// survive, unless it can sneak there.
	//
	// It is off by default, and that default is the shipped search exactly as
	// it was. On a grid a standing body never walks off anything — it moves
	// cell to cell and the fall edge already refuses an unsurvivable drop — so
	// this is a caller's policy about how close to an edge it wants to be
	// steered, not a fact about the game. A follower that overshoots a cell
	// centre has a reason to want it; a server-side mob does not.
	AvoidLedges bool
	// SneakTicks is the cost of crossing one block crouched, and enabling it
	// is what lets a body cross a ledge while AvoidLedges is on. Zero means
	// the body cannot sneak, and with AvoidLedges on it routes around every
	// ledge instead.
	//
	// Sneaking is a posture rather than a flag on this value because it is a
	// per-position decision: a body sneaks at the one cell that needs it and
	// stands everywhere else, and a flag would make it sneak for a whole route
	// or none of it. The posture is also what a follower reads to know when to
	// hold the key down.
	//
	// It buys no headroom, deliberately. Both supported versions shorten the
	// body when it crouches — 1.8 to 1.5 in 26.1.2 — and on a block grid that
	// changes nothing at all, because both heights need the same two cells.
	// The posture that fits where standing does not is CrawlHeight below, and
	// only one version has it.
	SneakTicks float64
	// CrawlHeight is how tall the body is while crawling, in blocks. Zero
	// produces no crawl edges.
	//
	// This is the posture that reaches what the others cannot: a body under a
	// block is about half a block tall, so it passes through a one-block gap
	// that stops a standing and a crouched body alike. It is one field rather
	// than a flag beside a body for the reason JumpReach is one field — two
	// values that can disagree about whether a body crawls is a state the
	// search has no use for.
	//
	// Only the height changes. The footprint and the step height are the
	// body's, and a version that changed those would be changing what the body
	// is rather than what it is doing.
	//
	// 1.8.9 has no crawl and supplies zero. That asymmetry is asserted rather
	// than tolerated; see the version gate in the tests.
	CrawlHeight float64
	// CrawlTicks is the cost of crossing one block on the body's front.
	CrawlTicks float64
	// WaterLandingDepth is how many blocks of fluid a column needs before a
	// drop past SafeFall may land in it. Zero produces no water drops.
	//
	// It is the caller's number because how deep the water must be differs by
	// version and by how far the body fell, and this package types no version
	// constant. A caller that has not measured its version supplies zero and
	// gets the search it had before, where every drop is bounded by SafeFall
	// alone.
	WaterLandingDepth float64
	// CanClimb allows climb edges up and down a ladder or a vine.
	CanClimb bool
	// ClimbTicks is the cost of moving one cell vertically on a climbable
	// block.
	ClimbTicks float64
	// CanOpenDoors allows door edges. A body with no hands, or one a caller
	// does not want opening things, leaves it off.
	CanOpenDoors bool
	// DoorTicks is the cost of opening a door and stepping into its cell.
	DoorTicks float64
	// CanPlace allows the placement edges. A mob is this value with it off,
	// which is how one search serves a body with no inventory.
	CanPlace bool
	// PlaceTicks is the cost of putting one block down, in ticks.
	PlaceTicks float64
	// BlockBudget is how many placeable blocks the body carries. A path may not
	// contain more placements than this.
	BlockBudget int
	// BlockTicks is what one placed block is worth, in ticks.
	//
	// Every cost here is in ticks so that "bridge the gap" and "walk around it"
	// are compared in one unit rather than through a weighting nobody can
	// justify. A body holding two blocks routes differently from one holding a
	// stack, and this is the number that makes it so.
	BlockTicks float64
	// PlacedBlock is the handle the body puts down.
	//
	// It is carried because the overlay has to answer BlockState for a cell the
	// body filled, and a handle means nothing outside the profile that minted
	// it. The shape is assumed to be a full cube: a body bridging a gap carries
	// something to stand on, and which placeables are legal where is M9.5's
	// question rather than this one's.
	PlacedBlock world.BlockRef
	// VerticalEnvelope is how far above or below the start a search may expand,
	// in blocks. Zero means no bound.
	//
	// It exists because placing makes every Y coordinate reachable from every
	// position, and a search that can climb forever will spend its whole node
	// budget doing so to reach a horizontal detour it should have walked. A
	// body that wants the surface states the surface as its goal and sizes the
	// envelope around it.
	VerticalEnvelope int32
	// MaxPillarHeight is how many pillar edges may stack in one column. Zero
	// means no bound.
	//
	// It is the other half of the same problem: without it a search builds a
	// tower to reach something it could have walked to.
	MaxPillarHeight int
	// CandidateLimit bounds one terrain query's collision sweep. A
	// non-positive value means no limit.
	CandidateLimit int
	// Breaker answers how long this body takes to break a block, and whether
	// it can at all. A nil breaker is a body with no tool: it produces no dig
	// edge and the search is exactly what it was.
	//
	// There is no CanDig flag beside it, deliberately. A nil breaker cannot
	// answer the question, and a non-nil one is a caller saying it holds
	// something to dig with; a second knob would only let the two disagree.
	Breaker Breaker
	// DigBudget bounds how many cells one route may break. A non-positive
	// value means no bound.
	//
	// It is not BlockBudget's twin, and the difference is the point: a placed
	// block is consumed from a finite stack, so running out is a state the
	// search has to model, while a pickaxe does not run out part-way along a
	// route in any way the search can see. This budget exists because a caller
	// that does not want a tunnel driven through a mountain needs a way to say
	// so, and cost alone does not say it.
	DigBudget int
	// HazardPenalty is added to the cost of any edge arriving in a cell with
	// a hazardous horizontal neighbour, in ticks. Zero means hazards beside
	// the route cost nothing, which is exactly the search before this field
	// existed.
	//
	// It is a penalty rather than a refusal: the cell itself is legal —
	// arrivalAt already refuses a hazardous one outright — but a body walking
	// the rim of a lava lake pays for every step it spends there, so a route
	// one lane over wins whenever one exists. A body with no route but the rim
	// still takes it.
	HazardPenalty float64
}

// perBlockFloor returns the lowest cost this capability can pay for one block
// of Manhattan distance closed.
//
// It is deliberately not the cheapest edge. A step closes two blocks — one
// across, one up — for one step's cost, and a fall of depth D closes 1+D blocks
// for FallTicks*D, which is cheapest per block at D=1. Scaling distance by the
// cheapest edge instead overestimates on both, and an overestimating heuristic
// lets the search settle a goal on a route that is not shortest.
//
// Every edge the capability can produce has to enter this, which is why adding
// one means coming back here. A jump of distance D costs JumpTicks*D and closes
// D blocks, so its rate is JumpTicks flat, and a capability whose jump is
// cheaper per block than its walk would otherwise be searched with a heuristic
// that overestimates every route containing one.
func (c Capability) perBlockFloor() float64 {
	lowest := c.WalkTicks
	for _, cost := range []float64{c.StepTicks / 2, c.FallTicks / 2} {
		if cost < lowest {
			lowest = cost
		}
	}
	if c.CanSwim && c.SwimTicks < lowest {
		lowest = c.SwimTicks
	}
	if c.JumpReach > 0 && c.JumpTicks < lowest {
		lowest = c.JumpTicks
	}
	if c.SneakTicks > 0 && c.SneakTicks < lowest {
		lowest = c.SneakTicks
	}
	if c.CrawlHeight > 0 && c.CrawlTicks < lowest {
		lowest = c.CrawlTicks
	}
	if c.CanClimb && c.ClimbTicks < lowest {
		lowest = c.ClimbTicks
	}
	if c.CanOpenDoors && c.DoorTicks < lowest {
		lowest = c.DoorTicks
	}
	// A placement closes one block for a placement's price plus what the block
	// is worth, and a pillar closes one block of height for the same. Both
	// enter the floor, and this is the reason the floor is recomputed rather
	// than left alone whenever an edge is added: the moment placing is cheaper
	// per block than walking, a floor that ignored it would overestimate, and
	// the symptom is a route that is merely suboptimal — the hardest kind of
	// wrongness to notice and to trace back here.
	if c.CanPlace {
		if placed := c.PlaceTicks + c.BlockTicks; placed < lowest {
			lowest = placed
		}
	}
	// A water drop of depth D closes 1+D blocks for FallTicks*D, which is the
	// same rate the ordinary fall is priced at and is cheapest per block at
	// D=1. It adds no new floor, and it is named here so the next person to
	// add an edge does not have to work that out again.

	return lowest
}

// Postures returns the postures this body has, in their declared order.
//
// It is derived from the fields rather than declared beside them. A capability
// that carried both a crawl height and a list saying whether it crawls has two
// answers to one question, and the day they disagree the search believes one
// and the gate believes the other.
//
// It exists because the asymmetry between the two supported versions is a
// property of the value rather than a branch in the search: a 26.1.2 capability
// has a crawl and a 1.8.9 one does not, and a gate can assert that by asking
// the value instead of by knowing which version it came from.
func (c Capability) Postures() []Posture {
	postures := []Posture{PostureStand}
	if c.CanSwim {
		postures = append(postures, PostureSwim)
	}
	if c.SneakTicks > 0 {
		postures = append(postures, PostureSneak)
	}
	if c.CrawlHeight > 0 && c.CrawlHeight < c.Body.Height {
		postures = append(postures, PostureCrawl)
	}

	return postures
}

// crawling returns the capability with its prone body, for the queries that ask
// what fits while the body is down.
//
// It returns a copy rather than mutating, because the search holds one
// capability for a whole route and a body that stayed prone after one cell
// would crawl the rest of the way.
func (c Capability) crawling() Capability {
	c.Body.Height = c.CrawlHeight

	return c
}

// query returns the terrain query this capability asks with.
func (c Capability) query(view terrainView, facts terrain.Facts) terrain.Query {
	return terrain.Query{View: view, Facts: facts, Body: c.Body, Limit: c.CandidateLimit}
}
