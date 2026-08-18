package combat_test

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/combat"
)

func TestAFullyChargedStrikeBeatsAnUnchargedOne(t *testing.T) {
	t.Parallel()

	full := combat.Damage(combat.Strike{Base: 7, Charge: 1})
	weak := combat.Damage(combat.Strike{Base: 7, Charge: 0.2})
	if full <= weak {
		t.Fatalf("charge 1 dealt %v and charge 0.2 dealt %v", full, weak)
	}
}

func TestChargeDoesNotAffectAVersionWithoutACooldown(t *testing.T) {
	t.Parallel()

	// The 1.8.9 path: charge is always 1, at which the scale factor is
	// exactly 1 in float32, so the shared formula reduces to the pre-1.9 one
	// with no branch anywhere.
	if got, want := combat.Damage(combat.Strike{Base: 7, Charge: 1}), 7.0; got != want {
		t.Fatalf("Damage = %v with no modifiers, want the base %v", got, want)
	}
}

func TestACriticalMultipliesByHalfAgain(t *testing.T) {
	t.Parallel()

	plain := combat.Damage(combat.Strike{Base: 6, Charge: 1})
	critical := combat.Damage(combat.Strike{Base: 6, Charge: 1, Critical: true})
	if critical != plain*1.5 {
		t.Fatalf("a critical dealt %v over a plain %v, want half again", critical, plain)
	}
}

func TestSharpnessIsAddedAfterTheCritical(t *testing.T) {
	t.Parallel()

	// Both games add the enchantment modifier after the critical multiplier,
	// so sharpness is never itself multiplied.
	got := combat.Damage(combat.Strike{Base: 6, Charge: 1, Sharpness: 2, Critical: true})
	if want := 6*1.5 + 2.0; got != want {
		t.Fatalf("Damage = %v, want %v: sharpness added after the critical", got, want)
	}
}

func TestWeaknessCanReduceDamageToZeroButNotBelow(t *testing.T) {
	t.Parallel()

	// Negative damage heals the target, which is a real bug shape and not a
	// theoretical one.
	got := combat.Damage(combat.Strike{Base: 1, Charge: 1, Weakness: 100})
	if got < 0 {
		t.Fatalf("Damage = %v; negative damage heals the target", got)
	}
}
