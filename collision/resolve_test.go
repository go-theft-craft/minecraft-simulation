package collision

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// player returns a box roughly the size of a 1.8.9 player, standing on the
// floor built by floorAt.
func player() geom.AABB {
	return geom.AABB{MinX: -0.3, MinY: 0, MinZ: -0.3, MaxX: 0.3, MaxY: 1.8, MaxZ: 0.3}
}

// worldWithFloor returns a view whose ground is solid across a wide area up to
// y=-1 and whose remaining cells up to y=4 are air.
//
// The ground is described down to y=-8 rather than only at y=-1 because a
// sweep is gathered from the body stretched along the whole motion: a fall of
// five blocks reaches cells well below the surface it lands on, and a view
// that leaves them undescribed makes Resolve report an incomplete tick instead
// of a landing.
func worldWithFloor() *world.Blocks {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -8, Y: -8, Z: -8}, geom.BlockPos{X: 8, Y: -1, Z: 8}, geom.FullCube())
	blocks.Fill(geom.BlockPos{X: -8, Y: 0, Z: -8}, geom.BlockPos{X: 8, Y: 4, Z: 8}, geom.EmptyShape())

	return blocks
}

func TestResolveAppliesFreeMotionUntouched(t *testing.T) {
	got, err := Resolve(worldWithFloor(), Move{
		Body:     player().Offset(geom.Vec3{Y: 1}),
		Motion:   geom.Vec3{X: 0.2, Y: 0.1, Z: -0.3},
		OnGround: false,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Applied != (geom.Vec3{X: 0.2, Y: 0.1, Z: -0.3}) {
		t.Fatalf("Applied = %+v, want the motion unchanged", got.Applied)
	}
	if got.CollidedX || got.CollidedY || got.CollidedZ || got.OnGround {
		t.Fatalf("free motion reported a collision: %+v", got)
	}
}

func TestResolveStopsOnTheFloorAndSetsOnGround(t *testing.T) {
	got, err := Resolve(worldWithFloor(), Move{
		Body:   player(),
		Motion: geom.Vec3{Y: -5},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Applied.Y != 0 {
		t.Fatalf("Applied.Y = %v, want 0: the body already rests on the floor", got.Applied.Y)
	}
	if !got.CollidedY || !got.OnGround {
		t.Fatalf("landing did not set the vertical flags: %+v", got)
	}
}

func TestResolveFallsToTheFloorExactly(t *testing.T) {
	got, err := Resolve(worldWithFloor(), Move{
		Body:   player().Offset(geom.Vec3{Y: 2}),
		Motion: geom.Vec3{Y: -5},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Applied.Y != -2 {
		t.Fatalf("Applied.Y = %v, want -2", got.Applied.Y)
	}
	if got.Body.MinY != 0 {
		t.Fatalf("Body.MinY = %v, want the body flush with the floor", got.Body.MinY)
	}
	if !got.OnGround {
		t.Error("OnGround is false after landing")
	}
}

func TestResolveStopsAtAWall(t *testing.T) {
	blocks := worldWithFloor()
	blocks.Fill(geom.BlockPos{X: 2, Y: 0, Z: -8}, geom.BlockPos{X: 2, Y: 4, Z: 8}, geom.FullCube())

	got, err := Resolve(blocks, Move{
		Body:     player(),
		Motion:   geom.Vec3{X: 5},
		OnGround: true,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Applied.X != 1.7 {
		t.Fatalf("Applied.X = %v, want 1.7", got.Applied.X)
	}
	if !got.CollidedX || !got.CollidedHorizontally() {
		t.Fatalf("hitting a wall did not set the horizontal flags: %+v", got)
	}
	if got.CollidedZ {
		t.Error("CollidedZ set for motion that had no Z component")
	}
}

func TestResolveMovesYBeforeXSoTheXPassSeesTheMovedBody(t *testing.T) {
	// A one-block ledge occupying x=1, y=0. The body starts above it and
	// descends while moving forward.
	//
	// Resolving Y first drops the body to y=0, where the ledge blocks X and
	// only 0.7 of the motion applies. Resolving X first would test the body at
	// its old height, where the ledge is below it and nothing blocks, applying
	// the full 1. The two orders give different answers, which is what makes
	// this a real ordering test.
	blocks := worldWithFloor()
	blocks.Fill(geom.BlockPos{X: 1, Y: 0, Z: -1}, geom.BlockPos{X: 1, Y: 0, Z: 0}, geom.FullCube())

	got, err := Resolve(blocks, Move{
		Body:   player().Offset(geom.Vec3{Y: 1.2}),
		Motion: geom.Vec3{X: 1, Y: -1.2},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Applied.Y != -1.2 {
		t.Fatalf("Applied.Y = %v, want the full -1.2", got.Applied.Y)
	}
	if got.Applied.X != 0.7 {
		t.Fatalf("Applied.X = %v, want 0.7; 1 means X resolved before Y", got.Applied.X)
	}
	if !got.CollidedX {
		t.Error("CollidedX not set after the ledge blocked the move")
	}
}

func TestResolveReportsUnknownAndDoesNotMove(t *testing.T) {
	blocks := world.NewBlocks()
	// Nothing described at all: every cell is unknown.
	got, err := Resolve(blocks, Move{
		Body:   player(),
		Motion: geom.Vec3{X: 1},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(got.Unknown) == 0 {
		t.Fatal("Resolve did not report unknown cells")
	}
	if !got.Applied.IsZero() {
		t.Fatalf("Applied = %+v, want no motion on an incomplete view", got.Applied)
	}
	if got.Body != player() {
		t.Fatalf("Body = %+v, want the body unmoved", got.Body)
	}
}

func TestResolveZeroMotionIsAFixedPoint(t *testing.T) {
	body := player()
	got, err := Resolve(worldWithFloor(), Move{Body: body, OnGround: true})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Body != body {
		t.Fatalf("Body = %+v, want %+v", got.Body, body)
	}
	if !got.Applied.IsZero() {
		t.Fatalf("Applied = %+v, want zero", got.Applied)
	}
}
