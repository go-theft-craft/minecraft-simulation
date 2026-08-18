package v26_1

import (
	"context"
	"fmt"
	"math"

	"github.com/go-theft-craft/minecraft-simulation/collision"
	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/movement"
	"github.com/go-theft-craft/minecraft-simulation/sim"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// The constants of this version's land tick, each at the width the game holds
// it. Every one of them differs from 1.8.9's in value, in width, or in what it
// is multiplied by, which is why none of them is imported from there.
const (
	// inputDecay multiplies both input axes, in the shared entity tick and again
	// as the first step of the client's own shaping.
	inputDecay float32 = 0.98
	// sneakingSpeed is the sneaking-speed attribute a player has with no
	// modifiers. 1.8.9 has no such attribute and uses a double literal of the
	// same value.
	sneakingSpeed float32 = 0.3
	// horizontalThreshold is the squared horizontal motion below which a player
	// stops. It is 0.003 squared, and it applies to the vector rather than to
	// each axis.
	horizontalThreshold = 9.0e-6
	// verticalThreshold is the vertical motion the same rule discards, tested on
	// its own magnitude.
	verticalThreshold = 0.003
	// jumpMinimum is the jump power at or below which a jump does nothing.
	jumpMinimum float32 = 1.0e-5
	// sprintJumpImpulse is the horizontal boost a sprinting jump adds. The
	// impulse is a double literal here where 1.8.9's is a float, so the product
	// with the single-width sine forms at double width.
	sprintJumpImpulse = 0.2
	// jumpDelay is what a jump sets the counter to.
	jumpDelay int32 = 10
	// airBlockFriction is the friction of "the block below" for an airborne
	// body: not a block's value at all, but the one the game substitutes.
	airBlockFriction float32 = 1.0
	// frictionDrag is what the block friction is multiplied by to give the
	// tick's horizontal drag.
	frictionDrag float32 = 0.91
	// accelerationNumerator is what a grounded body's acceleration divides by
	// the cube of the raw block friction. 1.8.9 divides 0.16277136F by the cube
	// of the drag instead, and the two are the same rule with different
	// arithmetic: this constant is that one divided by 0.91³.
	accelerationNumerator float32 = 0.21600002
	// airSpeed and sprintAirSpeed are what replaces the acceleration while
	// airborne. 1.8.9 carries this as a field on the entity; this version reads
	// it from the sprint flag, so Locomotion.JumpFactor is not what this profile
	// steers with.
	airSpeed       float32 = 0.02
	sprintAirSpeed float32 = 0.025999999
	// inputThreshold is the squared input magnitude below which the tick applies
	// no heading. It is a double here and 1.0E-4F in 1.8.9.
	inputThreshold = 1.0e-7
	// normalizeThreshold is the length below which the game's normalize returns
	// a zero vector rather than dividing. It is a float literal widened.
	normalizeThreshold = float64(float32(1.0e-5))
	// degreesToRadians is the pre-divided conversion this version's heading and
	// jump impulse both use, narrowed once.
	degreesToRadians float32 = math.Pi / 180
	// tableScale converts radians to a table index. It is a double here, where
	// 1.8.9's is a float, and the truncation goes through a long rather than an
	// int.
	tableScale = 10430.378350470453
	// cosineOffset is the quarter turn that turns a sine read into a cosine
	// read. It is added to the scaled angle before the truncation, at double
	// width.
	cosineOffset = 16384.0
	// supportOffset is how far below its position a body looks for the block it
	// is standing on. The odd last digit is the game's: it is just over half a
	// block so that a body standing on a slab reads the slab's own cell.
	supportOffset = float64(float32(0.500001))
	// supportProbe is how far below its floor a body probes for a supporting
	// block.
	supportProbe = 1.0e-6
	// cellMargin is the slack the game's own block sweep adds to a region before
	// it decides which cells to visit.
	cellMargin = 1.0e-7
	// movedThreshold is the squared motion a move has to carry to be applied at
	// all. A shorter one still moves the body when it is most of what was asked
	// for; what it stops is a body pressed into a wall creeping along it.
	movedThreshold = 1.0e-7
)

// body is the per-tick working state for one entity.
//
// The phases carry their intermediate values here rather than writing them
// through the tick, so a tick that turns out to be incomplete leaves nothing
// behind: the kernel drops operations, and the commit phase is the only one that
// produces any.
type body struct {
	id    entity.ID
	state entity.State
	loco  movement.Locomotion

	strafe  float32
	forward float32
	// blockFriction is the friction of the block below, raw. Both the drag and
	// the acceleration derive from it, and they derive differently, which is
	// this version's largest formula difference from 1.8.9.
	blockFriction float32
	drag          float32
	speed         float32
	// applied is the motion the move actually carried, which the support record
	// probes backwards along.
	applied geom.Vec3

	collided bool
	landed   bool
	// present is false for an entity in scope that the store does not hold, or
	// holds without locomotion state. Later phases skip it.
	present bool
}

// scratch is one tick's working state, shared by the phases of one phase list.
//
// A phase list is stateful within a tick and belongs to one kernel: NewKernel
// takes the list once, and a kernel steps one tick at a time, so nothing here is
// ever touched by two ticks at once. Phases() returns a fresh list precisely so
// that two kernels built from one profile do not share this.
//
// The first phase resets it, so a tick that failed halfway cannot leak a value
// into the next one.
type scratch struct {
	bodies []body
}

// buildPhases returns the thirteen phases of the 26.1.2 land tick, in order.
//
// It is not 1.8.9's eleven with different constants. Two steps this version has
// and that one does not — the input shaping and the block speed factor — are
// phases of their own, and the friction phase computes two numbers where 1.8.9's
// computes one, because this version derives the acceleration from the raw block
// friction and the drag from the product.
func (p *profile) buildPhases() []sim.Phase {
	shared := &scratch{}

	return []sim.Phase{
		phase{id: "v26_1.jump-countdown", run: func(tick *sim.TickState) error {
			return p.adoptInput(tick, shared)
		}},
		phase{id: "v26_1.motion-threshold", run: func(*sim.TickState) error {
			return p.motionThreshold(shared)
		}},
		phase{id: "v26_1.input-shaping", run: func(*sim.TickState) error {
			return p.shapeInput(shared)
		}},
		phase{id: "v26_1.jump", run: func(*sim.TickState) error {
			return p.jump(shared)
		}},
		phase{id: "v26_1.friction", run: func(tick *sim.TickState) error {
			return p.friction(tick, shared)
		}},
		phase{id: "v26_1.acceleration", run: func(*sim.TickState) error {
			return p.acceleration(shared)
		}},
		phase{id: "v26_1.apply-input", run: func(*sim.TickState) error {
			return p.applyInput(shared)
		}},
		// ItemEntity.tick calls applyGravity before it moves, where a player
		// falls after. One ordered list holds both because each phase skips
		// the families it is not about.
		phase{id: "v26_1.item-gravity", run: func(*sim.TickState) error {
			return p.itemGravity(shared)
		}},
		phase{id: "v26_1.move", run: func(tick *sim.TickState) error {
			return p.move(tick, shared)
		}},
		phase{id: "v26_1.block-speed-factor", run: func(*sim.TickState) error {
			return p.blockSpeedFactor(shared)
		}},
		phase{id: "v26_1.gravity", run: func(*sim.TickState) error {
			return p.gravity(shared)
		}},
		phase{id: "v26_1.vertical-drag", run: func(*sim.TickState) error {
			return p.verticalDrag(shared)
		}},
		phase{id: "v26_1.horizontal-drag", run: func(*sim.TickState) error {
			return p.horizontalDrag(shared)
		}},
		phase{id: "v26_1.item-drag", run: func(tick *sim.TickState) error {
			return p.itemDrag(tick, shared)
		}},
		phase{id: "v26_1.item-bounce", run: func(*sim.TickState) error {
			return p.itemBounce(shared)
		}},
		phase{id: "v26_1.arrow-inertia", run: func(*sim.TickState) error {
			return p.arrowInertia(shared)
		}},
		phase{id: "v26_1.arrow-gravity", run: func(*sim.TickState) error {
			return p.arrowGravity(shared)
		}},
		phase{id: "v26_1.commit", run: func(tick *sim.TickState) error {
			return p.commit(tick, shared)
		}},
	}
}

// phase adapts a function to sim.Phase.
type phase struct {
	id  string
	run func(*sim.TickState) error
}

// ID implements sim.Phase.
func (p phase) ID() string { return p.id }

// Run implements sim.Phase.
func (p phase) Run(_ context.Context, tick *sim.TickState) error { return p.run(tick) }

// adoptInput starts the tick: it gathers the bodies in scope, decrements each
// jump counter, and adopts this tick's input flags and facing.
func (p *profile) adoptInput(tick *sim.TickState, shared *scratch) error {
	shared.bodies = shared.bodies[:0]

	inputs := latestInputs(tick.Commands())
	for _, id := range tick.Scope().Entities {
		working := body{id: id}

		state, ok := tick.Entity(id)
		if !ok {
			shared.bodies = append(shared.bodies, working)

			continue
		}
		loco, ok := tick.Locomotion(id)
		if !ok {
			shared.bodies = append(shared.bodies, working)

			continue
		}

		// A family with no constants is refused here rather than at the phase
		// that first needs a number, because the phases are guarded by family
		// and an unknown one would otherwise fall through all of them and
		// commit unmoved.
		if _, err := p.constantsFor(&body{id: id, state: state}); err != nil {
			return err
		}

		working.state = state
		working.loco = movement.Countdown(loco)
		working.present = true

		if input, ok := inputs[id]; ok {
			working.loco.Jumping = input.Jump
			working.loco.Sprinting = input.Sprint
			working.loco.Sneaking = input.Sneak
			working.loco.Yaw = input.Yaw
			working.loco.Pitch = input.Pitch
			working.strafe = input.Strafe
			working.forward = input.Forward
		}

		shared.bodies = append(shared.bodies, working)
	}

	return nil
}

// latestInputs indexes the tick's movement intents by body, keeping the last one
// for each. A controller that sent two intents for one tick meant the second.
func latestInputs(commands []sim.Command) map[entity.ID]movement.Input {
	inputs := make(map[entity.ID]movement.Input, len(commands))
	for _, command := range commands {
		if input, ok := command.(movement.Input); ok {
			inputs[input.Entity] = input
		}
	}

	return inputs
}

// motionThreshold discards a motion too small to matter.
//
// The horizontal test is on the vector and the vertical on its own axis, which
// is the player's rule in this version. Every other entity tests each horizontal
// axis separately, so a profile that grows past the player has a branch to add
// here rather than a constant to change.
func (p *profile) motionThreshold(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		if working.state.Family != entity.FamilyPlayer {
			continue
		}

		working.state.Motion = clampMotion(working.state.Motion)
	}

	return nil
}

