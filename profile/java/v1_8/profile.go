package v1_8

import (
	"errors"
	"fmt"

	"github.com/go-theft-craft/minecraft-protocol/data"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/movement"
	"github.com/go-theft-craft/minecraft-simulation/sim"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// ErrInvalidProfile reports a data set this profile cannot be built from.
var ErrInvalidProfile = errors.New("v1_8: invalid profile data")

// Identity is what this profile answers for sim.Profile.ID.
//
// The rules revision is ours, not the game's: a fix to our implementation of
// 1.8.9 moves it without touching the game version, and a replay has to be able
// to tell those apart.
var Identity = sim.ProfileID{Edition: "java", GameVersion: "1.8.9", RulesRevision: "1"}

// jumpUpwards is the vertical motion a jump sets. It is a float literal in the
// game, so it is one here.
const jumpUpwards float32 = 0.42

// inputDecay multiplies both input axes before they are used.
const inputDecay float32 = 0.98

// defaultJumpFactor is the airborne movement factor for a body with no
// modifiers.
const defaultJumpFactor float32 = 0.02

// profile implements sim.Profile for Java Edition 1.8.9.
type profile struct {
	blocks blockTable
	table  movement.Table
	motion map[entity.Family]sim.MotionConstants
}

// New builds the 1.8.9 profile from a data set.
//
// The set supplies the constants, the block table, and the trigonometry table.
// Nothing in this constructor computes a game value: every number comes from the
// dataset, at the width the dataset records it.
func New(set *data.Set) (sim.Profile, error) {
	blocks, err := newBlockTable(set)
	if err != nil {
		return nil, err
	}

	physics := set.Physics()
	table, err := movement.NewTable(physics.SinTable)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidProfile, err)
	}

	player, ok := physics.EntityMotion["player"]
	if !ok {
		return nil, fmt.Errorf("%w: the data set carries no player motion constants", ErrInvalidProfile)
	}

	built := &profile{
		blocks: blocks,
		table:  table,
		motion: map[entity.Family]sim.MotionConstants{
			entity.FamilyPlayer: {
				Gravity:        player.Gravity,
				HorizontalDrag: player.HorizontalDrag,
				VerticalDrag:   player.VerticalDrag,
				StepHeight:     player.StepHeight,
			},
		},
	}
	return built, nil
}

// ID implements sim.Profile.
func (p *profile) ID() sim.ProfileID { return Identity }

// Slipperiness implements sim.Profile.
//
// The interface returns float64 because it is version neutral, and the rules
// need the value at single width. The narrowing happened when the table was
// built; this widens the already-narrowed value, so the round trip costs
// nothing.
func (p *profile) Slipperiness(ref world.BlockRef) float64 {
	return float64(p.blocks.slipperiness(ref))
}

// slipperiness returns the value at the width the friction product needs.
func (p *profile) slipperiness(ref world.BlockRef) float32 {
	return p.blocks.slipperiness(ref)
}

// Motion implements sim.Profile.
func (p *profile) Motion(family entity.Family) sim.MotionConstants {
	return p.motion[family]
}

// Shape implements sim.Profile.
func (p *profile) Shape(ref world.BlockRef) (geom.Shape, bool) {
	return p.blocks.shape(ref)
}

// Phases implements sim.Profile.
//
// Each call returns a fresh list. The phases of one list share a scratch
// structure for the tick they are running, so a list belongs to the one kernel
// that took it: NewKernel reads this once, and a kernel steps one tick at a
// time. Two kernels built from this profile therefore get separate scratch, and
// stepping them concurrently is safe.
func (p *profile) Phases() []sim.Phase { return p.buildPhases() }

// Ref resolves a block name to the handle this profile minted for it.
//
// It is exported because a caller loading a world names blocks: a handle is
// meaningless outside the profile that minted it, so there has to be a way in.
func Ref(built sim.Profile, name string) (world.BlockRef, bool) {
	owner, ok := built.(*profile)
	if !ok {
		return 0, false
	}

	return owner.blocks.ref(name)
}

// Spawn returns the body and locomotion state of a player standing at a
// position, with this version's constants already applied.
//
// It exists so that a consumer does not have to know the player's box dimensions
// or its default attributes, which are version facts like every other.
func Spawn(built sim.Profile, pos geom.Vec3, yaw, pitch float32) (entity.State, movement.Locomotion, bool) {
	owner, ok := built.(*profile)
	if !ok {
		return entity.State{}, movement.Locomotion{}, false
	}

	constants := owner.Motion(entity.FamilyPlayer)

	return entity.State{
			Family: entity.FamilyPlayer,
			Box: geom.AABB{
				MinX: pos.X - playerHalfWidth, MinY: pos.Y, MinZ: pos.Z - playerHalfWidth,
				MaxX: pos.X + playerHalfWidth, MaxY: pos.Y + float64(playerHeight),
				MaxZ: pos.Z + playerHalfWidth,
			},
			OnGround:   true,
			StepHeight: constants.StepHeight,
		}, movement.Locomotion{
			Yaw:        yaw,
			Pitch:      pitch,
			MoveSpeed:  defaultMoveSpeed,
			JumpFactor: defaultJumpFactor,
		}, true
}

// The player's collision box in 1.8.9: 0.6 wide and 1.8 tall, centred on the
// position, with the position at its feet.
//
// The game holds both as floats and builds the box by halving the width in
// float arithmetic and adding the height to a double position. So the box a
// vanilla player stands in is not 0.6 by 1.8 of a block: it is a sixteenth of a
// millionth wider and a tenth of that shorter, and every collision the body has
// is decided by those edges. The oracle caught this on the first tick it
// compared, before any rule had run.
const (
	playerWidth  float32 = 0.6
	playerHeight float32 = 1.8
)

// playerHalfWidth is the horizontal offset from the position to a face, at the
// width the game computes it: a float halving, widened once.
const playerHalfWidth = float64(playerWidth / 2)

// defaultMoveSpeed is the player's movement-speed attribute with no modifiers.
// The game holds it as a float and reads it through the attribute map.
const defaultMoveSpeed float32 = 0.1
