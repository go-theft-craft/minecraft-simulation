package oracle_test

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/mctest"
)

// miningCorpusDirectory is where the break-time gate reads its corpora from. It
// sits beside the arithmetic it gates rather than under mctest's testdata,
// because the gate cannot live anywhere else: mining is what mining/vanilla_test
// checks, and a corpus a reader has to go looking for is a corpus nobody reads.
const miningCorpusDirectory = "../../mining/testdata/vanilla"

// miningQuestion is one row of the matrix, in version-neutral terms.
//
// The block and the tool are keys rather than registry names because the two
// versions do not share a vocabulary: 1.8.9's cobweb is "web" and 26.1's is
// "cobweb", 1.8.9's wool is "wool" and 26.1's is "white_wool". Naming them here
// by key and resolving per version is what stops the matrix drifting into
// asking the two versions different questions.
type miningQuestion struct {
	name string
	// block is a key in miningBlocks; tool is a key in miningTools, empty for
	// a bare hand.
	block, tool string
	efficiency  int
	// haste and fatigue are amplifiers, or -1 for the effect being absent.
	haste, fatigue       int
	underwater, airborne bool
	// divergence records that this version's generated data is known to
	// disagree with the jar here. It is keyed by version, because a defect in
	// one version's dataset says nothing about the other's.
	divergence map[string]string
}

// miningBlocks names each block in both vocabularies.
var miningBlocks = map[string]map[string]string{
	"stone":    {"1.8.9": "minecraft:stone", "26.1.2": "minecraft:stone"},
	"dirt":     {"1.8.9": "minecraft:dirt", "26.1.2": "minecraft:dirt"},
	"wood":     {"1.8.9": "minecraft:log", "26.1.2": "minecraft:oak_log"},
	"leaves":   {"1.8.9": "minecraft:leaves", "26.1.2": "minecraft:oak_leaves"},
	"wool":     {"1.8.9": "minecraft:wool", "26.1.2": "minecraft:white_wool"},
	"web":      {"1.8.9": "minecraft:web", "26.1.2": "minecraft:cobweb"},
	"obsidian": {"1.8.9": "minecraft:obsidian", "26.1.2": "minecraft:obsidian"},
	"bedrock":  {"1.8.9": "minecraft:bedrock", "26.1.2": "minecraft:bedrock"},
	"glass":    {"1.8.9": "minecraft:glass", "26.1.2": "minecraft:glass"},
	"ore":      {"1.8.9": "minecraft:gold_ore", "26.1.2": "minecraft:gold_ore"},
	// The hardness-zero case, which is a real hardness rather than an absent
	// one and must cost the tick it is broken in.
	"grass": {"1.8.9": "minecraft:tallgrass", "26.1.2": "minecraft:short_grass"},
}

// miningTools names each tool in both vocabularies. They happen to agree on
// every one of these, and are written out per version anyway: the day one moves,
// the table is where it is said rather than a lookup that silently misses.
var miningTools = map[string]map[string]string{
	"wooden_pickaxe":  {"1.8.9": "minecraft:wooden_pickaxe", "26.1.2": "minecraft:wooden_pickaxe"},
	"diamond_pickaxe": {"1.8.9": "minecraft:diamond_pickaxe", "26.1.2": "minecraft:diamond_pickaxe"},
	"diamond_shovel":  {"1.8.9": "minecraft:diamond_shovel", "26.1.2": "minecraft:diamond_shovel"},
	"diamond_axe":     {"1.8.9": "minecraft:diamond_axe", "26.1.2": "minecraft:diamond_axe"},
	"diamond_sword":   {"1.8.9": "minecraft:diamond_sword", "26.1.2": "minecraft:diamond_sword"},
	"shears":          {"1.8.9": "minecraft:shears", "26.1.2": "minecraft:shears"},
}

