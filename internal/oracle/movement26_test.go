package oracle_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	gen26 "github.com/go-theft-craft/minecraft-protocol/generated/java/v26_1"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/movement"
	v26_1 "github.com/go-theft-craft/minecraft-simulation/profile/java/v26_1"
	"github.com/go-theft-craft/minecraft-simulation/runtime"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// The attributes the two moving scenarios carry. A sprinting body's
// movement-speed attribute gains a modifier of thirty percent, computed as a
// double and narrowed where the tick reads it. This milestone takes the speed as
// an input, so it is stated here rather than derived: whatever fills Locomotion
// owns the attribute, and the tick multiplies by what it is handed.
var (
	walkMoveSpeed26   = float32(0.1)
	sprintMoveSpeed26 = float32(0.1 * 1.3)
)

// surfaces26 are the frictions a random world scatters over its floor.
//
// Blue ice is here because this version has it and 1.8.9 does not, and because
// its 0.989 is the one value that is neither the default nor the 0.98 the other
// ices carry.
//
// Three blocks are deliberately absent, and each absence is a rule this
// milestone does not implement rather than a block that is awkward to place.
// Slime negates a landing body's vertical motion where an ordinary block zeroes
// it, and the landing callback is per block; soul sand and honey multiply the
// horizontal motion by a speed factor the dataset does not publish. A world
// containing any of the three would be testing a rule nobody has written, and
// the oracle would report the divergence as a failure of the tick.
var surfaces26 = []string{"ice", "packed_ice", "blue_ice"}

// movementScenarios26 are the six the milestone's exit criterion names.
//
// Every one of them runs over a randomly obstructed floor, so walking is never
// only walking: a scattering of stone, slabs, and three different ices puts
// step-ups, blocked axes, and four frictions into all six.
func movementScenarios26() []movementScenario {
	standing := geom.Vec3{X: 0.5, Y: 1, Z: 0.5}

	return []movementScenario{
		{
			name: "walk", spawn: standing, moveSpeed: walkMoveSpeed26,
			input: func(tick int, yaw float32) movement.Input {
				// The facing turns through the run, and past zero, because the
				// sine table is read at a truncated index and truncation toward
				// zero is what a negative angle disagrees about. This version
				// truncates a double through a long where 1.8.9 truncates a float
				// through an int, so the same sweep is a different check here.
				return movement.Input{
					Entity: movementEntity, Forward: 1, Yaw: yaw + float32(tick)*7 - 180,
				}
			},
		},
		{
			name: "sprint", spawn: standing, moveSpeed: sprintMoveSpeed26,
			input: func(tick int, yaw float32) movement.Input {
				return movement.Input{
					Entity: movementEntity, Forward: 1, Sprint: true,
					Yaw: yaw + float32(tick)*3,
				}
			},
		},
		{
			name: "jump", spawn: standing, moveSpeed: walkMoveSpeed26,
			input: func(tick int, yaw float32) movement.Input {
				// Held for stretches rather than every tick, so that the counter
				// is exercised both while it runs down and after a release
				// zeroes it.
				return movement.Input{
					Entity: movementEntity, Forward: 1, Yaw: yaw, Jump: tick%13 < 9,
				}
			},
		},
		{
			name: "sneak", spawn: standing, moveSpeed: walkMoveSpeed26,
			input: func(tick int, yaw float32) movement.Input {
				// Both axes, because sneaking is where the client's shaping shows:
				// a diagonal input reaches the clamp and a single axis does not.
				return movement.Input{
					Entity: movementEntity, Forward: 1, Strafe: 1, Sneak: true,
					Yaw: yaw - float32(tick)*5,
				}
			},
		},
		{
			name: "fall", spawn: geom.Vec3{X: 0.5, Y: 8, Z: 0.5}, airborne: true,
			moveSpeed: walkMoveSpeed26,
			input: func(_ int, yaw float32) movement.Input {
				// Steering in the air is the only customer of the airborne speed,
				// so the fall carries input rather than none.
				return movement.Input{Entity: movementEntity, Forward: 1, Yaw: yaw}
			},
		},
		{
			name: "collide", spawn: standing, moveSpeed: walkMoveSpeed26,
			obstacles: func(place func(pos geom.BlockPos, name string)) {
				// A wall two blocks away and three high, so the body reaches it
				// within a few ticks and cannot step over it.
				for z := int32(-4); z <= 4; z++ {
					for y := int32(1); y <= 3; y++ {
						place(geom.BlockPos{X: 2, Y: y, Z: z}, "stone")
					}
				}
			},
			input: func(_ int, yaw float32) movement.Input {
				return movement.Input{
					Entity: movementEntity, Forward: 1, Yaw: -90 + yaw/24,
				}
			},
		},
	}
}

