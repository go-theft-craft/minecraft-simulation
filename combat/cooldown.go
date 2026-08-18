package combat

// Cooldown is the attack-cooldown rule, or the absence of one.
//
// Absence is a value, not a nil: a profile that returned nil would force every
// caller to nil-check and one of them would forget. A profile with no cooldown
// returns one whose Charge is always full, and says so in Reason.
type Cooldown interface {
	// Charge is how filled the cooldown is, in [0,1]. A version without a
	// cooldown returns 1 always.
	Charge(ticksSinceAttack int, attackSpeed float64) float64
	// Present reports whether this version has the mechanic at all.
	Present() bool
	// Reason explains an absence, and is empty when Present is true. It is
	// required rather than optional because the conformance report prints it,
	// and "absent" with no reason is indistinguishable from "not checked".
	Reason() string
}

// NoCooldown is the rule for a version that has none.
//
// The reason is the caller's, because the caller is the version that knows why
// — and it must not be empty, or the report the reason exists for prints
// nothing.
func NoCooldown(reason string) Cooldown { return noCooldown{reason: reason} }

type noCooldown struct{ reason string }

// Charge implements Cooldown: always full, so shared damage code needs no
// version branch.
func (noCooldown) Charge(int, float64) float64 { return 1 }

// Present implements Cooldown.
func (noCooldown) Present() bool { return false }

// Reason implements Cooldown.
func (n noCooldown) Reason() string { return n.reason }

// TickedCooldown is the rule for a version whose swings charge over ticks.
//
// Transcribed from the deobfuscated 26.1.2 server jar minecraft-reference
// pins, on 2026-08-18:
//
//	Player.getCurrentItemAttackStrengthDelay:
//	    (float)(1.0 / attackSpeed * 20.0)
//	Player.getAttackStrengthScale(0.5F):
//	    Mth.clamp((attackStrengthTicker + 0.5F) / delay, 0.0F, 1.0F)
//
// The ticker counts ticks since the last swing, incremented once per tick and
// zeroed by an attack, so the tick after a swing charges to (1 + 0.5)/delay.
// The arithmetic is float32, as the game's is: the delay is a Java double
// narrowed to a float, and the clamp runs at float width. A float64 curve
// would agree at 0 and 1 and drift in between, which is exactly where the
// corpus samples.
func TickedCooldown() Cooldown { return tickedCooldown{} }

type tickedCooldown struct{}

// Charge implements Cooldown.
func (tickedCooldown) Charge(ticksSinceAttack int, attackSpeed float64) float64 {
	if attackSpeed <= 0 {
		// A non-positive attack speed divides by zero. No version's attribute
		// permits one — 26.1.2 clamps the attribute to (0, 1024] — so this is
		// a caller's bug, answered with an empty charge rather than an Inf
		// that would spread through the damage arithmetic.
		return 0
	}

	delay := float32(1.0 / attackSpeed * 20.0)
	charge := (float32(ticksSinceAttack) + 0.5) / delay
	if charge < 0 {
		return 0
	}
	if charge > 1 {
		return 1
	}

	return float64(charge)
}

// Present implements Cooldown.
func (tickedCooldown) Present() bool { return true }

// Reason implements Cooldown.
func (tickedCooldown) Reason() string { return "" }
