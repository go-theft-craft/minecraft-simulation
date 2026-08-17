package oracle_test

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/collision"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// The described region is filled with air so that no sweep can reach an
// undescribed cell, and blocks are placed well inside it. A move that still
// reported unknown cells would mean the sweep left the region, which is a bug
// in this test rather than a finding about collision.
const (
	describedRadius = 10
	blockRadius     = 6
)

// blockKind is what the harness understands: a full cube or a bottom slab.
// Two shapes are enough to exercise both step-up outcomes, because 0.6 of step
// height clears one and not the other.
type blockKind int

const (
	kindStone blockKind = 0
	kindSlab  blockKind = 1
)

func (k blockKind) shape() geom.Shape {
	if k == kindSlab {
		return geom.NewShape(geom.AABB{MaxX: 1, MaxY: 0.5, MaxZ: 1})
	}

	return geom.FullCube()
}

// moveScene is one random world plus the moves tried against it.
//
// It is not a scene.World: this harness places shapes directly, because M8.2
// predates the block table and checks collision against geometry rather than
// against a profile.
type moveScene struct {
	blocks *world.Blocks
	placed []placement
	moves  []collision.Move
}

type placement struct {
	pos  geom.BlockPos
	kind blockKind
}

// buildScene generates a world with a floor, a scattering of stone and slabs,
// and a batch of moves starting from air.
func buildScene(random *rand.Rand, moves int) moveScene {
	blocks := world.NewBlocks()
	blocks.Fill(
		geom.BlockPos{X: -describedRadius, Y: -describedRadius, Z: -describedRadius},
		geom.BlockPos{X: describedRadius, Y: describedRadius, Z: describedRadius},
		geom.EmptyShape(),
	)

	var placed []placement
	place := func(pos geom.BlockPos, kind blockKind) {
		blocks.Set(pos, kind.shape())
		placed = append(placed, placement{pos: pos, kind: kind})
	}

	// A floor at y=-1, so that most moves have something to stand on and the
	// step-up path is reachable rather than incidental.
	for x := int32(-blockRadius); x <= blockRadius; x++ {
		for z := int32(-blockRadius); z <= blockRadius; z++ {
			place(geom.BlockPos{X: x, Y: -1, Z: z}, kindStone)
		}
	}

	// Obstacles at and above floor level: walls to be blocked by, slabs and
	// single cubes to step onto or fail to step onto, and ceilings to clamp a
	// rise.
	for range 40 {
		pos := geom.BlockPos{
			X: int32(random.IntN(2*blockRadius+1)) - blockRadius,
			Y: int32(random.IntN(4)),
			Z: int32(random.IntN(2*blockRadius+1)) - blockRadius,
		}
		kind := kindStone
		if random.IntN(2) == 0 {
			kind = kindSlab
		}
		place(pos, kind)
	}

	var list []collision.Move
	for range moves {
		// The body is a 1.8.9 player box placed on a grid that lands flush
		// with cell boundaries as often as it lands between them, because the
		// clamps decide contact with <= and >=.
		origin := geom.Vec3{
			X: snap(random, 8),
			Y: snap(random, 6),
			Z: snap(random, 8),
		}
		body := geom.AABB{MinX: -0.3, MinY: 0, MinZ: -0.3, MaxX: 0.3, MaxY: 1.8, MaxZ: 0.3}.
			Offset(origin)

		list = append(list, collision.Move{
			Body: body,
			Motion: geom.Vec3{
				X: snap(random, 3),
				Y: snap(random, 3),
				Z: snap(random, 3),
			},
			OnGround:   random.IntN(2) == 0,
			StepHeight: stepHeights[random.IntN(len(stepHeights))],
		})
	}

	// Random moves reach the step-up retry only occasionally, so approaches are
	// generated on purpose too: a body standing on the floor, walking into a
	// block that sits on it, carrying the downward motion a real tick carries.
	// Without these the step-up path would be checked a few dozen times in two
	// thousand cases, which is not a check of it.
	for _, block := range placed {
		if block.pos.Y != 0 {
			continue
		}
		for _, approach := range []geom.Vec3{{X: 1}, {X: -1}, {Z: 1}, {Z: -1}} {
			// Stand one cell back from the block, centred on its column.
			centre := geom.Vec3{
				X: float64(block.pos.X) + 0.5 - approach.X,
				Y: 0,
				Z: float64(block.pos.Z) + 0.5 - approach.Z,
			}
			body := geom.AABB{MinX: -0.3, MinY: 0, MinZ: -0.3, MaxX: 0.3, MaxY: 1.8, MaxZ: 0.3}.
				Offset(centre)

			// A walking tick's horizontal speed and one tick of gravity.
			list = append(list, collision.Move{
				Body: body,
				Motion: geom.Vec3{
					X: approach.X * 0.2158,
					Y: -0.0784000015258789,
					Z: approach.Z * 0.2158,
				},
				OnGround:   true,
				StepHeight: stepHeights[random.IntN(len(stepHeights))],
			})
		}
	}

	return moveScene{blocks: blocks, placed: placed, moves: list}
}

