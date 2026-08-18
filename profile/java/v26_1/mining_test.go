package v26_1

import (
	"slices"
	"strings"
	"testing"

	"github.com/go-theft-craft/minecraft-protocol/data"

	"github.com/go-theft-craft/minecraft-simulation/mining"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// classifier returns the profile as the optional interface, asserted rather
// than assumed.
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

// refOf resolves a block by name to the handle of its default state.
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

	// This version calls stone's material "mineable/pickaxe" where 1.8.9 calls
	// it "rock". They are not renames of one another, and a lookup shared
	// between the versions would resolve neither.
	got := conditions(t, "stone", "wooden_pickaxe")
	if got.Speed != 2 {
		t.Errorf("Speed = %v for a wooden pickaxe on stone, want 2", got.Speed)
	}
	if !got.Harvestable {
		t.Error("a wooden pickaxe cannot harvest stone, according to this profile")
	}
}

func TestATierTaggedBlockRecoversItsToolClass(t *testing.T) {
	t.Parallel()

	// The defect this version's classifier exists for. Gold ore's material is
	// "incorrect_for_wooden_tool", whose table lists the four wooden tools at
	// speed two and nothing else — so read as a speed table it gives a diamond
	// pickaxe no speed at all, and a wooden shovel the pickaxe's.
	if got := conditions(t, "gold_ore", "diamond_pickaxe"); got.Speed != 8 {
		t.Errorf("Speed = %v for a diamond pickaxe on gold ore, want 8", got.Speed)
	}
	if got := conditions(t, "gold_ore", "wooden_shovel"); got.Speed != 1 {
		t.Errorf("Speed = %v for a wooden shovel on gold ore, want 1: a shovel "+
			"is not a pickaxe, whatever the tier table lists", got.Speed)
	}

	// And the tier is still what decides the drop, which is the question the
	// tag was actually answering.
	if conditions(t, "gold_ore", "stone_pickaxe").Harvestable {
		t.Error("a stone pickaxe harvests gold ore, according to this profile")
	}
	if !conditions(t, "gold_ore", "iron_pickaxe").Harvestable {
		t.Error("an iron pickaxe cannot harvest gold ore, according to this profile")
	}
}

func TestACompoundMaterialIsOneRegistryKey(t *testing.T) {
	t.Parallel()

	// "leaves;mineable/hoe" looks like two materials joined and is one key,
	// with its own merged table. Splitting on the semicolon and merging by hand
	// would recompute what upstream already states.
	if got := conditions(t, "oak_leaves", "iron_hoe"); got.Speed <= 1 {
		t.Errorf("Speed = %v for a hoe on leaves; the compound material did not resolve", got.Speed)
	}
}

func TestAnIneffectiveToolGetsSpeedOneRatherThanZero(t *testing.T) {
	t.Parallel()

	got := conditions(t, "stone", "wooden_shovel")
	if got.Speed != 1 {
		t.Errorf("Speed = %v for a shovel on stone, want 1", got.Speed)
	}
	if got.Harvestable {
		t.Error("a shovel harvests stone, according to this profile")
	}
}

func TestBedrockIsUnbreakableByANegativeHardnessHere(t *testing.T) {
	t.Parallel()

	// The two versions say "unbreakable" differently, and this is the half a
	// shared rule would get wrong. 1.8.9 leaves bedrock's hardness absent;
	// 26.1.2 records it as -1, which is the game's own sentinel —
	// BlockBehaviour.getDestroyProgress returns zero progress for a destroy
	// speed of -1.0F. A rule that only checked for nil would compute a
	// negative break time here and call it fast.
	hardness := classifier(t).Hardness(refOf(t, "bedrock"))
	if hardness == nil || *hardness != -1 {
		t.Fatalf("Hardness = %v for bedrock, want -1", hardness)
	}
	if _, err := mining.BreakTicks(hardness, mining.Conditions{Speed: 8, Harvestable: true}); err == nil {
		t.Fatal("bedrock broke")
	}

	if got := classifier(t).Hardness(refOf(t, "stone")); got == nil || *got != 1.5 {
		t.Fatalf("Hardness for stone = %v, want 1.5", got)
	}
}

func TestAnUnknownHandleIsReportedRatherThanAnswered(t *testing.T) {
	t.Parallel()

	if _, err := classifier(t).Conditions(0, mining.Held{}, mining.Effects{}, false, false); err == nil {
		t.Fatal("Conditions answered for the reserved handle")
	}
}