// clampMotion is the threshold rule itself, on one motion.
//
// The horizontal test is on the vector: a motion of 0.0025 along each of two
// axes is longer than 0.003 and survives here, where 1.8.9's per-axis test
// discards both components of it.
func clampMotion(motion geom.Vec3) geom.Vec3 {
	if motion.HorizontalLengthSquared() < horizontalThreshold {
		motion.X = 0
		motion.Z = 0
	}
	if math.Abs(motion.Y) < verticalThreshold {
		motion.Y = 0
	}

	return motion
}

// shapeInput applies the client's own shaping to the two input axes.
func (p *profile) shapeInput(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		if working.state.Family != entity.FamilyPlayer {
			continue
		}

		working.strafe, working.forward = ShapeInput(
			working.strafe, working.forward, working.loco.Sneaking,
		)
	}

	return nil
}

// jump applies the jump impulse to every body whose state permits one.
//
// The counter is set whenever the branch is taken, even when the power was too
// small to move the body: the game sets it outside the impulse and not inside
// it, so a body with no jump strength still cannot re-enter this branch for ten
// ticks.
func (p *profile) jump(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		if working.state.Family != entity.FamilyPlayer {
			continue
		}

		if !working.loco.Jumping {
			working.loco.JumpTicks = 0

			continue
		}
		if !working.state.OnGround || working.loco.JumpTicks > 0 {
			continue
		}

		working.state.Motion = p.jumpImpulse(working.state.Motion, working.loco)
		working.loco.JumpTicks = jumpDelay
	}

	return nil
}

