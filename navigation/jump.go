package navigation

import (
	"math"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/terrain"
)

// jumps returns every jump edge leaving a node.
//
// It is separate from expand's four-neighbour walk because a jump is not a
// neighbour: it reaches cells two and sometimes three or more away, and folding
// that into the step loop would make the neighbour order depend on the
// capability. The order here is the step order, then increasing distance, which
// keeps the frontier's tie-breaking total and the search deterministic.
//
// A jump starts from the ground, and only from the ground. A body in the air has
// nothing left to push off, and a body in water has nothing solid to push off at
// all — it is floating, and floating bodies do not leap. Neither posture
// produces an edge here.
//
// The swimming case was found by a route rather than by reading: a bot in the
// middle of a pool hopped from water cell to water cell, because the landing was
// legal for a swimmer and nothing asked what it had jumped from. Every cell in
// that route was individually admissible and the route as a whole was something
// no body can do.
func (c Capability) jumps(o oracle, from node) ([]Edge, error) {
	if c.JumpReach <= 0 || !c.jumpsFrom(from.Posture) {
		return nil, nil
	}

	// The reach is a distance between cell centres, and centres are one block
	// apart, so a reach of 2.7 admits a jump of two and refuses one of three.
	furthest := int32(math.Floor(c.JumpReach))
	if furthest < minJump {
		return nil, nil
	}

	edges := make([]Edge, 0, len(steps))
	for _, step := range steps {
		for distance := int32(minJump); distance <= furthest; distance++ {
			landing := geom.BlockPos{
				X: from.Pos.X + step.X*distance,
				Y: from.Pos.Y,
				Z: from.Pos.Z + step.Z*distance,
			}

			// Passable first, then arriveAt, in that order and for the same
			// reason every other builder does it: arriveAt answers about
			// hazards and fluid and says nothing about geometry, so a landing
			// taken on its word alone can be a hole, a wall, or a cell with a
			// block resting on the body's head.
			passable, err := o.passable(landing)
			if err != nil {
				return nil, err
			}
			if passable != terrain.Clear {
				continue
			}

			arr, err := o.arriveAt(landing)
			if err != nil {
				return nil, err
			}
			if !arr.ok {
				continue
			}

			admits, err := c.arcIsClear(o, from.Pos, step, distance)
			if err != nil {
				return nil, err
			}
			if !admits {
				// A blocked arc at this distance says nothing about a longer
				// one. The arc rises and falls, so a ceiling that stops a short
				// hop can sit well above where a longer one passes.
				continue
			}

			edges = append(edges, Edge{
				Kind: EdgeJumpGap, From: from.Pos, To: landing,
				Posture: arr.posture, Cost: c.JumpTicks * float64(distance),
			})
		}
	}

	return edges, nil
}

// jumpsFrom reports whether a body in this posture can begin a jump.
//
// Standing and crouching both push off the ground; a crouched body jumps in the
// game and the search prices it the same. Falling and swimming cannot, and
// crawling is a body under a block with nowhere to go but along.
func (c Capability) jumpsFrom(posture Posture) bool {
	switch posture {
	case PostureStand, PostureSneak:
		return true
	case PostureFall, PostureSwim, PostureCrawl:
		return false
	}

	return false
}

// minJump is the shortest jump the search produces. A jump of one lands in the
// adjacent cell, which is a walk, and offering both would give the search two
// edges to the same node differing only in price.
const minJump = 2

// arcIsClear reports whether the body passes over every cell between take-off
// and landing without meeting anything.
//
// It asks about the cells the body's feet pass through over each intervening
// column, from the take-off level up to the arc's height there. Checking the
// endpoints alone is the failure this exists to stop: the landing can be
// perfectly clear while a block one above the middle of the gap is exactly what
// the body's head hits.
//
// It re-derives no collision. Every answer comes from the oracle's fit test,
// which is the same terrain query every other edge is built from.
func (c Capability) arcIsClear(o oracle, from, step geom.BlockPos, distance int32) (bool, error) {
	for i := int32(1); i < distance; i++ {
		column := geom.BlockPos{
			X: from.X + step.X*i,
			Y: from.Y,
			Z: from.Z + step.Z*i,
		}

		// The feet rise from the take-off level to the arc's height here, so
		// every whole cell in between is one the body occupies on the way.
		top := int32(math.Floor(c.arcRise(i, distance)))
		for level := int32(0); level <= top; level++ {
			cell := geom.BlockPos{X: column.X, Y: column.Y + level, Z: column.Z}

			admits, err := o.clear(cell)
			if err != nil {
				return false, err
			}
			if !admits {
				return false, nil
			}
		}
	}

	return true, nil
}

// arcRise returns how far above the take-off the body's feet are over the
// column at horizontal offset i of a jump covering distance.
//
// The arc is modelled as the parabola through the take-off, the measured peak,
// and the landing. That is an approximation of what the kernel does — the real
// trajectory is drag applied per tick and is not symmetric — and it is the
// approximation this check wants: it is exact at the two endpoints, highest in
// the middle where the real arc is highest, and it needs only the peak, which
// navigation/reach measures.
//
// The alternative is replaying the kernel per candidate, which would put a
// movement simulation inside every node expansion of an A*. What that would buy
// is a more exact answer about a clearance the search then rounds to whole
// cells anyway.
func (c Capability) arcRise(i, distance int32) float64 {
	if distance <= 0 {
		return 0
	}
	t := float64(i) / float64(distance)

	return 4 * c.JumpRise * t * (1 - t)
}
