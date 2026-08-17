package sim

import "errors"

// ErrLimitExhausted reports that a tick asked for more work than its
// deterministic budget allows. The budget exists so that a malformed input
// cannot make one tick run without bound, and so that the same input costs the
// same work on every machine.
var ErrLimitExhausted = errors.New("sim: deterministic work limit exhausted")

// Limits bounds the work one tick may do.
//
// These are the budgets this milestone can actually exhaust. The parent design
// lists more, covering scheduled events, explosions, fluids, and extensions;
// each arrives with the mechanic that can spend it, because a budget nothing
// counts against is a field that drifts out of date.
//
// A zero field means the default. A caller who leaves the struct blank wants
// sensible bounds, not a tick that refuses to do anything.
type Limits struct {
	// EntitySteps bounds how many bodies one tick may move.
	EntitySteps int
	// BlockUpdates bounds how many block writes one tick may produce.
	BlockUpdates int
	// CollisionCandidates bounds the cells one sweep may visit. It is passed
	// straight to collision.Move.
	CollisionCandidates int
	// Events bounds how many domain and presentation events one tick may emit,
	// counted together.
	Events int
}

// defaultLimits are chosen to be far above any legitimate tick and far below
// anything that could hang a server. They are not tuned; they are a ceiling.
var defaultLimits = Limits{
	EntitySteps:         4096,
	BlockUpdates:        4096,
	CollisionCandidates: 32768,
	Events:              4096,
}

// withDefaults replaces every non-positive budget with its default.
func (l Limits) withDefaults() Limits {
	if l.EntitySteps <= 0 {
		l.EntitySteps = defaultLimits.EntitySteps
	}
	if l.BlockUpdates <= 0 {
		l.BlockUpdates = defaultLimits.BlockUpdates
	}
	if l.CollisionCandidates <= 0 {
		l.CollisionCandidates = defaultLimits.CollisionCandidates
	}
	if l.Events <= 0 {
		l.Events = defaultLimits.Events
	}

	return l
}
