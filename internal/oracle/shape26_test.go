package oracle_test

import (
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/collision"
	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// jar26Path is the analysis jar for a version Mojang ships unobfuscated.
//
// There is no named.jar here and there does not need to be: this version's
// classes carry their real names, so the harness compiles against the shipped
// server directly. The 1.8.9 oracle points at named.jar for the opposite
// reason.
const (
	jar26Path       = "../../reference/work/versions/26.1.2/server/executable.jar"
	libraries26Path = "../../reference/work/versions/26.1.2/libraries"
)

// newOracle26 compiles the 26.1.2 harness, or skips when the workspace or the
// JDK is absent.
func newOracle26(t *testing.T) *oracle {
	t.Helper()

	jar, err := filepath.Abs(jar26Path)
	if err != nil {
		t.Fatalf("resolve jar path: %v", err)
	}
	if _, err := os.Stat(jar); err != nil {
		t.Skipf("no prepared 26.1.2 server jar at %s; run task reference:prepare", jar26Path)
	}
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac is not on PATH")
	}
	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("java is not on PATH")
	}

	// The server jar names its dependencies rather than shading them, so the
	// libraries the workspace downloaded are part of the classpath.
	classpath := []string{jar}
	libraries, err := filepath.Abs(libraries26Path)
	if err != nil {
		t.Fatalf("resolve libraries path: %v", err)
	}
	if err := filepath.Walk(libraries, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".jar") {
			classpath = append(classpath, path)
		}

		return nil
	}); err != nil {
		t.Skipf("no prepared 26.1.2 libraries at %s: %v", libraries26Path, err)
	}

	classes := t.TempDir()
	build := exec.Command(javac, "-nowarn",
		"-cp", strings.Join(classpath, string(os.PathListSeparator)),
		"-d", classes, "java/ShapeOracle26.java")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compile the 26.1.2 harness: %v\n%s", err, out)
	}

	return &oracle{
		classpath: strings.Join(append([]string{classes}, classpath...), string(os.PathListSeparator)),
		workDir:   t.TempDir(),
	}
}

// shapeScene is a set of colliders and the body moving through them.
//
// A collider is a block-local box in a cell rather than a box in world space,
// because that is how the game builds one: it snaps the local box to a
// power-of-two grid inside the cell and then moves the result. Building shapes
// straight from world coordinates skips the snapping, and the snapping is what
// the step-up heights are collected from.
type shapeScene struct {
	colliders []collider
	body      geom.AABB
	motion    geom.Vec3
}

type collider struct {
	cell  geom.BlockPos
	local geom.AABB
}

// boxes returns the colliders in world space, which is what the clamp works on.
func (s shapeScene) boxes() []geom.AABB {
	var boxes []geom.AABB
	for _, placed := range s.colliders {
		boxes = geom.NewShape(placed.local).BoxesAt(placed.cell, boxes)
	}

	return boxes
}

// stepCoords returns the heights the colliders offer, in world space.
func (s shapeScene) stepCoords() []float64 {
	var coords []float64
	for _, placed := range s.colliders {
		for _, coord := range geom.NewShape(placed.local).GridY() {
			coords = append(coords, coord+float64(placed.cell.Y))
		}
	}

	return coords
}

// buildShapeScene generates colliders in a player-sized body's way.
//
// The colliders sit on the grid cells around the body rather than anywhere in a
// spread, and the motion is small enough to stay in reach of them. An earlier
// version of this scattered boxes over six blocks and only one case in sixty
// touched anything: it agreed with the game about doing nothing. The coverage
// assertions in each test are there because that is not obvious from a green
// run.
//
// Body coordinates land on cell boundaries and half-blocks as often as they land
// between them, because that is where the tolerances decide the answer: a face
// exactly at the body's edge is the case the 1.8.9 algorithm and this one
// disagree about.
func buildShapeScene(random *rand.Rand, colliders int) shapeScene {
	coord := func(spread float64) float64 {
		value := (random.Float64() - 0.5) * spread
		switch random.IntN(3) {
		case 0:
			return math.Round(value)
		case 1:
			return math.Round(value*2) / 2
		default:
			return value
		}
	}

	body := geom.AABB{MinX: coord(3), MinY: coord(3), MinZ: coord(3)}
	body.MaxX = body.MinX + 0.6
	body.MaxY = body.MinY + 1.8
	body.MaxZ = body.MinZ + 0.6

	scene := shapeScene{
		body: body,
		motion: geom.Vec3{
			X: (random.Float64() - 0.5) * 1.6,
			Y: (random.Float64() - 0.5) * 1.6,
			Z: (random.Float64() - 0.5) * 1.6,
		},
	}

	// The cells around the body's feet, which is where anything it can walk
	// into, stand on, or step over has to be.
	base := geom.BlockPos{
		X: geom.Floor(body.MinX),
		Y: geom.Floor(body.MinY),
		Z: geom.Floor(body.MinZ),
	}
	for range colliders {
		cell := geom.BlockPos{
			X: base.X + int32(random.IntN(3)) - 1,
			Y: base.Y + int32(random.IntN(3)) - 1,
			Z: base.Z + int32(random.IntN(3)) - 1,
		}
		// A full cube, a slab, a plate, and a box that lands on no grid line at
		// all. The last one matters: it is the case where the game keeps the two
		// faces it was given rather than a grid, and a shape model that always
		// snapped would get it wrong.
		local := []geom.AABB{
			{MaxX: 1, MaxY: 1, MaxZ: 1},
			{MaxX: 1, MaxY: 0.5, MaxZ: 1},
			{MaxX: 1, MaxY: 0.125, MaxZ: 1},
			{MinX: 0.0625, MaxX: 0.9375, MaxY: 0.3, MaxZ: 1},
		}[random.IntN(4)]
		scene.colliders = append(scene.colliders, collider{cell: cell, local: local})
	}

	return scene
}

