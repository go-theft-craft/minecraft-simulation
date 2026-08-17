package scene_test

import (
	"errors"
	"strings"
	"testing"

	gen "github.com/go-theft-craft/minecraft-protocol/generated/java/v1_8"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	v1_8 "github.com/go-theft-craft/minecraft-simulation/profile/java/v1_8"
	"github.com/go-theft-craft/minecraft-simulation/scene"
	"github.com/go-theft-craft/minecraft-simulation/sim"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

func profile(t *testing.T) sim.Profile {
	t.Helper()

	set, err := gen.Data()
	if err != nil {
		t.Fatalf("load the 1.8.9 data set: %v", err)
	}
	built, err := v1_8.New(set)
	if err != nil {
		t.Fatalf("build the 1.8.9 profile: %v", err)
	}

	return built
}

// collect describes a world into a map, which is enough to check what was
// written without depending on a store.
func collect(t *testing.T, built sim.Profile, described scene.World) map[geom.BlockPos]world.BlockRef {
	t.Helper()

	written := make(map[geom.BlockPos]world.BlockRef)
	if err := described.Describe(built, func(pos geom.BlockPos, ref world.BlockRef) error {
		written[pos] = ref

		return nil
	}); err != nil {
		t.Fatalf("Describe: %v", err)
	}

	return written
}

func TestTheRegionIsFilledAndTheBlocksWinOverIt(t *testing.T) {
	built := profile(t)
	to := geom.BlockPos{X: 1, Y: 0, Z: 1}

	written := collect(t, built, scene.World{
		Min:  geom.BlockPos{X: -1, Y: -1, Z: -1},
		Max:  geom.BlockPos{X: 1, Y: 2, Z: 1},
		Fill: "air",
		Blocks: []scene.Block{
			{Pos: geom.BlockPos{X: -1, Y: 0, Z: -1}, To: &to, Name: "stone"},
			{Pos: geom.BlockPos{X: 0, Y: 0, Z: 0}, Name: "ice"},
		},
	})

	if got, want := len(written), 3*4*3; got != want {
		t.Fatalf("described %d cells, want the whole region, %d", got, want)
	}

	air, _ := v1_8.Ref(built, "air")
	stone, _ := v1_8.Ref(built, "stone")
	ice, _ := v1_8.Ref(built, "ice")

	if got := written[geom.BlockPos{X: 0, Y: 2, Z: 0}]; got != air {
		t.Errorf("a cell the blocks do not mention holds %v, want the fill", got)
	}
	if got := written[geom.BlockPos{X: 1, Y: 0, Z: 1}]; got != stone {
		t.Errorf("a cell inside the span holds %v, want stone", got)
	}
	// The later entry wins, which is what lets a description lay a floor and
	// then replace patches of it.
	if got := written[geom.BlockPos{X: 0, Y: 0, Z: 0}]; got != ice {
		t.Errorf("the overlapping cell holds %v, want the later entry, ice", got)
	}
}

func TestAWorldNamingAnUnknownBlockSaysWhich(t *testing.T) {
	err := scene.World{
		Min: geom.BlockPos{}, Max: geom.BlockPos{}, Fill: "air",
		Blocks: []scene.Block{{Pos: geom.BlockPos{}, Name: "unobtainium"}},
	}.Describe(profile(t), func(geom.BlockPos, world.BlockRef) error { return nil })

	if !errors.Is(err, scene.ErrScene) {
		t.Fatalf("Describe returned %v, want a refusal", err)
	}
	if got := err.Error(); !strings.Contains(got, "unobtainium") {
		t.Fatalf("the error does not name the block: %s", got)
	}
}

func TestAWorldWithNoFillIsRefused(t *testing.T) {
	// An empty fill would describe nothing, and every tick over the region would
	// then report itself incomplete over cells the description meant to cover.
	err := scene.World{Max: geom.BlockPos{X: 1, Y: 1, Z: 1}}.
		Describe(profile(t), func(geom.BlockPos, world.BlockRef) error { return nil })

	if !errors.Is(err, scene.ErrScene) {
		t.Fatalf("Describe returned %v, want a refusal", err)
	}
}

func TestAnInsideOutRegionIsRefused(t *testing.T) {
	err := scene.World{
		Min: geom.BlockPos{X: 4}, Max: geom.BlockPos{X: -4}, Fill: "air",
	}.Validate()

	if !errors.Is(err, scene.ErrScene) {
		t.Fatalf("Validate returned %v, want a refusal", err)
	}
}

func TestABlockOutsideTheRegionIsRefused(t *testing.T) {
	// Silent otherwise: the block would be written, the region around it would
	// not be, and the first tick that swept near it would report itself
	// incomplete over cells whose absence the description never hinted at.
	err := scene.World{
		Min: geom.BlockPos{}, Max: geom.BlockPos{X: 2, Y: 2, Z: 2}, Fill: "air",
		Blocks: []scene.Block{{Pos: geom.BlockPos{X: 9, Y: 9, Z: 9}, Name: "stone"}},
	}.Validate()

	if !errors.Is(err, scene.ErrScene) {
		t.Fatalf("Validate returned %v, want a refusal", err)
	}
}

func TestAnInsideOutSpanIsRefusedRatherThanNamingNothing(t *testing.T) {
	far := geom.BlockPos{X: -3}
	err := scene.World{
		Min: geom.BlockPos{X: -4}, Max: geom.BlockPos{X: 4, Y: 4, Z: 4}, Fill: "air",
		Blocks: []scene.Block{{Pos: geom.BlockPos{X: 3}, To: &far, Name: "stone"}},
	}.Validate()

	if !errors.Is(err, scene.ErrScene) {
		t.Fatalf("Validate returned %v, want a refusal", err)
	}
}

func TestASingleCellIsOneCell(t *testing.T) {
	var visited []geom.BlockPos
	scene.Block{Pos: geom.BlockPos{X: 1, Y: 2, Z: 3}, Name: "stone"}.
		Cells(func(pos geom.BlockPos) { visited = append(visited, pos) })

	if len(visited) != 1 || visited[0] != (geom.BlockPos{X: 1, Y: 2, Z: 3}) {
		t.Fatalf("a single cell walked %v", visited)
	}
}

func TestTheRegionReportsItsSize(t *testing.T) {
	got := scene.World{Min: geom.BlockPos{X: -1, Y: -1, Z: -1}, Max: geom.BlockPos{X: 1, Y: 1, Z: 1}}.Cells()
	if want := 27; got != want {
		t.Fatalf("Cells = %d, want %d", got, want)
	}
	if got := (scene.World{Min: geom.BlockPos{X: 1}, Max: geom.BlockPos{X: -1}}).Cells(); got != 0 {
		t.Fatalf("an inside-out region reports %d cells, want none", got)
	}
}
