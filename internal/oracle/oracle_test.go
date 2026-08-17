// Package oracle_test checks this module's collision primitives against the
// ones a real Java Edition 1.8.9 server jar executes.
//
// Every other test in this repository states what vanilla does. These tests
// ask it. A case list is generated here, answered by the jar's own
// AxisAlignedBB methods through the harness in java/AabbOracle.java, and
// compared bit for bit against geom.
//
// The jar is a local, unredistributed artifact prepared by minecraft-reference
// and is not committed. When it, javac, or java is missing, these tests skip:
// a contributor without a prepared workspace still gets a green run, and the
// check is meaningful wherever the workspace exists.
package oracle_test

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// jarPath is where minecraft-reference leaves the deobfuscated 1.8.9 server.
const jarPath = "../../reference/work/versions/1.8.9/server/named.jar"

// oracle is a compiled harness bound to one jar.
type oracle struct {
	classpath string
	// workDir is where the harness runs. The game's logger writes a logs
	// directory beside the working directory as soon as it initializes, so the
	// harness is run outside the repository rather than left to litter it.
	workDir string
}

// newOracle compiles the harness, or skips the test when the workspace or the
// JDK is absent.
func newOracle(t *testing.T) *oracle {
	t.Helper()

	jar, err := filepath.Abs(jarPath)
	if err != nil {
		t.Fatalf("resolve jar path: %v", err)
	}
	if _, err := os.Stat(jar); err != nil {
		t.Skipf("no prepared 1.8.9 server jar at %s; run task reference:prepare", jarPath)
	}
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac is not on PATH")
	}
	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("java is not on PATH")
	}

	classes := t.TempDir()
	build := exec.Command(javac, "-nowarn", "-cp", jar, "-d", classes,
		"java/AabbOracle.java", "java/MoveOracle.java", "java/MovementOracle.java")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compile the oracle harness: %v\n%s", err, out)
	}

	return &oracle{
		classpath: classes + string(os.PathListSeparator) + jar,
		workDir:   t.TempDir(),
	}
}

// ask runs every case through the jar's AxisAlignedBB harness and returns one
// answer per case.
func (o *oracle) ask(t *testing.T, cases []string) []string {
	t.Helper()

	return o.run(t, "AabbOracle", cases, len(cases))
}

// run drives one harness class and requires exactly want answer lines.
func (o *oracle) run(t *testing.T, mainClass string, input []string, want int) []string {
	t.Helper()

	run := exec.Command("java", "-cp", o.classpath, mainClass)
	run.Dir = o.workDir
	run.Stdin = strings.NewReader(strings.Join(input, "\n") + "\n")

	var stdout, stderr bytes.Buffer
	run.Stdout = &stdout
	run.Stderr = &stderr
	if err := run.Run(); err != nil {
		t.Fatalf("run the oracle harness: %v\n%s", err, stderr.String())
	}

	var answers []string
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		answers = append(answers, strings.TrimSpace(scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read the oracle output: %v", err)
	}
	if len(answers) != want {
		t.Fatalf("oracle returned %d answers, want %d", len(answers), want)
	}

	return answers
}

// hex renders a double the way the harness parses it: exactly.
func hex(value float64) string {
	return strconv.FormatFloat(value, 'x', -1, 64)
}

// parseHex reads a double the harness printed.
func parseHex(t *testing.T, text string) float64 {
	t.Helper()

	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		t.Fatalf("parse %q from the oracle: %v", text, err)
	}

	return value
}

// renderBox writes a box as the six doubles the harness expects.
func renderBox(b geom.AABB) string {
	return fmt.Sprintf("%s %s %s %s %s %s",
		hex(b.MinX), hex(b.MinY), hex(b.MinZ), hex(b.MaxX), hex(b.MaxY), hex(b.MaxZ))
}

// identical compares two doubles by their bits, so that a sign of zero or a
// one-ulp drift is a failure rather than a rounding detail.
func identical(a, b float64) bool {
	return math.Float64bits(a) == math.Float64bits(b)
}

// caseWorld generates the geometry the differential tests share: a mover near
// the origin and a block placed around it, at coordinates that land on cell
// boundaries as often as they land between them. Contact cases are where the
// comparison operators matter, so they must not be rare.
func caseWorld(random *rand.Rand) (block, mover geom.AABB, motion float64) {
	// Snap roughly half the coordinates to halves and whole numbers, which is
	// where vanilla's <= and >= comparisons decide the answer.
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

	mover = geom.AABB{MinX: coord(4), MinY: coord(4), MinZ: coord(4)}
	mover.MaxX = mover.MinX + 0.6
	mover.MaxY = mover.MinY + 1.8
	mover.MaxZ = mover.MinZ + 0.6

	block = geom.AABB{MinX: coord(6), MinY: coord(6), MinZ: coord(6)}
	block.MaxX = block.MinX + 1
	block.MaxY = block.MinY + 1
	block.MaxZ = block.MinZ + 1

	motion = coord(8)

	return block, mover, motion
}

