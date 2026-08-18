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

// metadataValues is how many values this version's metadata can take.
//
// Four bits, because that is what the wire carries beside a block id and what
// Block.getMetaFromState produces. Every block gets a run of that many handles
// whether or not it uses them, so that a handle's metadata is arithmetic rather
// than a second table to keep in step.
const metadataValues = 16

// blockTable turns the dataset's block, shape, and physics tables into the
// opaque handles world.BlockRef promises.
//
// A handle names a block *state*: one block and one metadata value. This
// version addresses a state as a block id and four bits, and a table with one
// handle per block cannot express a top slab, a stair facing west, or a log
// lying along X — the placement rules have nowhere to put their answer, and the
// collision shape a top slab resolves to would be the bottom slab's.
//
// The layout is a run of metadataValues handles per block, in dataset order,
// starting at one so the zero handle stays meaningless. A name resolves to the
// run's first handle, which is metadata zero, so everything that names a block
// — a fixture, a scene, a test — resolves exactly what it resolved before this
// table learned about metadata.
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
	// Handle zero is reserved, so the first block's metadata zero takes handle
	// one. A slot is kept for it so that a handle indexes directly.
	table.names = append(table.names, "")
	table.shapes = append(table.shapes, geom.EmptyShape())
	table.frictions = append(table.frictions, float32(physics.DefaultSlipperiness))

	for _, block := range blocks {
		table.byName[block.Name] = world.BlockRef(len(table.names))

		// Narrowed once, here at the boundary. The friction product is a float
		// product in the game, and blocks_test asserts that every value in the
		// dataset survives the narrowing. It is the block's rather than the
		// state's: the game holds slipperiness on the block, so every metadata
		// value answers alike.
		friction := float32(physics.Slipperiness(block.Name))

		for metadata := range metadataValues {
			shape, err := shapeOf(shapes, block.Name, metadata)
			if err != nil {
				return blockTable{}, err
			}

			table.names = append(table.names, block.Name)
			table.shapes = append(table.shapes, shape)
			table.frictions = append(table.frictions, friction)
		}
	}

	return table, nil
}

// shapeOf builds one block state's collision shape from the dataset.
//
// The dataset indexes a block's shapes by metadata where they differ and
// carries a single one where they do not: stone_slab lists sixteen — the
// bottom half for the low eight and the top half for the high eight — and
// stone lists one. A block the shape index does not mention has no collision,
// which is what air is.
//
// A metadata beyond what the block lists takes the block's first shape. Those
// are the values the game never produces for that block, and answering with
// the shape it does produce is closer to true than answering with none.
func shapeOf(shapes data.CollisionShapes, name string, metadata int) (geom.Shape, error) {
	ids, ok := shapes.Blocks[name]
	if !ok || len(ids) == 0 {
		return geom.EmptyShape(), nil
	}

	id := ids[0]
	if metadata < len(ids) {
		id = ids[metadata]
	}

	boxes, ok := shapes.Shapes[id]
	if !ok {
		return geom.EmptyShape(), fmt.Errorf(
			"%w: block %q names collision shape %d, which the data set does not hold",
			ErrInvalidProfile, name, id,
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

// refState resolves a block name and a metadata value to their handle.
//
// It is what a placement rule answers with: the rule computes the metadata the
// game would, and this turns that into a handle the world can hold. A metadata
// outside four bits is refused rather than wrapped, because a wrapped one names
// a different state and would place a different block.
func (t blockTable) refState(name string, metadata int) (world.BlockRef, bool) {
	base, ok := t.byName[name]
	if !ok || metadata < 0 || metadata >= metadataValues {
		return 0, false
	}

	return base + world.BlockRef(metadata), true
}

// metadata returns the metadata a handle carries, or zero for one this table
// did not mint.
func (t blockTable) metadata(ref world.BlockRef) int {
	if ref == 0 || int(ref) >= len(t.names) {
		return 0
	}

	return int((ref - 1) % metadataValues)
}

// name returns the block a handle names, for diagnostics.
func (t blockTable) name(ref world.BlockRef) string {
	if ref == 0 || int(ref) >= len(t.names) {
		return ""
	}

	return t.names[ref]
}
