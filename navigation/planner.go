package navigation

import (
	"context"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/terrain"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// Options configures a Planner. A zero Options takes every default.
type Options struct {
	// MemoCells bounds how many cached answers of each kind the planner keeps.
	// A non-positive value takes the default.
	MemoCells int
}

// Planner is one body's memory of one world.
//
// Find searches fresh every time, which is correct and is the wrong shape for a
// body that routes repeatedly through terrain that mostly did not change. A
// Planner caches what terrain said and drops exactly the answers a reported
// change invalidates.
//
// A Planner is NOT safe for concurrent use, and it is bound to one Capability:
// its cached answers are keyed by cell, which is sound only while the body
// asking stays the same. One body owns one planner — which is also what leaves
// across-body parallelism free, since Find and Plan share no mutable state
// between planners.
//
// The caller is responsible for reporting world changes through Observe. A
// planner that is never told cannot know, and will happily return a route
// through a wall built after its last look.
type Planner struct {
	capability Capability
	memo       *memoOracle
	view       world.View
	facts      terrain.Facts
}

// NewPlanner returns a planner for one body over one view.
func NewPlanner(view world.View, facts terrain.Facts, capability Capability, options Options) (*Planner, error) {
	if capability.Body.HalfWidth <= 0 || capability.Body.Height <= 0 {
		return nil, ErrNoBody
	}

	return &Planner{
		capability: capability,
		memo:       newMemoOracle(view, facts, capability, options.MemoCells),
		// The base view and its facts are kept because a body that can place
		// blocks needs its winning route validated against the world as it
		// stands, and that validation reads the world directly rather than
		// through the cache: it is asking about a world with pending
		// placements in it, which is not the world the cache answers for.
		view:  view,
		facts: facts,
	}, nil
}

// Plan routes from one cell to another.
//
// It returns what Find returns for the same inputs. The cache changes how long
// that takes, never what it says.
func (p *Planner) Plan(ctx context.Context, from, goal geom.BlockPos, budget Budget) (Path, error) {
	return plan(ctx, p.memo, p.capability, p.view, p.facts, from, goal, budget)
}

// Observe reports cells whose block state changed, dropping every cached answer
// computed from any of them.
//
// A caller knows exactly which cells moved: a client receives block-change
// packets and a server owns its own edits. Nothing here scans for changes,
// because scanning would cost more than the cache saves.
func (p *Planner) Observe(cells []geom.BlockPos) {
	p.memo.invalidate(cells)
}

// Reset drops every cached answer, for a caller whose world changed wholesale
// — and for one whose answers depended on something the recording view never
// saw, which Observe cannot reach. See memoOracle's doc comment.
func (p *Planner) Reset() {
	p.memo.reset()
}
