package oracle_test

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/collision"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// palette26 names the blocks every assembly case is built from.
//
// Six blocks, chosen for their shapes rather than for their names: a full cube,
// a half slab, a stair whose two boxes have different footprints, a path whose
// top face at fifteen sixteenths lands on no grid line, a table at three
// quarters that lands on quarters, and a trapdoor three sixteenths thick.
// Between them they offer rises below, at, and above a player's step height, and
// they exercise both branches of the shape model: the grid and the fallback to a
// box's own two faces.
//
// Every one has a shape that is a fact about the block. None consults a
// neighbour, a block entity, or the moving body's collision context, which is
// what makes it sound for the harness to report the shape with none of those and
// for this side to place it as a constant.
var palette26 = []string{
	"stone",
	"smooth_stone_slab",
	"stone_brick_stairs",
	"dirt_path",
	"enchanting_table",
	"oak_trapdoor",
}

// paletteCommands asks the harness for each palette block's shape and for the Y
// coordinates that shape offers, in that order.
func paletteCommands() []string {
	commands := make([]string, 0, 2*len(palette26))
	for _, name := range palette26 {
		commands = append(commands, "Q "+name)
	}
	for _, name := range palette26 {
		commands = append(commands, "Y "+name)
	}

	return commands
}

// paletteShapes reads the shapes the harness reported for the palette.
//
// The shapes come from the game rather than from a table here, so a version that
// changes a block's collider changes what this test places rather than silently
// disagreeing with it.
func paletteShapes(t *testing.T, answers []string) map[string]geom.Shape {
	t.Helper()

	shapes := make(map[string]geom.Shape, len(palette26))
	for index, name := range palette26 {
		shapes[name] = geom.NewShape(parseBoxes(t, answers[index])...)
	}

	return shapes
}

func TestTheBlockShapeGridMatchesTheGame(t *testing.T) {
	// The premise the whole-collide check rests on: a shape rebuilt on this side
	// from the boxes the game stores offers the same rises the game's own shape
	// offers. If that failed, a mismatch in the assembly test could be a shape
	// this side could not represent rather than an assembly this side got wrong.
	jar := newOracle26(t)

	answers := jar.run(t, "MoveOracle26", paletteCommands(), 2*len(palette26))
	shapes := paletteShapes(t, answers)

	for index, name := range palette26 {
		got := shapes[name].GridY()
		want := parseCoords(t, answers[len(palette26)+index])

		if len(got) != len(want) {
			t.Fatalf("%s offers %v, the game says %v", name, got, want)
		}
		for at := range got {
			if !identical(got[at], want[at]) {
				t.Fatalf("%s coordinate %d: got %v, the game says %v\nfull %v against %v",
					name, at, got[at], want[at], got, want)
			}
		}
	}

	t.Logf("checked the grid of %d block shapes against the game", len(palette26))
}

// collidePlacement is one block in a scene.
type collidePlacement struct {
	cell geom.BlockPos
	name string
}

// collideScene is a world of blocks and a body about to move through it.
type collideScene struct {
	placements []collidePlacement
	body       geom.AABB
	motion     geom.Vec3
	onGround   bool
	step       float32
}