// buildMovementRun26 generates one world and the intents run against it.
//
// It is the 1.8.9 generator with this version's block names and surfaces, and
// with one addition: the floor is described a cell deeper, because this version's
// friction reads the block a body stands on through a probe that sweeps the cells
// around its feet, and that sweep reaches a cell below the floor.
func buildMovementRun26(scenario movementScenario, room movementRoom, seed uint64) movementRun {
	random := rand.New(rand.NewPCG(seed, 0))

	run := movementRun{scenario: scenario, room: room, seed: seed}
	run.yaw = float32(random.Float64()*720 - 360)

	span := func(from, to geom.BlockPos, name string) {
		run.placed = append(run.placed, movementPlacement{min: from, max: to, name: name})
	}
	place := func(pos geom.BlockPos, name string) { span(pos, pos, name) }

	span(
		geom.BlockPos{X: -room.radius, Y: 0, Z: -room.radius},
		geom.BlockPos{X: room.radius, Y: 0, Z: room.radius},
		"stone",
	)

	for _, wall := range []struct{ min, max geom.BlockPos }{
		{geom.BlockPos{X: -room.wall, Y: 1, Z: -room.wall}, geom.BlockPos{X: room.wall, Y: 6, Z: -room.wall}},
		{geom.BlockPos{X: -room.wall, Y: 1, Z: room.wall}, geom.BlockPos{X: room.wall, Y: 6, Z: room.wall}},
		{geom.BlockPos{X: -room.wall, Y: 1, Z: -room.wall}, geom.BlockPos{X: -room.wall, Y: 6, Z: room.wall}},
		{geom.BlockPos{X: room.wall, Y: 1, Z: -room.wall}, geom.BlockPos{X: room.wall, Y: 6, Z: room.wall}},
	} {
		span(wall.min, wall.max, "stone")
	}

	inside := func() int32 {
		return int32(random.IntN(int(2*room.wall-1))) - room.wall + 1
	}
	for range room.surfaces {
		place(geom.BlockPos{X: inside(), Y: 0, Z: inside()}, surfaces26[random.IntN(len(surfaces26))])
	}

	for range room.obstacles {
		pos := geom.BlockPos{X: inside(), Y: int32(random.IntN(2)) + 1, Z: inside()}
		// The spawn column stays clear, so that a body never starts inside a
		// block: what the game does then is a separate question from movement,
		// and one Task 4a recorded a known divergence for.
		if pos.X >= -1 && pos.X <= 1 && pos.Z >= -1 && pos.Z <= 1 {
			continue
		}
		name := "stone"
		if random.IntN(2) == 0 {
			// This version's half block. 1.8.9 calls it stone_slab.
			name = "smooth_stone_slab"
		}
		place(pos, name)
	}

	if scenario.obstacles != nil {
		scenario.obstacles(place)
	}

	for tick := range room.ticks {
		run.inputs = append(run.inputs, scenario.input(tick, run.yaw))
	}

	return run
}

