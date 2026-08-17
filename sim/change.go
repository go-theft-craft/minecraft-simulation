package sim

import (
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// OpKind says what an operation does.
type OpKind uint8

const (
	// OpSetEntity writes a body, creating it if the store has none.
	OpSetEntity OpKind = iota + 1
	// OpRemoveEntity drops a body.
	OpRemoveEntity
	// OpSetBlock writes a block state.
	OpSetBlock
)

// String returns the kind's name.
func (o OpKind) String() string {
	switch o {
	case OpSetEntity:
		return "set-entity"
	case OpRemoveEntity:
		return "remove-entity"
	case OpSetBlock:
		return "set-block"
	default:
		return fmt.Sprintf("OpKind(%d)", uint8(o))
	}
}

// Op is one state change.
//
// The block field carries a handle rather than a shape. A geom.Shape keeps its
// boxes unexported and cannot be canonically encoded, and the store can resolve
// a handle through the profile, which is where block data belongs.
//
// Fields a kind does not use are zero.
type Op struct {
	Kind   OpKind
	Entity entity.ID
	State  entity.State
	Block  geom.BlockPos
	Ref    world.BlockRef
}

// ChangeSet is every state change one tick produced.
//
// It is fully applicable or not applicable. There is no partial apply, and the
// operations are in the order the tick produced them: a later operation may
// overwrite an earlier one, so reordering them would change their meaning.
type ChangeSet struct {
	// BaseRevision is the store revision the tick was computed against. A store
	// that has moved on refuses the whole set.
	BaseRevision Revision
	// Ops are the changes, in tick order.
	Ops []Op
}

// IsEmpty reports whether the set changes nothing.
func (c ChangeSet) IsEmpty() bool {
	return len(c.Ops) == 0
}
