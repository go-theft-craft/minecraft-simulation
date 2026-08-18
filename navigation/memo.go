package navigation

import (
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/terrain"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// defaultMemoCells bounds each cache when Options does not. It is a count of
// cells, not bytes: an entry is a small value plus its dependency list, and a
// body's working set over a few chunks is well inside this.
const defaultMemoCells = 16_384

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

// clearEntry is one cached fit and the cells it was computed from.
type clearEntry struct {
	value bool
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
	crawlQuery terrain.Query
	capability Capability

	pass   map[geom.BlockPos]passEntry
	arrive map[geom.BlockPos]arriveEntry
	// crawl is the passability of a cell to the prone body, cached apart from
	// pass because a shorter body reads a shorter span and gets a different
	// answer for the same cell.
	crawl map[geom.BlockPos]passEntry
	// fits answers whether the body's box fits in a cell, which the jump arc
	// asks about cells no route ever stands in. It is keyed by cell like the
	// other two, because it depends on nothing else.
	fits map[geom.BlockPos]clearEntry
	// dependents maps a cell to the answers computed from it, so invalidation
	// drops exactly what a change affects rather than everything.
	dependents map[geom.BlockPos]*dependentSet

	// limit bounds each cache. passOrder and arriveOrder are the insertion
	// order eviction walks, oldest first. Which answer is evicted changes only
	// whether it must be recomputed, never what it is, so the policy is free to
	// be the simplest one that bounds the memory.
	limit       int
	passOrder   []geom.BlockPos
	arriveOrder []geom.BlockPos
	fitsOrder   []geom.BlockPos
	crawlOrder  []geom.BlockPos

	// misses counts recomputations, for tests and for the benchmark report.
	misses int
}

// dependentSet is the answers depending on one cell, split by kind so
// invalidation touches the right map.
type dependentSet struct {
	pass   map[geom.BlockPos]struct{}
	arrive map[geom.BlockPos]struct{}
	fits   map[geom.BlockPos]struct{}
	crawl  map[geom.BlockPos]struct{}
}

// newMemoOracle returns an empty memo over a view.
func newMemoOracle(view world.View, facts terrain.Facts, capability Capability, limit int) *memoOracle {
	recorder := &recordingView{view: view}
	if limit <= 0 {
		limit = defaultMemoCells
	}

	return &memoOracle{
		recorder:   recorder,
		query:      capability.query(recorder, facts),
		crawlQuery: capability.crawling().query(recorder, facts),
		capability: capability,
		pass:       make(map[geom.BlockPos]passEntry),
		arrive:     make(map[geom.BlockPos]arriveEntry),
		fits:       make(map[geom.BlockPos]clearEntry),
		crawl:      make(map[geom.BlockPos]passEntry),
		dependents: make(map[geom.BlockPos]*dependentSet),
		limit:      limit,
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

	deps := m.claim(cell, answerPass)
	m.pass[cell] = passEntry{value: value, deps: deps}

	m.passOrder = append(m.passOrder, cell)
	for len(m.pass) > m.limit && len(m.passOrder) > 0 {
		oldest := m.passOrder[0]
		m.passOrder = m.passOrder[1:]
		m.forgetPass(oldest)
	}

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

	deps := m.claim(cell, answerArrive)
	m.arrive[cell] = arriveEntry{value: value, deps: deps}

	m.arriveOrder = append(m.arriveOrder, cell)
	for len(m.arrive) > m.limit && len(m.arriveOrder) > 0 {
		oldest := m.arriveOrder[0]
		m.arriveOrder = m.arriveOrder[1:]
		m.forgetArrive(oldest)
	}

	return value, nil
}

// clear implements oracle.
func (m *memoOracle) clear(cell geom.BlockPos) (bool, error) {
	if entry, ok := m.fits[cell]; ok {
		return entry.value, nil
	}

	m.misses++
	m.recorder.reset()
	fit, err := m.query.Fits(terrain.FeetOf(cell))
	if err != nil {
		return false, err
	}
	value := fit == terrain.FitClear

	deps := m.claim(cell, answerFits)
	m.fits[cell] = clearEntry{value: value, deps: deps}

	m.fitsOrder = append(m.fitsOrder, cell)
	for len(m.fits) > m.limit && len(m.fitsOrder) > 0 {
		oldest := m.fitsOrder[0]
		m.fitsOrder = m.fitsOrder[1:]
		m.forgetFits(oldest)
	}

	return value, nil
}

// passableCrawling implements oracle.
func (m *memoOracle) passableCrawling(cell geom.BlockPos) (terrain.Passability, error) {
	if entry, ok := m.crawl[cell]; ok {
		return entry.value, nil
	}

	m.misses++
	m.recorder.reset()
	value, err := m.crawlQuery.Passable(cell)
	if err != nil {
		return terrain.Unknown, err
	}

	deps := m.claim(cell, answerCrawl)
	m.crawl[cell] = passEntry{value: value, deps: deps}

	m.crawlOrder = append(m.crawlOrder, cell)
	for len(m.crawl) > m.limit && len(m.crawlOrder) > 0 {
		oldest := m.crawlOrder[0]
		m.crawlOrder = m.crawlOrder[1:]
		m.forgetCrawl(oldest)
	}

	return value, nil
}

// answerKind names which cache an answer belongs to, so one dependency index
// serves all three.
type answerKind uint8

const (
	answerPass answerKind = iota
	answerArrive
	answerFits
	answerCrawl
)

// claim copies the recorder's log and files the answer under every cell it read.
func (m *memoOracle) claim(cell geom.BlockPos, kind answerKind) []geom.BlockPos {
	read := m.recorder.read()
	deps := make([]geom.BlockPos, len(read))
	copy(deps, read)

	for _, dep := range deps {
		set, ok := m.dependents[dep]
		if !ok {
			set = &dependentSet{
				pass:   make(map[geom.BlockPos]struct{}),
				arrive: make(map[geom.BlockPos]struct{}),
				fits:   make(map[geom.BlockPos]struct{}),
				crawl:  make(map[geom.BlockPos]struct{}),
			}
			m.dependents[dep] = set
		}
		switch kind {
		case answerPass:
			set.pass[cell] = struct{}{}
		case answerArrive:
			set.arrive[cell] = struct{}{}
		case answerFits:
			set.fits[cell] = struct{}{}
		case answerCrawl:
			set.crawl[cell] = struct{}{}
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
		for key := range set.fits {
			m.forgetFits(key)
		}
		for key := range set.crawl {
			m.forgetCrawl(key)
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

// forgetFits drops one cached fit and its index entries.
func (m *memoOracle) forgetFits(cell geom.BlockPos) {
	entry, ok := m.fits[cell]
	if !ok {
		return
	}
	for _, dep := range entry.deps {
		if set, ok := m.dependents[dep]; ok {
			delete(set.fits, cell)
		}
	}
	delete(m.fits, cell)
}

// forgetCrawl drops one cached prone passability and its index entries.
func (m *memoOracle) forgetCrawl(cell geom.BlockPos) {
	entry, ok := m.crawl[cell]
	if !ok {
		return
	}
	for _, dep := range entry.deps {
		if set, ok := m.dependents[dep]; ok {
			delete(set.crawl, cell)
		}
	}
	delete(m.crawl, cell)
}

// reset drops every cached answer.
func (m *memoOracle) reset() {
	clear(m.pass)
	clear(m.arrive)
	clear(m.fits)
	clear(m.crawl)
	clear(m.dependents)
	m.passOrder = m.passOrder[:0]
	m.arriveOrder = m.arriveOrder[:0]
	m.fitsOrder = m.fitsOrder[:0]
	m.crawlOrder = m.crawlOrder[:0]
}