// shearsDatasetDefect is the reason the 26.1 lane cannot pass for shears on
// leaves and wool. It is recorded on the case rather than the case being
// dropped, so the day upstream corrects the dataset the gate fails and says so.
// shearsDatasetDefect is why neither lane can pass for shears on leaves and on
// wool. Both versions' generated data gets the same two combinations wrong, for
// the same reason and by different amounts: the extractor kept the rule that
// says which blocks shears mine and dropped the rule that says how fast.
//
// Recorded on the case rather than the case being dropped, so the day upstream
// corrects one, the gate fails and the marker comes off.
var shearsDatasetDefect = map[string]string{
	"1.8.9": "1.8.9's generated data gives shears a speed of 6 against leaves and 4.8 against " +
		"wool; ItemShears.getStrVsBlock returns 15 for leaves and 5 for a cloth material. " +
		"Pinned by TestTheDatasetToolSpeedsThisVersionGetsWrong in profile/java/v1_8.",
	"26.1.2": "26.1's generated data gives shears a speed of 1 against leaves and wool; " +
		"the jar's ShearsItem.createToolProperties overrides them at 15 and 5. " +
		"Pinned by TestTheDatasetToolSpeedsThisVersionGetsWrong in profile/java/v26_1.",
}

// miningMatrix is the sample of the cross product this corpus holds.
//
// The axes are blocks, tools, one enchantment, two effects, and two player
// states, which multiply to thousands of combinations. What is taken is the
// combinations that exercise a distinct branch of the arithmetic; what is not
// taken is recorded in miningDropped and travels with the corpus.
func miningMatrix() []miningQuestion {
	none := func(q miningQuestion) miningQuestion {
		q.haste, q.fatigue = -1, -1

		return q
	}

	var matrix []miningQuestion
	add := func(q miningQuestion) { matrix = append(matrix, none(q)) }

	// Every block with a bare hand: the hardness axis on its own, including the
	// two blocks that cannot be broken and the one whose hardness is zero.
	for _, block := range []string{
		"stone", "dirt", "wood", "leaves", "wool", "web",
		"obsidian", "bedrock", "glass", "ore", "grass",
	} {
		add(miningQuestion{name: block + "/bare-hand", block: block})
	}

	// Every tool against one block: the tool axis on its own. A shovel, an axe,
	// a sword, and shears on stone are the cases where a version could
	// mistakenly treat "not effective" as "cannot break".
	for _, tool := range []string{
		"wooden_pickaxe", "diamond_pickaxe", "diamond_shovel",
		"diamond_axe", "diamond_sword", "shears",
	} {
		add(miningQuestion{name: "stone/" + tool, block: "stone", tool: tool})
	}

	// The tool that matches the block, which is the common case, plus the three
	// where speed and harvest legality come apart.
	add(miningQuestion{name: "dirt/diamond_shovel", block: "dirt", tool: "diamond_shovel"})
	add(miningQuestion{name: "wood/diamond_axe", block: "wood", tool: "diamond_axe"})
	add(miningQuestion{
		name: "wool/shears", block: "wool", tool: "shears",
		divergence: shearsDatasetDefect,
	})
	add(miningQuestion{
		name: "leaves/shears", block: "leaves", tool: "shears",
		divergence: shearsDatasetDefect,
	})
	add(miningQuestion{name: "web/diamond_sword", block: "web", tool: "diamond_sword"})
	add(miningQuestion{name: "web/shears", block: "web", tool: "shears"})
	// Effective and unharvestable: a wooden pickaxe eventually breaks obsidian
	// and drops nothing, and gold ore is the same shape one tier down.
	add(miningQuestion{name: "obsidian/wooden_pickaxe", block: "obsidian", tool: "wooden_pickaxe"})
	add(miningQuestion{name: "obsidian/diamond_pickaxe", block: "obsidian", tool: "diamond_pickaxe"})
	add(miningQuestion{name: "ore/wooden_pickaxe", block: "ore", tool: "wooden_pickaxe"})
	add(miningQuestion{name: "ore/diamond_pickaxe", block: "ore", tool: "diamond_pickaxe"})

	// The enchantment axis. Level one and level five bracket it, and the bare
	// hand with a level-five book is the case both games gate: the bonus only
	// helps a tool whose speed already exceeds one.
	add(miningQuestion{
		name:  "stone/diamond_pickaxe/efficiency1",
		block: "stone", tool: "diamond_pickaxe", efficiency: 1,
	})
	add(miningQuestion{
		name:  "stone/diamond_pickaxe/efficiency5",
		block: "stone", tool: "diamond_pickaxe", efficiency: 5,
	})
	add(miningQuestion{name: "stone/bare-hand/efficiency5", block: "stone", efficiency: 5})
	add(miningQuestion{
		name:  "dirt/diamond_shovel/efficiency1",
		block: "dirt", tool: "diamond_shovel", efficiency: 1,
	})

	// The effect axis. Haste and fatigue each at two amplifiers, the amplifier
	// that puts fatigue in its default branch, and the pair together — which is
	// the case an if-else would get wrong while passing every single-effect
	// test.
	matrix = append(
		matrix,
		miningQuestion{
			name:  "stone/diamond_pickaxe/haste1",
			block: "stone", tool: "diamond_pickaxe", haste: 0, fatigue: -1,
		},
		miningQuestion{
			name:  "stone/diamond_pickaxe/haste2",
			block: "stone", tool: "diamond_pickaxe", haste: 1, fatigue: -1,
		},
		miningQuestion{
			name:  "stone/bare-hand/haste1",
			block: "stone", haste: 0, fatigue: -1,
		},
		miningQuestion{
			name:  "stone/diamond_pickaxe/fatigue1",
			block: "stone", tool: "diamond_pickaxe", haste: -1, fatigue: 0,
		},
		miningQuestion{
			name:  "stone/diamond_pickaxe/fatigue2",
			block: "stone", tool: "diamond_pickaxe", haste: -1, fatigue: 1,
		},
		miningQuestion{
			name:  "stone/diamond_pickaxe/fatigue3",
			block: "stone", tool: "diamond_pickaxe", haste: -1, fatigue: 2,
		},
		miningQuestion{
			name:  "stone/diamond_pickaxe/fatigue4",
			block: "stone", tool: "diamond_pickaxe", haste: -1, fatigue: 3,
		},
		miningQuestion{
			name:  "stone/diamond_pickaxe/haste1+fatigue1",
			block: "stone", tool: "diamond_pickaxe", haste: 0, fatigue: 0,
		},
	)

	// The player axis, singly and together. Both penalties apply at once, and
	// an implementation that chose between them would pass every case but the
	// last.
	add(miningQuestion{
		name:  "stone/diamond_pickaxe/underwater",
		block: "stone", tool: "diamond_pickaxe", underwater: true,
	})
	add(miningQuestion{
		name:  "stone/diamond_pickaxe/airborne",
		block: "stone", tool: "diamond_pickaxe", airborne: true,
	})
	add(miningQuestion{
		name:  "stone/diamond_pickaxe/underwater+airborne",
		block: "stone", tool: "diamond_pickaxe", underwater: true, airborne: true,
	})
	add(miningQuestion{name: "stone/bare-hand/underwater", block: "stone", underwater: true})
	add(miningQuestion{
		name:  "dirt/diamond_shovel/airborne",
		block: "dirt", tool: "diamond_shovel", airborne: true,
	})

	// Every modifier at once, and the case that never finishes: obsidian by
	// hand under the worst fatigue, submerged and airborne, is a fraction the
	// game's own float32 accumulator cannot add up.
	matrix = append(
		matrix,
		miningQuestion{
			name:  "stone/diamond_pickaxe/everything",
			block: "stone", tool: "diamond_pickaxe", efficiency: 5,
			haste: 1, fatigue: 0, underwater: true, airborne: true,
		},
		miningQuestion{
			name:  "obsidian/bare-hand/never-breaks",
			block: "obsidian", haste: -1, fatigue: 3, underwater: true, airborne: true,
		},
	)

	return matrix
}

