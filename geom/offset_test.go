package geom

import "testing"

// mover is a one-cube entity box sitting at the origin.
func mover() AABB {
	return AABB{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1, MaxY: 1, MaxZ: 1}
}

func TestClampXStopsAtTheBlockFace(t *testing.T) {
	// A block occupying x in [2,3]. Moving +5 may only travel 1.
	block := AABB{MinX: 2, MinY: 0, MinZ: 0, MaxX: 3, MaxY: 1, MaxZ: 1}

	if got := block.ClampX(mover(), 5); got != 1 {
		t.Fatalf("ClampX = %v, want 1", got)
	}
	if got := block.ClampX(mover(), 0.25); got != 0.25 {
		t.Fatalf("ClampX shortened a motion that does not reach: %v", got)
	}
	if got := block.ClampX(mover(), -5); got != -5 {
		t.Fatalf("ClampX clamped motion moving away from the block: %v", got)
	}
}

func TestClampXStopsAtTheBlockFaceMovingNegative(t *testing.T) {
	// A block occupying x in [-3,-2]. Moving -5 may only travel -2.
	block := AABB{MinX: -3, MinY: 0, MinZ: 0, MaxX: -2, MaxY: 1, MaxZ: 1}

	if got := block.ClampX(mover(), -5); got != -2 {
		t.Fatalf("ClampX = %v, want -2", got)
	}
}

func TestClampIgnoresBlocksThatDoNotOverlapTheOtherAxes(t *testing.T) {
	// Directly ahead in X, but one whole cell up in Y: no overlap, no clamp.
	block := AABB{MinX: 2, MinY: 5, MinZ: 0, MaxX: 3, MaxY: 6, MaxZ: 1}

	if got := block.ClampX(mover(), 5); got != 5 {
		t.Fatalf("ClampX = %v, want the motion unchanged", got)
	}
}

func TestClampTreatsTouchingAsNoOverlap(t *testing.T) {
	// The block's Y range starts exactly where the mover's ends. 1.8.9 uses
	// >= and <= on the far edges, so this is not an overlap and must not clamp.
	block := AABB{MinX: 2, MinY: 1, MinZ: 0, MaxX: 3, MaxY: 2, MaxZ: 1}

	if got := block.ClampX(mover(), 5); got != 5 {
		t.Fatalf("ClampX = %v, want the motion unchanged", got)
	}
}

func TestClampYAndClampZMirrorClampX(t *testing.T) {
	above := AABB{MinX: 0, MinY: 2, MinZ: 0, MaxX: 1, MaxY: 3, MaxZ: 1}
	if got := above.ClampY(mover(), 5); got != 1 {
		t.Fatalf("ClampY = %v, want 1", got)
	}

	ahead := AABB{MinX: 0, MinY: 0, MinZ: 2, MaxX: 1, MaxY: 1, MaxZ: 3}
	if got := ahead.ClampZ(mover(), 5); got != 1 {
		t.Fatalf("ClampZ = %v, want 1", got)
	}
}

func TestClampNeverReversesMotion(t *testing.T) {
	// A box already overlapping the mover must not push it backwards.
	overlapping := AABB{MinX: 0.5, MinY: 0, MinZ: 0, MaxX: 1.5, MaxY: 1, MaxZ: 1}

	if got := overlapping.ClampX(mover(), 2); got != 2 {
		t.Fatalf("ClampX = %v, want the motion unchanged", got)
	}
	if got := overlapping.ClampX(mover(), -2); got != -2 {
		t.Fatalf("ClampX = %v, want the motion unchanged", got)
	}
}
