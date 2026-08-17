package terrain

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

func TestPassableReportsClearFlatGround(t *testing.T) {
	query := Query{View: room(), Body: testBody}

	got, err := query.Passable(geom.BlockPos{X: 0, Y: 0, Z: 0})
	if err != nil {
		t.Fatalf("Passable returned an error: %v", err)
	}
	if got != Clear {
		t.Fatalf("Passable = %v, want Clear", got)
	}
}

// One solid block with a body's worth of room above it is something to climb,
// not something to route around. Folding this into Blocked makes a bot walk
// around every doorstep.
func TestPassableReportsAOneBlockRiseAsSteppable(t *testing.T) {
	blocks := room()
	blocks.Set(geom.BlockPos{X: 1, Y: 0, Z: 0}, geom.FullCube())
	query := Query{View: blocks, Body: testBody}

	got, err := query.Passable(geom.BlockPos{X: 1, Y: 0, Z: 0})
	if err != nil {
		t.Fatalf("Passable returned an error: %v", err)
	}
	if got != Steppable {
		t.Fatalf("Passable = %v, want Steppable", got)
	}
}

func TestPassableReportsATwoBlockWallAsBlocked(t *testing.T) {
	blocks := room()
	blocks.Set(geom.BlockPos{X: 1, Y: 0, Z: 0}, geom.FullCube())
	blocks.Set(geom.BlockPos{X: 1, Y: 1, Z: 0}, geom.FullCube())
	query := Query{View: blocks, Body: testBody}

	got, err := query.Passable(geom.BlockPos{X: 1, Y: 0, Z: 0})
	if err != nil {
		t.Fatalf("Passable returned an error: %v", err)
	}
	if got != Blocked {
		t.Fatalf("Passable = %v, want Blocked", got)
	}
}

// A hole is not somewhere to stand. It is also not a wall, and Task 7's Fall
// edge is what crosses it; Passable's job is only to say it is not Clear.
func TestPassableReportsAHoleAsBlocked(t *testing.T) {
	blocks := room()
	blocks.SetAir(geom.BlockPos{X: 1, Y: -1, Z: 0})
	query := Query{View: blocks, Body: testBody}

	got, err := query.Passable(geom.BlockPos{X: 1, Y: 0, Z: 0})
	if err != nil {
		t.Fatalf("Passable returned an error: %v", err)
	}
	if got != Blocked {
		t.Fatalf("Passable = %v, want Blocked", got)
	}
}

// An unloaded cell is refused, never guessed. A bot that read unknown as a
// wall would stop at the edge of its own render distance, and one that read it
// as air would walk into a wall it could not see.
func TestPassableReportsUnknownForAnUndescribedCell(t *testing.T) {
	blocks := room()
	blocks.Forget(geom.BlockPos{X: 1, Y: 0, Z: 0})
	query := Query{View: blocks, Body: testBody}

	got, err := query.Passable(geom.BlockPos{X: 1, Y: 0, Z: 0})
	if err != nil {
		t.Fatalf("Passable returned an error: %v", err)
	}
	if got != Unknown {
		t.Fatalf("Passable = %v, want Unknown", got)
	}
}

// A one-block body fits where a two-block body does not. The body is a
// parameter for exactly this reason.
func TestPassableAcceptsAOneBlockBody(t *testing.T) {
	blocks := room()
	blocks.Set(geom.BlockPos{X: 1, Y: 1, Z: 0}, geom.FullCube())
	small := Query{View: blocks, Body: Body{HalfWidth: 0.3, Height: 0.9, StepHeight: 0.6}}
	tall := Query{View: blocks, Body: testBody}

	got, err := small.Passable(geom.BlockPos{X: 1, Y: 0, Z: 0})
	if err != nil {
		t.Fatalf("Passable returned an error: %v", err)
	}
	if got != Clear {
		t.Fatalf("small body Passable = %v, want Clear", got)
	}

	got, err = tall.Passable(geom.BlockPos{X: 1, Y: 0, Z: 0})
	if err != nil {
		t.Fatalf("Passable returned an error: %v", err)
	}
	if got != Blocked {
		t.Fatalf("tall body Passable = %v, want Blocked", got)
	}
}

func TestPassabilityStringNamesEveryValue(t *testing.T) {
	cases := map[Passability]string{
		Unknown:   "unknown",
		Clear:     "clear",
		Steppable: "steppable",
		Blocked:   "blocked",
	}
	for value, want := range cases {
		if got := value.String(); got != want {
			t.Fatalf("Passability(%d).String() = %q, want %q", value, got, want)
		}
	}
}

// world.Blocks is the view under test throughout. Asserting the interface here
// makes a change to world.View fail in this package rather than mysteriously
// in navigation.
var _ world.View = (*world.Blocks)(nil)