// miningDropped is what this matrix does not ask, and why. It is written into
// every corpus, because a sample that does not say what it left out reads as
// having covered everything.
var miningDropped = []string{
	fmt.Sprintf("The full cross product is 11 blocks × 7 tools × 3 enchantment levels × "+
		"4 effect combinations × 4 player states = 3696 cases. This matrix takes %d.",
		len(miningMatrix())),
	"Tool tiers between wooden and diamond (stone, iron, golden): the tier changes " +
		"one number in a table the version already resolves, and the two ends bracket it.",
	"Efficiency levels 2 through 4: the bonus is level² + 1 with no branch between " +
		"levels, so 1 and 5 bracket every value of it.",
	"Haste amplifiers above 1: the scaling is linear in the amplifier with no branch. " +
		"Fatigue is the opposite and is taken at every amplifier it branches on.",
	"Most block × tool pairs: what changes a break time is the material and the harvest " +
		"legality, not the identity of either, so one pair per branch is taken.",
	"The 26.1-only tools (copper, and the tier materials 1.8.9 has no counterpart for): " +
		"a case a version cannot ask its counterpart is a one-version lane, and this " +
		"stage refuses those.",
}

// TestGenerateMiningCorpus records the break-time matrix from both games.
//
// The flag that makes it write is the movement generator's, deliberately: one
// run rewrites every committed expectation or none of them, and regenerating is
// a deliberate act with a diff to read.
//
// Without the flag it checks that the committed corpora still say what the jars
// say, which is what makes a stale corpus a failure rather than a quiet pass.
func TestGenerateMiningCorpus(t *testing.T) {
	for _, lane := range []struct {
		version string
		file    string
		class   string
		build   func(*testing.T) *oracle
	}{
		{version: "1.8.9", file: "1_8_9.json", class: "MiningOracle", build: newOracle},
		{version: "26.1.2", file: "26_1_2.json", class: "MiningOracle26", build: newOracle26},
	} {
		t.Run(lane.version, func(t *testing.T) {
			jar := lane.build(t)

			matrix := miningMatrix()
			questions := make([]string, 0, len(matrix))
			for _, question := range matrix {
				questions = append(questions, renderMiningQuestion(t, lane.version, question))
			}

			answers := jar.runTagged(t, lane.class, questions, len(questions))

			corpus := mctest.MiningCorpus{
				Version: lane.version,
				Source: fmt.Sprintf(
					"asked of a Java Edition %s server jar through internal/oracle/java/%s.java",
					lane.version, lane.class,
				),
				Dropped: miningDropped,
			}
			for at, question := range matrix {
				corpus.Cases = append(corpus.Cases,
					readMiningAnswer(t, lane.version, question, answers[at]))
			}

			path := filepath.Join(miningCorpusDirectory, lane.file)
			if !*writeFixtures {
				committed, err := mctest.LoadMiningCorpus(path)
				if err != nil {
					t.Fatalf("%s: %v (pass -write-fixtures to record it)", lane.version, err)
				}
				compareMiningCorpora(t, committed, corpus)

				return
			}

			if err := corpus.Save(path); err != nil {
				t.Fatalf("%s: save: %v", lane.version, err)
			}
			t.Logf("wrote %s: %d cases", path, len(corpus.Cases))
		})
	}
}

