package v26_1

import (
	"fmt"
	"slices"
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
var ErrNotPlaceable = fmt.Errorf("v26_1: the item places no block")

// placementTable answers which block an item places, and how that block's
// states are numbered.
//
// The numbering is the part worth having in a table. This version addresses a
// state as a flat id inside the block's own range, and the offset within that
// range is a mixed-radix number over the block's properties in their published
// order: the last property varies fastest. That is not a convention this
// package invented — it is what the generated data's own default state decodes
// under, and the corpus in placement/testdata/vanilla is what says it holds.
type placementTable struct {
	// blocks is the block each item id places, by name.
	blocks map[data.ItemID]string
	// states is each block's property list and range, by block name.
	states map[string]blockStates
}

// blockStates is one block's share of the numbering.
type blockStates struct {
	minimum    data.BlockStateID
	defaultsAt data.BlockStateID
	properties data.BlockStates
}

// newPlacementTable pairs each item with the block of the same name and records
// how that block's states are numbered.
func newPlacementTable(set *data.Set) (placementTable, error) {
	if set == nil {
		return placementTable{}, fmt.Errorf("%w: no data set", ErrInvalidProfile)
	}

	table := placementTable{
		blocks: make(map[data.ItemID]string),
		states: make(map[string]blockStates),
	}
	// A data set with no items builds a table that places nothing, for the
	// reason the mining table answers hardness without materials: refusing to
	// build takes the whole version down because one dataset is partial.
	items, blocks := set.Items(), set.Blocks()
	if blocks == nil {
		return table, nil
	}

	for _, block := range blocks.All() {
		table.states[block.Name] = blockStates{
			minimum:    block.MinStateID,
			defaultsAt: block.DefaultState,
			properties: block.States,
		}
	}
	if items == nil {
		return table, nil
	}

	for _, item := range items.All() {
		if _, ok := table.states[item.Name]; ok {
			table.blocks[item.ID] = item.Name
		}
	}

	return table, nil
}

// PlacedState implements placement.Placer.
//
// The properties a placement sets are named rather than positional — "facing",
// "half", "type", "axis" — and the offset they produce is computed from the
// block's own published property list. A block whose properties are ordered
// differently from another's is handled by that alone, which is why this
// version needs no per-family bit layout the way 1.8.9 does.
//
// A family this stage does not carry places the block's default state. That is
// right for every block with no orientation and wrong for the ones a later
// stage adds, and the corpus records which families it covers rather than
// leaving a reader to assume all of them.
func (p *profile) PlacedState(
	item data.ItemID, target placement.Target, face mining.Face, yaw float32, cursor geom.Vec3,
) (world.BlockRef, error) {
	name, ok := p.placement.blocks[item]
	if !ok {
		return 0, fmt.Errorf("%w: item %d", ErrNotPlaceable, item)
	}

	states, ok := p.placement.states[name]
	if !ok {
		return 0, fmt.Errorf("%w: block %q", mining.ErrUnknownBlock, name)
	}

	state, err := states.with(propertiesFor(name, states, face, yaw, cursor))
	if err != nil {
		return 0, fmt.Errorf("%w: block %q", err, name)
	}

	ref, ok := p.blocks.refState(state)
	if !ok {
		return 0, fmt.Errorf("%w: state %d of block %q", mining.ErrUnknownBlock, state, name)
	}

	return ref, nil
}

// propertiesFor is what a placement sets on each family this stage carries.
//
// It returns names and values in the version's own vocabulary, because that is
// what the state numbering is keyed by. The rules deciding *which* value —
// which way the block faces, which half it takes, which axis it lies along —
// are version-neutral and live in placement.
func propertiesFor(
	name string, states blockStates, face mining.Face, yaw float32, cursor geom.Vec3,
) map[string]string {
	switch {
	case strings.HasSuffix(name, "_stairs") && states.has("facing") && states.has("half"):
		return map[string]string{
			"facing": placement.FacingFromYaw(yaw).String(),
			"half":   placement.HalfFor(face, cursor).String(),
		}
	case strings.HasSuffix(name, "_slab") && states.has("type"):
		// A slab's half is a value of "type" rather than of "half", and its
		// third value — double — is what a slab placed into another slab
		// becomes. That case is a placement into an occupied cell, which this
		// stage refuses before it ever reaches here.
		return map[string]string{"type": placement.HalfFor(face, cursor).String()}
	case states.has("axis"):
		// The pillar family, chosen by the property rather than by the name.
		// A name-shaped rule gets this wrong: mushroom_stem ends in "_stem" and
		// has no axis at all — it is six booleans for which sides carry skin —
		// and asking it for one is how the sweep found this.
		return map[string]string{"axis": placement.AxisFor(face).String()}
	default:
		return nil
	}
}

// has reports whether the block declares a property by that name.
func (b blockStates) has(name string) bool {
	return slices.ContainsFunc(b.properties, func(property data.BlockState) bool {
		return property.Name == name
	})
}

// with returns the state id for the block's default state with the named
// properties overridden.
//
// The default is decoded into per-property values first, so a property this
// placement does not set keeps whatever the game's own default state has for
// it — a stair's shape, a block's waterlogging — rather than being reset to the
// first value in its list.
func (b blockStates) with(values map[string]string) (data.BlockStateID, error) {
	if len(values) == 0 || len(b.properties) == 0 {
		return b.defaultsAt, nil
	}

	indices, err := b.decode(b.defaultsAt)
	if err != nil {
		return 0, err
	}

	for name, want := range values {
		at := slices.IndexFunc(b.properties, func(property data.BlockState) bool {
			return property.Name == name
		})
		if at < 0 {
			return 0, fmt.Errorf("has no property %q", name)
		}

		index := slices.Index(valuesOf(b.properties[at]), want)
		if index < 0 {
			return 0, fmt.Errorf("property %q has no value %q", name, want)
		}
		indices[at] = index
	}

	return b.encode(indices), nil
}

// decode splits a state id into one index per property.
func (b blockStates) decode(state data.BlockStateID) ([]int, error) {
	offset := int(state - b.minimum)
	if offset < 0 {
		return nil, fmt.Errorf("state %d is below the block's range", state)
	}

	indices := make([]int, len(b.properties))
	for at := len(b.properties) - 1; at >= 0; at-- {
		count := b.properties[at].NumValues
		if count <= 0 {
			return nil, fmt.Errorf("property %q takes no values", b.properties[at].Name)
		}
		indices[at] = offset % count
		offset /= count
	}
	if offset != 0 {
		return nil, fmt.Errorf("state %d is beyond what the block's properties describe", state)
	}

	return indices, nil
}

// encode folds one index per property back into a state id.
func (b blockStates) encode(indices []int) data.BlockStateID {
	offset := 0
	for at, index := range indices {
		offset = offset*b.properties[at].NumValues + index
	}

	return b.minimum + data.BlockStateID(offset)
}

// valuesOf returns a property's values in their published order.
//
// A boolean property publishes none, because upstream leaves them implicit. The
// game's own order for one is true then false, which is what the default states
// in this dataset decode under: a stair's default is not waterlogged and sits
// at the odd offset that ordering puts it at.
func valuesOf(property data.BlockState) []string {
	if len(property.Values) != 0 {
		return property.Values
	}
	if property.Type == "bool" {
		return []string{"true", "false"}
	}

	return nil
}