// commands renders the scene's colliders.
func (s shapeScene) commands() []string {
	commands := []string{"C"}
	for _, placed := range s.colliders {
		commands = append(commands, fmt.Sprintf("S %d %d %d %s",
			placed.cell.X, placed.cell.Y, placed.cell.Z, renderBox(placed.local)))
	}

	return commands
}

func TestTheShapeClampMatchesTheGame(t *testing.T) {
	jar := newOracle26(t)

	type probe struct {
		scene    shapeScene
		axis     string
		distance float64
	}

	var probes []probe
	var commands []string
	for _, seed := range []uint64{1, 2, 3, 5, 8} {
		random := rand.New(rand.NewPCG(seed, 0))
		for range 200 {
			scene := buildShapeScene(random, 3)
			commands = append(commands, scene.commands()...)
			for _, name := range []string{"X", "Y", "Z"} {
				distance := scene.motion.X
				switch name {
				case "Y":
					distance = scene.motion.Y
				case "Z":
					distance = scene.motion.Z
				}
				probes = append(probes, probe{scene: scene, axis: name, distance: distance})
				commands = append(commands, fmt.Sprintf("A %s %s %s",
					name, renderBox(scene.body), hex(distance)))
			}
		}
	}

	answers := jar.run(t, "ShapeOracle26", commands, len(probes))
	clamped := 0
	for index, p := range probes {
		got := collision.ClampAxis(p.scene.boxes(), p.scene.body, namedAxis(t, p.axis), p.distance)
		want := parseHex(t, answers[index])
		if !identical(got, want) {
			t.Fatalf("clamp %s: got %v, the game says %v\nbody %+v\nboxes %+v\ndistance %v",
				p.axis, got, want, p.scene.body, p.scene.boxes(), p.distance)
		}
		if got != p.distance {
			clamped++
		}
	}

	// A case where nothing is in the way agrees trivially. The check is only
	// about the clamp if the clamp fires, so how often it fires is asserted
	// rather than hoped for.
	t.Logf("checked %d shape clamps against the game, %d of which clamped", len(probes), clamped)
	if clamped*20 < len(probes) {
		t.Errorf("only %d of %d cases were blocked by anything; the generator is missing",
			clamped, len(probes))
	}
}

func TestTheShapeResolveMatchesTheGame(t *testing.T) {
	jar := newOracle26(t)

	var scenes []shapeScene
	var commands []string
	for _, seed := range []uint64{11, 13, 17, 19, 23} {
		random := rand.New(rand.NewPCG(seed, 0))
		for range 200 {
			scene := buildShapeScene(random, 4)
			scenes = append(scenes, scene)
			commands = append(commands, scene.commands()...)
			commands = append(commands, fmt.Sprintf("R %s %s %s %s",
				renderBox(scene.body),
				hex(scene.motion.X), hex(scene.motion.Y), hex(scene.motion.Z)))
		}
	}

	answers := jar.run(t, "ShapeOracle26", commands, len(scenes))
	blocked, zeroed := 0, 0
	for index, scene := range scenes {
		got := collision.ResolveShapes(scene.boxes(), scene.body, scene.motion)
		want := parseVec(t, answers[index])
		if !identicalVec(got, want) {
			t.Fatalf("resolve: got %+v, the game says %+v\nbody %+v\nmotion %+v\nboxes %+v",
				got, want, scene.body, scene.motion, scene.boxes())
		}
		if got != scene.motion {
			blocked++
		}
		if (got.X == 0 && scene.motion.X != 0) || (got.Z == 0 && scene.motion.Z != 0) {
			zeroed++
		}
	}

	t.Logf("checked %d shape resolves against the game, %d blocked, %d with an axis zeroed",
		len(scenes), blocked, zeroed)
	if blocked*10 < len(scenes) {
		t.Errorf("only %d of %d resolves hit anything", blocked, len(scenes))
	}
	if zeroed == 0 {
		t.Error("no resolve zeroed a horizontal axis; the tolerance branch never ran")
	}
}

