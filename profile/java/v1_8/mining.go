package v1_8

import (
	"fmt"

	"github.com/go-theft-craft/minecraft-protocol/data"

	"github.com/go-theft-craft/minecraft-simulation/mining"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// Asserted here rather than left to a caller's hope. The M7 defect the master
// plan records is exactly a seam satisfied by assertion that nobody asserted.
var _ mining.Classifier = (*profile)(nil)

// miningTable answers what this version needs to know about breaking a block.
//
// It is keyed by block name rather than by handle because the two versions
// number handles differently — 1.8.9 by block, 26.1.2 by block state — and the
// mining facts belong to the block on both.
type miningTable struct {
	blocks map[string]blockMining
	// speeds is the material registry, by material name.
	speeds map[string]data.ToolSpeedIndex
}

// blockMining is one block's share of the answer.
type blockMining struct {
	hardness *float64
	material string
	harvest  data.HarvestToolSet
}

// newMiningTable reads the block and material registries.
//
// It does not fail on a block whose material the registry does not name. Task 0
// checked that every Material value in this version is a registry key, and this
// stays true by the sweep test rather than by a constructor that refuses to
// build the whole profile because one block moved.
//
// A data set with no material registry builds a table that answers hardness and
// refuses conditions. Refusing to build at all would take the whole version
// down because one dataset is partial — the profile's motion families already
// take the other route, and a test that assembles a set from blocks and
// physics alone has no business failing over a question it will never ask.
func newMiningTable(set *data.Set) (miningTable, error) {
	if set == nil {
		return miningTable{}, fmt.Errorf("%w: no data set", ErrInvalidProfile)
	}

	table := miningTable{blocks: make(map[string]blockMining)}
	if registry := set.Materials(); registry != nil {
		materials := registry.All()

		table.speeds = make(map[string]data.ToolSpeedIndex, len(materials))
		for _, material := range materials {
			table.speeds[material.Name] = material.ToolSpeeds
		}
	}

	for _, block := range set.Blocks().All() {
		table.blocks[block.Name] = blockMining{
			hardness: block.Hardness,
			material: block.Material,
			harvest:  block.HarvestTools,
		}
	}

	return table, nil
}

// Hardness implements mining.Classifier.
func (p *profile) Hardness(ref world.BlockRef) *float64 {
	block, ok := p.mining.blocks[p.blocks.name(ref)]
	if !ok {
		return nil
	}

	return block.hardness
}

// Conditions implements mining.Classifier.
//
// 1.8.9's vocabulary is eight plain material names — dirt, leaves, melon,
// plant, rock, web, wood, wool — and a block names exactly one. The speed is
// that material's entry for the held item, and the harvest legality is the
// block's own HarvestTools. The two are different questions and the game asks
// them separately: a wooden pickaxe on obsidian is effective and cannot
// harvest.
func (p *profile) Conditions(
	ref world.BlockRef, held mining.Held, effects mining.Effects, underwater, airborne bool,
) (mining.Conditions, error) {
	if p.mining.speeds == nil {
		return mining.Conditions{}, fmt.Errorf(
			"%w: the data set carries no materials, so no tool has a speed", ErrInvalidProfile,
		)
	}

	name := p.blocks.name(ref)
	block, ok := p.mining.blocks[name]
	if !ok {
		return mining.Conditions{}, fmt.Errorf("%w: handle %d", mining.ErrUnknownBlock, ref)
	}

	speeds, ok := p.mining.speeds[block.material]
	if !ok && block.material != "" {
		return mining.Conditions{}, fmt.Errorf(
			"%w: block %q names material %q, which the data set does not hold",
			ErrInvalidProfile, name, block.material,
		)
	}

	return mining.Conditions{
		Speed:         toolSpeed(speeds, held.Item),
		Harvestable:   harvestable(block.harvest, held.Item),
		Efficiency:    held.Efficiency,
		Haste:         effects.Haste,
		MiningFatigue: effects.MiningFatigue,
		HasHaste:      effects.HasHaste,
		HasFatigue:    effects.HasFatigue,
		Underwater:    underwater,
		Airborne:      airborne,
	}, nil
}

// toolSpeed returns the held item's multiplier against a material.
//
// One, not zero, for a tool the material does not list. A shovel on stone is
// slow rather than stuck, and a speed of zero would divide a break time into
// infinity — which reads as "unbreakable" to a caller that only checks the
// error.
func toolSpeed(speeds data.ToolSpeedIndex, held data.ItemID) float64 {
	if speed, ok := speeds[held]; ok {
		return speed
	}

	return 1
}

// harvestable reports whether the held item is good enough to drop the block.
//
// An empty set means "no tool required" rather than "no tool works": dirt,
// gravel, and every plant drop to a bare hand, and the dataset says so by
// listing nothing.
func harvestable(tools data.HarvestToolSet, held data.ItemID) bool {
	if len(tools) == 0 {
		return true
	}

	return tools[held]
}
