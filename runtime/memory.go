package runtime

import (
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/sim"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// Memory is the reference Store, held entirely in memory.
//
// It is the reference implementation and not a fast one: Snapshot copies every
// map it holds. A copy-on-write store belongs to whichever consumer measures a
// problem with that, and building one now would bake in a structure nobody has
// evidence for.
//
// Memory is not safe for concurrent use. One goroutine drives one store.
type Memory struct {
	profile  sim.Profile
	revision sim.Revision
	blocks   *world.Blocks
	bodies   *entity.Bodies
}

// NewMemory returns an empty store at revision zero.
//
// The profile is what resolves the block handles a change set carries, which is
// why a store is built from one rather than being handed handles it cannot
// interpret.
func NewMemory(profile sim.Profile) *Memory {
	return &Memory{
		profile: profile,
		blocks:  world.NewBlocks(),
		bodies:  entity.NewBodies(),
	}
}

// Revision implements Store.
func (m *Memory) Revision() sim.Revision { return m.revision }

// Blocks implements Store.
func (m *Memory) Blocks() world.View { return m.blocks }

// Entities implements Store.
func (m *Memory) Entities() entity.View { return m.bodies }

// SetBlock records a block state directly, without a change set and without
// advancing the revision. It is how a consumer loads observed or generated world
// state, which is not something a tick decided.
func (m *Memory) SetBlock(pos geom.BlockPos, ref world.BlockRef) error {
	shape, ok := m.profile.Shape(ref)
	if !ok {
		return fmt.Errorf("%w: block %d at (%d,%d,%d)", ErrUnknownBlock, ref, pos.X, pos.Y, pos.Z)
	}
	m.blocks.SetBlock(pos, ref, shape)

	return nil
}

// SetEntity records a body directly, without a change set and without advancing
// the revision. It is how a consumer loads an entity a server told it about.
func (m *Memory) SetEntity(id entity.ID, state entity.State) {
	m.bodies.Set(id, state)
}

// Apply implements Store.
//
// It checks the revision, then validates every operation, then writes. The
// validation pass is what makes "fully applicable or not applicable" true rather
// than aspirational: a set holding one unresolvable handle changes nothing at
// all, including the operations that came before it.
func (m *Memory) Apply(changes sim.ChangeSet) error {
	if changes.BaseRevision != m.revision {
		return fmt.Errorf("%w: set is based at %d, store is at %d",
			ErrStaleRevision, changes.BaseRevision, m.revision)
	}

	shapes := make(map[world.BlockRef]geom.Shape)
	for index, op := range changes.Ops {
		if op.Kind != sim.OpSetBlock {
			continue
		}
		shape, ok := m.profile.Shape(op.Ref)
		if !ok {
			return fmt.Errorf("%w: operation %d names block %d", ErrUnknownBlock, index, op.Ref)
		}
		shapes[op.Ref] = shape
	}

	for _, op := range changes.Ops {
		switch op.Kind {
		case sim.OpSetEntity:
			m.bodies.Set(op.Entity, op.State)
		case sim.OpRemoveEntity:
			m.bodies.Remove(op.Entity)
		case sim.OpSetBlock:
			m.blocks.SetBlock(op.Block, op.Ref, shapes[op.Ref])
		default:
			// Unreachable: the validation pass would have to be extended
			// alongside a new kind, and a kind nobody wrote a case for must not
			// apply silently.
			return fmt.Errorf("runtime: change set holds an unknown operation %s", op.Kind)
		}
	}
	m.revision++

	return nil
}

// Snapshot returns a store at the same revision that shares no state with this
// one.
//
// This is the fork a client predicts against. It copies every map it holds, so
// the cost is proportional to the state, and the revision it carries is what
// makes a predicted change set impossible to apply to the authoritative store by
// accident.
func (m *Memory) Snapshot() *Memory {
	return &Memory{
		profile:  m.profile,
		revision: m.revision,
		blocks:   m.blocks.Clone(),
		bodies:   m.bodies.Clone(),
	}
}
