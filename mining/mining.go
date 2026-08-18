// Package mining computes how long a block takes to break.
//
// The arithmetic is shared between versions and the classification is not.
// Both editions compute a per-tick progress fraction the same way — a tool
// speed, divided by the block's hardness, divided by 30 when the tool can
// harvest it and 100 when it cannot — and a break is that fraction reaching one.
// What differs is how a version decides the tool speed and the harvest
// legality, because the two classify blocks by incompatible vocabularies. That
// half is a profile's, behind Classifier.
//
// Nothing here reads the world or the generated data. This package takes
// numbers and returns a number, which is what makes it testable as a table
// rather than as a server.
package mining

import "errors"

// ErrUnbreakable reports a block with no hardness — bedrock, barrier, the void
// air. It is an error rather than an infinite tick count because "never" and "a
// very large number of ticks" behave differently in every caller that has a
// timeout.
var ErrUnbreakable = errors.New("mining: block is unbreakable")

// Conditions is everything outside the block that changes how fast it breaks.
type Conditions struct {
	// Speed is the tool's multiplier against this block's material, or 1 when
	// the tool is not effective. It is not derived here: which tool is
	// effective against which material is version-owned, and the two versions
	// disagree about the vocabulary entirely.
	Speed float64
	// Harvestable reports whether the held tool is good enough to drop the
	// block. It changes the divisor, not the speed, which is why it is
	// separate from Speed and not folded into it.
	Harvestable bool
	// Efficiency is the enchantment level on the held tool, zero for none.
	Efficiency int
	// Haste and MiningFatigue are effect amplifiers, zero for none. They are
	// separate fields rather than a signed one because a player can have both
	// and vanilla applies both — a guardian's fatigue on a player standing in
	// a beacon's haste is not a contrived case.
	Haste, MiningFatigue int
	// HasHaste and HasFatigue say whether the effect is present at all, because
	// amplifier zero is haste I rather than no haste.
	HasHaste, HasFatigue bool
	// Underwater reports the player's head being in water without the
	// aqua-affinity enchantment; Airborne reports the player not on ground.
	Underwater, Airborne bool
}

// Damage is the per-tick progress fraction.
//
// This is the value the game actually computes and compares against one, and it
// is exported because the crack texture a client draws is driven by it rather
// than by a tick count.
//
// Transcribed from the two games rather than from a wiki, on 2026-08-18, out of
// the deobfuscated jars minecraft-reference pins:
//
//	1.8.9  Block.getPlayerRelativeBlockHardness and
//	       EntityPlayer.getToolDigEfficiency
//	26.1.2 BlockBehaviour.getDestroyProgress and Player.getDestroySpeed
//
// The two agree step for step. Where 1.8.9 writes a bare division by five for a
// submerged player, 26.1.2 multiplies by a SUBMERGED_MINING_SPEED attribute
// whose default is 0.2 and which aqua affinity raises; where 1.8.9 adds
// level*level+1 for Efficiency inline, 26.1.2 adds a MINING_EFFICIENCY
// attribute that the same enchantment sets to Mth.square(level) + 1. Both
// differences are in how the number is reached, not in the number. 26.1.2 also
// multiplies by a BLOCK_BREAK_SPEED attribute that defaults to one and that
// nothing in the vanilla game modifies, so it is not modelled.
//
// Every arithmetic step below runs in float32, because the games do. A product
// computed in float64 where the game computes it in float32 does not match bit
// for bit, which M8.1 found the expensive way.
func Damage(hardness *float64, c Conditions) (float64, error) {
	if hardness == nil {
		return 0, ErrUnbreakable
	}

	// A negative hardness is the other way a block says it cannot be broken.
	// Both games return a progress of zero for it rather than a negative one,
	// and zero progress per tick is a break that never completes — so it is
	// the same outcome as no hardness, reported the same way.
	if *hardness < 0 {
		return 0, ErrUnbreakable
	}

	return float64(damage32(*hardness, c)), nil
}

// damage32 is Damage in the width the game computes it.
func damage32(hardness float64, c Conditions) float32 {
	divisor := unharvestableDivisor
	if c.Harvestable {
		divisor = harvestableDivisor
	}

	return speed(c) / float32(hardness) / divisor
}

