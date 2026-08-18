package combat

// Strike is everything that decides how hard one hit lands.
//
// Every number here is supplied. Which enchantment gives what, and which
// effect scales which term, is version data, and this package holds none: a
// 1.8.9 strength effect multiplies the attribute and a 26.1.2 one adds to it,
// and both arrive here already resolved into Base or Strength.
type Strike struct {
	// Base is the weapon's attack-damage attribute.
	Base float64
	// Charge is the cooldown charge, 1 on a version with no cooldown.
	Charge float64
	// Sharpness is the added damage from the weapon's enchantment.
	Sharpness float64
	// Strength is the added damage from the effect.
	Strength float64
	// Weakness is the subtracted damage from the effect.
	Weakness float64
	// Critical is a falling, unmounted, sober attacker's bonus.
	Critical bool
	// Sprinting adds knockback, and on 26.1.2 only lands it at full charge.
	Sprinting bool
	// KnockbackLevel is the weapon's knockback enchantment level.
	KnockbackLevel int
}

// Damage returns the damage one strike deals, before armour.
//
// Armour is deliberately out of scope: it is a per-target reduction over
// attribute and enchantment data that neither version's generated set has been
// checked for, and folding an unverified reduction into a verified strike
// would make the whole number unverified.
//
// Transcribed from the deobfuscated jars minecraft-reference pins, on
// 2026-08-18:
//
//	1.8.9   EntityPlayer.attackTargetEntityWithCurrentItem: the attribute,
//	        times 1.5 on a critical, plus the enchantment modifier.
//	26.1.2  Player.attack: the attribute, times baseDamageScaleFactor
//	        (0.2F + charge² × 0.8F), times 1.5 on a critical, plus the
//	        enchantment modifier scaled by the charge.
//
// The two are one formula because 1.8.9's charge is always 1, at which the
// scale factor is exactly 1 in float32 — so the shared arithmetic reduces to
// the pre-1.9 formula with no branch anywhere. The arithmetic runs at float32
// width, as both games' does.
func Damage(s Strike) float64 {
	base := float32(s.Base) + float32(s.Strength) - float32(s.Weakness)
	if base < 0 {
		// Negative damage heals the target. The attribute system clamps the
		// value at zero in both versions, so a weakness larger than the base
		// lands a hit that does nothing.
		base = 0
	}

	charge := float32(s.Charge)
	base *= 0.2 + charge*charge*0.8
	if s.Critical {
		base *= 1.5
	}

	return float64(base + float32(s.Sharpness)*charge)
}
