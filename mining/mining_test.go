package mining_test

import (
	"errors"
	"math"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/mining"
)

// ptr returns a pointer to a hardness, which is how the generated data carries
// one.
func ptr(value float64) *float64 { return &value }

// ticks breaks a block and fails the test rather than returning an error.
func ticks(t *testing.T, hardness *float64, c mining.Conditions) int {
	t.Helper()

	got, err := mining.BreakTicks(hardness, c)
	if err != nil {
		t.Fatalf("BreakTicks: %v", err)
	}

	return got
}

func withEfficiency(c mining.Conditions, level int) mining.Conditions {
	c.Efficiency = level

	return c
}

func withHaste(c mining.Conditions, amplifier int) mining.Conditions {
	c.Haste, c.HasHaste = amplifier, true

	return c
}

func withFatigue(c mining.Conditions, amplifier int) mining.Conditions {
	c.MiningFatigue, c.HasFatigue = amplifier, true

	return c
}

func withUnderwater(c mining.Conditions) mining.Conditions {
	c.Underwater = true

	return c
}

func withAirborne(c mining.Conditions) mining.Conditions {
	c.Airborne = true

	return c
}

func TestABlockWithNoHardnessIsUnbreakable(t *testing.T) {
	t.Parallel()

	// Bedrock has no hardness. It does not have a hardness of zero, and the
	// generated data says so by declaring Hardness as *float64 and leaving it
	// nil. A rule that dereferences without checking breaks bedrock instantly.
	if _, err := mining.BreakTicks(nil, mining.Conditions{Speed: 1}); !errors.Is(err, mining.ErrUnbreakable) {
		t.Fatalf("err = %v, want ErrUnbreakable", err)
	}

	// The other way a block says it: a negative hardness, which both games
	// answer with zero progress rather than negative progress.
	if _, err := mining.BreakTicks(ptr(-1), mining.Conditions{Speed: 1}); !errors.Is(err, mining.ErrUnbreakable) {
		t.Fatalf("err for a negative hardness = %v, want ErrUnbreakable", err)
	}
}

func TestAZeroHardnessBlockBreaksInOneTick(t *testing.T) {
	t.Parallel()

	// Torches and tall grass have a hardness of zero, which is a real value
	// and must not be confused with the nil above. Vanilla still takes one
	// tick, not zero: a break is a tick's worth of progress reaching one, and
	// the tick it is added in is spent.
	if got := ticks(t, ptr(0), mining.Conditions{Speed: 1, Harvestable: true}); got != 1 {
		t.Fatalf("BreakTicks = %d, want 1", got)
	}
}

func TestHarvestabilityChangesTheDivisorNotTheSpeed(t *testing.T) {
	t.Parallel()

	// A wooden pickaxe on obsidian is the case that separates the two. It is
	// effective — it has a tool speed — and it cannot harvest, so the block
	// breaks far slower than the tool speed alone predicts and drops nothing.
	// Folding harvestability into speed gets the common case right and this
	// one wrong, which is exactly what a matrix test exists to catch.
	fast := ticks(t, ptr(50), mining.Conditions{Speed: 2, Harvestable: true})
	slow := ticks(t, ptr(50), mining.Conditions{Speed: 2})

	if slow <= fast {
		t.Fatalf("unharvestable took %d ticks and harvestable took %d; the "+
			"divisor must differ", slow, fast)
	}
	// And by the ratio the games state, which is the stronger claim: 100
	// against 30. Both counts are rounded up from a fraction, so they agree on
	// the ratio to within the rounding rather than exactly.
	if want := fast * 10 / 3; slow < want-3 || slow > want+3 {
		t.Fatalf("unharvestable took %d ticks against %d harvestable, want "+
			"about %d — the ratio is 100 to 30", slow, fast, want)
	}
}

func TestEachModifierMovesTheResultInTheRightDirection(t *testing.T) {
	t.Parallel()

	base := mining.Conditions{Speed: 4, Harvestable: true}
	baseline := ticks(t, ptr(1.5), base)

	for _, test := range []struct {
		name string
		with mining.Conditions
		want string
	}{
		{"efficiency", withEfficiency(base, 3), "faster"},
		{"haste", withHaste(base, 0), "faster"},
		{"mining fatigue", withFatigue(base, 0), "slower"},
		{"underwater", withUnderwater(base), "slower"},
		{"airborne", withAirborne(base), "slower"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := ticks(t, ptr(1.5), test.with)
			if test.want == "faster" && got >= baseline {
				t.Fatalf("%s gave %d ticks against a baseline of %d, want fewer",
					test.name, got, baseline)
			}
			if test.want == "slower" && got <= baseline {
				t.Fatalf("%s gave %d ticks against a baseline of %d, want more",
					test.name, got, baseline)
			}
		})
	}
}

func TestEfficiencyDoesNotHelpATooltheBlockResists(t *testing.T) {
	t.Parallel()

	// Both games gate the bonus on the speed already exceeding one, so an
	// Efficiency pickaxe swung at a block it has no speed against is a bare
	// hand. A rule that added the bonus unconditionally would make every
	// ineffective tool five times faster than the game does.
	base := mining.Conditions{Speed: 1, Harvestable: true}
	if withBonus, plain := ticks(t, ptr(1.5), withEfficiency(base, 5)), ticks(t, ptr(1.5), base); withBonus != plain {
		t.Fatalf("efficiency changed an ineffective tool from %d ticks to %d",
			plain, withBonus)
	}
}

