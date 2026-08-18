package v1_8

import (
	"testing"

	"github.com/go-theft-craft/minecraft-protocol/data"

	"github.com/go-theft-craft/minecraft-simulation/mining"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// classifier returns the profile as the optional interface, asserted rather
// than assumed. New returns sim.Profile, and a seam nobody asserts is the M7
// defect the master plan records.
func classifier(t *testing.T) mining.Classifier {
	t.Helper()

	built, err := New(dataset(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	classifier, ok := built.(mining.Classifier)
	if !ok {
		t.Fatalf("%T does not classify mining", built)
	}

	return classifier
}

// refOf resolves a block by name, failing the test rather than returning zero.
func refOf(t *testing.T, name string) world.BlockRef {
	t.Helper()

	ref, ok := table(t).ref(name)
	if !ok {
		t.Fatalf("this version has no block %q", name)
	}

	return ref
}

// itemOf resolves an item by name to the id the tool tables are keyed by.
func itemOf(t *testing.T, name string) data.ItemID {
	t.Helper()

	item, ok := dataset(t).Items().ByName(name)
	if !ok {
		t.Fatalf("this version has no item %q", name)
	}

	return item.ID
}

func conditions(t *testing.T, block, held string) mining.Conditions {
	t.Helper()

	var hand mining.Held
	if held != "" {
		hand = mining.Held{Item: itemOf(t, held)}
	}

	got, err := classifier(t).Conditions(refOf(t, block), hand, mining.Effects{}, false, false)
	if err != nil {
		t.Fatalf("Conditions(%s, %s): %v", block, held, err)
	}

	return got
}

func TestStoneIsClassifiedByThisVersionsVocabulary(t *testing.T) {
	t.Parallel()

	// 1.8.9 calls stone's material "rock". The name is the point: if a later
	// data regeneration changes it, this fails here rather than as a wrong
	// break time somewhere in a matrix of two hundred cases.
	got := conditions(t, "stone", "wooden_pickaxe")
	if got.Speed != 2 {
		t.Errorf("Speed = %v for a wooden pickaxe on stone, want 2", got.Speed)
	}
	if !got.Harvestable {
		t.Error("a wooden pickaxe cannot harvest stone, according to this profile")
	}
}

func TestAnIneffectiveToolGetsSpeedOneRatherThanZero(t *testing.T) {
	t.Parallel()

	// A shovel on stone is slow, not stuck. Speed zero would make BreakTicks
	// divide by zero, and infinity reads as "unbreakable" to a caller that
	// only checks the error.
	got := conditions(t, "stone", "wooden_shovel")
	if got.Speed != 1 {
		t.Errorf("Speed = %v for a shovel on stone, want 1", got.Speed)
	}
	if got.Harvestable {
		t.Error("a shovel harvests stone, according to this profile")
	}
}

func TestHarvestAndSpeedAreDifferentQuestions(t *testing.T) {
	t.Parallel()

	// The case the whole separation exists for. A wooden pickaxe on obsidian
	// is effective — it has a speed — and cannot harvest it.
	got := conditions(t, "obsidian", "wooden_pickaxe")
	if got.Speed <= 1 {
		t.Errorf("Speed = %v for a wooden pickaxe on obsidian, want a tool speed", got.Speed)
	}
	if got.Harvestable {
		t.Error("a wooden pickaxe harvests obsidian, according to this profile")
	}
}

func TestABlockThatNeedsNoToolIsHarvestableByHand(t *testing.T) {
	t.Parallel()

	// An empty HarvestTools means "no tool required", not "no tool works".
	// Reading it the other way makes every plant and every dirt block drop
	// nothing, which is a hundred wrong break times and no dropped items.
	if got := conditions(t, "dirt", ""); !got.Harvestable {
		t.Error("a bare hand cannot harvest dirt, according to this profile")
	}
}

func TestBedrockHasNoHardness(t *testing.T) {
	t.Parallel()

	if got := classifier(t).Hardness(refOf(t, "bedrock")); got != nil {
		t.Fatalf("Hardness = %v for bedrock, want nil", *got)
	}
	if got := classifier(t).Hardness(refOf(t, "stone")); got == nil || *got != 1.5 {
		t.Fatalf("Hardness for stone = %v, want 1.5", got)
	}
}

func TestAnUnknownHandleIsReportedRatherThanAnswered(t *testing.T) {
	t.Parallel()

	// The zero handle is reserved and names no block. Answering it with a
	// default would give every unminted handle a plausible break time.
	if _, err := classifier(t).Conditions(0, mining.Held{}, mining.Effects{}, false, false); err == nil {
		t.Fatal("Conditions answered for the reserved handle")
	}
}

func TestEveryDiggableBlockResolvesItsMaterial(t *testing.T) {
	t.Parallel()

	// The sweep that catches a vocabulary mismatch wholesale rather than one
	// block at a time. A profile that resolves stone and misses eight hundred
	// other blocks passes every hand-written case above.
	classifier, built := classifier(t), table(t)

	var missed []string
	for _, block := range dataset(t).Blocks().All() {
		if !block.Diggable {
			continue
		}

		ref, ok := built.ref(block.Name)
		if !ok {
			missed = append(missed, block.Name+" (no handle)")

			continue
		}
		if _, err := classifier.Conditions(ref, mining.Held{}, mining.Effects{}, false, false); err != nil {
			missed = append(missed, block.Name)
		}
	}

	if len(missed) != 0 {
		t.Fatalf("%d diggable blocks resolved no material: %v", len(missed), missed[:min(len(missed), 20)])
	}
}

func TestKnownVanillaBreakTimes(t *testing.T) {
	t.Parallel()

	// The end-to-end check: a block name, a tool name, and the number of ticks
	// a player can time in game. It crosses the classifier and the arithmetic
	// together, which is the only way a wrong material lookup shows up as a
	// wrong number rather than as a passing unit test on either side.
	//
	// Each count is the seconds the game takes, times twenty, and two of them
	// are one tick above that: stone and oak by hand are 7.5 and 3.0 seconds
	// by the reciprocal and 151 and 61 ticks by the accumulation the game
	// actually performs. See mining.BreakTicks.
	for name, test := range map[string]struct {
		block, held string
		want        int
	}{
		"stone with a wooden pickaxe":  {"stone", "wooden_pickaxe", 23},
		"stone with a diamond pickaxe": {"stone", "diamond_pickaxe", 6},
		"stone with a bare hand":       {"stone", "", 151},
		"dirt with a bare hand":        {"dirt", "", 15},
		"dirt with a wooden shovel":    {"dirt", "wooden_shovel", 8},
		"oak wood with a bare hand":    {"log", "", 61},
		"oak wood with an iron axe":    {"log", "iron_axe", 10},
		// Effective and unharvesting, which is what the two divisors separate.
		"obsidian with a wooden pickaxe":  {"obsidian", "wooden_pickaxe", 2500},
		"obsidian with a diamond pickaxe": {"obsidian", "diamond_pickaxe", 188},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c := conditions(t, test.block, test.held)

			got, err := mining.BreakTicks(classifier(t).Hardness(refOf(t, test.block)), c)
			if err != nil {
				t.Fatalf("BreakTicks: %v", err)
			}
			if got != test.want {
				t.Fatalf("%s took %d ticks (%.2fs), want %d (%.2fs); conditions were %+v",
					name, got, float64(got)/20, test.want, float64(test.want)/20, c)
			}
		})
	}
}
