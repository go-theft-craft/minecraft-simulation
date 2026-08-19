package navigation

import (
	"context"
	"errors"
	"math"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/terrain"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// ErrNoBody reports a capability whose body has no volume. A zero body fits
// everywhere, which would return routes through solid stone.
var ErrNoBody = errors.New("navigation: a capability needs a body with a width and a height")

// ErrNoGoal reports a nil goal. There is no default destination.
var ErrNoGoal = errors.New("navigation: a search needs a goal")

// Budget bounds one search.
//
// Both bounds exist because they stop different runaways: Nodes stops a search
// over a large open world, and Ceiling stops one that would return a route far
// more expensive than the caller would ever walk.
type Budget struct {
	// Nodes is how many nodes may be expanded. A non-positive value means the
	// search expands until the frontier empties.
	Nodes int
	// Ceiling is the highest total cost a route may have, in ticks. A
	// non-positive value means no ceiling.
	Ceiling float64
}

// steps are the four horizontal neighbours, in a fixed order. Diagonals are
// absent: they need a corner-cutting rule, and a wrong one walks a body
// through the gap between two blocks.
var steps = [4]geom.BlockPos{
	{X: 1}, {X: -1}, {Z: 1}, {Z: -1},
}

// maxFallSearch is the floor on how far below a neighbour the column walk looks
// for a landing. The walk runs to max(maxFallSearch, ceil(SafeFall)), so a
// capability whose safe fall is deeper than this floor can still reach a legal
// landing below it; the per-drop SafeFall check is what refuses a drop the body
// could not survive. The bound is finite so a malformed capability cannot walk
// the world.
const maxFallSearch = 32

// Find searches a route from one cell to another.
//
// It returns a Path rather than an error when it cannot reach the goal: an
// incomplete path with Reason set is more useful to a moving body than a
// refusal. An error means the search could not run at all.
func Find(
	ctx context.Context,
	view terrainView,
	facts terrain.Facts,
	capability Capability,
	from geom.BlockPos,
	goal Goal,
	budget Budget,
) (Path, error) {
	if capability.Body.HalfWidth <= 0 || capability.Body.Height <= 0 {
		return Path{}, ErrNoBody
	}
	if goal == nil {
		return Path{}, ErrNoGoal
	}

	// Declared as the interface, not as directOracle, so the conversion happens
	// once. The value is wider than a word, and converting it at every expand
	// call cost one heap allocation per node expanded — 3,000 of them on
	// BenchmarkFindLong.
	var o oracle = directOracle{
		query:      capability.query(view, facts),
		capability: capability,
		crawlQuery: capability.crawling().query(view, facts),
	}

	return plan(ctx, o, capability, view, facts, from, goal, budget)
}

// plan runs the search and, for a body that can change the world, the
// re-run-and-ban loop that keeps the answer self-consistent.
//
// Find and Planner.Plan both go through here, which is what makes the planner's
// documented promise true: the cache changes how long a route takes to find,
// never what it says.
func plan(
	ctx context.Context,
	o oracle,
	capability Capability,
	view terrainView,
	facts terrain.Facts,
	from geom.BlockPos,
	goal Goal,
	budget Budget,
) (Path, error) {
	// A body that cannot change the world produces no mutating edge, so its
	// path cannot be self-inconsistent and there is nothing to validate. That
	// is the read-only search, unchanged and un-slowed by any of this.
	if !capability.mutates() {
		return search(ctx, o, capability, from, goal, budget, nil)
	}

	// The re-run-and-ban loop the parent design specifies. The search expands
	// nodes with no notion of which placements a route has already made, so a
	// winning path can put a block in a cell one of its own later edges has to
	// pass through — a conflict between two branches that were never compared.
	// Validating the winner and banning the offending edge is what resolves it,
	// and each round bans one edge, so it makes progress.
	banned := make(map[Edge]struct{})
	var last Path
	for range maxValidationRounds {
		path, err := search(ctx, o, capability, from, goal, budget, banned)
		if err != nil {
			return Path{}, err
		}
		last = path

		offender, conflicted, err := capability.validate(view, facts, path)
		if err != nil {
			return Path{}, err
		}
		if !conflicted {
			return path, nil
		}
		banned[offender] = struct{}{}
	}

	// The rounds ran out. Returning the last route beats refusing, for the same
	// reason an exhausted search returns a partial path: a body that walks most
	// of the way and searches again is better off than one that stands still.
	return last, nil
}

// search is the A* both Find and Planner.Plan run. It is separate from Find so
// that a planner can supply a caching oracle without Find changing shape.
func search(
	ctx context.Context,
	o oracle,
	capability Capability,
	from geom.BlockPos,
	goal Goal,
	budget Budget,
	banned map[Edge]struct{},
) (Path, error) {
	start := node{Pos: from, Posture: PostureStand}

	cameFrom := make(map[node]link)
	cost := map[node]float64{start: 0}

	var open frontier
	open.push(start, capability.toward(goal, from))

	best, bestScore := start, capability.toward(goal, from)
	reason := ReasonUnreachable
	expanded := 0

	for {
		if err := ctx.Err(); err != nil {
			return Path{}, err
		}

		current, ok := open.pop()
		if !ok {
			break
		}
		if goal.Reached(current.Pos) {
			best, reason = current, ReasonFound

			break
		}
		if budget.Nodes > 0 && expanded >= budget.Nodes {
			reason = ReasonBudget

			break
		}
		expanded++

		// The closest node seen is the fallback a partial path is built from,
		// so an exhausted search still returns progress toward the goal.
		if score := capability.toward(goal, current.Pos); score < bestScore {
			best, bestScore = current, score
		}

		moves, err := capability.expand(o, current)
		if err != nil {
			return Path{}, err
		}

		for _, move := range moves {
			if _, refused := banned[move]; refused {
				// Banned by an earlier validation round. Skipping it here is
				// what makes the next search find a route that does not repeat
				// the conflict.
				continue
			}
			if !capability.withinEnvelope(from, move.To) {
				continue
			}
			next := node{Pos: move.To, Posture: move.Posture}
			through := cost[current] + move.Cost
			if budget.Ceiling > 0 && through > budget.Ceiling {
				if reason == ReasonUnreachable {
					reason = ReasonCeiling
				}

				continue
			}
			if seen, ok := cost[next]; ok && seen <= through {
				continue
			}
			cost[next] = through
			cameFrom[next] = link{edge: move, parent: current}
			open.push(next, through+capability.toward(goal, next.Pos))
		}
	}

	return assemble(cameFrom, cost, start, best, reason), nil
}

// link is how a node was reached: the edge that arrived, and the node it came
// from.
//
// The parent is stored rather than recovered from the edge, because an edge
// names cells and a node is a cell plus a posture. Two nodes share a cell when
// a body can both stand in it and swim it, and a trace-back that guessed which
// one an edge left would return a path that is right about where it goes and
// wrong about how.
type link struct {
	edge   Edge
	parent node
}

// assemble walks the parent links back from the end node and returns the path
// in travel order.
func assemble(cameFrom map[node]link, cost map[node]float64, start, end node, reason Reason) Path {
	var reversed []Edge
	for current := end; current != start; {
		step, ok := cameFrom[current]
		if !ok {
			break
		}
		reversed = append(reversed, step.edge)
		current = step.parent
	}

	edges := make([]Edge, 0, len(reversed))
	for i := len(reversed) - 1; i >= 0; i-- {
		edges = append(edges, reversed[i])
	}

	return Path{
		Edges:    edges,
		Cost:     cost[end],
		Complete: reason == ReasonFound,
		Reason:   reason,
	}
}

// toward scales a goal's bound, in blocks, into ticks by the cheapest cost
// this capability pays to close one block.
//
// The scale is per block closed rather than per edge because a step and a fall
// each close two blocks at once. It never overestimates, which is what keeps
// the search returning shortest paths.
func (c Capability) toward(goal Goal, pos geom.BlockPos) float64 {
	return goal.Heuristic(pos) * c.perBlockFloor()
}

// expand returns every edge leaving a node, in the fixed neighbour order.
func (c Capability) expand(o oracle, from node) ([]Edge, error) {
	edges := make([]Edge, 0, len(steps))

	for _, step := range steps {
		neighbour := geom.BlockPos{X: from.Pos.X + step.X, Y: from.Pos.Y, Z: from.Pos.Z + step.Z}

		passable, err := o.passable(neighbour)
		if err != nil {
			return nil, err
		}

		switch passable {
		case terrain.Clear:
			edge, ok, err := c.enter(o, from.Pos, neighbour)
			if err != nil {
				return nil, err
			}
			if ok {
				edges = append(edges, edge)
			}
		case terrain.Steppable:
			above := geom.BlockPos{X: neighbour.X, Y: neighbour.Y + 1, Z: neighbour.Z}
			arr, err := o.arriveAt(above)
			if err != nil {
				return nil, err
			}
			if arr.ok {
				edges = append(edges, Edge{
					Kind: EdgeStep, From: from.Pos, To: above,
					Posture: arr.posture, Cost: c.StepTicks,
				})
			}
		case terrain.Blocked:
			// Crawling is tried before the cell is treated as a hole. Blocked
			// covers both "the body does not fit" and "nothing holds it up",
			// and only the first of those a shorter body can answer
			// differently: a hole is a hole at any height, so a cell with no
			// floor produces no crawl edge here and falls through to the drop
			// below exactly as it did before crawling existed.
			crawl, ok, err := c.crawl(o, from.Pos, neighbour)
			if err != nil {
				return nil, err
			}
			if ok {
				edges = append(edges, crawl)

				break
			}

			fall, ok, err := c.fall(o, from.Pos, neighbour)
			if err != nil {
				return nil, err
			}
			if ok {
				edges = append(edges, fall)

				break
			}

			// A drop the safe fall refuses may still be survivable because of
			// what is at the bottom of it. This is tried only after the
			// ordinary fall declines, so a shallow drop keeps the price it has
			// always had.
			plunge, ok, err := c.waterDrop(o, from.Pos, neighbour)
			if err != nil {
				return nil, err
			}
			if ok {
				edges = append(edges, plunge)
			}
		case terrain.Unknown:
			// An undescribed neighbour is refused, never guessed: append no
			// edge and go on to the next of the four steps.
		}
	}

	// Jumps come after the four-step walk so that a capability which cannot
	// jump expands exactly the edges it did before this existed, in exactly
	// the order it did.
	jumps, err := c.jumps(o, from)
	if err != nil {
		return nil, err
	}
	edges = append(edges, jumps...)

	// Climbs are last and are vertical, so they change no horizontal ordering
	// at all. A capability that cannot climb expands exactly what it did.
	climbs, err := c.climbs(o, from)
	if err != nil {
		return nil, err
	}
	edges = append(edges, climbs...)

	doors, err := c.doors(o, from)
	if err != nil {
		return nil, err
	}
	edges = append(edges, doors...)

	// The mutating edges are last, so a body that cannot place expands exactly
	// what it did before either of them existed, in exactly the order it did.
	places, err := c.places(o, from)
	if err != nil {
		return nil, err
	}
	edges = append(edges, places...)

	pillars, err := c.pillars(o, from)
	if err != nil {
		return nil, err
	}
	edges = append(edges, pillars...)

	// Digging is last for the reason placing is: a body with no breaker
	// expands exactly what it did before this existed, in exactly the order it
	// did.
	dug, err := c.digs(o, from)
	if err != nil {
		return nil, err
	}
	edges = append(edges, dug...)

	if c.HazardPenalty > 0 {
		for i := range edges {
			near, err := hazardBeside(o, edges[i].To)
			if err != nil {
				return nil, err
			}
			if near {
				edges[i].Cost += c.HazardPenalty
			}
		}
	}

	return edges, nil
}

// hazardBeside reports whether any horizontal neighbour of a cell holds a
// hazard. Only the four flat neighbours are asked: the cell itself is already
// refused by arrival when hazardous, and the cell below is already refused as
// support when it burns.
func hazardBeside(o oracle, cell geom.BlockPos) (bool, error) {
	for _, step := range steps {
		neighbour := geom.BlockPos{X: cell.X + step.X, Y: cell.Y, Z: cell.Z + step.Z}

		hazard, err := o.hazardous(neighbour)
		if err != nil {
			return false, err
		}
		if hazard {
			return true, nil
		}
	}

	return false, nil
}

// arrival is what arriveAt decides: whether a body may come to rest in a cell,
// and if so in what posture. A refusal carries no posture. The type exists so
// there is one answer to "may this body arrive here, and how," rather than one
// per builder.
type arrival struct {
	ok      bool
	posture Posture
}

// refused is the empty arrival every rejection returns.
var refused = arrival{}

// arrivalAt is the one gate on a body coming to rest in a cell.
//
// Neither a fluid nor a fire carries a collision shape, so Passable calls all
// of them Clear on geometry alone. This is the only place Facts is consulted,
// which is what stops a body from walking, stepping, or falling into fire, into
// lava, or into water it cannot swim. Every builder that lands a body in a cell
// goes through here so that all three agree.
//
// An unknown lookup from either fact refuses the arrival: unknown is never
// guessed safe. Any hazard refuses it. Water is accepted only for a swimmer,
// and then in PostureSwim; open air is accepted in PostureStand.
func (c Capability) arrivalAt(query terrain.Query, cell geom.BlockPos) (arrival, error) {
	hazard, lookup, err := query.HazardAt(cell)
	if err != nil {
		return refused, err
	}
	if lookup == world.LookupUnknown {
		return refused, nil
	}
	if hazard != terrain.HazardNone {
		return refused, nil
	}

	fluid, lookup, err := query.FluidAt(cell)
	if err != nil {
		return refused, err
	}
	if lookup == world.LookupUnknown {
		return refused, nil
	}

	switch fluid {
	case terrain.FluidNone:
		return arrival{ok: true, posture: PostureStand}, nil
	case terrain.FluidWater:
		if !c.CanSwim {
			return refused, nil
		}

		return arrival{ok: true, posture: PostureSwim}, nil
	case terrain.FluidLava:
		// Lava is refused, and so is any fluid a later version adds: fluid
		// traversal beyond water is its own work, and a body that swam through
		// lava because nothing said not to is worse than one that took the long
		// way. This arm names lava for the exhaustive linter; a Go case exits
		// the switch rather than falling through, so it lands on the refusal
		// below, which is also where an unnamed future fluid arrives.
	}

	return refused, nil
}

// enter decides how a body crosses into an adjacent cell it geometrically fits
// in: a walk on the level, or a swim through water.
func (c Capability) enter(o oracle, from, to geom.BlockPos) (Edge, bool, error) {
	arr, err := o.arriveAt(to)
	if err != nil {
		return Edge{}, false, err
	}
	if !arr.ok {
		return Edge{}, false, nil
	}

	switch arr.posture {
	case PostureStand:
		if c.AvoidLedges {
			onEdge, err := c.ledge(o, to)
			if err != nil {
				return Edge{}, false, err
			}
			if onEdge {
				if c.SneakTicks <= 0 {
					// The caller asked not to be steered along edges and gave
					// the body no way to cross one carefully, so the cell is
					// refused and the search finds a way round.
					return Edge{}, false, nil
				}

				return Edge{
					Kind: EdgeWalk, From: from, To: to,
					Posture: PostureSneak, Cost: c.SneakTicks,
				}, true, nil
			}
		}

		return Edge{
			Kind: EdgeWalk, From: from, To: to,
			Posture: PostureStand, Cost: c.WalkTicks,
		}, true, nil
	case PostureSwim:
		return Edge{
			Kind: EdgeSwim, From: from, To: to,
			Posture: PostureSwim, Cost: c.SwimTicks,
		}, true, nil
	case PostureFall, PostureSneak, PostureCrawl:
		// arrivalAt returns neither: it answers about hazards and fluid, and a
		// body that has come to rest in a cell is standing or swimming. Which
		// of those postures a crouched body is in is decided by sneak, which
		// is the only thing that knows the body was too tall. The arm is here
		// because a posture nobody handles is a posture that gets a silent
		// default the day something starts producing it.
	}

	return Edge{}, false, nil
}

// crawl decides whether a cell the standing body cannot enter admits the prone
// one.
//
// It exists because a body under a block is about half a block tall and passes
// through a one-block gap that stops a standing body — and, on a grid, stops a
// crouched one too, since 1.8 and 1.5 both need two cells. A capability with no
// crawl height produces nothing here, which is what keeps the shipped four
// edges byte for byte what they were, and what keeps 1.8.9 out of a posture it
// does not have.
func (c Capability) crawl(o oracle, from, to geom.BlockPos) (Edge, bool, error) {
	if c.CrawlHeight <= 0 || c.CrawlHeight >= c.Body.Height {
		return Edge{}, false, nil
	}

	passable, err := o.passableCrawling(to)
	if err != nil {
		return Edge{}, false, err
	}
	if passable != terrain.Clear {
		return Edge{}, false, nil
	}

	// The hazard and fluid gate is the standing body's, because neither
	// question depends on how tall the body is. A prone body drowns and burns
	// exactly as fast.
	arr, err := o.arriveAt(to)
	if err != nil {
		return Edge{}, false, err
	}
	if !arr.ok || arr.posture != PostureStand {
		// A cell that would be entered swimming is not a crawl: the body is in
		// fluid, and which of the two it is doing there is a question this edge
		// has no answer for.
		return Edge{}, false, nil
	}

	return Edge{
		Kind: EdgeWalk, From: from, To: to,
		Posture: PostureCrawl, Cost: c.CrawlTicks,
	}, true, nil
}

// ledge reports whether a cell sits beside a drop the body could not survive.
//
// It is the four horizontal neighbours, and it asks the same question the fall
// edge asks: a neighbour is a drop when nothing holds a body up there and no
// landing is within reach. A cell beside a wall is not a ledge, and neither is
// one beside a step down the body can take.
func (c Capability) ledge(o oracle, cell geom.BlockPos) (bool, error) {
	for _, step := range steps {
		neighbour := geom.BlockPos{X: cell.X + step.X, Y: cell.Y, Z: cell.Z + step.Z}

		passable, err := o.passable(neighbour)
		if err != nil {
			return false, err
		}
		if passable != terrain.Blocked {
			continue
		}

		// Blocked is a wall or a hole, and only a hole is a ledge. The two are
		// told apart by whether the body fits there at all: it fits in a hole
		// and nothing holds it up, and it does not fit in a wall.
		admits, err := o.clear(neighbour)
		if err != nil {
			return false, err
		}
		if !admits {
			continue
		}

		// A drop the body survives is a route rather than a hazard, and the
		// fall builder is what already knows which is which.
		if _, ok, err := c.fall(o, cell, neighbour); err != nil {
			return false, err
		} else if ok {
			continue
		}

		return true, nil
	}

	return false, nil
}

// fall looks down a neighbouring column for a landing within the safe fall.

// fall looks down a neighbouring column for a landing within the safe fall.
//
// It runs only where Passable said Blocked, which is the answer a hole gives:
// nothing holds the body up there. A wall gives the same answer and finds no
// landing, which is why the column walk stops at the first cell the body does
// not fit through.
func (c Capability) fall(o oracle, from, neighbour geom.BlockPos) (Edge, bool, error) {
	bound := int32(math.Ceil(c.SafeFall))
	if bound < maxFallSearch {
		bound = maxFallSearch
	}

	for drop := int32(1); drop <= bound; drop++ {
		if float64(drop) > c.SafeFall {
			return Edge{}, false, nil
		}

		landing := geom.BlockPos{X: neighbour.X, Y: neighbour.Y - drop, Z: neighbour.Z}

		passable, err := o.passable(landing)
		if err != nil {
			return Edge{}, false, err
		}
		switch passable {
		case terrain.Clear:
			// The first Clear cell is where the body stops, because Clear means
			// solid ground holds it there. If it is fire, lava, or unswimmable
			// water, the fall is refused rather than deferred: nothing below
			// this cell is reachable, so there is no safer landing to descend
			// to.
			arr, err := o.arriveAt(landing)
			if err != nil {
				return Edge{}, false, err
			}
			if !arr.ok {
				return Edge{}, false, nil
			}

			return Edge{
				Kind: EdgeFall, From: from, To: landing,
				Posture: arr.posture, Cost: c.FallTicks * float64(drop),
			}, true, nil
		case terrain.Unknown, terrain.Steppable:
			return Edge{}, false, nil
		case terrain.Blocked:
			// A solid cell partway down a shaft is not the landing and is not a
			// reason to abandon the column: append no edge and keep descending
			// to the next drop.
		}
	}

	return Edge{}, false, nil
}
