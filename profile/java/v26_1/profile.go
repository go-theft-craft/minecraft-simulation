// Package v26_1 supplies the Java Edition 26.1.2 rules for a player on land:
// the constants, the widths they are computed at, the block table, and the
// order the tick's phases run in.
//
// It is one of the two packages in this module that import game data, and it is
// deliberately not a copy of the 1.8.9 one. Everything below it — geom, world,
// entity, collision, sim, runtime, and movement — is version neutral, and a rule
// that needed a 26.1.2 number receives it from here.
//
// Where this version and 1.8.9 hold a quantity at the same width, the two
// packages say so with the same type. Where they do not, the type is this
// version's: the block friction is a float in both, so it is a float32 here as
// it is there, while the input vector this version normalizes at double width
// has no float32 in it at all.
//
// # The tick, as this version runs it
//
// Read from the version's own land-movement path rather than from 1.8.9's list.
// Each statement says what 1.8.9 does where the two differ, because the
// differences are what a profile written by analogy would get wrong.
//
//  1. The jump delay counts down, if it is above zero. The same rule and the
//     same counter as 1.8.9's.
//  2. The motion threshold is a player rule in this version, and a vector one.
//     For a player, the two horizontal components are zeroed together when the
//     horizontal motion's squared length is below 9.0E-6 — that is, when the
//     motion is shorter than 0.003 as a vector. Every other entity tests each
//     horizontal axis separately against 0.003. The vertical is tested on its
//     own against 0.003 either way. 1.8.9 tests all three axes separately
//     against 0.005, so this version discards a smaller motion and discards it
//     as a vector.
//  3. The input is shaped before the jump, where 1.8.9 decays it after. The
//     shared entity tick decays both axes by 0.98F; a client's own player
//     replaces that with a decay, a sneaking-speed factor, and a stretch of a
//     diagonal onto the unit square. This profile simulates a client's own
//     player, so its phase is the client's — see ShapeInput. The jump reads
//     neither axis, so the position in the order changes no number by itself.
//  4. The jump, when the body is standing and the delay is zero. Its power is
//     the jump-strength attribute — 0.42 for a player with no modifiers — times
//     the block jump factor, plus the jump-boost bonus. A power of 1.0E-5F or
//     less does nothing at all. Otherwise the vertical motion becomes the larger
//     of the jump power and the motion it already had, where 1.8.9 assigns 0.42
//     over whatever was there; a sprinting body then gains 0.2 along its facing,
//     from a float sine and cosine through a double multiply; and the delay is
//     set to 10 ticks whether or not the power was large enough to move the
//     body.
//  5. The friction of the block below gives two different numbers, and this is
//     the formula difference that matters most. The block friction is the
//     block's own when the body is on the ground and 1.0F when it is not. The
//     tick's horizontal drag is blockFriction * 0.91F at single width, as in
//     1.8.9. The acceleration is the movement speed times 0.21600002F divided by
//     the cube of the raw block friction. 1.8.9 divides 0.16277136F by the cube
//     of the product. On stone the two denominators differ by a factor of
//     0.91³, and the numerator changed to match, so a profile that ported the
//     1.8.9 rule and swapped the constant would be wrong on every surface that
//     is not stone.
//  6. "The block below" is the block the body is standing on, not the block
//     under its feet. It comes from the supporting block the last collision
//     recorded, read half a block down. 1.8.9 takes the cell at floor(x),
//     floor(box.minY) - 1, floor(z) and keeps no such record. A body standing on
//     the edge of a block can therefore disagree between the versions about
//     which block it is standing on.
//  7. The input becomes motion at double width. The two input axes and the
//     vertical axis form a vector; a squared length below 1.0E-7 contributes
//     nothing; a squared length above 1 is normalized. The result scales by the
//     acceleration from (5) and rotates by the body's yaw. Only the sine and
//     cosine are single width, taken from the table by an index this version
//     computes with a double multiplier through a long. 1.8.9's counterpart is
//     float throughout, thresholds at 1.0E-4F, and indexes the same table with a
//     float multiplier through an int. The table itself is byte-identical
//     between the two versions, which is measured rather than assumed.
//  8. The move resolves the motion against the world through this version's
//     shape-based collision, collision.ResolveVoxel, which reports the applied
//     motion and the collision flags. A horizontal axis that fell short by a
//     hundred-thousandth or more is zeroed; the vertical motion is zeroed by the
//     block landed on rather than by the move itself, which is why a slime block
//     bounces without the move knowing anything about it.
//  9. A block speed factor multiplies the two horizontal components after the
//     move. It comes from the block at the body's position, falls back to the
//     block below when that one is neutral, and is interpolated toward 1 by the
//     movement-efficiency attribute. Soul sand and honey are what it exists for.
//     1.8.9 has no such step in the tick: it slows a body from inside the
//     block's own collision callback instead, which is a different place in the
//     order and a different set of blocks.
//  10. Gravity is subtracted from the vertical motion as a double: the gravity
//     attribute, 0.08 for a player, or at most 0.01 while falling with slow
//     falling. 1.8.9 subtracts a literal.
//  11. The drags apply after gravity, as in 1.8.9: the vertical motion times
//     0.98F and each horizontal times the tick's friction from (5). Both
//     constants are floats widened against a double motion, so the products form
//     at double width.
//
// Two further facts that are not steps in the order:
//
//   - The movement speed a tick moves with is the one the previous tick left.
//     The player's tick reads the movement-speed attribute into the field that
//     drives (5) after the travel that used it. 1.8.9 does the same thing in the
//     same place, so this is a shared fact rather than a difference — and both
//     profiles take the speed as an input rather than reading an attribute.
//   - What is deliberately not here: swimming, elytra, climbing, levitation,
//     powder snow, and riding all branch before or inside the land path and are
//     out of this milestone's scope.
//
// # What this profile does not carry, and why
//
// Three quantities the tick reads are per-block facts the dataset does not
// publish: the block jump factor of (4), the block speed factor of (9), and the
// callback that zeroes or reverses the vertical motion on landing in (8). The
// dataset measures slipperiness and no other movement property, so this profile
// answers 1 for both factors and zeroes the vertical motion on any landing,
// which is what every ordinary block does. Honey, soul sand, slime, and beds are
// therefore ordinary here. The phases exist and are in the right place, so
// filling them in is a dataset change rather than a reordering.
//
// The input shaping a client applies to its own player is ShapeInput, and it is
// the tick's third phase here where 1.8.9's counterpart is a plain decay. See its
// documentation for why the two cannot be written the same way.
package v26_1

