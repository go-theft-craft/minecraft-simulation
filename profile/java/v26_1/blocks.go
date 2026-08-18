package v26_1

import (
	"fmt"

	"github.com/go-theft-craft/minecraft-protocol/data"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// blockTable turns the dataset's block, shape, and physics tables into the
// opaque handles world.BlockRef promises.
//
// A handle is a block state's identifier plus one, so the zero handle stays
// meaningless and only the profile that minted a handle can say what it names.
// That the handle is a state rather than a block is this version's own doing:
// the flattening replaced the block-and-metadata pair with a single state
// number, and the collision shape belongs to the state — a slab's two halves and
// a stair's eighty orientations are states of one block, and they do not stand
// in the same volume.
//
// Friction is the other way round. The game holds it on the block, so every
// state of a block answers with the block's value, and the table stores it once
// per block rather than once per state.
type blockTable struct {
	// owner is the block each handle belongs to, as an index into names and
	// frictions. Index zero is the reserved handle's block, which is no block.
	owner []uint32
	// names is each block's name, in the order the dataset lists them.
	names []string
	// frictions is each block's slipperiness, at the width the friction product
	// is formed at.
	frictions []float32
	// shapes is the collision shape each handle resolves to.
	shapes []geom.Shape
	// byName resolves a block name to the handle of its default state, for
	// tests and fixtures.
	byName map[string]world.BlockRef
}

// newBlockTable builds the table from a dataset.
//
// It rejects a set with no physics, because a profile whose slipperiness table
// silently defaulted would produce a body that walks on ice like it walks on
// stone and no test would say why. It rejects a set whose state numbering
// overlaps or leaves a hole for the same reason: a state nothing claims would
// answer with an empty shape and the default friction, which is a description of
// air, and the cell it was read from is not air.
func newBlockTable(set *data.Set) (blockTable, error) {
	if set == nil {
		return blockTable{}, fmt.Errorf("%w: no data set", ErrInvalidProfile)
	}

	physics := set.Physics()
	if len(physics.BlockSlipperiness) == 0 {
		return blockTable{}, fmt.Errorf("%w: the data set carries no block slipperiness", ErrInvalidProfile)
	}

	registry := set.Blocks()
	if registry == nil {
		return blockTable{}, fmt.Errorf("%w: the data set carries no blocks", ErrInvalidProfile)
	}

	blocks := registry.All()
	if len(blocks) == 0 {
		return blockTable{}, fmt.Errorf("%w: the data set carries no blocks", ErrInvalidProfile)
	}

	shapes := set.CollisionShapes()
	// One geom.Shape per shape identifier rather than per state: this version
	// describes roughly thirty thousand states with five thousand shapes, and
	// the states that share one share it here too.
	converted := make(map[data.ShapeID]geom.Shape, len(shapes.Shapes))

	highest := data.BlockStateID(0)
	for _, block := range blocks {
		if block.MaxStateID > highest {
			highest = block.MaxStateID
		}
	}

	table := blockTable{
		owner:     make([]uint32, int(highest)+2),
		shapes:    make([]geom.Shape, int(highest)+2),
		byName:    make(map[string]world.BlockRef, len(blocks)),
		names:     []string{""},
		frictions: []float32{float32(physics.DefaultSlipperiness)},
	}
	table.shapes[0] = geom.EmptyShape()

	for _, block := range blocks {
		if block.MaxStateID < block.MinStateID {
			return blockTable{}, fmt.Errorf(
				"%w: block %q spans states %d to %d, which is no span at all",
				ErrInvalidProfile, block.Name, block.MinStateID, block.MaxStateID,
			)
		}

		index := uint32(len(table.names))
		table.names = append(table.names, block.Name)
		// Narrowed once, here at the boundary. The friction product is a float
		// product in this version as it is in 1.8.9, and blocks_test asserts
		// that every value in the dataset survives the narrowing.
		table.frictions = append(table.frictions, float32(physics.Slipperiness(block.Name)))

		ids := shapes.Blocks[block.Name]
		span := int(block.MaxStateID-block.MinStateID) + 1
		// The dataset describes a block's shapes either once for the whole
		// block or once per state, and nothing else means anything: a list of
		// some other length would be a state-to-shape mapping this table would
		// have to guess at. A block the index does not mention has no collision,
		// which is what the format says about a plant and what air is.
		if len(ids) != 0 && len(ids) != 1 && len(ids) != span {
			return blockTable{}, fmt.Errorf(
				"%w: block %q spans %d states and names %d collision shapes",
				ErrInvalidProfile, block.Name, span, len(ids),
			)
		}

		for state := block.MinStateID; state <= block.MaxStateID; state++ {
			handle := world.BlockRef(state) + 1
			if table.owner[handle] != 0 {
				return blockTable{}, fmt.Errorf(
					"%w: state %d belongs to both %q and %q",
					ErrInvalidProfile, state, table.names[table.owner[handle]], block.Name,
				)
			}

			shape := geom.EmptyShape()
			if len(ids) > 0 {
				id := ids[0]
				if len(ids) == span {
					id = ids[state-block.MinStateID]
				}

				resolved, err := shapeOf(shapes, converted, block.Name, id)
				if err != nil {
					return blockTable{}, err
				}
				shape = resolved
			}

			table.owner[handle] = index
			table.shapes[handle] = shape
		}

		if block.DefaultState < block.MinStateID || block.DefaultState > block.MaxStateID {
			return blockTable{}, fmt.Errorf(
				"%w: block %q defaults to state %d, outside its own span %d to %d",
				ErrInvalidProfile, block.Name, block.DefaultState, block.MinStateID, block.MaxStateID,
			)
		}
		table.byName[block.Name] = world.BlockRef(block.DefaultState) + 1
	}

	for handle := 1; handle < len(table.owner); handle++ {
		if table.owner[handle] == 0 {
			return blockTable{}, fmt.Errorf(
				"%w: state %d is claimed by no block", ErrInvalidProfile, handle-1,
			)
		}
	}

	return table, nil
}

// shapeOf resolves one shape identifier, building it the first time it is asked
// for and returning the same shape every time after.
//
// A shape identifier the index does not hold is an error rather than an empty
// shape: the dataset named a volume it does not describe, and a block that
// quietly lost its collider is the kind of fault that shows up as a body falling
// through a floor several milestones later.
func shapeOf(
	shapes data.CollisionShapes,
	converted map[data.ShapeID]geom.Shape,
	name string,
	id data.ShapeID,
) (geom.Shape, error) {
	if shape, ok := converted[id]; ok {
		return shape, nil
	}

	boxes, ok := shapes.Shapes[id]
	if !ok {
		return geom.EmptyShape(), fmt.Errorf(
			"%w: block %q names collision shape %d, which the data set does not hold",
			ErrInvalidProfile, name, id,
		)
	}

	shape := geom.EmptyShape()
	if len(boxes) > 0 {
		volumes := make([]geom.AABB, 0, len(boxes))
		for _, box := range boxes {
			volumes = append(volumes, geom.AABB{
				MinX: box.MinX, MinY: box.MinY, MinZ: box.MinZ,
				MaxX: box.MaxX, MaxY: box.MaxY, MaxZ: box.MaxZ,
			})
		}
		shape = geom.NewShape(volumes...)
	}
	converted[id] = shape

	return shape, nil
}

// shape resolves a handle. It reports false for a handle this table did not
// mint, including the zero handle.
func (t blockTable) shape(ref world.BlockRef) (geom.Shape, bool) {
	if ref == 0 || int(ref) >= len(t.shapes) {
		return geom.EmptyShape(), false
	}

	return t.shapes[ref], true
}

// slipperiness returns a handle's friction, or the default for a handle this
// table did not mint.
func (t blockTable) slipperiness(ref world.BlockRef) float32 {
	if ref == 0 || int(ref) >= len(t.owner) {
		return t.frictions[0]
	}

	return t.frictions[t.owner[ref]]
}

// ref resolves a block name to the handle of its default state. Fixtures and
// tests name blocks, because a handle means nothing outside the table that
// minted it.
//
// A caller that means a particular state — a stair facing east, a slab in the
// top half — names the state instead, through refState. There is no way to name
// a state by its properties here, because the dataset publishes which properties
// a block varies over and not which state a combination of them lands on.
func (t blockTable) ref(name string) (world.BlockRef, bool) {
	handle, ok := t.byName[name]

	return handle, ok
}

// refState resolves a block state identifier, which is what this version's
// protocol carries and what a world loaded from a real server arrives as.
func (t blockTable) refState(state data.BlockStateID) (world.BlockRef, bool) {
	if state < 0 || int(state)+1 >= len(t.owner) {
		return 0, false
	}

	return world.BlockRef(state) + 1, true
}

// name returns the block a handle names, for diagnostics.
func (t blockTable) name(ref world.BlockRef) string {
	if ref == 0 || int(ref) >= len(t.owner) {
		return ""
	}

	return t.names[t.owner[ref]]
}
