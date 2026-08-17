package sim

import (
	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// Scope names what one tick simulates.
//
// It is a struct rather than a bare slice so that later milestones can add
// dimensions and regions without changing every signature that carries it. The
// entity order is the order the kernel walks them in, so it is part of what a
// digest covers.
type Scope struct {
	// Entities are the bodies this tick simulates, in order. An empty slice
	// means the tick simulates no body, not every body: a caller that wants a
	// body stepped has to name it.
	Entities []entity.ID
}

// TickInput is everything one tick may read.
//
// The views must stay valid and unchanged for the whole of a Step call: the same
// position and the same identifier answer the same way from the first phase to
// the last. The kernel reads nothing else. There is no clock, no global random
// state, and no mutable application object behind this struct, which is what
// makes a tick reproducible from its input alone.
type TickInput struct {
	// Profile supplies the rules. It must be the profile the kernel was built
	// from.
	Profile Profile
	// Revision is the store revision the tick is computed against, and the base
	// revision the resulting change set carries.
	Revision Revision
	// Tick is the tick number being simulated.
	Tick Tick
	// Blocks is the world the tick reads.
	Blocks world.View
	// Entities are the bodies the tick reads.
	Entities entity.View
	// Scope names what the tick simulates.
	Scope Scope
	// Commands are the intents this tick was asked to apply, in order.
	Commands []Command
	// Random is the random state the tick draws from.
	Random RandomState
	// Limits bounds the work the tick may do. A zero field means the default.
	Limits Limits
}

// TickResult is the record of one tick.
//
// An incomplete result carries no operations and no events: applying one is
// impossible rather than merely discouraged. It still carries a digest, because
// an incomplete tick is a fact a replay may want to record.
type TickResult struct {
	// Revision is the revision the tick was computed against.
	Revision Revision
	// Tick is the tick number that was simulated.
	Tick Tick
	// Changes are the state changes the tick produced, in tick order.
	Changes ChangeSet
	// Domain are the simulation facts the tick reported.
	Domain []DomainEvent
	// Presentation are the particles, sounds, and animations the tick asked
	// for. A consumer may ignore every one and still hold correct state.
	Presentation []PresentationEvent
	// Outcomes say what the tick did with each command it was given.
	Outcomes []CommandOutcome
	// Random is the random state the tick left behind.
	Random RandomState
	// Read is every dependency the tick consulted, sorted and deduplicated. It
	// is part of the digest: a rule that starts consulting different cells has
	// changed behaviour even when its output happens to match.
	Read []Dependency
	// Completeness reports whether the tick had everything it needed.
	Completeness Completeness
	// Digest is the canonical hash of every other field, under the profile
	// identity the tick ran with.
	Digest Digest
}
