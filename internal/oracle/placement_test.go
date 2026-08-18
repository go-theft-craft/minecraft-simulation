package oracle_test

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/mctest"
)

// placementCorpusDirectory is where the placement gate reads its corpora from.
const placementCorpusDirectory = "../../placement/testdata/vanilla"

// placementQuestion is one click, in version-neutral terms.
//
// The item is a key rather than a registry name because the two versions do not
// agree on the names: 1.8.9's oak slab is "wooden_slab" and 26.1's is
// "oak_slab", 1.8.9's log is "log" and 26.1's is "oak_log".
type placementQuestion struct {
	name string
	// item is a key in placementItems.
	item string
	// face is the wire's own numbering, as mining.Face uses it.
	face uint8
	// cursor is where in the clicked face the click landed, block-local.
	cursor [3]float64
	yaw    float32
}

// placementItems names each item in both vocabularies.
var placementItems = map[string]map[string]string{
	"stone":  {"1.8.9": "minecraft:stone", "26.1.2": "minecraft:stone"},
	"stairs": {"1.8.9": "minecraft:oak_stairs", "26.1.2": "minecraft:oak_stairs"},
	"slab":   {"1.8.9": "minecraft:stone_slab", "26.1.2": "minecraft:smooth_stone_slab"},
	"log":    {"1.8.9": "minecraft:log", "26.1.2": "minecraft:oak_log"},
}

// placementMatrix is what the corpus asks.
//
// The axes are the ones a placement rule branches on: the face clicked, where
// in that face the cursor landed, and which way the player was looking. Each
// family is asked the questions its own rule reads and no others — a log does
// not read the yaw, and asking it four times over would pad the corpus without
// covering anything.
func placementMatrix() []placementQuestion {
	middle := [3]float64{0.5, 0.5, 0.5}

	matrix := []placementQuestion{
		// A block with no orientation, clicked from every side. Whatever the
		// click, the answer is the block's default state — and on 26.1.2 that
		// is not the same number as zero.
		{name: "stone/top", item: "stone", face: 1, cursor: [3]float64{0.5, 1, 0.5}},
		{name: "stone/bottom", item: "stone", face: 0, cursor: [3]float64{0.5, 0, 0.5}},
		{name: "stone/side", item: "stone", face: 5, cursor: [3]float64{1, 0.5, 0.5}, yaw: 137},
	}

	// Stairs take their facing from the player's yaw and their half from the
	// face and the cursor. The four yaws are the quadrant centres; the two
	// extra ones are the boundaries the game's rounding decides.
	for _, yaw := range []float32{0, 90, 180, 270, 45, 359} {
		matrix = append(matrix, placementQuestion{
			name: "stairs/yaw" + strconv.Itoa(int(yaw)),
			item: "stairs", face: 1, cursor: [3]float64{0.5, 1, 0.5}, yaw: yaw,
		})
	}
	matrix = append(matrix,
		placementQuestion{
			name: "stairs/underside", item: "stairs",
			face: 0, cursor: [3]float64{0.5, 0, 0.5},
		},
		placementQuestion{
			name: "stairs/side-low", item: "stairs",
			face: 5, cursor: [3]float64{1, 0.2, 0.5},
		},
		placementQuestion{
			name: "stairs/side-high", item: "stairs",
			face: 5, cursor: [3]float64{1, 0.8, 0.5},
		},
		// The exact half, which the game decides with a comparison that is not
		// symmetric: hitY <= 0.5 is the bottom half.
		placementQuestion{
			name: "stairs/side-exactly-half", item: "stairs",
			face: 5, cursor: [3]float64{1, 0.5, 0.5},
		},
	)

	// A slab takes only the half, by the same rule.
	matrix = append(matrix,
		placementQuestion{name: "slab/top-face", item: "slab", face: 1, cursor: [3]float64{0.5, 1, 0.5}},
		placementQuestion{name: "slab/underside", item: "slab", face: 0, cursor: [3]float64{0.5, 0, 0.5}},
		placementQuestion{name: "slab/side-low", item: "slab", face: 3, cursor: [3]float64{0.5, 0.2, 1}},
		placementQuestion{name: "slab/side-high", item: "slab", face: 3, cursor: [3]float64{0.5, 0.8, 1}},
		placementQuestion{
			name: "slab/side-exactly-half", item: "slab",
			face: 3, cursor: [3]float64{0.5, 0.5, 1},
		},
	)

	// A log takes only the axis, from the face alone. All six, because the
	// three axes come in pairs and a rule that read the sign would pass on
	// three of them.
	for face := range uint8(6) {
		matrix = append(matrix, placementQuestion{
			name: "log/face" + strconv.Itoa(int(face)),
			item: "log", face: face, cursor: middle, yaw: 41,
		})
	}

	return matrix
}

// placementCovers is what the corpus checks, said where a reader of a green run
// sees it.
var placementCovers = []string{
	"blocks with no orientation, which take their default state",
	"stairs: facing from the player's yaw, half from the face and the cursor",
	"slabs: half from the face and the cursor",
	"logs and pillars: axis from the face",
}