// buildCollideScene generates a body standing on a floor with something in its way.
//
// The obstacle goes in the cell the body is about to walk into, because a
// step-up only runs when a horizontal axis was actually blocked: a scene that
// scatters blocks agrees with the game about doing nothing. The rest go in the
// cells around the feet, where anything the body can stand on, climb, or bump
// its head against has to be.
//
// Feet land on cell boundaries and on the tops of the palette's shapes as often
// as they land between them. That is where the assembly's own comparisons
// decide: a rise exactly equal to the vertical motion the flat move applied is
// dropped from the candidate list, and a rise exactly at the step height is
// kept.
func buildCollideScene(random *rand.Rand, shapes map[string]geom.Shape) collideScene {
	// The floor. It is stone rather than a shape this side invented, so that
	// both sides stand on the same thing.
	var placements []collidePlacement
	for x := int32(-3); x <= 3; x++ {
		for z := int32(-3); z <= 3; z++ {
			placements = append(placements, collidePlacement{
				cell: geom.BlockPos{X: x, Y: 0, Z: z},
				name: "stone",
			})
		}
	}

	// Feet on the floor, or a fraction above it, or on top of one of the
	// palette's shapes.
	feet := 1.0
	switch random.IntN(4) {
	case 0:
		feet = 1 + []float64{0.5, 0.75, 0.9375, 0.1875}[random.IntN(4)]
	case 1:
		feet = 1 + random.Float64()*0.4
	}

	// The direction the body walks, and how far short of the boundary it starts.
	//
	// Aiming the move at a cell boundary rather than into a random direction is
	// what makes the step branch run: a body has to be blocked before it can
	// climb, and a randomly aimed move is mostly a move through open air. A gap
	// of zero means the body starts flush against the boundary, which is the case
	// the clamp's tolerance decides.
	alongX := random.IntN(2) == 0
	forwards := random.IntN(2) == 0
	gap := []float64{0, 0, 0.05, 0.25, random.Float64() * 0.4}[random.IntN(5)]

	// Across the lane, the body sits anywhere in the cell it can fit in.
	across := 0.3 + random.Float64()*0.4
	lead := 1 - 0.3 - gap
	if !forwards {
		lead = -lead
	}

	body := geom.AABB{MinY: feet, MaxY: feet + 1.8}
	x, z := lead, across
	if !alongX {
		x, z = across, lead
	}
	body.MinX, body.MaxX = x-0.3, x+0.3
	body.MinZ, body.MaxZ = z-0.3, z+0.3

	// Enough to cross the gap, with a fall to go with it. A tick's real motion is
	// under a fifth of a block horizontally and a twelfth down, and the larger
	// values here are what put a body against a face in one move rather than
	// after eight.
	forward := gap + 0.02 + random.Float64()*0.5
	if !forwards {
		forward = -forward
	}
	sideways := (random.Float64() - 0.5) * 0.3
	motion := geom.Vec3{X: forward, Y: -random.Float64() * 0.3, Z: sideways}
	if !alongX {
		motion.X, motion.Z = motion.Z, motion.X
	}
	if random.IntN(4) == 0 {
		motion.Y = (random.Float64() - 0.5) * 0.8
	}

	scene := collideScene{
		placements: placements,
		body:       body,
		motion:     motion,
		// A body that is standing is the common case and the one that steps, so
		// it is weighted rather than even. Airborne bodies are here for the
		// probe's extra reach downward, which only they get.
		onGround: random.IntN(4) != 0,
		// The player's own step height is the widened float, which is what a
		// profile hands the resolve. Zero must appear: it is the branch that
		// never steps at all.
		step: []float32{float32(0.6), float32(0.6), 1, 0.5, 0}[random.IntN(5)],
	}

	// The cell the feet are walking into, which is where an obstacle has to be
	// for a step to run at all.
	ahead := geom.BlockPos{
		X: geom.Floor(body.MinX + motion.X),
		Y: geom.Floor(body.MinY),
		Z: geom.Floor(body.MinZ + motion.Z),
	}
	if ahead.Y < 1 {
		ahead.Y = 1
	}
	scene.place(shapes, collidePlacement{
		cell: ahead,
		name: palette26[random.IntN(len(palette26))],
	})

	// And two more around the feet, which is what turns a single obstacle into a
	// staircase, a corner, or a ceiling over the step.
	for range 2 {
		scene.place(shapes, collidePlacement{
			cell: geom.BlockPos{
				X: ahead.X + int32(random.IntN(3)) - 1,
				Y: ahead.Y + int32(random.IntN(2)),
				Z: ahead.Z + int32(random.IntN(3)) - 1,
			},
			name: palette26[random.IntN(len(palette26))],
		})
	}

	return scene
}

// place adds a block unless it would put the body inside it.
//
// A body that starts inside a collider is the one case these two algorithms are
// known to answer differently, and the difference is in the flat clamp rather
// than in the assembly this test is for. The game clamps against the grid of a
// shape, scanning from the cell after the one the moving face is in, so a body
// already inside a multi-cell shape is stopped at that shape's next interior
// grid line; this module clamps against the shape's boxes, where a face already
// past a box does not stop at all. The divergence is recorded as its own finding
// with the case that found it. Keeping it out of these scenes is what lets a
// failure here mean the assembly.
func (s *collideScene) place(shapes map[string]geom.Shape, placed collidePlacement) {
	for _, box := range shapes[placed.name].BoxesAt(placed.cell, nil) {
		if s.body.Intersects(box) {
			return
		}
	}

	s.placements = append(s.placements, placed)
}

// commands renders the scene's world and the move to run in it.
func (s collideScene) commands() []string {
	commands := []string{"C"}
	for _, placed := range s.placements {
		commands = append(commands, fmt.Sprintf("B %d %d %d %s",
			placed.cell.X, placed.cell.Y, placed.cell.Z, placed.name))
	}
	commands = append(commands, fmt.Sprintf("M %s %s %s %s %t %s",
		renderBox(s.body), hex(s.motion.X), hex(s.motion.Y), hex(s.motion.Z),
		s.onGround, single(s.step)))

	return commands
}

// blocks builds the view this side resolves against, from the game's own shapes.
//
// Everything the sweep can reach is filled with air first, because an unanswered
// cell is not an empty one: the resolve reports it as unknown and refuses to
// move, which would pass this comparison for the wrong reason.
func (s collideScene) blocks(shapes map[string]geom.Shape) *world.Blocks {
	blocks := world.NewBlocks()
	blocks.Fill(
		geom.BlockPos{X: -8, Y: -4, Z: -8},
		geom.BlockPos{X: 8, Y: 8, Z: 8},
		geom.EmptyShape(),
	)
	for _, placed := range s.placements {
		blocks.Set(placed.cell, shapes[placed.name])
	}

	return blocks
}