// stepHeights covers the disabled case, the vanilla player value, and a value
// that clears a whole cube, so the retry is exercised in all three regimes.
//
// Each is written through float32 because that is the width the game stores
// stepHeight at, and the harness parses the field with Float.parseFloat. A
// float64 that is not exactly representable as a float32 would be a different
// number on the two sides of the comparison, and the mismatch it produced would
// be an artefact of this test rather than a finding about collision.
var stepHeights = []float64{
	0,
	float64(float32(0.5)),
	float64(float32(0.6)),
	float64(float32(1)),
}

// snap returns a coordinate that is a whole number, a half, or arbitrary, in
// roughly equal measure.
func snap(random *rand.Rand, spread float64) float64 {
	value := (random.Float64() - 0.5) * spread
	switch random.IntN(3) {
	case 0:
		return float64(int(value))
	case 1:
		return float64(int(value*2)) / 2
	default:
		return value
	}
}

// TestMoveMatchesTheGame is the milestone's strongest check: for random worlds
// and random moves, the box this module's collision.Resolve produces must be
// bit-identical to the box the game's Entity.moveEntity produces, and the three
// collision flags the game exposes must agree.
//
// The game does its own candidate gathering here. Nothing about the axis order,
// the two step-up attempts, the settle, or the flags is supplied by this test.
func TestMoveMatchesTheGame(t *testing.T) {
	jar := newOracle(t)

	const (
		scenes         = 24
		movesPerScene  = 80
		expectedChecks = scenes * movesPerScene
	)

	var (
		input  []string
		built  []moveScene
		wanted int
	)
	for seed := range uint64(scenes) {
		random := rand.New(rand.NewPCG(seed+1, 0x5eed))
		current := buildScene(random, movesPerScene)
		built = append(built, current)

		input = append(input, "C")
		for _, block := range current.placed {
			input = append(input, fmt.Sprintf("B %d %d %d %d",
				block.pos.X, block.pos.Y, block.pos.Z, int(block.kind)))
		}
		for _, move := range current.moves {
			input = append(input, fmt.Sprintf("M %s %s %s %s %t %s",
				renderBox(move.Body),
				hex(move.Motion.X), hex(move.Motion.Y), hex(move.Motion.Z),
				move.OnGround, hex(move.StepHeight)))
			wanted++
		}
	}

	answers := jar.run(t, "MoveOracle", input, wanted)

	checked, skipped, stepped := 0, 0, 0
	index := 0
	for sceneIndex, current := range built {
		for moveIndex, move := range current.moves {
			answer := answers[index]
			index++

			got, err := collision.Resolve(current.blocks, move)
			if err != nil {
				t.Fatalf("scene %d move %d: Resolve: %v", sceneIndex, moveIndex, err)
			}
			if len(got.Unknown) != 0 {
				skipped++

				continue
			}

			wantBody, wantHorizontal, wantVertical, wantGround := parseMove(t, answer)
			where := fmt.Sprintf("scene %d move %d\nbody %+v\nmotion %+v onGround %t step %v",
				sceneIndex, moveIndex, move.Body, move.Motion, move.OnGround, move.StepHeight)

			if !identicalBox(got.Body, wantBody) {
				t.Fatalf("%s\nbody after move = %+v\nthe game says      %+v", where, got.Body, wantBody)
			}
			if got.CollidedHorizontally() != wantHorizontal {
				t.Fatalf("%s\ncollidedHorizontally = %v, the game says %v",
					where, got.CollidedHorizontally(), wantHorizontal)
			}
			if got.CollidedY != wantVertical {
				t.Fatalf("%s\ncollidedVertically = %v, the game says %v",
					where, got.CollidedY, wantVertical)
			}
			if got.OnGround != wantGround {
				t.Fatalf("%s\nonGround = %v, the game says %v", where, got.OnGround, wantGround)
			}

			checked++
			if got.Stepped {
				stepped++
			}
		}
	}

	if checked == 0 {
		t.Fatal("no move was checked against the game")
	}
	if stepped == 0 {
		t.Error("no move exercised the step-up retry, so this run proves nothing about it")
	}
	t.Logf("checked %d moves against the game, %d of them stepping; skipped %d as incomplete",
		checked, stepped, skipped)
}

