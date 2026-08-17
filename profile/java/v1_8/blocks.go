// Package v1_8 supplies the Java Edition 1.8.9 rules: the constants, the widths
// they are computed at, the block table, and the order the tick's phases run in.
//
// It is the only package in this module that imports game data. Everything below
// it — geom, world, entity, collision, sim, runtime, and movement — is version
// neutral, and a rule that needed a 1.8.9 number receives it from here.
//
// Every quantity the game holds as a float is held here as a float32 and widened
// once, at the point the game widens it. A signature carrying float32 is stating
// the width the product is formed at.
package v1_8

import (
	"fmt"

	"github.com/go-theft-craft/minecraft-protocol/data"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// blockTable turns the dataset's block, shape, and physics tables into the
// opaque handles world.BlockRef promises.
//
// A handle is an index into this table plus one, so the zero handle stays
// meaningless and only the profile that minted a handle can say what it names.
type blockTable struct {
	// names is the block each handle names, in handle order.
	names []string
	// shapes is the collision shape each handle resolves to.
	shapes []geom.Shape
	// frictions is each handle's slipperiness, at the width the friction product
	// is formed at.
	frictions []float32
	// byName resolves a block name to its handle, for tests and fixtures.
	byName map[string]world.BlockRef
}

// newBlockTable builds the table from a dataset.
//
// It rejects a set with no physics, because a profile whose slipperiness table
// silently defaulted would produce a body that walks on ice like it walks on
// stone and no test would say why.
func newBlockTable(set *data.Set) (blockTable, error) {
	if set == nil {
		return blockTable{}, fmt.Errorf("%w: no data set", ErrInvalidProfile)
	}

	physics := set.Physics()
	if len(physics.BlockSlipperiness) == 0 {
		return blockTable{}, fmt.Errorf("%w: the data set carries no block slipperiness", ErrInvalidProfile)
	}

	shapes := set.CollisionShapes()
	blocks := set.Blocks().All()
	if len(blocks) == 0 {
		return blockTable{}, fmt.Errorf("%w: the data set carries no blocks", ErrInvalidProfile)
	}

	table := blockTable{byName: make(map[string]world.BlockRef, len(blocks))}
	// Handle zero is reserved, so the first block takes handle one. A slot is
	// kept for it so that a handle indexes directly.
	table.names = append(table.names, "")
	table.shapes = append(table.shapes, geom.EmptyShape())
	table.frictions = append(table.frictions, float32(physics.DefaultSlipperiness))

	for _, block := range blocks {
		shape, err := shapeOf(shapes, block.Name)
		if err != nil {
			return blockTable{}, err
		}

		table.byName[block.Name] = world.BlockRef(len(table.names))
		table.names = append(table.names, block.Name)
		table.shapes = append(table.shapes, shape)
		// Narrowed once, here at the boundary. The friction product is a float
		// product in the game, and blocks_test asserts that every value in the
		// dataset survives the narrowing.
		table.frictions = append(table.frictions, float32(physics.Slipperiness(block.Name)))
	}

	return table, nil
}

// shapeOf builds one block's collision shape from the dataset.
//
// A block with several state shapes uses the first, because 1.8.9 carries
// metadata variants rather than the flattened states later versions have and this
// milestone simulates the land tick over full blocks. A block the shape index
// does not mention has no collision, which is what air is.
func shapeOf(shapes data.CollisionShapes, name string) (geom.Shape, error) {
	ids, ok := shapes.Blocks[name]
	if !ok || len(ids) == 0 {
		return geom.EmptyShape(), nil
	}

	boxes, ok := shapes.Shapes[ids[0]]
	if !ok {
		return geom.EmptyShape(), fmt.Errorf(
			"%w: block %q names collision shape %d, which the data set does not hold",
			ErrInvalidProfile, name, ids[0],
		)
	}
	if len(boxes) == 0 {
		return geom.EmptyShape(), nil
	}

	converted := make([]geom.AABB, 0, len(boxes))
	for _, box := range boxes {
		converted = append(converted, geom.AABB{
			MinX: box.MinX, MinY: box.MinY, MinZ: box.MinZ,
			MaxX: box.MaxX, MaxY: box.MaxY, MaxZ: box.MaxZ,
		})
	}

	return geom.NewShape(converted...), nil
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
	if ref == 0 || int(ref) >= len(t.frictions) {
		return t.frictions[0]
	}

	return t.frictions[ref]
}

// ref resolves a block name to its handle. Fixtures and tests name blocks,
// because a handle means nothing outside the table that minted it.
func (t blockTable) ref(name string) (world.BlockRef, bool) {
	ref, ok := t.byName[name]

	return ref, ok
}

// name returns the block a handle names, for diagnostics.
func (t blockTable) name(ref world.BlockRef) string {
	if ref == 0 || int(ref) >= len(t.names) {
		return ""
	}

	return t.names[ref]
}