func TestUnderwaterAndAirborneCompound(t *testing.T) {
	t.Parallel()

	// Both penalties apply at once in vanilla. A rule written as an
	// if-else-if applies whichever it checks first and looks correct in every
	// single-condition test.
	base := mining.Conditions{Speed: 4, Harvestable: true}
	both := ticks(t, ptr(1.5), withAirborne(withUnderwater(base)))
	one := ticks(t, ptr(1.5), withUnderwater(base))

	if both <= one {
		t.Fatalf("underwater and airborne together gave %d ticks, underwater "+
			"alone gave %d; the penalties must compound", both, one)
	}
}

func TestMiningFatigueCanOutweighHaste(t *testing.T) {
	t.Parallel()

	// A player with both effects is not a contrived case: it is what a
	// guardian does to a player wearing a beacon's haste. Vanilla applies both.
	base := mining.Conditions{Speed: 4, Harvestable: true}
	both := ticks(t, ptr(1.5), withFatigue(withHaste(base, 0), 3))
	hasteOnly := ticks(t, ptr(1.5), withHaste(base, 0))

	if both <= hasteOnly {
		t.Fatalf("haste with fatigue gave %d ticks and haste alone gave %d; "+
			"one of the two effects is being ignored", both, hasteOnly)
	}
}

func TestAmplifierZeroIsTheFirstLevelOfAnEffect(t *testing.T) {
	t.Parallel()

	// The protocol sends haste I as amplifier zero, so a rule that treated a
	// zero amplifier as "no effect" would silently drop every first-level
	// effect — which is the level a beacon gives.
	base := mining.Conditions{Speed: 4, Harvestable: true}
	if withHasteI, none := ticks(t, ptr(20), withHaste(base, 0)), ticks(t, ptr(20), base); withHasteI >= none {
		t.Fatalf("haste I gave %d ticks and no haste gave %d", withHasteI, none)
	}
}

func TestAnUnreachableBreakIsCountedRatherThanRefused(t *testing.T) {
	t.Parallel()

	// Fatigue IV against obsidian with a bare hand is a break nobody will wait
	// for — six million ticks, which is three and a half days — and it is not
	// the same claim as "unbreakable": the block has a hardness and a better
	// tool would finish it. Reporting an error would tell a caller to give up
	// for the wrong reason.
	if slow := ticks(t, ptr(50), withFatigue(mining.Conditions{Speed: 1}, 3)); slow < 1_000_000 {
		t.Fatalf("BreakTicks = %d, want a count in the millions", slow)
	}
}

func TestProgressTooSmallToAccumulateNeverBreaks(t *testing.T) {
	t.Parallel()

	// A third outcome, and not a rounding nicety. The game holds the progress
	// in a float32, whose spacing near one is about 6e-8; a per-tick fraction
	// below half of that rounds away and the total stops moving. That is a
	// player mining forever, which is a different answer from "a very long
	// time" and from "unbreakable" alike, and a caller with a timeout is the
	// one that most needs to be told which.
	_, err := mining.BreakTicks(ptr(math.MaxFloat32), withFatigue(mining.Conditions{Speed: 1}, 3))
	if !errors.Is(err, mining.ErrNeverBreaks) {
		t.Fatalf("err = %v, want ErrNeverBreaks", err)
	}
}

func TestDamageIsWhatTheGameCompares(t *testing.T) {
	t.Parallel()

	// The reciprocal relationship is the contract: the crack texture is drawn
	// from the fraction and the tick count is derived from it, so the two must
	// not be computed independently.
	hardness, c := ptr(1.5), mining.Conditions{Speed: 4, Harvestable: true}

	damage, err := mining.Damage(hardness, c)
	if err != nil {
		t.Fatalf("Damage: %v", err)
	}
	if want := int(math.Ceil(1 / damage)); ticks(t, hardness, c) != want {
		t.Fatalf("BreakTicks and Damage disagree: %d against %v", ticks(t, hardness, c), damage)
	}
}

func TestKnownVanillaBreakTimes(t *testing.T) {
	t.Parallel()

	// The check that the transcription is of the right method. Each of these
	// is a number a player can time in game, and they are the same on both
	// editions because the arithmetic is.
	//
	// A tick is a twentieth of a second, so 23 ticks is the 1.15 seconds
	// vanilla takes to break stone with a wooden pickaxe.
	//
	// These are the ticks that add progress. A dig from the button going down
	// costs one more: the tick the button falls on calls clickBlock, which
	// zeroes the progress and adds none, and every tick after it adds.
	for name, test := range map[string]struct {
		hardness float64
		c        mining.Conditions
		want     int
	}{
		"stone with a wooden pickaxe":  {1.5, mining.Conditions{Speed: 2, Harvestable: true}, 23},
		"stone with a diamond pickaxe": {1.5, mining.Conditions{Speed: 8, Harvestable: true}, 6},
		// 151 rather than the 150 a reciprocal gives, and the extra tick is
		// real: the game adds 0.006666667 to a float32 a hundred and fifty
		// times and lands just short of one.
		"stone with a bare hand": {1.5, mining.Conditions{Speed: 1}, 151},
		"dirt with a bare hand":  {0.5, mining.Conditions{Speed: 1, Harvestable: true}, 15},
		"obsidian with a diamond pickaxe": {
			50, mining.Conditions{Speed: 8, Harvestable: true}, 188,
		},
		// The case the harvest divisor exists for: effective and unharvesting.
		"obsidian with a wooden pickaxe": {50, mining.Conditions{Speed: 2}, 2500},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := ticks(t, ptr(test.hardness), test.c); got != test.want {
				t.Fatalf("BreakTicks = %d ticks (%.2fs), want %d (%.2fs)",
					got, float64(got)/20, test.want, float64(test.want)/20)
			}
		})
	}
}