// TestAFlushFaceIsNotACollider pins the rule the random scenes cannot reach.
//
// The game gathers a block's collider only when the sweep overlaps the block's
// box in a volume, and sharing a face is not overlapping. A body already flush
// against a wall therefore has no wall in its candidate list until the sweep
// reaches past the face — and a motion small enough to vanish into the body's own
// coordinates never reaches past it.
//
// It takes a hand-built case because it takes a motion of 1e-18: the swept
// region's face has to land exactly on the block's, and any motion large enough
// to see does not. The two cases differ in nothing but that, and the game
// answers them differently.
//
// This is what a gather that visited cells rather than shapes got wrong: it
// stopped the invisible move and reported a collision the game does not report.
func TestAFlushFaceIsNotACollider(t *testing.T) {
	jar := newOracle(t)

	// Feet on a floor, shoulder against a wall at x = 1.
	body := geom.AABB{MinX: 0.4, MinY: 0, MinZ: 0.2, MaxX: 1, MaxY: 1.8, MaxZ: 0.8}
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -1, Y: -2, Z: -1}, geom.BlockPos{X: 2, Y: 2, Z: 2}, geom.EmptyShape())
	blocks.Set(geom.BlockPos{X: 0, Y: -1}, geom.FullCube())
	blocks.Set(geom.BlockPos{X: 1, Y: -1}, geom.FullCube())
	blocks.Set(geom.BlockPos{X: 1}, geom.FullCube())

	moves := []collision.Move{
		{Body: body, Motion: geom.Vec3{X: 1e-18}, OnGround: true},
		{Body: body, Motion: geom.Vec3{X: 0.2}, OnGround: true},
	}

	input := []string{"C", "B 0 -1 0 0", "B 1 -1 0 0", "B 1 0 0 0"}
	for _, move := range moves {
		input = append(input, fmt.Sprintf("M %s %s %s %s %t %s",
			renderBox(move.Body),
			hex(move.Motion.X), hex(move.Motion.Y), hex(move.Motion.Z),
			move.OnGround, hex(move.StepHeight)))
	}

	answers := jar.run(t, "MoveOracle", input, len(moves))
	for index, move := range moves {
		got, err := collision.Resolve(blocks, move)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}

		wantBody, wantHorizontal, _, _ := parseMove(t, answers[index])
		if !identicalBox(got.Body, wantBody) {
			t.Fatalf("motion %v: body after move = %+v, the game says %+v",
				move.Motion.X, got.Body, wantBody)
		}
		if got.CollidedHorizontally() != wantHorizontal {
			t.Fatalf("motion %v: collidedHorizontally = %v, the game says %v",
				move.Motion.X, got.CollidedHorizontally(), wantHorizontal)
		}
	}

	// Stated as well as compared, so that a harness that started answering
	// "false" to everything could not pass this test quietly.
	if _, horizontal, _, _ := parseMove(t, answers[0]); horizontal {
		t.Error("the game reported a collision against a face it only touches")
	}
	if _, horizontal, _, _ := parseMove(t, answers[1]); !horizontal {
		t.Error("the game reported no collision against a wall it moved into")
	}
}

// parseMove reads one answer line from the movement harness.
func parseMove(t *testing.T, text string) (body geom.AABB, horizontal, vertical, ground bool) {
	t.Helper()

	fields := strings.Fields(text)
	if len(fields) != 9 {
		t.Fatalf("oracle returned %d fields for a move: %q", len(fields), text)
	}

	body = geom.AABB{
		MinX: parseHex(t, fields[0]),
		MinY: parseHex(t, fields[1]),
		MinZ: parseHex(t, fields[2]),
		MaxX: parseHex(t, fields[3]),
		MaxY: parseHex(t, fields[4]),
		MaxZ: parseHex(t, fields[5]),
	}

	return body, fields[6] == "true", fields[7] == "true", fields[8] == "true"
}
