// Package runtime holds the state a simulation runs against: a store that
// applies change sets under a revision check, and a runner that drives one tick
// after another.
//
// The store is where a consumer's authority lives. A server applies what its
// kernel decided; a client applies a prediction to a fork and throws the fork
// away when the server disagrees. The revision check is what makes the second
// safe: a change set computed against a fork can never reach a store that has
// moved on.
package runtime

import (
	"errors"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/movement"
	"github.com/go-theft-craft/minecraft-simulation/sim"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// ErrStaleRevision reports that a change set was computed against state the
// store no longer holds. It is not a failure of the tick that produced the set:
// a client predicting locally is expected to see this, discard its fork, and
// replay from the authoritative snapshot.
var ErrStaleRevision = errors.New("runtime: change set was computed against another revision")

// ErrUnknownBlock reports that a change set names a block handle the profile
// cannot resolve. The whole set is refused, because a partly applied set would
// leave a store nobody can reason about.
var ErrUnknownBlock = errors.New("runtime: change set names a block the profile cannot resolve")

// Store is the state one kernel runs against.
//
// Apply is all or nothing. A store that has advanced past a change set's base
// revision refuses it whole, and a store that cannot resolve one of its handles
// refuses it whole as well.
type Store interface {
	// Revision returns the number of change sets this store has applied.
	Revision() sim.Revision
	// Blocks returns the block view a tick reads.
	Blocks() world.View
	// Entities returns the entity view a tick reads.
	Entities() entity.View
	// Locomotion returns the movement state a tick reads.
	Locomotion() movement.LocomotionView
	// Apply writes a change set, or reports why it wrote nothing.
	Apply(changes sim.ChangeSet) error
}
