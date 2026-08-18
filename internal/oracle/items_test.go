package oracle_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/movement"
	"github.com/go-theft-craft/minecraft-simulation/runtime"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// itemEntity is the body every item trajectory simulates.
const itemEntity = entity.ID(1)

// The size 1.8.9 gives a dropped item, from EntityItem's own setSize.
const (
	itemWidth  = 0.25
	itemHeight = 0.25
)

// itemRun is one trajectory: a world, a spawn with a motion, and a tick count.
type itemRun struct {
	name   string
	placed []movementPlacement
	spawn  geom.Vec3
	motion geom.Vec3
	ticks  int
}

func (r itemRun) commands() []string {
	commands := []string{"C"}
	for _, block := range r.placed {
		block.cells(func(pos geom.BlockPos) {
			commands = append(commands, fmt.Sprintf("B %d %d %d %s",
				pos.X, pos.Y, pos.Z, block.name))
		})
	}

	commands = append(commands, fmt.Sprintf("S %s %s %s %s %s %s",
		hex(r.spawn.X), hex(r.spawn.Y), hex(r.spawn.Z),
		hex(r.motion.X), hex(r.motion.Y), hex(r.motion.Z)))

	for range r.ticks {
		commands = append(commands, "T")
	}

	return commands
}

// answers is how many lines this run's commands produce: the spawn and a tick.
func (r itemRun) answers() int { return 1 + r.ticks }

// TestADroppedItemMatchesTheGame is M9.2's primary gate for the item family.
//
// A wire capture cannot do this: a server sends an item's position once every
// twenty ticks, so nothing taken off the wire says what a single tick did. The
// jar does, and this compares every tick bit for bit — the gravity that runs
// before the move, the friction taken from the block the item landed on, the two
// drags, and the bounce.
func TestADroppedItemMatchesTheGame(t *testing.T) {
	jar := newOracle(t)
	profile := movementProfile(t)

	runs := itemRuns()

	var commands []string
	want := 0
	for _, run := range runs {
		commands = append(commands, run.commands()...)
		want += run.answers()
	}

	answers := jar.run(t, "ItemOracle", commands, want)

	at := 0
	var landed, slid int
	for _, run := range runs {
		l, s := compareItemRun(t, profile, run, answers[at:at+run.answers()])
		landed += l
		slid += s
		at += run.answers()
	}

	t.Logf("checked %d item ticks against the game over %d runs", want-len(runs), len(runs))

	// The runs are asserted to have done the things the rules are about, so a
	// gate that quietly stopped exercising one fails rather than passes.
	if landed == 0 {
		t.Error("no run reached the floor; the friction and the bounce never ran")
	}
	if slid == 0 {
		t.Error("no run slid along the floor; the block's own friction never entered")
	}
}