// commands26 renders this run as the harness protocol.
//
// The two axes are shaped before they are sent, because the entity tick this
// harness drives no longer decays them: a client's own player replaces that decay
// with the shaping, and the profile's phase does the same. The I command is what
// checks the shaping itself.
func commands26(r movementRun) []string {
	commands := []string{"C"}
	for _, block := range r.placed {
		block.cells(func(pos geom.BlockPos) {
			commands = append(commands, fmt.Sprintf("B %d %d %d %s",
				pos.X, pos.Y, pos.Z, block.name))
		})
	}

	commands = append(commands, fmt.Sprintf("S %s %s %s %s %s %t %s",
		hex(r.scenario.spawn.X), hex(r.scenario.spawn.Y), hex(r.scenario.spawn.Z),
		single(r.yaw), single(0), !r.scenario.airborne, single(r.scenario.moveSpeed)))

	for _, input := range r.inputs {
		strafe, forward := v26_1.ShapeInput(input.Strafe, input.Forward, input.Sneak)
		commands = append(commands, fmt.Sprintf("T %s %s %s %s %t %t",
			single(strafe), single(forward), single(input.Yaw), single(input.Pitch),
			input.Jump, input.Sprint))
	}

	return commands
}

// movementProfile26 builds the profile under test.
func movementProfile26(t *testing.T) sim.Profile {
	t.Helper()

	set, err := gen26.Data()
	if err != nil {
		t.Fatalf("load the 26.1.2 data set: %v", err)
	}

	profile, err := v26_1.New(set)
	if err != nil {
		t.Fatalf("build the 26.1.2 profile: %v", err)
	}

	return profile
}

// TestTheInputShapingMatchesTheGamesArithmetic checks the one rule in the
// profile that lives in a class this jar does not carry.
//
// The composition is transcribed on both sides — the harness builds it out of the
// game's own Vec2 and Mth, and the profile builds it out of Go — so what this
// compares is the widths and the order, which is where a transcription goes
// wrong. It is a weaker check than the rest of this file makes, and the gap is
// stated rather than hidden.
func TestTheInputShapingMatchesTheGamesArithmetic(t *testing.T) {
	jar := newOracle26(t)

	type shaping struct {
		strafe, forward float32
		sneaking        bool
	}

	var cases []shaping
	// The keyboard's own nine, sneaking and not.
	for _, strafe := range []float32{-1, 0, 1} {
		for _, forward := range []float32{-1, 0, 1} {
			cases = append(cases, shaping{strafe, forward, false})
			cases = append(cases, shaping{strafe, forward, true})
		}
	}
	// And a spread of fractional intents, which is what a controller sends and
	// where the stretch onto the unit square stops being the identity.
	random := rand.New(rand.NewPCG(7, 0))
	for range 60 {
		cases = append(cases, shaping{
			strafe:   float32(random.Float64()*2 - 1),
			forward:  float32(random.Float64()*2 - 1),
			sneaking: random.IntN(2) == 0,
		})
	}

	commands := make([]string, 0, len(cases))
	for _, one := range cases {
		commands = append(commands, fmt.Sprintf("I %s %s %t",
			single(one.strafe), single(one.forward), one.sneaking))
	}

	answers := jar.run(t, "MovementOracle26", commands, len(cases))
	for index, one := range cases {
		fields := strings.Fields(answers[index])
		if len(fields) != 2 {
			t.Fatalf("the shaping answer is %q", answers[index])
		}

		gotStrafe, gotForward := v26_1.ShapeInput(one.strafe, one.forward, one.sneaking)
		wantStrafe := parseHexFloat(t, fields[0])
		wantForward := parseHexFloat(t, fields[1])

		if gotStrafe != wantStrafe || gotForward != wantForward {
			t.Fatalf("shaping (%v, %v) sneaking=%v gives (%v, %v), the game says (%v, %v)",
				one.strafe, one.forward, one.sneaking,
				gotStrafe, gotForward, wantStrafe, wantForward)
		}
	}

	t.Logf("checked %d shaped inputs against the game's own arithmetic", len(cases))
}