import (
	"errors"
	"fmt"
	"math"

	"github.com/go-theft-craft/minecraft-protocol/data"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/movement"
	"github.com/go-theft-craft/minecraft-simulation/sim"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// Identity is what this profile answers for sim.Profile.ID.
//
// The rules revision is ours, not the game's: a fix to our implementation of
// 26.1.2 moves it without touching the game version, and a replay has to be able
// to tell those apart.
var Identity = sim.ProfileID{Edition: "java", GameVersion: "26.1.2", RulesRevision: "1"}

// profile implements sim.Profile for Java Edition 26.1.2.
type profile struct {
	blocks    blockTable
	mining    miningTable
	placement placementTable
	table     movement.Table
	motion    map[entity.Family]sim.MotionConstants
	// dataDigest hashes the numbers above. It is computed once, because the
	// trigonometry table alone is a quarter of a megabyte and a replay asks for
	// this per recording.
	dataDigest sim.Digest
}

// New builds the 26.1.2 profile from a data set.
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

	mining, err := newMiningTable(set)
	if err != nil {
		return nil, err
	}

	placing, err := newPlacementTable(set)
	if err != nil {
		return nil, err
	}

	built := &profile{
		blocks:    blocks,
		mining:    mining,
		placement: placing,
		table:     table,
		motion: map[entity.Family]sim.MotionConstants{
			entity.FamilyPlayer: {
				Gravity:        player.Gravity,
				HorizontalDrag: player.HorizontalDrag,
				VerticalDrag:   player.VerticalDrag,
				StepHeight:     player.StepHeight,
			},
		},
	}

	// The other families are taken when the dataset carries them and left out
	// when it does not. Refusing to build would take the whole version down
	// because one family is missing, and the tick already refuses a family it
	// has no constants for — at the point of use, where the failure names the
	// body it is about.
	for family, name := range map[entity.Family]string{
		entity.FamilyItem:  "item",
		entity.FamilyArrow: "arrow",
	} {
		constants, carried := physics.EntityMotion[name]
		if !carried {
			continue
		}

		built.motion[family] = sim.MotionConstants{
			Gravity:        constants.Gravity,
			HorizontalDrag: constants.HorizontalDrag,
			VerticalDrag:   constants.VerticalDrag,
			StepHeight:     constants.StepHeight,
		}
	}
	built.dataDigest = computeDataDigest(built.blocks, physics.SinTable, built.motion)

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

// Ref implements sim.BlockNames, answering with the block's default state.
//
// A caller loading a world names blocks: a handle is meaningless outside the
// profile that minted it, so there has to be a way in. A caller holding this
// version's own state identifiers — which is what its protocol carries — uses
// RefState instead.
func (p *profile) Ref(name string) (world.BlockRef, bool) {
	return p.blocks.ref(name)
}

// Ref resolves a block name against a profile this package built.
//
// It is the same lookup as the method, for callers holding a sim.Profile that
// they know came from here. Callers that do not know should assert
// sim.BlockNames instead.
func Ref(built sim.Profile, name string) (world.BlockRef, bool) {
	owner, ok := built.(sim.BlockNames)
	if !ok {
		return 0, false
	}

	return owner.Ref(name)
}

// RefState resolves a block state identifier against a profile this package
// built.
//
// It is what a world loader uses. The flattening made the state number the
// block's whole identity, and it is the number the protocol carries, so a
// consumer decoding a chunk has one of these per cell and nothing else.
func RefState(built sim.Profile, state data.BlockStateID) (world.BlockRef, bool) {
	owner, ok := built.(*profile)
	if !ok {
		return 0, false
	}

	return owner.blocks.refState(state)
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
			Family:     entity.FamilyPlayer,
			Box:        playerBox(pos),
			Position:   pos,
			OnGround:   true,
			StepHeight: constants.StepHeight,
		}, movement.Locomotion{
			Yaw:        yaw,
			Pitch:      pitch,
			MoveSpeed:  defaultMoveSpeed,
			JumpFactor: airSpeed,
		}, true
}