// TestClampsMatchTheGame is the core check: the three single-axis clamps must
// return exactly what the jar's calculateXOffset, calculateYOffset, and
// calculateZOffset return, on every case.
func TestClampsMatchTheGame(t *testing.T) {
	jar := newOracle(t)

	type probe struct {
		op     string
		block  geom.AABB
		mover  geom.AABB
		motion float64
	}

	var probes []probe
	var cases []string
	for _, seed := range []uint64{1, 2, 3, 5, 8, 13, 21, 34} {
		random := rand.New(rand.NewPCG(seed, 0))
		for range 500 {
			block, mover, motion := caseWorld(random)
			for _, op := range []string{"X", "Y", "Z"} {
				probes = append(probes, probe{op: op, block: block, mover: mover, motion: motion})
				cases = append(cases, fmt.Sprintf("%s %s %s %s",
					op, renderBox(block), renderBox(mover), hex(motion)))
			}
		}
	}

	answers := jar.ask(t, cases)
	for index, p := range probes {
		want := parseHex(t, answers[index])

		var got float64
		switch p.op {
		case "X":
			got = p.block.ClampX(p.mover, p.motion)
		case "Y":
			got = p.block.ClampY(p.mover, p.motion)
		case "Z":
			got = p.block.ClampZ(p.mover, p.motion)
		}

		if !identical(got, want) {
			t.Fatalf("Clamp%s: got %v, the game says %v\nblock %+v\nmover %+v\nmotion %v",
				p.op, got, want, p.block, p.mover, p.motion)
		}
	}

	t.Logf("checked %d clamp cases against the game", len(cases))
}

// TestStretchMatchesAddCoord checks the sweep expansion against the method
// vanilla gathers its collision candidates with.
func TestStretchMatchesAddCoord(t *testing.T) {
	jar := newOracle(t)

	type probe struct {
		box   geom.AABB
		delta geom.Vec3
	}

	var probes []probe
	var cases []string
	for _, seed := range []uint64{1, 2, 3, 5} {
		random := rand.New(rand.NewPCG(seed, 0))
		for range 500 {
			_, box, _ := caseWorld(random)
			delta := geom.Vec3{
				X: (random.Float64() - 0.5) * 8,
				Y: (random.Float64() - 0.5) * 8,
				Z: (random.Float64() - 0.5) * 8,
			}
			probes = append(probes, probe{box: box, delta: delta})
			cases = append(cases, fmt.Sprintf("A %s %s %s %s",
				renderBox(box), hex(delta.X), hex(delta.Y), hex(delta.Z)))
		}
	}

	answers := jar.ask(t, cases)
	for index, p := range probes {
		got := p.box.Stretch(p.delta)
		want := parseBox(t, answers[index])
		if !identicalBox(got, want) {
			t.Fatalf("Stretch: got %+v, the game says %+v\nbox %+v delta %+v",
				got, want, p.box, p.delta)
		}
	}
}

// TestOffsetMatchesTheGame checks the translation collision applies after
// every axis pass.
func TestOffsetMatchesTheGame(t *testing.T) {
	jar := newOracle(t)

	type probe struct {
		box   geom.AABB
		delta geom.Vec3
	}

	var probes []probe
	var cases []string
	for _, seed := range []uint64{7, 11} {
		random := rand.New(rand.NewPCG(seed, 0))
		for range 500 {
			_, box, _ := caseWorld(random)
			delta := geom.Vec3{
				X: (random.Float64() - 0.5) * 8,
				Y: (random.Float64() - 0.5) * 8,
				Z: (random.Float64() - 0.5) * 8,
			}
			probes = append(probes, probe{box: box, delta: delta})
			cases = append(cases, fmt.Sprintf("O %s %s %s %s",
				renderBox(box), hex(delta.X), hex(delta.Y), hex(delta.Z)))
		}
	}

	answers := jar.ask(t, cases)
	for index, p := range probes {
		got := p.box.Offset(p.delta)
		want := parseBox(t, answers[index])
		if !identicalBox(got, want) {
			t.Fatalf("Offset: got %+v, the game says %+v\nbox %+v delta %+v",
				got, want, p.box, p.delta)
		}
	}
}

// TestIntersectsMatchesTheGame checks the overlap predicate, including the
// touching-faces case the property tests depend on.
func TestIntersectsMatchesTheGame(t *testing.T) {
	jar := newOracle(t)

	type probe struct {
		a, b geom.AABB
	}

	var probes []probe
	var cases []string
	for _, seed := range []uint64{2, 4, 6, 9} {
		random := rand.New(rand.NewPCG(seed, 0))
		for range 500 {
			block, mover, _ := caseWorld(random)
			probes = append(probes, probe{a: mover, b: block})
			cases = append(cases, fmt.Sprintf("I %s %s", renderBox(mover), renderBox(block)))
		}
	}

	answers := jar.ask(t, cases)
	for index, p := range probes {
		got := p.a.Intersects(p.b)
		want := answers[index] == "true"
		if got != want {
			t.Fatalf("Intersects: got %v, the game says %v\na %+v\nb %+v", got, want, p.a, p.b)
		}
	}
}

// parseBox reads the six doubles the harness prints for a box.
func parseBox(t *testing.T, text string) geom.AABB {
	t.Helper()

	fields := strings.Fields(text)
	if len(fields) != 6 {
		t.Fatalf("oracle returned %d fields for a box: %q", len(fields), text)
	}

	return geom.AABB{
		MinX: parseHex(t, fields[0]),
		MinY: parseHex(t, fields[1]),
		MinZ: parseHex(t, fields[2]),
		MaxX: parseHex(t, fields[3]),
		MaxY: parseHex(t, fields[4]),
		MaxZ: parseHex(t, fields[5]),
	}
}

// identicalBox compares every face by its bits.
func identicalBox(a, b geom.AABB) bool {
	return identical(a.MinX, b.MinX) && identical(a.MinY, b.MinY) && identical(a.MinZ, b.MinZ) &&
		identical(a.MaxX, b.MaxX) && identical(a.MaxY, b.MaxY) && identical(a.MaxZ, b.MaxZ)
}