// TestOneTickOfStandingMatchesTheGame26 is the smoke test: the smallest
// trajectory there is, checked before the differential test runs a hundred.
//
// A body handed no motion is airborne for its first tick, because the vertical
// collision compares the motion it moved with against the motion it asked for and
// a body asking for nothing is stopped by nothing. That is what the game does,
// and it is worth pinning on its own: if this disagrees, nothing the larger test
// reports will be easy to read.
func TestOneTickOfStandingMatchesTheGame26(t *testing.T) {
	jar := newOracle26(t)
	profile := movementProfile26(t)

	run := movementRun{
		room: movementRoom{radius: 3, wall: 3, floor: -2, ceiling: 4, ticks: 5},
		scenario: movementScenario{
			name:      "stand",
			spawn:     geom.Vec3{X: 0.5, Y: 1, Z: 0.5},
			moveSpeed: walkMoveSpeed26,
			input: func(int, float32) movement.Input {
				return movement.Input{Entity: movementEntity}
			},
		},
	}
	run.placed = append(run.placed, movementPlacement{
		min:  geom.BlockPos{X: -3, Y: 0, Z: -3},
		max:  geom.BlockPos{X: 3, Y: 0, Z: 3},
		name: "stone",
	})
	for tick := range 5 {
		run.inputs = append(run.inputs, run.scenario.input(tick, 0))
	}

	answers := jar.run(t, "MovementOracle26", commands26(run), run.answers())
	compareMovementRun26(t, profile, run, answers)
}

// TestMovementMatchesTheGame26 is the milestone's gate.
//
// The same six scenarios M8.4 gates 1.8.9 on, over the same number of random
// worlds, compared every tick rather than at the end. The first differing tick
// names the phase that diverged; an endpoint comparison would name only the
// scenario.
func TestMovementMatchesTheGame26(t *testing.T) {
	jar := newOracle26(t)
	profile := movementProfile26(t)

	var runs []movementRun
	var commands []string
	want := 0
	for _, scenario := range movementScenarios26() {
		for _, seed := range movementSeeds {
			run := buildMovementRun26(scenario, wideRoom, seed)
			runs = append(runs, run)
			commands = append(commands, commands26(run)...)
			want += run.answers()
		}
	}

	answers := jar.run(t, "MovementOracle26", commands, want)

	at := 0
	var total movementCoverage
	for _, run := range runs {
		total = total.add(compareMovementRun26(t, profile, run, answers[at:at+run.answers()]))
		at += run.answers()
	}

	t.Logf("checked %d ticks against the game over %d worlds: %+v",
		want-len(runs), len(runs), total)

	// The worlds are random, so what they contain is asserted rather than
	// assumed: a gate that quietly stopped exercising a rule must fail.
	if total.collided == 0 {
		t.Error("no tick collided horizontally; the collide scenario is not reaching its wall")
	}
	if total.airborne == 0 {
		t.Error("no tick was airborne; the fall and jump scenarios are not leaving the ground")
	}
	if total.landed == 0 {
		t.Error("no tick landed; the vertical clamp is never exercised")
	}
	if total.rose == 0 {
		t.Error("no tick rose; no jump ever fired")
	}
	if total.stepped == 0 {
		t.Error("no tick stepped up; the obstacles are never being climbed")
	}
}