// runTagged drives a harness whose jar writes to standard output too.
//
// The mining harnesses load the game rather than only bootstrapping it, and
// loading writes progress lines — "Loaded 1515 recipes" — onto the same stream
// an answer goes out on. So an answer is marked, and everything unmarked is the
// game talking. run cannot do this for every harness: a harness that answers
// with an unmarked line would have every answer discarded, which reads as a
// harness that said nothing.
func (o *oracle) runTagged(t *testing.T, mainClass string, input []string, want int) []string {
	t.Helper()

	run := exec.Command("java", "-cp", o.classpath, mainClass)
	run.Dir = o.workDir
	run.Stdin = strings.NewReader(strings.Join(input, "\n") + "\n")

	var stdout, stderr bytes.Buffer
	run.Stdout = &stdout
	run.Stderr = &stderr
	if err := run.Run(); err != nil {
		t.Fatalf("run the %s harness: %v\n%s", mainClass, err, stderr.String())
	}

	var answers []string
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if marked, ok := strings.CutPrefix(line, "A "); ok {
			answers = append(answers, marked)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read the %s output: %v", mainClass, err)
	}
	if len(answers) != want {
		t.Fatalf("%s answered %d times, want %d\n%s", mainClass, len(answers), want, stderr.String())
	}

	return answers
}