// itemRuns are the trajectories the gate compares.
//
// Ice is in the list because an item takes the friction of the block it ended
// the tick on, at that block's own slipperiness: a run over stone alone would
// agree with a rule that ignored the block entirely.
//
// Slime is deliberately not, and the reason is a gap rather than an oversight.
// The game gives a block a callback on landing — slime reverses the vertical
// motion, and a bed does something else again — and this module zeroes it for
// every block because the dataset publishes no such property. An item landing
// on slime therefore disagrees with the game by the whole bounce, which was
// measured here at 0.129 blocks a tick on the sixth tick of a slide. That is a
// finding about the block table and it belongs to whichever milestone gives a
// block a landing rule; putting slime in this list would report it as an item
// defect every run.
func itemRuns() []itemRun {
	floor := func(name string) movementPlacement {
		return movementPlacement{
			min:  geom.BlockPos{X: -16, Y: 0, Z: -16},
			max:  geom.BlockPos{X: 16, Y: 0, Z: 16},
			name: name,
		}
	}

	runs := []itemRun{
		{
			name:   "dropped from a height",
			placed: []movementPlacement{floor("stone")},
			spawn:  geom.Vec3{X: 0.5, Y: 6, Z: 0.5},
			ticks:  40,
		},
		{
			name:   "tossed the way a player drops one",
			placed: []movementPlacement{floor("stone")},
			spawn:  geom.Vec3{X: 0.5, Y: 3, Z: 0.5},
			// The velocity a 1.8.9 player's drop gives an item, without the
			// random horizontal part: 0.3 forward and 0.1 up.
			motion: geom.Vec3{X: 0.3, Y: 0.1, Z: 0},
			ticks:  40,
		},
		{
			name:   "sliding on ice",
			placed: []movementPlacement{floor("ice")},
			spawn:  geom.Vec3{X: 0.5, Y: 2, Z: 0.5},
			motion: geom.Vec3{X: 0.4, Z: 0.2},
			ticks:  60,
		},
	}

	// And a spread of random launches, so the gate is not four trajectories
	// somebody chose.
	random := rand.New(rand.NewPCG(0x1751, 0x9))
	for index := range 6 {
		runs = append(runs, itemRun{
			name:   fmt.Sprintf("random launch %d", index),
			placed: []movementPlacement{floor("stone")},
			spawn:  geom.Vec3{X: 0.5, Y: 2 + random.Float64()*3, Z: 0.5},
			motion: geom.Vec3{
				X: random.Float64()*0.6 - 0.3,
				Y: random.Float64() * 0.4,
				Z: random.Float64()*0.6 - 0.3,
			},
			ticks: 50,
		})
	}

	return runs
}

// compareItemRun drives one trajectory through both and compares every tick.
func compareItemRun(
	t *testing.T, profile sim.Profile, run itemRun, answers []string,
) (landed, slid int) {
	t.Helper()

	// The room is described to its edges, air included, so a sweep cannot reach
	// a cell nobody described: an incomplete tick is then a fault in this test's
	// world rather than a finding about the item.
	store := movementStore(t, profile, movementRun{
		room:   movementRoom{radius: 16, floor: -1, ceiling: 10},
		placed: run.placed,
	})

	state := entity.State{
		Family: entity.FamilyItem,
		Box:    movement.Box(run.spawn, itemWidth, itemHeight),
		Motion: run.motion,
	}
	store.SetEntity(itemEntity, state)

	spawned := parseMovementState(t, answers[0])
	if !identicalBox(state.Box, spawned.box) {
		t.Fatalf("%s: the spawned item is %s, the game says %s",
			run.name, formatBox(state.Box), formatBox(spawned.box))
	}

	kernel, err := sim.NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	runner := runtime.NewRunner(store, kernel)
	runner.SetScope(sim.Scope{Entities: []entity.ID{itemEntity}})

	previous := state
	for tick := range run.ticks {
		result, err := runner.Step(context.Background(), nil)
		if err != nil {
			t.Fatalf("%s tick %d: Step: %v", run.name, tick, err)
		}
		if !result.Completeness.Complete {
			t.Fatalf("%s tick %d left the described world: %+v",
				run.name, tick, result.Completeness.Missing)
		}

		got, ok := store.Entities().Entity(itemEntity)
		if !ok {
			t.Fatalf("%s tick %d: the item is gone", run.name, tick)
		}
		want := parseMovementState(t, answers[1+tick])

		if !identicalBox(got.Box, want.box) {
			t.Fatalf("%s tick %d: item %s\n     the game says %s",
				run.name, tick, formatBox(got.Box), formatBox(want.box))
		}
		if !identicalVec(got.Motion, want.motion) {
			t.Fatalf("%s tick %d: motion %s\n     the game says %s",
				run.name, tick, formatVec(got.Motion), formatVec(want.motion))
		}
		if got.OnGround != want.onGround {
			t.Fatalf("%s tick %d: onGround %v, the game says %v",
				run.name, tick, got.OnGround, want.onGround)
		}

		if !previous.OnGround && got.OnGround {
			landed++
		}
		if got.OnGround && (got.Box.MinX != previous.Box.MinX || got.Box.MinZ != previous.Box.MinZ) {
			slid++
		}
		previous = got
	}

	return landed, slid
}
