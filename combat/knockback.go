package combat

import (
	"math"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// knockStrength is the base impulse every landed hit applies, in blocks per
// tick. Both jars carry the same literal: 1.8.9's EntityLivingBase.knockBack
// hardcodes 0.4F, and 26.1.2's LivingEntity.hurtServer calls knockback(0.4F,
// …). It is a widened Java float, not 0.4 — the corpus comparison is exact,
// and the doubles differ in their low bits.
var knockStrength = float64(float32(0.4))

// verticalCap is the most a base knockback may lift, from the same two
// methods, at the same width.
var verticalCap = float64(float32(0.4))

// extraLift is the vertical push the sprint-and-enchantment bonus adds, from
// EntityPlayer.attackTargetEntityWithCurrentItem (1.8.9) and Player.push via
// causeExtraKnockback (26.1.2). This one is a Java double literal.
const extraLift = 0.1

// extraScale halves the bonus knockback's horizontal strength, from the same
// two call sites: a widened Java float again.
var extraScale = float64(float32(0.5))

// Knockback returns the motion the target is left with after one strike.
//
// It is an impulse and not a position, because a knockback that writes a
// position skips collision and puts entities through walls. The returned
// vector replaces the target's motion, and the next movement tick resolves it
// through the swept AABB path M8.2 already proved bit-identical to a real
// server.
//
// Transcribed from the deobfuscated jars minecraft-reference pins, on
// 2026-08-18:
//
//	1.8.9   EntityLivingBase.knockBack: halve the motion, push 0.4 directly
//	        away from the attacker in the horizontal plane, lift by 0.4
//	        capped at 0.4. EntityPlayer.attackTargetEntityWithCurrentItem
//	        then adds (knockback level + 1 if sprinting) × 0.5 along the
//	        attacker's yaw, plus 0.1 up.
//	26.1.2  LivingEntity.knockback is the same base impulse, and
//	        Player.causeExtraKnockback the same bonus, with two recorded
//	        differences: the lift applies only to a target on the ground, and
//	        a sprint bonus lands only at full charge. Both are the phase's to
//	        decide — this function is handed the strike it should apply.
//
// Two deliberate deviations from the transcription, both stated rather than
// silent:
//
//   - The bonus is pushed away from the attacker's position rather than along
//     the attacker's yaw. The kernel validates that the target is in reach of
//     the attacker's eye, so the two directions agree to within the target's
//     own box; carrying a yaw here would add a parameter every caller derives
//     from the same two positions.
//   - At zero horizontal distance both games nudge the direction by
//     Math.random until it is not zero. A pure function cannot, so it applies
//     the vertical part alone — deterministic, and never NaN, which the
//     kernel would reject as ErrNaNInResult.
func Knockback(from, to geom.Vec3, s Strike, base geom.Vec3) geom.Vec3 {
	deltaX, deltaZ := to.X-from.X, to.Z-from.Z
	// MathHelper.sqrt_double returns a Java float, and the divisions below
	// widen it back — so the distance is a float32 at double width, and using
	// a double sqrt here would disagree with the corpus in the last bits.
	distance := float64(float32(math.Sqrt(deltaX*deltaX + deltaZ*deltaZ)))

	out := geom.Vec3{X: base.X / 2, Y: base.Y/2 + verticalCap, Z: base.Z / 2}
	if out.Y > verticalCap {
		out.Y = verticalCap
	}

	// The same guard both games use before dividing: 1.8.9 loops while
	// dx² + dz² < 1.0E-4, 26.1.2 while < 1.0E-5F.
	aligned := distance*distance >= 1e-4
	if aligned {
		out.X += deltaX / distance * knockStrength
		out.Z += deltaZ / distance * knockStrength
	}

	bonus := float64(s.KnockbackLevel)
	if s.Sprinting {
		bonus++
	}
	if bonus > 0 {
		if aligned {
			out.X += deltaX / distance * bonus * extraScale
			out.Z += deltaZ / distance * bonus * extraScale
		}
		out.Y += extraLift
	}

	return out
}
