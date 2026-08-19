package navigation

import (
	"math"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// Goal is what a search is trying to reach.
//
// Heuristic answers in blocks, not ticks: a lower bound on how far pos is
// from satisfying the goal. The search scales blocks into ticks with the
// capability's cheapest per-block cost, which is what lets one goal serve
// every body. An overestimate makes the search return routes that are not
// shortest, so a goal that cannot bound tightly answers small rather than
// clever.
//
// Reached reports that pos satisfies the goal. The search stops at the first
// expanded node it holds for.
type Goal interface {
	Heuristic(pos geom.BlockPos) float64
	Reached(pos geom.BlockPos) bool
}

// manhattan is the block distance the search's own former heuristic used:
// admissible for a search whose steps change one coordinate at a time.
func manhattan(a, b geom.BlockPos) float64 {
	return math.Abs(float64(a.X-b.X)) +
		math.Abs(float64(a.Y-b.Y)) +
		math.Abs(float64(a.Z-b.Z))
}

// euclidean never exceeds manhattan, so a goal built on it stays admissible.
func euclidean(a, b geom.BlockPos) float64 {
	dx := float64(a.X - b.X)
	dy := float64(a.Y - b.Y)
	dz := float64(a.Z - b.Z)

	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// GoalBlock is the goal every search had before goals existed: one exact
// cell, reached there and nowhere else.
type GoalBlock struct {
	Pos geom.BlockPos
}

// Heuristic implements Goal.
func (g GoalBlock) Heuristic(pos geom.BlockPos) float64 { return manhattan(pos, g.Pos) }

// Reached implements Goal.
func (g GoalBlock) Reached(pos geom.BlockPos) bool { return pos == g.Pos }

// GoalXZ is a column: reached at any height. It is the long-distance travel
// goal — a caller crossing the world does not know the terrain height where
// it is going.
type GoalXZ struct {
	X, Z int32
}

// Heuristic implements Goal.
func (g GoalXZ) Heuristic(pos geom.BlockPos) float64 {
	return math.Abs(float64(pos.X-g.X)) + math.Abs(float64(pos.Z-g.Z))
}

// Reached implements Goal.
func (g GoalXZ) Reached(pos geom.BlockPos) bool { return pos.X == g.X && pos.Z == g.Z }

// GoalYLevel is a height: reached in any column. A body that wants the
// surface, or a mining level, states the height and nothing else.
type GoalYLevel struct {
	Y int32
}

// Heuristic implements Goal.
func (g GoalYLevel) Heuristic(pos geom.BlockPos) float64 {
	return math.Abs(float64(pos.Y - g.Y))
}

// Reached implements Goal.
func (g GoalYLevel) Reached(pos geom.BlockPos) bool { return pos.Y == g.Y }

// GoalNear is a sphere around a cell: close enough, by straight-line
// distance. It is the follower's goal — standing inside the radius is the
// point, and which cell inside does not matter.
type GoalNear struct {
	Pos    geom.BlockPos
	Radius float64
}

// Heuristic implements Goal. Straight-line distance never exceeds the block
// steps a route needs, so the bound is admissible.
func (g GoalNear) Heuristic(pos geom.BlockPos) float64 {
	return math.Max(0, euclidean(pos, g.Pos)-g.Radius)
}

// Reached implements Goal.
func (g GoalNear) Reached(pos geom.BlockPos) bool {
	return euclidean(pos, g.Pos) <= g.Radius
}

// GoalGetToBlock is a cell's six face neighbours: beside it, above it, or
// below it, never inside it. It is the goal for working a block — digging
// it, opening it, reading it — where standing in its cell is exactly wrong.
type GoalGetToBlock struct {
	Pos geom.BlockPos
}

// Heuristic implements Goal.
func (g GoalGetToBlock) Heuristic(pos geom.BlockPos) float64 {
	return math.Max(0, manhattan(pos, g.Pos)-1)
}

// Reached implements Goal.
func (g GoalGetToBlock) Reached(pos geom.BlockPos) bool {
	return manhattan(pos, g.Pos) == 1
}

// GoalRunAway is distance from every source at once: reached when the
// nearest one is at least Distance away. Flee is this goal with the threats
// as sources.
type GoalRunAway struct {
	From     []geom.BlockPos
	Distance float64
}

// Heuristic implements Goal. Each source demands the body make up what that
// source still lacks; the worst violation is a floor on the travel left.
func (g GoalRunAway) Heuristic(pos geom.BlockPos) float64 {
	var worst float64
	for _, source := range g.From {
		if need := g.Distance - euclidean(pos, source); need > worst {
			worst = need
		}
	}

	return worst
}

// Reached implements Goal. No sources means nothing to escape.
func (g GoalRunAway) Reached(pos geom.BlockPos) bool {
	for _, source := range g.From {
		if euclidean(pos, source) < g.Distance {
			return false
		}
	}

	return true
}

// GoalComposite is any of its members: reached when one is, guided by the
// nearest. A miner with six known veins states all six and takes whichever
// the terrain favours. An empty composite is never reached and guides
// nothing, so a search holding one runs to its budget.
type GoalComposite []Goal

// Heuristic implements Goal.
func (g GoalComposite) Heuristic(pos geom.BlockPos) float64 {
	if len(g) == 0 {
		return 0
	}
	best := g[0].Heuristic(pos)
	for _, member := range g[1:] {
		if h := member.Heuristic(pos); h < best {
			best = h
		}
	}

	return best
}

// Reached implements Goal.
func (g GoalComposite) Reached(pos geom.BlockPos) bool {
	for _, member := range g {
		if member.Reached(pos) {
			return true
		}
	}

	return false
}

// GoalInverted runs from its inner goal rather than toward it. It is never
// reached, so a search holding it always spends its whole node budget and
// returns the partial path that got farthest; the budget is how a caller
// says how far to flee. A negated bound is not admissible in the shortest-
// path sense, and does not need to be: there is no shortest path away.
type GoalInverted struct {
	Goal Goal
}

// Heuristic implements Goal.
func (g GoalInverted) Heuristic(pos geom.BlockPos) float64 {
	return -g.Goal.Heuristic(pos)
}

// Reached implements Goal.
func (g GoalInverted) Reached(geom.BlockPos) bool { return false }

// GoalAxis is the nearest of the four lines x=0, z=0, x=z, and x=-z, at one
// height. Each step changes one coordinate by one, so reaching x=z costs
// |x-z| changes and reaching x=-z costs |x+z|.
type GoalAxis struct {
	Y int32
}

// Heuristic implements Goal.
func (g GoalAxis) Heuristic(pos geom.BlockPos) float64 {
	x := math.Abs(float64(pos.X))
	z := math.Abs(float64(pos.Z))
	toAxis := math.Min(x, z)
	toDiagonal := math.Min(
		math.Abs(float64(pos.X-pos.Z)),
		math.Abs(float64(pos.X+pos.Z)),
	)

	return math.Min(toAxis, toDiagonal) + math.Abs(float64(pos.Y-g.Y))
}

// Reached implements Goal.
func (g GoalAxis) Reached(pos geom.BlockPos) bool {
	if pos.Y != g.Y {
		return false
	}

	return pos.X == 0 || pos.Z == 0 || pos.X == pos.Z || pos.X == -pos.Z
}