// jumpImpulse is the jump itself.
//
// The power is a float product of the jump-strength attribute and the block jump
// factor, plus the jump-boost bonus; this profile has neither a jump factor nor
// an effect, so the power is the attribute. The vertical motion takes the larger
// of the power and what was already there, where 1.8.9 assigns over it — a body
// already rising faster than a jump keeps its speed here and loses it there.
func (p *profile) jumpImpulse(motion geom.Vec3, loco movement.Locomotion) geom.Vec3 {
	power := defaultJumpStrength * blockJumpFactor
	if !(power > jumpMinimum) {
		return motion
	}

	if float64(power) > motion.Y {
		motion.Y = float64(power)
	}
	if loco.Sprinting {
		angle := loco.Yaw * degreesToRadians
		motion.X -= float64(p.sin(angle)) * sprintJumpImpulse
		motion.Z += float64(p.cos(angle)) * sprintJumpImpulse
	}

	return motion
}

// friction reads the block below and derives the tick's two numbers from it.
//
// Both come from the raw block friction and they come from it differently: the
// drag is the product with 0.91, and the acceleration divides by the cube of the
// raw value. 1.8.9 cubes the product. Reading the block here rather than in the
// drag phase is what makes a player who walks off ice keep ice friction for the
// tick that leaves it.
func (p *profile) friction(tick *sim.TickState, shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		if working.state.Family != entity.FamilyPlayer {
			continue
		}

		working.blockFriction = airBlockFriction
		if working.state.OnGround {
			ref, lookup := tick.BlockState(blockBelow(working.state))
			if lookup == world.LookupUnknown {
				// The tick is already incomplete; the kernel drops its work.
				continue
			}
			working.blockFriction = p.slipperiness(ref)
		}
		working.drag = working.blockFriction * frictionDrag
	}

	return nil
}