// renderMiningQuestion writes one question in the harness's protocol.
func renderMiningQuestion(t *testing.T, version string, question miningQuestion) string {
	t.Helper()

	block, ok := miningBlocks[question.block][version]
	if !ok {
		t.Fatalf("%s: no block named for %q", version, question.block)
	}

	held := "-"
	if question.tool != "" {
		named, ok := miningTools[question.tool][version]
		if !ok {
			t.Fatalf("%s: no tool named for %q", version, question.tool)
		}
		held = named
	}

	return fmt.Sprintf("Q %s %s %d %d %d %t %t",
		block, held, question.efficiency, question.haste, question.fatigue,
		question.underwater, question.airborne)
}

// readMiningAnswer turns one harness line into a case.
func readMiningAnswer(
	t *testing.T, version string, question miningQuestion, answer string,
) mctest.MiningCase {
	t.Helper()

	fields := strings.Fields(answer)
	if len(fields) != 5 {
		t.Fatalf("%s/%s: the harness answered %q", version, question.name, answer)
	}

	var ticks int
	if _, err := fmt.Sscanf(fields[3], "%d", &ticks); err != nil {
		t.Fatalf("%s/%s: read the tick count from %q: %v", version, question.name, fields[3], err)
	}

	held := ""
	if question.tool != "" {
		held = miningTools[question.tool][version]
	}

	return mctest.MiningCase{
		Name:          question.name,
		Block:         miningBlocks[question.block][version],
		Held:          held,
		Efficiency:    question.efficiency,
		Haste:         max(question.haste, 0),
		MiningFatigue: max(question.fatigue, 0),
		HasHaste:      question.haste >= 0,
		HasFatigue:    question.fatigue >= 0,
		Underwater:    question.underwater,
		Airborne:      question.airborne,
		Hardness:      fields[0],
		Speed:         fields[1],
		Damage:        fields[2],
		Ticks:         ticks,
		Harvestable:   fields[4] == "true",
		Divergence:    question.divergence[version],
	}
}

// compareMiningCorpora fails on any difference between what is committed and
// what the jar just said.
//
// Field by field rather than by a whole-struct comparison, because the failure
// message is the point: "committed 151 ticks, the jar says 150" is a report, and
// a diff of two structs is a puzzle.
func compareMiningCorpora(t *testing.T, committed, fresh mctest.MiningCorpus) {
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
		if got.Hardness != want.Hardness || got.Speed != want.Speed ||
			got.Damage != want.Damage || got.Ticks != want.Ticks ||
			got.Harvestable != want.Harvestable {
			t.Errorf("%s: the corpus says hardness %s speed %s damage %s ticks %d harvestable %t; "+
				"the jar says %s %s %s %d %t",
				want.Name, got.Hardness, got.Speed, got.Damage, got.Ticks, got.Harvestable,
				want.Hardness, want.Speed, want.Damage, want.Ticks, want.Harvestable)
		}
	}
}