func TestTheWholeCollideMatchesTheGame(t *testing.T) {
	// The check ShapeOracle26 cannot make. It reaches the clamp, the resolve,
	// and the candidate list, all of them private statics; the assembly around
	// them is a private instance method that needs a level, so it was covered by
	// this module's own tests only. This runs it: the grounded box, the probe the
	// candidates come from, the choice of the first improving rise, and the drop
	// back to the original feet are all the game's here.
	jar := newOracle26(t)

	// The palette is asked for first, in its own run, because the scenes are
	// built from the game's own shapes: which cells a body would start inside is
	// not knowable until the shapes are known.
	shapes := paletteShapes(t, jar.run(t, "MoveOracle26", paletteCommands(), 2*len(palette26)))

	var scenes []collideScene
	var commands []string
	for _, seed := range []uint64{41, 43, 47, 53, 59, 61, 67, 71, 73, 79, 83, 89} {
		random := rand.New(rand.NewPCG(seed, 0))
		for range 200 {
			scene := buildCollideScene(random, shapes)
			scenes = append(scenes, scene)
			commands = append(commands, scene.commands()...)
		}
	}

	answers := jar.run(t, "MoveOracle26", commands, len(scenes))

	stepped, blocked, landed := 0, 0, 0
	for index, scene := range scenes {
		got, err := collision.ResolveVoxel(scene.blocks(shapes), collision.Move{
			Body:       scene.body,
			Motion:     scene.motion,
			OnGround:   scene.onGround,
			StepHeight: float64(scene.step),
		})
		if err != nil {
			t.Fatalf("ResolveVoxel: %v", err)
		}
		if len(got.Unknown) != 0 {
			t.Fatalf("the scene left %d cells unanswered: %v", len(got.Unknown), got.Unknown)
		}

		want := parseVec(t, answers[index])
		if !identical(got.Applied.X, want.X) ||
			!identical(got.Applied.Y, want.Y) ||
			!identical(got.Applied.Z, want.Z) {
			t.Fatalf("collide: got %+v, the game says %+v\n%s",
				got.Applied, want, scene.describe())
		}

		if got.Stepped {
			stepped++
		}
		if got.CollidedX || got.CollidedZ {
			blocked++
		}
		if got.OnGround {
			landed++
		}
	}

	// A run where nothing ever stepped would agree with the game about the
	// branch this test exists for, so the branch is counted rather than assumed.
	t.Logf("checked %d whole collides against the game, %d stepped, %d blocked, %d landed",
		len(scenes), stepped, blocked, landed)
	if stepped*20 < len(scenes) {
		t.Errorf("only %d of %d moves stepped; the assembly is barely covered", stepped, len(scenes))
	}
	if blocked*10 < len(scenes) {
		t.Errorf("only %d of %d moves were blocked horizontally", blocked, len(scenes))
	}
	if landed == 0 {
		t.Error("no move landed; the grounded box was never the clamped one")
	}
}

// describe renders a scene the way a failure needs to read it: the body, the
// move, and every block, so that the case can be replayed by hand.
func (s collideScene) describe() string {
	var out strings.Builder
	fmt.Fprintf(&out, "body %+v\nmotion %+v onGround %t step %v\n",
		s.body, s.motion, s.onGround, s.step)
	for _, placed := range s.placements {
		// The floor is the same in every scene and would bury the interesting
		// blocks under forty-nine lines of stone.
		if placed.cell.Y == 0 && placed.name == "stone" {
			continue
		}
		fmt.Fprintf(&out, "block %s at %+v\n", placed.name, placed.cell)
	}

	return out.String()
}

// parseBoxes reads the count and the boxes the harness prints for a shape.
func parseBoxes(t *testing.T, text string) []geom.AABB {
	t.Helper()

	fields := strings.Fields(text)
	if len(fields) == 0 {
		t.Fatalf("the oracle returned nothing for a shape")
	}
	count, err := strconv.Atoi(fields[0])
	if err != nil {
		t.Fatalf("parse the box count %q: %v", fields[0], err)
	}
	if len(fields) != 6*count+1 {
		t.Fatalf("the oracle said %d boxes and printed %d fields: %q", count, len(fields)-1, text)
	}

	boxes := make([]geom.AABB, 0, count)
	for at := range count {
		values := fields[1+6*at : 7+6*at]
		boxes = append(boxes, geom.AABB{
			MinX: parseHex(t, values[0]),
			MinY: parseHex(t, values[1]),
			MinZ: parseHex(t, values[2]),
			MaxX: parseHex(t, values[3]),
			MaxY: parseHex(t, values[4]),
			MaxZ: parseHex(t, values[5]),
		})
	}

	return boxes
}

// parseCoords reads the count and the coordinates the harness prints.
func parseCoords(t *testing.T, text string) []float64 {
	t.Helper()

	fields := strings.Fields(text)
	if len(fields) == 0 {
		t.Fatalf("the oracle returned nothing for a coordinate list")
	}
	count, err := strconv.Atoi(fields[0])
	if err != nil {
		t.Fatalf("parse the coordinate count %q: %v", fields[0], err)
	}
	if len(fields) != count+1 {
		t.Fatalf("the oracle said %d coordinates and printed %d: %q", count, len(fields)-1, text)
	}

	coords := make([]float64, 0, count)
	for _, field := range fields[1:] {
		coords = append(coords, parseHex(t, field))
	}

	return coords
}
