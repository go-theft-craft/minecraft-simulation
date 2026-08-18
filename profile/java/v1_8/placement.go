package v1_8

import (
	"fmt"
	"strings"

	"github.com/go-theft-craft/minecraft-protocol/data"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/mining"
	"github.com/go-theft-craft/minecraft-simulation/placement"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// Asserted here rather than left to a caller's hope.
var _ placement.Placer = (*profile)(nil)

// ErrNotPlaceable reports an item this version does not place as a block.
var ErrNotPlaceable = fmt.Errorf("v1_8: the item places no block")

// placementTable answers which block an item places.
//
// 1.8.9 names an item and the block it places identically — the item "stone"
// places the block "stone" — so the table is a name lookup rather than a
// mapping. What it is not is an assumption that every item is a block: a
// pickaxe places nothing, and asking for its state is an error rather than a
// stone.
type placementTable struct {
	// blocks is the block each item id places, by name. An item that places no
	// block is absent.
	blocks map[data.ItemID]string
}

// newPlacementTable pairs each item with the block of the same name.
func newPlacementTable(set *data.Set) (placementTable, error) {
	if set == nil {
		return placementTable{}, fmt.Errorf("%w: no data set", ErrInvalidProfile)
	}

	// A data set with no items builds a table that places nothing, for the
	// reason the mining table answers hardness without materials: refusing to
	// build takes the whole version down because one dataset is partial, and a
	// set assembled from blocks and physics alone — which is what the digest
	// tests build — has no business failing over a question it never asks.
	table := placementTable{blocks: make(map[data.ItemID]string)}
	items, blocks := set.Items(), set.Blocks()
	if items == nil || blocks == nil {
		return table, nil
	}

	for _, item := range items.All() {
		if _, ok := blocks.ByName(item.Name); ok {
			table.blocks[item.ID] = item.Name
		}
	}

	return table, nil
}

// PlacedState implements placement.Placer.
//
// The metadata is computed per family and every layout is transcribed from the
// jar, named at its rule, because this version publishes no property-to-metadata
// mapping anywhere in its generated data. What keeps a transcription honest is
// the corpus in placement/testdata/vanilla, which is what Block.onBlockPlaced
// answered for the same clicks.
//
// A family this stage does not carry places its block's default state, which is
// metadata zero. That is right for every block with no orientation and wrong
// for the ones a later stage adds — a door, a torch, a piston — so the corpus
// records which families it covers rather than leaving a reader to assume all
// of them.
func (p *profile) PlacedState(
	item data.ItemID, target placement.Target, face mining.Face, yaw float32, cursor geom.Vec3,
) (world.BlockRef, error) {
	name, ok := p.placement.blocks[item]
	if !ok {
		return 0, fmt.Errorf("%w: item %d", ErrNotPlaceable, item)
	}

	ref, ok := p.blocks.refState(name, metadataFor(name, face, yaw, cursor))
	if !ok {
		return 0, fmt.Errorf("%w: block %q", mining.ErrUnknownBlock, name)
	}

	return ref, nil
}

// metadataFor computes the four bits this version stores a placement in.
func metadataFor(name string, face mining.Face, yaw float32, cursor geom.Vec3) int {
	switch {
	case strings.HasSuffix(name, "_stairs"):
		return stairsMetadata(face, yaw, cursor)
	case strings.HasSuffix(name, "_slab") || name == "stone_slab2":
		return slabMetadata(face, cursor)
	case name == "log" || name == "log2":
		return logMetadata(face)
	default:
		return 0
	}
}

// stairsMetadata is BlockStairs.getMetaFromState over onBlockPlaced's state.
//
// The facing bits are 5 minus the game's own direction index — DOWN 0, UP 1,
// NORTH 2, SOUTH 3, WEST 4, EAST 5 — so east is 0, west 1, south 2, and north
// 3. The top half sets bit 4.
func stairsMetadata(face mining.Face, yaw float32, cursor geom.Vec3) int {
	metadata := 0
	switch placement.FacingFromYaw(yaw) {
	case placement.FacingEast:
		metadata = 0
	case placement.FacingWest:
		metadata = 1
	case placement.FacingSouth:
		metadata = 2
	case placement.FacingNorth:
		metadata = 3
	}

	if placement.HalfFor(face, cursor) == placement.HalfTop {
		metadata |= 4
	}

	return metadata
}

// slabMetadata is BlockSlab's half bit.
//
// The low three bits are the slab's variant — which stone, which wood — and
// they come from the item's own metadata rather than from the placement, so a
// placement of the default variant leaves them zero. Bit 3 is the half.
func slabMetadata(face mining.Face, cursor geom.Vec3) int {
	if placement.HalfFor(face, cursor) == placement.HalfTop {
		return 8
	}

	return 0
}

// logMetadata is BlockLog's axis bits.
//
// The low two bits are the wood variant, which a placement does not choose, and
// bits 2 and 3 are the axis: Y is 0, X is 4, Z is 8, and the fourth combination
// is the barked block no placement produces.
func logMetadata(face mining.Face) int {
	switch placement.AxisFor(face) {
	case placement.AxisX:
		return 4
	case placement.AxisZ:
		return 8
	default:
		return 0
	}
}