// placementDropped is what it does not check, and why.
var placementDropped = []string{
	"Every other orientable family — doors, torches, buttons, levers, ladders, " +
		"beds, chests, furnaces, pistons, repeaters, rails. The rule answers a " +
		"default state for each, which is right for none of them, and a stage " +
		"that adds one adds its cases here.",
	"Blocks that orient themselves in onBlockPlacedBy rather than onBlockPlaced " +
		"(1.8.9) — a bed, a piston, a chest. The harness asks the placement call " +
		"only, so a corpus of them would record a state the game replaces a tick " +
		"later.",
	"Neighbour-dependent shapes: a stair's inner and outer corners, a fence's " +
		"connections, a redstone wire's arms. The clicked cell's surroundings are " +
		"one stone block and nothing else, so every answer here is the shape a " +
		"placement takes before its neighbours are consulted.",
	"Waterlogging, which is decided by the fluid in the placed cell and the cell " +
		"in this harness is air.",
}

// TestGeneratePlacementCorpus records what each game does with a click.
//
// The flag that makes it write is the movement generator's, deliberately: one
// run rewrites every committed expectation or none of them.
func TestGeneratePlacementCorpus(t *testing.T) {
	for _, lane := range []struct {
		version string
		file    string
		class   string
		build   func(*testing.T) *oracle
	}{
		{version: "1.8.9", file: "1_8_9.json", class: "PlacementOracle", build: newOracle},
		{version: "26.1.2", file: "26_1_2.json", class: "PlacementOracle26", build: newOracle26},
	} {
		t.Run(lane.version, func(t *testing.T) {
			jar := lane.build(t)

			matrix := placementMatrix()
			questions := make([]string, 0, len(matrix))
			for _, question := range matrix {
				questions = append(questions, renderPlacementQuestion(t, lane.version, question))
			}

			answers := jar.runTagged(t, lane.class, questions, len(questions))

			corpus := mctest.PlacementCorpus{
				Version: lane.version,
				Source: fmt.Sprintf(
					"asked of a Java Edition %s server jar through internal/oracle/java/%s.java",
					lane.version, lane.class),
				Covers:  placementCovers,
				Dropped: placementDropped,
			}
			for at, question := range matrix {
				corpus.Cases = append(corpus.Cases,
					readPlacementAnswer(t, lane.version, question, answers[at]))
			}

			path := filepath.Join(placementCorpusDirectory, lane.file)
			if !*writeFixtures {
				committed, err := mctest.LoadPlacementCorpus(path)
				if err != nil {
					t.Fatalf("%s: %v (pass -write-fixtures to record it)", lane.version, err)
				}
				comparePlacementCorpora(t, committed, corpus)

				return
			}

			if err := corpus.Save(path); err != nil {
				t.Fatalf("%s: save: %v", lane.version, err)
			}
			t.Logf("wrote %s: %d cases", path, len(corpus.Cases))
		})
	}
}

// placementClicked is the cell every case clicks, and placementPlayer is where
// the player stands. Both are constants: what a placement rule reads is the
// face, the cursor, and the yaw, and moving the player between cases would
// leave a reader wondering which of the four mattered.
var (
	placementClicked = [3]int32{0, 64, 0}
	placementPlayer  = [3]float64{0.5, 65, 3.5}
)

// renderPlacementQuestion writes one question in the harness's protocol.
func renderPlacementQuestion(t *testing.T, version string, question placementQuestion) string {
	t.Helper()

	item, ok := placementItems[question.item][version]
	if !ok {
		t.Fatalf("%s: no item named for %q", version, question.item)
	}

	return fmt.Sprintf("P %s %d %d %d %d %g %g %g %g %g %g %g %g",
		item, placementClicked[0], placementClicked[1], placementClicked[2],
		question.face, question.cursor[0], question.cursor[1], question.cursor[2],
		question.yaw, 0.0,
		placementPlayer[0], placementPlayer[1], placementPlayer[2])
}

// readPlacementAnswer turns one harness line into a case.
func readPlacementAnswer(
	t *testing.T, version string, question placementQuestion, answer string,
) mctest.PlacementCase {
	t.Helper()

	fields := strings.Fields(answer)
	if len(fields) != 2 {
		t.Fatalf("%s/%s: the harness answered %q", version, question.name, answer)
	}
	if fields[0] == "none" {
		t.Fatalf("%s/%s: this version places no block for %s",
			version, question.name, placementItems[question.item][version])
	}

	state, err := strconv.Atoi(fields[0])
	if err != nil {
		t.Fatalf("%s/%s: read the state from %q: %v", version, question.name, fields[0], err)
	}

	return mctest.PlacementCase{
		Name:    question.name,
		Item:    placementItems[question.item][version],
		Clicked: placementClicked,
		Face:    question.face,
		Cursor:  question.cursor,
		Yaw:     question.yaw,
		Player:  placementPlayer,
		Block:   fields[1],
		State:   state,
	}
}

// comparePlacementCorpora fails on any difference between what is committed and
// what the jar just said.
func comparePlacementCorpora(t *testing.T, committed, fresh mctest.PlacementCorpus) {
	t.Helper()

	if len(committed.Cases) != len(fresh.Cases) {
		t.Fatalf("the committed corpus holds %d cases and the matrix asks %d; "+
			"pass -write-fixtures to record the change",
			len(committed.Cases), len(fresh.Cases))
	}

	for at, want := range fresh.Cases {
		got := committed.Cases[at]
		if got.Name != want.Name {
			t.Fatalf("case %d is %q in the corpus and %q in the matrix", at, got.Name, want.Name)
		}
		if got.Block != want.Block || got.State != want.State {
			t.Errorf("%s: the corpus says %s state %d; the jar says %s state %d",
				want.Name, got.Block, got.State, want.Block, want.State)
		}
	}
}
