package movement

import "testing"

func TestInputNamesItsKind(t *testing.T) {
	if got, want := (Input{}).CommandKind(), "movement.input"; got != want {
		t.Fatalf("CommandKind = %q, want %q", got, want)
	}
}

func TestLocomotionIsComparable(t *testing.T) {
	// A change set carries this by value and the digest encodes it, both of which
	// need every field to stay a scalar. A slice or a map added here would break
	// that silently, so the check is explicit.
	first := Locomotion{JumpTicks: 10, Yaw: 90, MoveSpeed: 0.1, Sprinting: true}
	second := first
	if first != second {
		t.Fatal("a copy of a locomotion state does not equal its original")
	}
}

func TestBodiesReturnsWhatItWasGiven(t *testing.T) {
	bodies := NewBodies()
	want := Locomotion{JumpTicks: 3, Yaw: 45, MoveSpeed: 0.1, JumpFactor: 0.02}
	bodies.Set(1, want)

	got, ok := bodies.Locomotion(1)
	if !ok {
		t.Fatal("Locomotion reported the body missing")
	}
	if got != want {
		t.Fatalf("Locomotion = %+v, want %+v", got, want)
	}
}

func TestBodiesReportsAMissingEntity(t *testing.T) {
	// Absent is not zero: a zero state has no movement speed, so a body given one
	// by accident would stand still forever and look like a physics bug.
	if _, ok := NewBodies().Locomotion(1); ok {
		t.Fatal("Locomotion found a body that was never set")
	}
}

func TestRemoveDropsTheState(t *testing.T) {
	bodies := NewBodies()
	bodies.Set(2, Locomotion{Yaw: 1})
	bodies.Remove(2)

	if _, ok := bodies.Locomotion(2); ok {
		t.Fatal("Locomotion found a removed body")
	}
}

func TestCloneDoesNotFollowItsOrigin(t *testing.T) {
	bodies := NewBodies()
	original := Locomotion{JumpTicks: 5, Yaw: 30}
	bodies.Set(1, original)

	clone := bodies.Clone()
	bodies.Set(1, Locomotion{JumpTicks: 0})
	bodies.Set(2, original)

	got, ok := clone.Locomotion(1)
	if !ok || got != original {
		t.Fatalf("the clone followed its origin: (%+v, %v)", got, ok)
	}
	if _, ok := clone.Locomotion(2); ok {
		t.Fatal("the clone gained a body its origin added afterwards")
	}
}

var _ LocomotionView = (*Bodies)(nil)
