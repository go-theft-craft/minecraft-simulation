package movement

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

const jumpUpwards float32 = 0.42

func TestCountdownStopsAtZero(t *testing.T) {
	state := Locomotion{JumpTicks: 2}
	for range 5 {
		state = Countdown(state)
	}
	if state.JumpTicks != 0 {
		t.Fatalf("JumpTicks = %d after counting past zero, want 0", state.JumpTicks)
	}
}

func TestJumpFiresOnlyWhenEveryConditionHolds(t *testing.T) {
	table := gameTable(t)

	for name, test := range map[string]struct {
		state    Locomotion
		onGround bool
		want     bool
	}{
		"held, grounded, counter clear": {Locomotion{Jumping: true}, true, true},
		"not held":                      {Locomotion{}, true, false},
		"airborne":                      {Locomotion{Jumping: true}, false, false},
		"counter still running":         {Locomotion{Jumping: true, JumpTicks: 3}, true, false},
	} {
		t.Run(name, func(t *testing.T) {
			_, motion, fired := Jump(table, test.state, geom.Vec3{}, test.onGround, jumpUpwards)
			if fired != test.want {
				t.Fatalf("Jump fired = %v, want %v", fired, test.want)
			}
			if test.want && motion.Y != float64(jumpUpwards) {
				t.Fatalf("Y = %v, want the impulse %v", motion.Y, jumpUpwards)
			}
			if !test.want && motion.Y != 0 {
				t.Fatalf("a jump that did not fire changed Y to %v", motion.Y)
			}
		})
	}
}

func TestAJumpSetsTheDelay(t *testing.T) {
	table := gameTable(t)

	state, _, fired := Jump(table, Locomotion{Jumping: true}, geom.Vec3{}, true, jumpUpwards)
	if !fired {
		t.Fatal("the jump did not fire")
	}
	if state.JumpTicks != jumpDelay {
		t.Fatalf("JumpTicks = %d after a jump, want %d", state.JumpTicks, jumpDelay)
	}
}

func TestReleasingTheJumpInputClearsTheDelay(t *testing.T) {
	// This is what makes tapping the key jump at once: the counter is zeroed the
	// moment the input is released rather than being left to run down.
	table := gameTable(t)

	state, _, fired := Jump(table, Locomotion{JumpTicks: 7}, geom.Vec3{}, true, jumpUpwards)
	if fired {
		t.Fatal("a jump fired with the input released")
	}
	if state.JumpTicks != 0 {
		t.Fatalf("JumpTicks = %d after release, want 0", state.JumpTicks)
	}
}

func TestASprintingJumpAddsTheHorizontalImpulse(t *testing.T) {
	table := gameTable(t)

	walking := Locomotion{Jumping: true, Yaw: 0}
	sprinting := Locomotion{Jumping: true, Sprinting: true, Yaw: 0}

	_, walkMotion, _ := Jump(table, walking, geom.Vec3{}, true, jumpUpwards)
	_, sprintMotion, _ := Jump(table, sprinting, geom.Vec3{}, true, jumpUpwards)

	if walkMotion.X != 0 || walkMotion.Z != 0 {
		t.Fatalf("a walking jump moved horizontally: %+v", walkMotion)
	}
	if sprintMotion.Z <= 0 {
		t.Fatalf("a sprinting jump at zero yaw moved Z by %v, want positive", sprintMotion.Z)
	}
	if sprintMotion.Y != walkMotion.Y {
		t.Fatalf("the sprint impulse changed the vertical motion: %v vs %v",
			sprintMotion.Y, walkMotion.Y)
	}
}

func TestTheSprintImpulseFollowsTheFacing(t *testing.T) {
	table := gameTable(t)

	// Facing a quarter turn away puts the impulse on the other axis, which is
	// what reading the table with the body's own yaw buys.
	state := Locomotion{Jumping: true, Sprinting: true, Yaw: 90}
	_, motion, _ := Jump(table, state, geom.Vec3{}, true, jumpUpwards)

	if motion.X >= 0 {
		t.Fatalf("at yaw 90 the impulse moved X by %v, want negative", motion.X)
	}
}
