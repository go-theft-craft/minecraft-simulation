package combat_test

import (
	"math"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/combat"
	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func TestKnockbackIsHorizontalAwayFromTheAttacker(t *testing.T) {
	t.Parallel()

	from := geom.Vec3{X: 0, Y: 64, Z: 0}
	to := geom.Vec3{X: 2, Y: 64, Z: 0}

	got := combat.Knockback(from, to, combat.Strike{Charge: 1}, geom.Vec3{})
	if got.X <= 0 {
		t.Fatalf("knockback X = %v, want positive: away from the attacker", got.X)
	}
	if got.Y <= 0 {
		t.Fatalf("knockback Y = %v, want positive: vanilla lifts the target", got.Y)
	}
	if got.Z != 0 {
		t.Fatalf("knockback Z = %v for an attack along X, want 0", got.Z)
	}
}

func TestKnockbackHalvesTheMotionItReplaces(t *testing.T) {
	t.Parallel()

	// Vanilla halves the target's motion before adding the impulse, which is
	// why a fleeing target is not launched by its own speed.
	from, to := geom.Vec3{}, geom.Vec3{X: 2}
	still := combat.Knockback(from, to, combat.Strike{Charge: 1}, geom.Vec3{})
	fleeing := combat.Knockback(from, to, combat.Strike{Charge: 1}, geom.Vec3{X: 1})

	if want := still.X + 0.5; fleeing.X != want {
		t.Fatalf("a target moving at 1 was left with %v, want %v: half its "+
			"motion plus the impulse", fleeing.X, want)
	}
}

func TestKnockbackLiftIsCapped(t *testing.T) {
	t.Parallel()

	// A target already moving up keeps at most 0.4 of lift, from the cap in
	// both games' knockback methods.
	from, to := geom.Vec3{}, geom.Vec3{X: 2}
	got := combat.Knockback(from, to, combat.Strike{Charge: 1}, geom.Vec3{Y: 3})
	if got.Y != 0.4 {
		t.Fatalf("knockback Y = %v for a rising target, want the 0.4 cap", got.Y)
	}
}

func TestKnockbackAtZeroDistanceIsNotNaN(t *testing.T) {
	t.Parallel()

	// Two entities at exactly the same position is rare and legal.
	// Normalising a zero vector produces NaN, and M8.3's kernel rejects a
	// result containing one — so this would surface as ErrNaNInResult rather
	// than as a wrong knockback, which is worse to diagnose.
	at := geom.Vec3{X: 1, Y: 64, Z: 1}
	got := combat.Knockback(at, at, combat.Strike{Charge: 1}, geom.Vec3{})
	if math.IsNaN(got.X) || math.IsNaN(got.Y) || math.IsNaN(got.Z) {
		t.Fatalf("knockback = %+v at zero distance", got)
	}
}

func TestSprintAndKnockbackEnchantmentBothIncreaseIt(t *testing.T) {
	t.Parallel()

	from, to := geom.Vec3{}, geom.Vec3{X: 2}
	base := combat.Knockback(from, to, combat.Strike{Charge: 1}, geom.Vec3{})
	sprinting := combat.Knockback(from, to, combat.Strike{Charge: 1, Sprinting: true}, geom.Vec3{})
	enchanted := combat.Knockback(from, to, combat.Strike{Charge: 1, KnockbackLevel: 2}, geom.Vec3{})

	if sprinting.X <= base.X {
		t.Fatalf("sprinting knockback %v did not exceed base %v", sprinting.X, base.X)
	}
	if enchanted.X <= sprinting.X {
		t.Fatalf("knockback II's %v did not exceed a sprint's %v", enchanted.X, sprinting.X)
	}
	if base.Y >= sprinting.Y {
		t.Fatalf("the bonus did not lift: base Y %v, sprinting Y %v", base.Y, sprinting.Y)
	}
}