// acceleration turns the block friction into this tick's input scale.
//
// Airborne, this version reads the sprint flag rather than a field: a sprinting
// body steers slightly better in the air, and the two values are literals in the
// player's own class rather than constants of the entity tick.
func (p *profile) acceleration(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		if working.state.Family != entity.FamilyPlayer {
			continue
		}

		if !working.state.OnGround {
			working.speed = airSpeed
			if working.loco.Sprinting {
				working.speed = sprintAirSpeed
			}

			continue
		}

		friction := working.blockFriction
		working.speed = working.loco.MoveSpeed * (accelerationNumerator / (friction * friction * friction))
	}

	return nil
}

// applyInput adds the scaled input to each body's motion.
//
// The whole of this is double-width arithmetic except the sine and the cosine,
// which are single-width table reads. That is the reverse of 1.8.9's heading,
// where everything is single width and only the finished bracket is widened, and
// it is why the two cannot share a function.
func (p *profile) applyInput(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		if working.state.Family != entity.FamilyPlayer {
			continue
		}

		working.state.Motion = p.inputVector(
			working.state.Motion, working.strafe, working.forward, working.speed, working.loco.Yaw,
		)
	}

	return nil
}

// inputVector is the game's own getInputVector, added to the motion.
//
// The input is a three-axis vector because the game builds one; land movement
// leaves the vertical axis at zero, and a later mechanic that swims or flies
// fills it in rather than adding a second rule here.
func (p *profile) inputVector(
	motion geom.Vec3, strafe, forward, speed, yaw float32,
) geom.Vec3 {
	input := geom.Vec3{X: float64(strafe), Z: float64(forward)}

	length := input.X*input.X + input.Y*input.Y + input.Z*input.Z
	if length < inputThreshold {
		return motion
	}
	if length > 1.0 {
		input = normalize(input)
	}
	input = input.Scale(float64(speed))

	angle := yaw * degreesToRadians
	sin := float64(p.sin(angle))
	cos := float64(p.cos(angle))

	motion.X += input.X*cos - input.Z*sin
	motion.Y += input.Y
	motion.Z += input.Z*cos + input.X*sin

	return motion
}

