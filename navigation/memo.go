package navigation

import (
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/terrain"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// passEntry is one cached Passable answer and the cells it was computed from.
type passEntry struct {
	value terrain.Passability
	deps  []geom.BlockPos
}

// arriveEntry is one cached arrival and the cells it was computed from.
type arriveEntry struct {
	value arrival
	deps  []geom.BlockPos
}

// memoOracle caches terrain answers for one body.
//
// Keying by cell alone is sound only because one memo serves one Capability: a
// body of a different height reads a different span and would need a different
// answer. NewPlanner takes the capability for this reason.
//
// Invalidation can only see what the recording view saw. An input that does
// not flow through that view leaves no dependency behind, so invalidate cannot
// drop the answers computed from it and they stay stale for the memo's whole
// life. Every input a future answer reads must therefore either be read
// through the recorder, or invalidate the memo wholesale through reset.
//
// Nothing here has such an input yet. Dig and place edges would: a server may
// refuse a break or a placement for reasons no block read can see, and the
// body's inventory and held tool decide what a place edge costs while changing
// under a route that is already cached. A denial that is genuinely per-cell is
// the easy case and needs nothing new — report the cell to invalidate and the
// reverse index drops exactly the right answers. A rule that covers a region,
// or a permission the body gains and loses, is not a cell and must not be
// faked as one.
//
// It is not safe for concurrent use. One body owns one memo, which is what
// leaves across-body parallelism free.
type memoOracle struct {
	recorder   *recordingView
	query      terrain.Query
	capability Capability

	pass   map[geom.BlockPos]passEntry
	arrive map[geom.BlockPos]arriveEntry
	// dependents maps a cell to the answers computed from it, so invalidation
	// drops exactly what a change affects rather than everything.
	dependents map[geom.BlockPos]*dependentSet

	// misses counts recomputations, for tests and for the benchmark report.
	misses int
}

// dependentSet is the answers depending on one cell, split by kind so
// invalidation touches the right map.
type dependentSet struct {
	pass   map[geom.BlockPos]struct{}
	arrive map[geom.BlockPos]struct{}
}

// newMemoOracle returns an empty memo over a view.
func newMemoOracle(view world.View, facts terrain.Facts, capability Capability) *memoOracle {
	recorder := &recordingView{view: view}

	return &memoOracle{
		recorder:   recorder,
		query:      capability.query(recorder, facts),
		capability: capability,
		pass:       make(map[geom.BlockPos]passEntry),
		arrive:     make(map[geom.BlockPos]arriveEntry),
		dependents: make(map[geom.BlockPos]*dependentSet),
	}
}

// passable implements oracle.
func (m *memoOracle) passable(cell geom.BlockPos) (terrain.Passability, error) {
	if entry, ok := m.pass[cell]; ok {
		return entry.value, nil
	}

	m.misses++
	m.recorder.reset()
	value, err := m.query.Passable(cell)
	if err != nil {
		return terrain.Unknown, err
	}

	deps := m.claim(cell, true)
	m.pass[cell] = passEntry{value: value, deps: deps}

	return value, nil
}

// arriveAt implements oracle.
func (m *memoOracle) arriveAt(cell geom.BlockPos) (arrival, error) {
	if entry, ok := m.arrive[cell]; ok {
		return entry.value, nil
	}

	m.misses++
	m.recorder.reset()
	value, err := m.capability.arrivalAt(m.query, cell)
	if err != nil {
		return refused, err
	}

	deps := m.claim(cell, false)
	m.arrive[cell] = arriveEntry{value: value, deps: deps}

	return value, nil
}

// claim copies the recorder's log and files the answer under every cell it read.
func (m *memoOracle) claim(cell geom.BlockPos, isPass bool) []geom.BlockPos {
	read := m.recorder.read()
	deps := make([]geom.BlockPos, len(read))
	copy(deps, read)

	for _, dep := range deps {
		set, ok := m.dependents[dep]
		if !ok {
			set = &dependentSet{
				pass:   make(map[geom.BlockPos]struct{}),
				arrive: make(map[geom.BlockPos]struct{}),
			}
			m.dependents[dep] = set
		}
		if isPass {
			set.pass[cell] = struct{}{}
		} else {
			set.arrive[cell] = struct{}{}
		}
	}

	return deps
}

// invalidate drops every answer computed from any of the given cells.
//
// Iterating the dependent sets is safe for determinism: invalidation decides
// what is recomputed, never what an answer is, so the order it visits them in
// cannot reach an output.
func (m *memoOracle) invalidate(cells []geom.BlockPos) {
	for _, cell := range cells {
		set, ok := m.dependents[cell]
		if !ok {
			continue
		}
		for key := range set.pass {
			m.forgetPass(key)
		}
		for key := range set.arrive {
			m.forgetArrive(key)
		}
		delete(m.dependents, cell)
	}
}

// forgetPass drops one cached Passable answer and its index entries.
func (m *memoOracle) forgetPass(cell geom.BlockPos) {
	entry, ok := m.pass[cell]
	if !ok {
		return
	}
	for _, dep := range entry.deps {
		if set, ok := m.dependents[dep]; ok {
			delete(set.pass, cell)
		}
	}
	delete(m.pass, cell)
}

// forgetArrive drops one cached arrival and its index entries.
func (m *memoOracle) forgetArrive(cell geom.BlockPos) {
	entry, ok := m.arrive[cell]
	if !ok {
		return
	}
	for _, dep := range entry.deps {
		if set, ok := m.dependents[dep]; ok {
			delete(set.arrive, cell)
		}
	}
	delete(m.arrive, cell)
}

// reset drops every cached answer.
func (m *memoOracle) reset() {
	clear(m.pass)
	clear(m.arrive)
	clear(m.dependents)
}
