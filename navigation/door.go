package navigation

import (
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/terrain"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// doorHeight is how many cells a door occupies. Both supported versions hang a
// door as two halves, and opening one opens both.
const doorHeight = 2

// doors returns the edges that pass through a door the body may open.
//
// It is the one edge in this package that changes the world, and it is here
// rather than in the mutating amendment on an argument the amendment itself
// makes: a door consumes nothing, is reversible, cannot fail for want of a
// resource, and cannot make an earlier edge illegal. The first three are plain.
// The fourth is the one that had to be checked rather than assumed, and
// TestAnOpenedDoorNeverInvalidatesAnEarlierEdge is where it was — an open door
// stays inside its own cell, so opening one can only ever free space.
//
// The edge arrives in the door's cell. A caller reading a path knows to work
// the door at Edge.To, which is why the toggle needs no field of its own.
func (c Capability) doors(o oracle, from node) ([]Edge, error) {
	if !c.CanOpenDoors {
		return nil, nil
	}

	edges := make([]Edge, 0, len(steps))
	for _, step := range steps {
		cell := geom.BlockPos{X: from.Pos.X + step.X, Y: from.Pos.Y, Z: from.Pos.Z + step.Z}

		door, err := o.doorAt(cell)
		if err != nil {
			return nil, err
		}
		// A locked door is refused rather than modelled. A bot that stands at
		// an iron door forever is worse than one that walks round.
		if door != terrain.DoorOperable {
			continue
		}

		// The cell has to be somewhere the body could stand once the door is
		// out of the way. Asking with the door open is the whole question: a
		// closed door and a wall look identical to a collision sweep.
		passable, err := o.passableThroughDoor(cell)
		if err != nil {
			return nil, err
		}
		if passable != terrain.Clear {
			continue
		}

		arr, err := o.arriveAt(cell)
		if err != nil {
			return nil, err
		}
		if !arr.ok {
			continue
		}

		edges = append(edges, Edge{
			Kind: EdgeDoor, From: from.Pos, To: cell,
			Posture: arr.posture, Cost: c.DoorTicks,
		})
	}

	return edges, nil
}

// openedView reports a door's cells as empty and defers everything else.
//
// It exists so the search can ask what a cell would be if the door in it were
// open, using the ordinary passability rules rather than a second set written
// for doors. Modelling the open door's own slab is unnecessary: it is three
// sixteenths of a block against one side of its cell, and a body is centred and
// six tenths wide, so the two never meet. That is also the finding that let this
// edge stay out of the mutating amendment.
type openedView struct {
	view world.View
	// door is the lower half. The upper half is the cell above it, because
	// both supported versions hang a door as two cells and open both together.
	door geom.BlockPos
}

// covers reports whether a cell is one of the door's two halves.
func (o openedView) covers(pos geom.BlockPos) bool {
	return pos.X == o.door.X && pos.Z == o.door.Z &&
		pos.Y >= o.door.Y && pos.Y < o.door.Y+doorHeight
}

// CollisionShape implements world.BlockView.
//
// An undescribed cell stays undescribed. Masking one would turn "nobody has
// said what is here" into "this is open", which is the one substitution every
// other rule in this package refuses to make.
func (o openedView) CollisionShape(pos geom.BlockPos) (geom.Shape, world.Lookup) {
	shape, lookup := o.view.CollisionShape(pos)
	if lookup == world.LookupUnknown || !o.covers(pos) {
		return shape, lookup
	}

	return geom.EmptyShape(), world.LookupAir
}

// BlockState implements world.StateView.
//
// The handle is left alone. An open door is still a door, and a caller asking
// what block is there should be told the truth; only its shape is being
// answered as if it had swung.
func (o openedView) BlockState(pos geom.BlockPos) (world.BlockRef, world.Lookup) {
	return o.view.BlockState(pos)
}