// normalize is the game's own, including its threshold: a vector shorter than
// a float's ten-millionth is zeroed rather than divided.
func normalize(v geom.Vec3) geom.Vec3 {
	length := math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z)
	if length < normalizeThreshold {
		return geom.Vec3{}
	}

	return geom.Vec3{X: v.X / length, Y: v.Y / length, Z: v.Z / length}
}

// sin returns this version's sine: the same table 1.8.9 reads, indexed the way
// this version indexes it — a double multiply truncated through a long.
func (p *profile) sin(angle float32) float32 {
	return p.table.At(int64(float64(angle) * tableScale))
}

// cos returns this version's cosine, which is the same read a quarter turn
// along. The quarter turn is added before the truncation and at double width,
// so it cannot be folded into the index.
func (p *profile) cos(angle float32) float32 {
	return p.table.At(int64(float64(angle)*tableScale + cosineOffset))
}

// move resolves each body's motion against the world through this version's
// shape-based collision.
//
// The two horizontal flags forgive a shortfall under a hundred-thousandth and
// the vertical one compares exactly, which collision.Result already reports. The
// vertical motion is zeroed by the block that was landed on rather than by the
// move: an ordinary block zeroes it, and the blocks that do something else —
// slime, beds, honey — are a per-block behaviour the dataset does not publish, so
// every block is ordinary here.
func (p *profile) move(tick *sim.TickState, shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		result, err := movement.StepWith(
			collision.ResolveVoxel, tick.Blocks(), working.state, tick.Limits().CollisionCandidates,
		)
		if err != nil {
			return fmt.Errorf("resolving entity %d: %w", working.id, err)
		}
		if len(result.Unknown) != 0 {
			// collision reports incompleteness in its own return value, so the
			// phase is what tells the kernel. Without this the tick would report
			// itself complete over a region nobody described.
			tick.MissingBlocks(result.Unknown)

			continue
		}

		wasAirborne := !working.state.OnGround

		// The position moves and the box is rebuilt around it, which is the
		// order this version's move works in and not the one 1.8.9 works in.
		// Offsetting the box instead differs in its last bits, and the oracle
		// found it on the first tick of the first scenario it compared.
		//
		// The move is only applied at all when it went somewhere: an applied
		// motion too short to matter is dropped unless it is most of what was
		// asked for, which is the game's own guard against a body creeping
		// while pressed into a wall.
		applied := result.Applied
		working.applied = applied
		asked := working.state.Motion
		length := applied.X*applied.X + applied.Y*applied.Y + applied.Z*applied.Z
		askedLength := asked.X*asked.X + asked.Y*asked.Y + asked.Z*asked.Z
		if length > movedThreshold || askedLength-length < movedThreshold {
			working.state.Position = working.state.Position.Add(applied)
			working.state.Box = playerBox(working.state.Position)
		}
		working.state.OnGround = result.OnGround
		working.collided = result.CollidedHorizontally()
		working.landed = wasAirborne && result.OnGround

		// The record of what is holding the body up, kept by the move because the
		// game keeps it there and read by the next tick's friction. A body that
		// is on the ground with nothing under it — one that has just walked off
		// the lip of a block, and is still standing because this tick's fall was
		// stopped by what it is leaving — probes again where it came from, which
		// is what makes it keep that block's friction for one more tick.
		if !p.recordSupport(tick, working) {
			continue
		}

		if result.CollidedX {
			working.state.Motion.X = 0
		}
		if result.CollidedZ {
			working.state.Motion.Z = 0
		}
		if result.CollidedY {
			working.state.Motion.Y = 0
		}
	}

	return nil
}