func TestTheStepUpCandidatesMatchTheGame(t *testing.T) {
	jar := newOracle26(t)

	type probe struct {
		scene shapeScene
		limit float32
		skip  float32
	}

	var probes []probe
	var commands []string
	for _, seed := range []uint64{29, 31, 37} {
		random := rand.New(rand.NewPCG(seed, 0))
		for range 200 {
			scene := buildShapeScene(random, 4)
			limit := []float32{0.6, float32(0.6), 1, 0.5}[random.IntN(4)]
			// The skip is the vertical motion a flat move already applied, and
			// zero is by far its most common value, so it is weighted rather
			// than uniform.
			skip := float32(0)
			if random.IntN(3) == 0 {
				skip = float32(scene.motion.Y)
			}

			probes = append(probes, probe{scene: scene, limit: limit, skip: skip})
			commands = append(commands, scene.commands()...)
			commands = append(commands, fmt.Sprintf("H %s %s %s",
				renderBox(scene.body), single(limit), single(skip)))
		}
	}

	answers := jar.run(t, "ShapeOracle26", commands, len(probes))
	offered, several := 0, 0
	for index, p := range probes {
		got := collision.StepHeights(p.scene.body, p.scene.stepCoords(), p.limit, p.skip)
		want := parseHeights(t, answers[index])

		if len(got) != len(want) {
			t.Fatalf("step heights: got %v, the game says %v\nbody %+v\nboxes %+v\nlimit %v skip %v",
				got, want, p.scene.body, p.scene.colliders, p.limit, p.skip)
		}
		for at := range got {
			if got[at] != want[at] {
				t.Fatalf("step height %d: got %v, the game says %v\nfull %v against %v",
					at, got[at], want[at], got, want)
			}
		}
		if len(want) > 0 {
			offered++
		}
		if len(want) > 1 {
			several++
		}
	}

	// An empty list agrees trivially, and a list of one never exercises the
	// ordering. Both are asserted, because this is the piece where 1.8.9's two
	// fixed attempts became a sorted list of whatever the obstacle offers.
	t.Logf("checked %d step-up candidate lists against the game, %d non-empty, %d with several",
		len(probes), offered, several)
	if offered*10 < len(probes) {
		t.Errorf("only %d of %d bodies were offered any step at all", offered, len(probes))
	}
	if several == 0 {
		t.Error("no body was offered more than one height; the ordering is untested")
	}
}

// namedAxis turns the harness's axis letter into the value the clamp takes.
func namedAxis(t *testing.T, name string) collision.Axis {
	t.Helper()

	switch name {
	case "X":
		return collision.AxisX
	case "Y":
		return collision.AxisY
	case "Z":
		return collision.AxisZ
	default:
		t.Fatalf("unknown axis %q", name)

		return collision.AxisX
	}
}

// parseVec reads the three doubles the harness prints for a motion.
func parseVec(t *testing.T, text string) geom.Vec3 {
	t.Helper()

	fields := strings.Fields(text)
	if len(fields) != 3 {
		t.Fatalf("the oracle returned %d fields for a vector: %q", len(fields), text)
	}

	return geom.Vec3{
		X: parseHex(t, fields[0]),
		Y: parseHex(t, fields[1]),
		Z: parseHex(t, fields[2]),
	}
}

// parseHeights reads the count and the heights the harness prints.
func parseHeights(t *testing.T, text string) []float32 {
	t.Helper()

	fields := strings.Fields(text)
	if len(fields) == 0 {
		t.Fatalf("the oracle returned nothing for a height list")
	}
	count, err := strconv.Atoi(fields[0])
	if err != nil {
		t.Fatalf("parse the height count %q: %v", fields[0], err)
	}
	if len(fields) != count+1 {
		t.Fatalf("the oracle said %d heights and printed %d: %q", count, len(fields)-1, text)
	}

	heights := make([]float32, 0, count)
	for _, field := range fields[1:] {
		value, err := strconv.ParseFloat(field, 32)
		if err != nil {
			t.Fatalf("parse the height %q: %v", field, err)
		}
		heights = append(heights, float32(value))
	}

	return heights
}