// The player's collision box in 26.1.2: 0.6 wide and 1.8 tall, centred on the
// position, with the position at its feet.
//
// The game halves the width in float arithmetic and adds the height to a double
// position, exactly as 1.8.9 does, so the box a player stands in is not 0.6 by
// 1.8 of a block: it is a sixteenth of a millionth wider and a tenth of that
// shorter. The two versions build it the same way from the same two floats,
// which is a shared fact rather than a coincidence — the dimensions are declared
// once per entity type and the box is made from them.
const (
	playerWidth  float32 = 0.6
	playerHeight float32 = 1.8
)

// defaultMoveSpeed is the player's movement-speed attribute with no modifiers.
// This version's attribute default is 0.1, read as a double and narrowed where
// the tick applies it.
const defaultMoveSpeed float32 = 0.1

// defaultJumpStrength is the player's jump-strength attribute with no modifiers.
//
// The attribute is a double whose value is a float's: the dataset publishes
// 0.41999998688697815, which is float32(0.42) widened. The tick narrows it back
// before multiplying, so the constant is held at the width the product forms at.
const defaultJumpStrength float32 = 0.42

// ShapeInput applies the input shaping a client performs on its own player,
// before the tick's other rules run.
//
// It is exported because the differential test has to hand the game the same
// numbers, and because a consumer holding a controller's raw axes needs it: the
// body this profile simulates is a client's own player, and that player shapes
// its input rather than decaying it.
//
// The three steps are the client's, in the client's order: both axes decay by
// 0.98F; a sneaking body scales by its sneaking-speed attribute; and a diagonal
// input is then stretched onto the unit square and clamped at one. The last step
// is why the order matters and why this cannot be split across the tick
// boundary. A keyboard diagonal reaches the clamp, and the clamp discards the
// decay along with everything else above one — so a body walking diagonally
// moves at the full input, not at 0.98 of it. A profile that applied the decay
// after the shaping would walk two percent slower on every diagonal.
//
// 1.8.9 has no counterpart. Its client scales for sneaking and its shared entity
// tick decays, in that order, with no shaping at all, which is why M8.4 could
// fold the scaling into a phase and this cannot.
//
// This is the one rule in this package that no jar-backed test covers as a rule:
// it lives in a class the server jar does not carry. The differential test
// checks its arithmetic against the game's own Vec2 and Mth — so the widths are
// gated — but the composition is transcribed, and saying so is the point.
func ShapeInput(strafe, forward float32, sneaking bool) (float32, float32) {
	if strafe == 0 && forward == 0 {
		return strafe, forward
	}

	strafe *= inputDecay
	forward *= inputDecay
	if sneaking {
		strafe *= sneakingSpeed
		forward *= sneakingSpeed
	}

	length := float32(math.Sqrt(float64(strafe*strafe + forward*forward)))
	if length <= 0 {
		return strafe, forward
	}

	// The game scales by the reciprocal rather than dividing each axis, and the
	// two disagree in the last bits: one rounding against two. The oracle found
	// this on the first sneaking diagonal it compared.
	inverse := 1 / length
	directionX := strafe * inverse
	directionZ := forward * inverse
	scaled := length * unitSquareDistance(directionX, directionZ)
	if scaled > 1 {
		scaled = 1
	}

	return directionX * scaled, directionZ * scaled
}

// unitSquareDistance returns how far the unit square reaches along a direction,
// which is one at the axes and the root of two at the diagonals.
func unitSquareDistance(x, z float32) float32 {
	x = float32(math.Abs(float64(x)))
	z = float32(math.Abs(float64(z)))

	tangent := z / x
	if z > x {
		tangent = x / z
	}

	return float32(math.Sqrt(float64(1 + tangent*tangent)))
}

// ErrInvalidProfile reports a data set this profile cannot be built from.
var ErrInvalidProfile = errors.New("v26_1: invalid profile data")