// recordSupport is the game's checkSupportingBlock, run after every move.
//
// It reports false when the world could not be read, which leaves the tick
// incomplete rather than recording a support that a described world might have
// contradicted.
func (p *profile) recordSupport(tick *sim.TickState, working *body) bool {
	if !working.state.OnGround {
		working.state.Support = entity.Support{}

		return true
	}

	box := working.state.Box
	probe := geom.AABB{
		MinX: box.MinX, MinY: box.MinY - supportProbe, MinZ: box.MinZ,
		MaxX: box.MaxX, MaxY: box.MinY, MaxZ: box.MaxZ,
	}

	block, found, ok := supportingBlock(tick.BlockShape, probe, working.state.Position)
	if !ok {
		return false
	}
	if found || working.state.Support.NoBlocks {
		working.state.Support = entity.Support{Block: block, Present: found}
	} else {
		// Nothing underneath, and the last look did find something: the game
		// probes where the body came from before it gives up.
		behind := probe.Offset(geom.Vec3{X: -working.applied.X, Z: -working.applied.Z})

		block, found, ok = supportingBlock(tick.BlockShape, behind, working.state.Position)
		if !ok {
			return false
		}
		working.state.Support = entity.Support{Block: block, Present: found}
	}
	working.state.Support.NoBlocks = !found

	return true
}

// blockSpeedFactor multiplies the two horizontal components by the factor of the
// block the body stands in, or of the block below when that one is neutral.
//
// The factor is 1 for every block here, because the dataset publishes no such
// measurement — soul sand and honey are what this step exists for, and they are
// ordinary until it does. The phase is in the order anyway: it runs between the
// move and gravity, which is where the game applies it, and a dataset that
// starts carrying the factor fills this in without moving anything.
func (p *profile) blockSpeedFactor(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		if working.state.Family != entity.FamilyPlayer {
			continue
		}

		factor := blockSpeedNeutral
		if factor == 1 {
			continue
		}

		working.state.Motion.X *= float64(factor)
		working.state.Motion.Z *= float64(factor)
	}

	return nil
}

// gravity applies one tick of fall to every body, at its own family's rate.
func (p *profile) gravity(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		constants, err := p.constantsFor(working)
		if err != nil {
			return err
		}

		working.state.Motion = movement.ApplyGravity(working.state.Motion, constants.Gravity)
	}

	return nil
}

// verticalDrag applies the vertical multiplier.
func (p *profile) verticalDrag(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		constants, err := p.constantsFor(working)
		if err != nil {
			return err
		}

		working.state.Motion = movement.ApplyVerticalDrag(
			working.state.Motion, float32(constants.VerticalDrag),
		)
	}

	return nil
}

// horizontalDrag applies the drag this tick's friction phase recorded.
func (p *profile) horizontalDrag(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		working.state.Motion = movement.ApplyHorizontalDrag(working.state.Motion, working.drag)
	}

	return nil
}

// commit writes the tick's results and reports what happened.
//
// It is the only phase that produces operations. Everything before it worked on
// the scratch, so an incomplete tick had nothing to drop.
func (p *profile) commit(tick *sim.TickState, shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		tick.SetEntity(working.id, working.state)
		tick.SetLocomotion(working.id, working.loco)

		if working.collided {
			tick.EmitDomain(sim.DomainEvent{Kind: "movement.collided", Entity: working.id})
		}
		if working.landed {
			tick.EmitDomain(sim.DomainEvent{Kind: "movement.landed", Entity: working.id})
		}
	}

	return nil
}

// blockJumpFactor is the jump factor of the block a body jumps from. Honey is
// what it exists for, and the dataset does not publish it, so it is one.
const blockJumpFactor float32 = 1

// blockSpeedNeutral is the speed factor of a block that does not slow anything,
// which is every block this dataset can describe.
const blockSpeedNeutral float32 = 1

// blockBelow returns the cell whose friction applies to a body.
//
// This is statement (6), and it is the rule most likely to be read as an
// optimization and rewritten into 1.8.9's. The game does not take the cell under
// the body's feet. It takes the block the last move recorded as supporting the
// body, keeps its column, and reads half a block below the body's position — so
// a body standing on the lip of a block reads that block rather than the air
// beside it, and a body that has just walked off the edge of a slab still reads
// the slab's column for the tick that leaves it.
//
// Half a block below the top of a slab is the cell under the slab, not the slab
// itself. That is the game's arithmetic and not a slip: the support supplies the
// column and the offset supplies the height, so a body standing on an ice slab is
// not standing on ice.
//
// One known gap, stated rather than papered over: walls and fence gates answer
// with the supporting cell itself rather than the offset one, and the dataset
// publishes no block tags to tell them apart from anything else.
func blockBelow(state entity.State) geom.BlockPos {
	below := geom.Floor(state.Position.Y - supportOffset)
	if !state.Support.Present {
		return geom.BlockPos{
			X: geom.Floor(state.Position.X), Y: below, Z: geom.Floor(state.Position.Z),
		}
	}

	return geom.BlockPos{X: state.Support.Block.X, Y: below, Z: state.Support.Block.Z}
}

