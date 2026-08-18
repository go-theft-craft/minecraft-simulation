// Package combat computes what one swing does: whether it lands, how hard,
// and where it sends the target.
//
// The arithmetic is shared between versions and the numbers are not. Reach
// distances, the cooldown rule, and the strike modifiers are all supplied by a
// profile through Fighter, because this is the stage where the two versions
// diverge most — 1.8.9 has no attack cooldown, and 26.1.2's damage depends on
// one. Nothing here reads generated data or the world: this package takes
// numbers and returns numbers, which is what makes it testable as a table
// rather than as a server.
package combat

import "github.com/go-theft-craft/minecraft-simulation/geom"

// Reach is how far an entity can act, in blocks.
//
// It is a value supplied by the profile rather than a constant here because
// the two versions differ and because survival and creative differ within
// each. A single number in this package would be wrong three ways out of four.
type Reach struct {
	// Attack is the maximum distance to a target's collision box.
	Attack float64
	// Interact is the maximum distance to a block face, which is not the same
	// number in either version.
	Interact float64
}

// InReach reports whether target's box is within r of eye.
//
// The distance is to the nearest point of the box, not to its centre. Using
// the centre makes a tall entity unhittable at its feet and a wide one
// hittable from outside its own edge, and both look like reach bugs to a
// caller. That is also how each game measures: 1.8.9's client intercepts a
// ray from the eye against the target's box and compares the hit point's
// distance (EntityRenderer.getMouseOver), and 26.1.2 clips the eye-to-box
// distance against the interaction-range attribute
// (Player.canInteractWithEntity).
func InReach(eye geom.Vec3, target geom.AABB, r float64) bool {
	nearest := target.Nearest(eye)
	delta := nearest.Sub(eye)

	return delta.X*delta.X+delta.Y*delta.Y+delta.Z*delta.Z <= r*r
}

// Fighter is a profile that can answer combat questions.
//
// It is optional for the same reason mining.Classifier is: nothing outside the
// attack phase reads it, and a profile assembled in a movement test has no
// combat numbers to answer with. A caller that needs it asserts for it and
// reports a profile that cannot answer.
//
// It is declared here rather than in sim because combat imports sim, so a sim
// interface returning a combat.Reach would be an import cycle.
type Fighter interface {
	// Reach is this version's survival-mode reach. It is the number the attack
	// phase validates against, because the scenarios that gate this stage run
	// in survival and because it is the stricter mode.
	Reach() Reach
	// CreativeReach is this version's creative-mode reach. The kernel does not
	// model game modes, so nothing in the phase reads it; it is declared so a
	// client that knows its own mode has both numbers from one source.
	CreativeReach() Reach
	// Cooldown is this version's attack-cooldown rule, or its recorded absence.
	Cooldown() Cooldown
	// BareHandDamage is the player's attack-damage attribute with no weapon,
	// no enchantment, and no effect. The tick models no inventory — that is
	// M9.7's — so the phase strikes bare-handed.
	BareHandDamage() float64
	// AttackSpeed is the player's attack-speed attribute, which the cooldown's
	// charge divides by. A version with no such attribute returns zero, and
	// its absent cooldown never reads it.
	AttackSpeed() float64
	// EyeHeight is a standing player's eye height above the bottom of its box,
	// which is where every reach measurement starts.
	EyeHeight() float64
}
