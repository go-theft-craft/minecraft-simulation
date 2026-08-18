package v26_1

import (
	"fmt"
	"strings"

	"github.com/go-theft-craft/minecraft-protocol/data"

	"github.com/go-theft-craft/minecraft-simulation/mining"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// Asserted here rather than left to a caller's hope. The M7 defect the master
// plan records is exactly a seam satisfied by assertion that nobody asserted.
var _ mining.Classifier = (*profile)(nil)

// tierMaterialPrefix names the materials that say which tools are *incorrect*
// for a block rather than how fast a tool breaks it.
//
// 26.1.2 encodes tool correctness as block tags — a ToolMaterial carries an
// incorrectBlocksForDrops tag, and gold ore is in the wooden and stone ones —
// and the dataset flattens a block's mining tags into one material name. For a
// block that carries both a mineable tag and a tier tag, only the tier tag
// survives that flattening, which is the whole difficulty of this version. See
// speedFor.
const tierMaterialPrefix = "incorrect_for_"

// mineablePrefix names the materials that are genuine tool-speed tables.
const mineablePrefix = "mineable/"

// miningTable answers what this version needs to know about breaking a block.
type miningTable struct {
	blocks map[string]blockMining
	// speeds is the material registry, by material name.
	speeds map[string]data.ToolSpeedIndex
	// mineable is the subset of speeds whose names are tool classes, kept
	// separately because recovering a tier-tagged block's class searches them.
	mineable map[string]data.ToolSpeedIndex
}

// blockMining is one block's share of the answer.
type blockMining struct {
	hardness *float64
	material string
	harvest  data.HarvestToolSet
	// speeds is the table this block's speed is read from, resolved once at
	// build time. For most blocks it is the material's own; for a block whose
	// material is a tier tag it is the recovered tool class's. It is nil for a
	// block no table could be found for, which means every tool is speed one.
	speeds data.ToolSpeedIndex
}

// newMiningTable reads the block and material registries.
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
		table.mineable = make(map[string]data.ToolSpeedIndex)
		for _, material := range materials {
			table.speeds[material.Name] = material.ToolSpeeds
			if strings.Contains(material.Name, mineablePrefix) {
				table.mineable[material.Name] = material.ToolSpeeds
			}
		}
	}

	for _, block := range set.Blocks().All() {
		entry := blockMining{
			hardness: block.Hardness,
			material: block.Material,
			harvest:  block.HarvestTools,
		}
		entry.speeds = table.speedsFor(entry)
		table.blocks[block.Name] = entry
	}

	return table, nil
}

// speedsFor picks the tool-speed table a block's speed is read from.
//
// For most blocks it is the material's own table, and the material name is a
// registry key even when it is a compound like "gourd;mineable/axe" — upstream
// publishes the merged table under the compound name, so splitting on the
// semicolon and merging by hand would recompute what is already stated, and
// differ the first time upstream merged by a rule other than "best speed wins".
//
// For a block whose material is a tier tag it is neither. Gold ore's material
// is "incorrect_for_wooden_tool", whose table lists the four wooden tools at
// speed two and nothing else: it says which tools cannot harvest the block, not
// how fast anything breaks it. Reading it as a speed table gives a wooden
// *shovel* the pickaxe's speed against ore and gives a diamond pickaxe no speed
// at all — 90 ticks against vanilla's 15.
//
// The tool class the flattening dropped is recoverable, because HarvestTools
// survives it: gold ore's harvest tools are the iron, diamond, and netherite
// pickaxes, and exactly one mineable material has all three as keys. Checked
// mechanically on 2026-08-18 across every block in this version: 107 of the 108
// tier-tagged blocks resolve to exactly one class this way, and the remaining
// one — the crafter — publishes no harvest tools at all and so resolves to
// none. The sweep test names it rather than hiding it.
func (t miningTable) speedsFor(block blockMining) data.ToolSpeedIndex {
	if !strings.HasPrefix(block.material, tierMaterialPrefix) {
		return t.speeds[block.material]
	}

	if len(block.harvest) == 0 {
		return nil
	}

	var found data.ToolSpeedIndex
	for _, speeds := range t.mineable {
		if !covers(speeds, block.harvest) {
			continue
		}
		if found != nil {
			// Two classes claim the block, so nothing here can say which one
			// the flattening dropped. Speed one is the honest answer, and the
			// sweep test is what reports it.
			return nil
		}

		found = speeds
	}

	return found
}

// covers reports whether every harvest tool is a key of the speed table.
func covers(speeds data.ToolSpeedIndex, tools data.HarvestToolSet) bool {
	for tool := range tools {
		if _, ok := speeds[tool]; !ok {
			return false
		}
	}

	return true
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
// The speed table is resolved at build time, for the reason speedsFor states.
// Harvest legality is the block's own HarvestTools and is a different question
// from speed on this version as on the other: a wooden pickaxe on obsidian is
// effective and cannot harvest.
func (p *profile) Conditions(
	ref world.BlockRef, held mining.Held, effects mining.Effects, underwater, airborne bool,
) (mining.Conditions, error) {
	if p.mining.speeds == nil {
		return mining.Conditions{}, fmt.Errorf(
			"%w: the data set carries no materials, so no tool has a speed", ErrInvalidProfile,
		)
	}

	block, ok := p.mining.blocks[p.blocks.name(ref)]
	if !ok {
		return mining.Conditions{}, fmt.Errorf("%w: handle %d", mining.ErrUnknownBlock, ref)
	}

	return mining.Conditions{
		Speed:         toolSpeed(block.speeds, held.Item),
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

// toolSpeed returns the held item's multiplier against a block's table.
//
// One, not zero, for a tool the table does not list. A shovel on stone is slow
// rather than stuck, and a speed of zero would divide a break time into
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