// shapeLookup is how the supporting-block probe reads the world.
//
// It is a function rather than a view so that the tick's own reader can be
// passed straight in: a cell this probe consults is a cell the tick depended on,
// and a probe that read around the tick would produce a complete result over
// state nobody recorded.
type shapeLookup func(geom.BlockPos) (geom.Shape, world.Lookup)

// supportingBlock finds the block under a region.
//
// It takes the cell whose collision shape reaches into the region and whose
// centre is nearest the body's position. Ties go to the greater cell in the
// game's own block ordering, which is by Y, then Z, then X.
//
// The third return reports whether every cell it needed was described. A tick
// that could not see one of them is incomplete, and the caller stops rather than
// answering from a partial world.
func supportingBlock(
	lookup shapeLookup, probe geom.AABB, pos geom.Vec3,
) (geom.BlockPos, bool, bool) {
	var (
		best     geom.BlockPos
		bestDist = math.Inf(1)
		found    bool
	)
	// The cell range is the game's own, and it is wider than the region by a
	// cell on every side. That margin is not caution: a block's shape may reach
	// outside its cell — a fence is a block and a half tall — so a cell the
	// region does not touch can still hold a collider that reaches into it.
	for x := geom.Floor(probe.MinX-cellMargin) - 1; x <= geom.Floor(probe.MaxX+cellMargin)+1; x++ {
		for y := geom.Floor(probe.MinY-cellMargin) - 1; y <= geom.Floor(probe.MaxY+cellMargin)+1; y++ {
			for z := geom.Floor(probe.MinZ-cellMargin) - 1; z <= geom.Floor(probe.MaxZ+cellMargin)+1; z++ {
				cell := geom.BlockPos{X: x, Y: y, Z: z}

				shape, seen := lookup(cell)
				if seen == world.LookupUnknown {
					return geom.BlockPos{}, false, false
				}
				if shape.IsEmpty() || !reaches(shape, cell, probe) {
					continue
				}

				distance := distanceToCentre(cell, pos)
				if distance < bestDist || (distance == bestDist && (!found || less(best, cell))) {
					best = cell
					bestDist = distance
					found = true
				}
			}
		}
	}

	return best, found, true
}

// reaches reports whether any box of a shape in a cell overlaps a region in a
// volume. A shape that only touches the region's face does not reach it, which
// is the same rule collision uses.
func reaches(shape geom.Shape, cell geom.BlockPos, region geom.AABB) bool {
	for _, box := range shape.BoxesAt(cell, nil) {
		if box.Intersects(region) {
			return true
		}
	}

	return false
}

// distanceToCentre is the squared distance from a point to a cell's centre,
// which is what the game compares supporting blocks by.
func distanceToCentre(cell geom.BlockPos, pos geom.Vec3) float64 {
	dx := float64(cell.X) + 0.5 - pos.X
	dy := float64(cell.Y) + 0.5 - pos.Y
	dz := float64(cell.Z) + 0.5 - pos.Z

	return dx*dx + dy*dy + dz*dz
}

// less orders two cells the way the game's block position does: by Y, then Z,
// then X. It decides ties between two equally near supporting blocks, and the
// game keeps the greater one.
func less(a, b geom.BlockPos) bool {
	if a.Y != b.Y {
		return a.Y < b.Y
	}
	if a.Z != b.Z {
		return a.Z < b.Z
	}

	return a.X < b.X
}

// playerBox is the box the game builds around a position.
//
// The construction is movement.Box, which both versions provably share; the two
// dimensions are this version's own. Rebuilding rather than offsetting is what
// keeps this profile's box identical to the game's after a move.
func playerBox(pos geom.Vec3) geom.AABB {
	return movement.Box(pos, playerWidth, playerHeight)
}