// ErrNeverBreaks reports progress the game's own accumulator cannot add up.
//
// It is a distinct outcome from ErrUnbreakable, and it is not a rounding
// nicety. Both games hold the progress so far in a float32 and add the
// per-tick fraction to it; near one, that float32's spacing is about 6e-8, so
// a fraction below half of that rounds away and the total stops moving. The
// block has a hardness and a better tool would finish it — but with this tool,
// under these effects, the player mines forever. Returning a very large tick
// count instead would promise a break that never comes.
var ErrNeverBreaks = errors.New("mining: progress is too small for the game to accumulate")

// BreakTicks returns how many ticks breaking the block takes.
//
// hardness is a pointer because the generated block data declares it as one and
// nil means unbreakable rather than zero. Bedrock has no hardness; it does not
// have a hardness of zero, and a rule that treats the two alike breaks bedrock
// instantly.
//
// The count is the number of times the game adds the per-tick fraction to its
// running total before the total reaches one — not the reciprocal of the
// fraction. The two differ, because the running total is a float32 and repeated
// addition of a constant is not multiplication by a count: stone with a bare
// hand is 150 ticks by the reciprocal and 151 by the addition, and the addition
// is what the game does.
//
// It counts the ticks that add progress, which is what a caller schedules
// between the start of a dig and its finish. A dig measured from the button
// going down costs one more tick than this: on 1.8.9 the falling edge calls
// PlayerControllerMP.clickBlock, which zeroes the progress and adds none, and
// every tick after it calls onPlayerDamageBlock, which adds.
//
// The floor is one tick, not zero. Progress is added once per tick and compared
// after, so a block whose per-tick fraction already exceeds one still costs the
// tick in which it was added — which is what a hardness of zero costs in the
// game, and zero is a real hardness rather than an absent one.
func BreakTicks(hardness *float64, c Conditions) (int, error) {
	if hardness == nil || *hardness < 0 {
		return 0, ErrUnbreakable
	}

	damage := damage32(*hardness, c)
	if damage <= 0 {
		return 0, ErrNeverBreaks
	}

	// The game's own loop: PlayerControllerMP.onPlayerDamageBlock on 1.8.9 and
	// MultiPlayerGameMode.continueDestroyBlock on 26.1.2 both add and then
	// compare against 1.0F.
	var progress float32
	for count := 1; ; count++ {
		next := progress + damage

		// The total stopped moving, so no number of further ticks will move
		// it either.
		if next == progress {
			return 0, ErrNeverBreaks
		}

		progress = next
		if progress >= 1 {
			return count, nil
		}
	}
}

// speed applies every modifier to the tool's own multiplier, in the order both
// games apply them. The order is not decoration: each step reads the running
// value, so moving one changes the result.
func speed(c Conditions) float32 {
	value := float32(c.Speed)

	// Efficiency only helps a tool that is already effective. A bare hand with
	// an Efficiency book in the other one is still a bare hand, and both games
	// gate the bonus on the speed already exceeding one.
	if value > 1 && c.Efficiency > 0 {
		value += float32(c.Efficiency*c.Efficiency) + 1
	}

	if c.HasHaste {
		value *= 1 + float32(c.Haste+1)*hastePerLevel
	}

	if c.HasFatigue {
		value *= fatigueScale(c.MiningFatigue)
	}

	// Both penalties, not one or the other. Written as an if-else this would
	// apply whichever it checked first and pass every single-condition test.
	if c.Underwater {
		value *= submergedScale
	}

	if c.Airborne {
		value /= airborneDivisor
	}

	return value
}

// fatigueScale is the multiplier one amplifier of mining fatigue applies.
//
// A table rather than a formula because the game's is a table: the four steps
// are not a curve, and the fourth is the value every higher amplifier gets.
func fatigueScale(amplifier int) float32 {
	switch amplifier {
	case 0:
		return 0.3
	case 1:
		return 0.09
	case 2:
		return 0.0027
	default:
		return 8.1e-4
	}
}

const (
	// harvestableDivisor and unharvestableDivisor are what separates a tool
	// that drops the block from one that merely breaks it. A wooden pickaxe on
	// obsidian is effective and cannot harvest: it breaks the block more than
	// three times slower than its speed alone predicts, and drops nothing.
	harvestableDivisor   float32 = 30
	unharvestableDivisor float32 = 100
	// hastePerLevel is what one level of haste adds, as a fraction.
	hastePerLevel float32 = 0.2
	// submergedScale is the penalty for mining with your head in water. It is
	// 1.8.9's division by five, written as 26.1.2's attribute default so the
	// shared rule keeps one shape.
	submergedScale float32 = 0.2
	// airborneDivisor is the penalty for mining while not standing on
	// anything. Both games divide by five.
	airborneDivisor float32 = 5
)
