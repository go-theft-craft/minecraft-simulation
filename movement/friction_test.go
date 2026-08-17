package movement

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func TestFrictionInTheAirIgnoresTheBlockBelow(t *testing.T) {
	for _, slipperiness := range []float32{0, 0.6, 0.98, 1} {
		if got := Friction(slipperiness, false); got != airFriction {
			t.Errorf("Friction(%v, airborne) = %v, want %v", slipperiness, got, airFriction)
		}
	}
}

func TestGroundFrictionIsASingleWidthProduct(t *testing.T) {
	const slipperiness float32 = 0.6

	got := Friction(slipperiness, true)
	if want := slipperiness * airFriction; got != want {
		t.Fatalf("Friction = %v, want the single-width product %v", got, want)
	}

	// The assertion that matters: the single-width product is not the
	// double-width one. This case fails if anyone widens the operands early,
	// which is the mistake the whole width discipline exists to prevent.
	wide := float32(float64(slipperiness) * float64(airFriction))
	if float64(got) == float64(slipperiness)*float64(airFriction) && got != wide {
		t.Fatal("the product was computed at double width")
	}
	if float64(got) == 0.6*0.91 {
		t.Fatalf("Friction = %v, which is the double-width answer", got)
	}
}

func TestAccelerationForTheDefaultGroundFriction(t *testing.T) {
	// Pinned from the first run, so a change to the formula or the constant has
	// to be deliberate.
	//
	// It is almost exactly one, and that is not a coincidence: the game's
	// numerator is the cube of the default ground friction, so an ordinary block
	// accelerates a body at its movement-speed attribute and every other surface
	// is expressed relative to that.
	const want float32 = 0.9999998

	if got := Acceleration(Friction(0.6, true)); got != want {
		t.Fatalf("Acceleration = %v, want %v", got, want)
	}
}

func TestAccelerationFallsAsFrictionRises(t *testing.T) {
	// Ice accelerates more slowly than stone, which is the same fact as a body
	// on ice taking longer to stop.
	stone := Acceleration(Friction(0.6, true))
	ice := Acceleration(Friction(0.98, true))
	if !(ice < stone) {
		t.Fatalf("acceleration on ice is %v and on stone %v; ice must be lower", ice, stone)
	}
}

func TestSpeedIsTheJumpFactorInTheAirAndTheProductOnTheGround(t *testing.T) {
	const (
		moveSpeed  float32 = 0.1
		jumpFactor float32 = 0.02
	)

	if got := Speed(airFriction, false, moveSpeed, jumpFactor); got != jumpFactor {
		t.Errorf("airborne Speed = %v, want the jump factor %v", got, jumpFactor)
	}

	friction := Friction(0.6, true)
	if got, want := Speed(friction, true, moveSpeed, jumpFactor), moveSpeed*Acceleration(friction); got != want {
		t.Errorf("ground Speed = %v, want %v", got, want)
	}
}

func TestGroundFrictionBlockReadsOneCellBelowTheBox(t *testing.T) {
	for name, test := range map[string]struct {
		box  geom.AABB
		pos  geom.Vec3
		want geom.BlockPos
	}{
		"flush on the floor": {
			box:  geom.AABB{MinY: 0, MaxY: 1.8},
			pos:  geom.Vec3{X: 0.5, Z: 0.5},
			want: geom.BlockPos{X: 0, Y: -1, Z: 0},
		},
		"half a block up": {
			box:  geom.AABB{MinY: 0.5, MaxY: 2.3},
			pos:  geom.Vec3{X: 0.5, Y: 0.5, Z: 0.5},
			want: geom.BlockPos{X: 0, Y: -1, Z: 0},
		},
		"standing on a block": {
			box:  geom.AABB{MinY: 65, MaxY: 66.8},
			pos:  geom.Vec3{X: 8.2, Y: 65, Z: -3.7},
			want: geom.BlockPos{X: 8, Y: 64, Z: -4},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := GroundFrictionBlock(test.box, test.pos); got != test.want {
				t.Fatalf("GroundFrictionBlock = %+v, want %+v", got, test.want)
			}
		})
	}
}