// constantsFor returns the body's own motion constants.
//
// Every phase that needs a number reads it through here rather than naming a
// family. Until M9.2 they all named entity.FamilyPlayer outright, so a dropped
// item in the world fell at the player's gravity and nothing said so.
func (p *profile) constantsFor(working *body) (sim.MotionConstants, error) {
	constants := p.Motion(working.state.Family)
	if constants == (sim.MotionConstants{}) {
		return sim.MotionConstants{}, fmt.Errorf("%w: entity %d is a %s",
			sim.ErrUnknownFamily, working.id, working.state.Family)
	}

	return constants, nil
}

// itemGravity applies a dropped item's fall, before it moves.
//
// ItemEntity.tick calls applyGravity at the top and a player falls after the
// move, so the two families cannot share a phase however alike the arithmetic
// looks. The gravity itself is a double literal in this version and a float
// literal in 1.8.9, which is why the two datasets carry different numbers for
// what reads as the same constant.
func (p *profile) itemGravity(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present || working.state.Family != entity.FamilyItem {
			continue
		}

		constants, err := p.constantsFor(working)
		if err != nil {
			return err
		}

		working.state.Motion = movement.ApplyGravity(working.state.Motion, constants.Gravity)
	}

	return nil
}

// itemDrag applies an item's two multipliers after it has moved.
//
// They are not the same number here. multiply(friction, 0.98, friction) takes a
// float friction — the block's own times 0.98F, formed at single width — on the
// horizontal axes and the double literal 0.98 on the vertical one. 1.8.9 uses
// the float on all three.
func (p *profile) itemDrag(tick *sim.TickState, shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present || working.state.Family != entity.FamilyItem {
			continue
		}

		constants, err := p.constantsFor(working)
		if err != nil {
			return err
		}

		slipperiness := airBlockFriction
		if working.state.OnGround {
			ref, lookup := tick.BlockState(blockBelow(working.state))
			if lookup == world.LookupUnknown {
				// The tick is already incomplete; the kernel drops its work.
				continue
			}
			slipperiness = p.slipperiness(ref)
		}

		friction := movement.FrictionWith(
			slipperiness, working.state.OnGround, float32(constants.HorizontalDrag),
		)
		working.state.Motion = movement.ApplyHorizontalDrag(working.state.Motion, friction)

		// The vertical multiplier is applied at double width, because the
		// literal is a double: narrowing it to a float first is a different
		// number in the last bits and compounds over a fall.
		working.state.Motion.Y *= constants.VerticalDrag
	}

	return nil
}

// itemBounce is the half-height hop an item takes off the ground.
//
// This version tests the direction — the motion has to be downward — where
// 1.8.9 applies it on any contact. An item on the ground with upward motion
// keeps it here and loses it there.
func (p *profile) itemBounce(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present || working.state.Family != entity.FamilyItem {
			continue
		}
		if !working.state.OnGround || working.state.Motion.Y >= 0 {
			continue
		}

		working.state.Motion.Y *= itemBounceFactor
	}

	return nil
}

// itemBounceFactor is the multiplier an item's vertical motion takes on the
// ground. It is a double literal in ItemEntity.tick.
const itemBounceFactor = -0.5

// arrowInertia applies an arrow's single multiplier to all three axes.
//
// AbstractArrow.applyInertia scales the whole vector, so this is one number on
// three axes rather than the two a player carries, and it runs after the move.
func (p *profile) arrowInertia(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present || working.state.Family != entity.FamilyArrow {
			continue
		}

		constants, err := p.constantsFor(working)
		if err != nil {
			return err
		}

		inertia := float32(constants.HorizontalDrag)
		working.state.Motion = movement.ApplyHorizontalDrag(working.state.Motion, inertia)
		working.state.Motion = movement.ApplyVerticalDrag(working.state.Motion, inertia)
	}

	return nil
}

// arrowGravity applies an arrow's fall, after its inertia.
func (p *profile) arrowGravity(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present || working.state.Family != entity.FamilyArrow {
			continue
		}

		constants, err := p.constantsFor(working)
		if err != nil {
			return err
		}

		working.state.Motion = movement.ApplyGravity(working.state.Motion, constants.Gravity)
	}

	return nil
}