func TestEveryDiggableBlockResolvesItsMaterial(t *testing.T) {
	t.Parallel()

	// The sweep. It is stronger here than on 1.8.9, because it also checks
	// that a tier-tagged block recovered a tool class: a block that names one
	// of the correct tools and still answers speed one for it is a block whose
	// class the recovery missed.
	classifier, built, set := classifier(t), table(t), dataset(t)
	copper := copperTools(t)

	var missed, unrecovered []string
	for _, block := range set.Blocks().All() {
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

			continue
		}

		for tool := range block.HarvestTools {
			if copper[tool] {
				continue
			}

			got, err := classifier.Conditions(ref, mining.Held{Item: tool}, mining.Effects{}, false, false)
			if err != nil || got.Speed <= 1 {
				unrecovered = append(unrecovered, block.Name)

				break
			}
		}
	}

	if len(missed) != 0 {
		t.Errorf("%d diggable blocks resolved no material: %v", len(missed), missed[:min(len(missed), 20)])
	}

	// The crafter is the one block whose class cannot be recovered: it names
	// no harvest tools at all, so there is nothing to match a tool class
	// against. Named rather than tolerated silently, so that a dataset that
	// fixes it fails here and the exception can go.
	//
	// The copper tools are excluded from the sweep above rather than listed
	// here, because their speed is wrong in the dataset for every block rather
	// than missing for some. See TestTheDatasetToolSpeedsThisVersionGetsWrong.
	if len(unrecovered) != 0 {
		t.Errorf("blocks whose correct tool has no speed: %v",
			unrecovered[:min(len(unrecovered), 20)])
	}

	// And the recovery itself, which the loop above cannot see: a tier-tagged
	// block that resolved no tool class answers speed one for every tool, and
	// it names no harvest tools for the loop to have noticed. The crafter is
	// the one block in this version's dataset in that position — it carries
	// the wooden tier tag and publishes no harvest tools at all.
	mining, err := newMiningTable(set)
	if err != nil {
		t.Fatalf("newMiningTable: %v", err)
	}

	var unclassed []string
	for name, block := range mining.blocks {
		if strings.HasPrefix(block.material, tierMaterialPrefix) && block.speeds == nil {
			unclassed = append(unclassed, name)
		}
	}
	slices.Sort(unclassed)

	if !slices.Equal(unclassed, []string{"crafter"}) {
		t.Errorf("tier-tagged blocks with no recovered tool class: %v, want exactly [crafter]", unclassed)
	}
}

func TestKnownVanillaBreakTimes(t *testing.T) {
	t.Parallel()

	// The end-to-end check, across the classifier and the arithmetic together.
	// Each count is the seconds the game takes, times twenty, except where the
	// float32 accumulation costs one more tick than the reciprocal.
	for name, test := range map[string]struct {
		block, held string
		want        int
	}{
		"stone with a wooden pickaxe":  {"stone", "wooden_pickaxe", 23},
		"stone with a diamond pickaxe": {"stone", "diamond_pickaxe", 6},
		"stone with a bare hand":       {"stone", "", 151},
		"dirt with a bare hand":        {"dirt", "", 15},
		"dirt with a wooden shovel":    {"dirt", "wooden_shovel", 8},
		// The tier-tagged blocks, which is where this version's recovery is
		// the difference between a right answer and a sixfold-wrong one.
		"gold ore with an iron pickaxe":     {"gold_ore", "iron_pickaxe", 15},
		"gold ore with a wooden pickaxe":    {"gold_ore", "wooden_pickaxe", 151},
		"obsidian with a diamond pickaxe":   {"obsidian", "diamond_pickaxe", 188},
		"ancient debris with a netherite p": {"ancient_debris", "netherite_pickaxe", 101},
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

// copperTools are the item ids whose recorded speed this dataset gets wrong.
// Resolved from the registry rather than written down, because an item id is a
// dataset fact and a hardcoded 919 would rot the first time one moved.
func copperTools(t *testing.T) map[data.ItemID]bool {
	t.Helper()

	tools := make(map[data.ItemID]bool, 5)
	for _, name := range []string{
		"copper_pickaxe", "copper_shovel", "copper_axe", "copper_hoe", "copper_sword",
	} {
		tools[itemOf(t, name)] = true
	}

	return tools
}

func TestTheDatasetToolSpeedsThisVersionGetsWrong(t *testing.T) {
	t.Parallel()

	// This is a record of an upstream defect, not a behaviour this profile
	// wants. Every number on the right is read off 26.1.2's own jar, and every
	// number the dataset gives instead is on the left. While these differ, the
	// break times this profile computes for the named combinations are wrong,
	// and no amount of testing on this side will make them right — the fix is
	// in the data.
	//
	// Pinned rather than worked around, for two reasons. Overriding a dataset
	// value with a constant typed into the profile is the one thing this
	// module does not do; and pinned, the day upstream fixes one of these,
	// this test fails and the exception goes.
	for name, test := range map[string]struct {
		block, held  string
		dataset, jar float64
	}{
		// ToolMaterial.COPPER declares a speed of 5.0F, between stone's 4 and
		// iron's 6. The dataset gives every copper tool a 1, which is the
		// value an unlisted tool gets — so a copper pickaxe mines stone at a
		// bare hand's rate here and at better than a stone pickaxe's in game.
		"a copper pickaxe on stone": {"stone", "copper_pickaxe", 1, 5},
		"a copper shovel on dirt":   {"dirt", "copper_shovel", 1, 5},
		// ShearsItem.createToolProperties overrides the speed for leaves at
		// 15.0F and for wool at 5.0F. The dataset carries neither, though it
		// does carry the same file's cobweb rule at 15 — so this is not the
		// extractor missing shears, it is the extractor missing the
		// overrideSpeed rules and keeping the minesAndDrops one.
		"shears on leaves": {"oak_leaves", "shears", 1, 15},
		"shears on wool":   {"white_wool", "shears", 1, 5},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := conditions(t, test.block, test.held).Speed; got != test.dataset {
				t.Fatalf("%s has speed %v, and this test records the dataset as "+
					"saying %v — the dataset changed, so check it against the "+
					"jar's %v and delete this case if it is now right",
					name, got, test.dataset, test.jar)
			}
		})
	}
}