// compareMovementRun26 simulates one run and compares every tick against the
// game's answer for it.
func compareMovementRun26(
	t *testing.T, profile sim.Profile, run movementRun, answers []string,
) movementCoverage {
	t.Helper()

	store := movementStore26(t, profile, run)

	state, loco, ok := v26_1.Spawn(profile, run.scenario.spawn, run.yaw, 0)
	if !ok {
		t.Fatal("Spawn did not recognize its own profile")
	}
	state.OnGround = !run.scenario.airborne
	loco.MoveSpeed = run.scenario.moveSpeed
	store.SetEntity(movementEntity, state)
	store.SetLocomotion(movementEntity, loco)

	label := fmt.Sprintf("%s/seed %d", run.scenario.name, run.seed)
	spawned := parseMovementState(t, answers[0])
	if !identicalBox(state.Box, spawned.box) {
		t.Fatalf("%s: the spawned body is %+v, the game says %+v", label, state.Box, spawned.box)
	}

	kernel, err := sim.NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	runner := runtime.NewRunner(store, kernel)
	runner.SetScope(sim.Scope{Entities: []entity.ID{movementEntity}})

	var coverage movementCoverage
	previous := state
	for tick, input := range run.inputs {
		result, err := runner.Step(context.Background(), []sim.Command{input})
		if err != nil {
			t.Fatalf("%s tick %d: Step: %v", label, tick, err)
		}
		if !result.Completeness.Complete {
			t.Fatalf("%s tick %d left the described world: %+v",
				label, tick, result.Completeness.Missing)
		}

		got, ok := store.Entities().Entity(movementEntity)
		if !ok {
			t.Fatalf("%s tick %d: the body is gone", label, tick)
		}
		want := parseMovementState(t, answers[1+tick])

		if !identicalBox(got.Box, want.box) {
			t.Fatalf("%s tick %d: body %s\n     the game says %s\n     input %+v",
				label, tick, formatBox(got.Box), formatBox(want.box), input)
		}
		if !identicalVec(got.Motion, want.motion) {
			t.Fatalf("%s tick %d: motion %s\n     the game says %s\n     input %+v",
				label, tick, formatVec(got.Motion), formatVec(want.motion), input)
		}
		if got.OnGround != want.onGround {
			t.Fatalf("%s tick %d: onGround %v, the game says %v",
				label, tick, got.OnGround, want.onGround)
		}
		if collided := hasEvent(result.Domain, "movement.collided"); collided != want.collidedHorizontally {
			t.Fatalf("%s tick %d: collided horizontally %v, the game says %v",
				label, tick, collided, want.collidedHorizontally)
		}

		if want.collidedHorizontally {
			coverage.collided++
		}
		if !want.onGround {
			coverage.airborne++
		}
		if !previous.OnGround && got.OnGround {
			coverage.landed++
		}
		if want.motion.Y > 0 {
			coverage.rose++
		}
		if previous.OnGround && got.OnGround && got.Box.MinY > previous.Box.MinY {
			coverage.stepped++
		}
		previous = got
	}

	return coverage
}

// movementStore26 describes the run's world to a store, air included, so that no
// sweep can reach a cell nobody described.
//
// The described region reaches one cell below the room's floor, because this
// version's friction probe sweeps the cells around a body's feet with the
// margin the game's own block sweep uses, and that margin reaches under the
// floor.
func movementStore26(t *testing.T, profile sim.Profile, run movementRun) *runtime.Memory {
	t.Helper()

	air, ok := v26_1.Ref(profile, "air")
	if !ok {
		t.Fatal("the profile does not know air")
	}

	store := runtime.NewMemory(profile)
	for x := -run.room.radius - 1; x <= run.room.radius+1; x++ {
		for y := run.room.floor - 1; y <= run.room.ceiling+1; y++ {
			for z := -run.room.radius - 1; z <= run.room.radius+1; z++ {
				if err := store.SetBlock(geom.BlockPos{X: x, Y: y, Z: z}, air); err != nil {
					t.Fatalf("SetBlock: %v", err)
				}
			}
		}
	}

	for _, block := range run.placed {
		ref, ok := v26_1.Ref(profile, block.name)
		if !ok {
			t.Fatalf("the profile does not know %q", block.name)
		}
		var failure error
		block.cells(func(pos geom.BlockPos) {
			if err := store.SetBlock(pos, ref); err != nil {
				failure = err
			}
		})
		if failure != nil {
			t.Fatalf("SetBlock: %v", failure)
		}
	}

	return store
}
