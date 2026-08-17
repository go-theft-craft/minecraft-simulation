package oracle_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"

	gen "github.com/go-theft-craft/minecraft-protocol/generated/java/v1_8"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/movement"
	v1_8 "github.com/go-theft-craft/minecraft-simulation/profile/java/v1_8"
	"github.com/go-theft-craft/minecraft-simulation/runtime"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// movementEntity is the body every trajectory here simulates.
const movementEntity = entity.ID(1)

// movementRoom is the world a set of trajectories runs in.
//
// A room is described to its edges and enclosed by walls, so that a run of
// sprinting cannot reach a cell nobody described: an incomplete tick is then a
// bug in the test that built the room rather than a finding about the tick.
type movementRoom struct {
	// radius is how far the described region reaches horizontally, and wall is
	// where the walls stand. The gap between them is what keeps a sweep that
	// touches a wall inside the description.
	radius int32
	wall   int32
	// floor and ceiling bound the described region vertically.
	floor   int32
	ceiling int32
	// surfaces and obstacles are how many of each the room scatters.
	surfaces  int
	obstacles int
	// ticks is how long each trajectory runs.
	ticks int
}

// wideRoom is what the differential gate runs in: large enough that a hundred
// ticks of sprinting stays interesting, and cheap because nothing is written
// down.
var wideRoom = movementRoom{
	radius: 12, wall: 11, floor: -2, ceiling: 10,
	surfaces: 60, obstacles: 40, ticks: 100,
}

// compactRoom is what the committed fixtures record. It is smaller and shorter
// on purpose: a fixture is a conformance check that has to live in the
// repository and be read in a diff, and the exhaustive coverage is the
// differential test's job.
var compactRoom = movementRoom{
	radius: 6, wall: 5, floor: -2, ceiling: 10,
	surfaces: 12, obstacles: 10, ticks: 40,
}

// movementSeeds are the worlds and facings each scenario is run against.
var movementSeeds = []uint64{1, 2, 3, 5, 8, 13, 21, 34}

// sneakFactor is the scale a sneaking client applies to both input axes before
// it sends them.
//
// It is applied here rather than asked of the jar because it is not part of the
// entity tick: the 1.8.9 client scales its movement input on the way out, and
// the server entity this harness drives never sees an unscaled value. The
// profile folds the same scale into its input-decay phase, so this expression is
// what that phase must agree with, at the width the client computes it — a
// double multiply narrowed to a float. profile/java/v1_8 pins it too.
const sneakFactor = 0.3

// The player's attributes for the two scenarios that change them. A sprinting
// 1.8.9 player carries a movement-speed modifier of +30% and an air factor
// raised by the same fraction, both computed as doubles and narrowed. They are
// stated here because this milestone takes them as inputs: whatever fills
// Locomotion owns them, and the tick only multiplies by what it is handed.
var (
	walkMoveSpeed    = float32(0.1)
	walkJumpFactor   = float32(0.02)
	sprintMoveSpeed  = float32(0.1 * 1.3)
	sprintJumpFactor = float32(float64(float32(0.02)) + float64(float32(0.02))*0.3)
)

// movementState is one answer from the harness: where the body ended up and
// what it hit.
type movementState struct {
	box      geom.AABB
	motion   geom.Vec3
	onGround bool
	// collidedHorizontally and collidedVertically are the game's own flags. Ours
	// are reported as domain events rather than stored on the body, so only the
	// horizontal one is compared: it is the one a consumer acts on.
	collidedHorizontally bool
	collidedVertically   bool
}

// movementScenario is one labelled trajectory: what the body is, and what the
// controller asks of it each tick.
type movementScenario struct {
	name string
	// spawn is where the feet start, and airborne says whether the body starts
	// off the ground.
	spawn    geom.Vec3
	airborne bool
	// moveSpeed and jumpFactor are the attributes the tick multiplies by.
	moveSpeed  float32
	jumpFactor float32
	// obstacles are the blocks this scenario needs beyond the random world.
	obstacles func(place func(pos geom.BlockPos, name string))
	// input is the controller's intent for one tick. The yaw a seed picked is
	// passed in so that every scenario runs against a spread of facings.
	input func(tick int, yaw float32) movement.Input
}

// movementScenarios are the six the milestone's exit criterion names.
//
// Every one of them runs over a randomly obstructed floor as well, so walking is
// never only walking: a scattering of stone, slabs, ice, slime, and soul sand
// puts step-ups, blocked axes, and four different frictions into all six.
func movementScenarios() []movementScenario {
	standing := geom.Vec3{X: 0.5, Y: 1, Z: 0.5}

	return []movementScenario{
		{
			name: "walk", spawn: standing,
			moveSpeed: walkMoveSpeed, jumpFactor: walkJumpFactor,
			input: func(tick int, yaw float32) movement.Input {
				// The facing turns through the run, and past zero, because the
				// sine table is read at a truncated index and truncation toward
				// zero is what a negative angle disagrees about.
				return movement.Input{
					Entity: movementEntity, Forward: 1, Yaw: yaw + float32(tick)*7 - 180,
				}
			},
		},
		{
			name: "sprint", spawn: standing,
			moveSpeed: sprintMoveSpeed, jumpFactor: sprintJumpFactor,
			input: func(tick int, yaw float32) movement.Input {
				return movement.Input{
					Entity: movementEntity, Forward: 1, Sprint: true,
					Yaw: yaw + float32(tick)*3,
				}
			},
		},
		{
			name: "jump", spawn: standing,
			moveSpeed: walkMoveSpeed, jumpFactor: walkJumpFactor,
			input: func(tick int, yaw float32) movement.Input {
				// Held for stretches rather than every tick, so that the counter
				// is exercised both while it runs down and after a release
				// zeroes it.
				return movement.Input{
					Entity: movementEntity, Forward: 1, Yaw: yaw,
					Jump: tick%13 < 9,
				}
			},
		},
		{
			name: "sneak", spawn: standing,
			moveSpeed: walkMoveSpeed, jumpFactor: walkJumpFactor,
			input: func(tick int, yaw float32) movement.Input {
				return movement.Input{
					Entity: movementEntity, Forward: 1, Strafe: 1, Sneak: true,
					Yaw: yaw - float32(tick)*5,
				}
			},
		},
		{
			name: "fall", spawn: geom.Vec3{X: 0.5, Y: 8, Z: 0.5}, airborne: true,
			moveSpeed: walkMoveSpeed, jumpFactor: walkJumpFactor,
			input: func(_ int, yaw float32) movement.Input {
				// Steering in the air is the jump factor's only customer, so the
				// fall carries input rather than none.
				return movement.Input{Entity: movementEntity, Forward: 1, Yaw: yaw}
			},
		},
		{
			name: "collide", spawn: standing,
			moveSpeed: walkMoveSpeed, jumpFactor: walkJumpFactor,
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
				// Yaw -90 faces +X, which is the wall. The seed's facing is added
				// as a small jitter so the approach is not always square on.
				return movement.Input{
					Entity: movementEntity, Forward: 1, Yaw: -90 + yaw/24,
				}
			},
		},
	}
}

// movementRun is one scenario against one random world.
type movementRun struct {
	scenario movementScenario
	room     movementRoom
	seed     uint64
	yaw      float32
	// placed is every non-air block, in placement order, so that the harness,
	// the store, and any fixture written from this run are built from one list.
	placed []movementPlacement
	inputs []movement.Input
}

// movementPlacement is a run of identical blocks, from min to max inclusive.
//
// A floor and a wall are each one placement rather than a hundred, which keeps
// a fixture written from a run small enough to read.
type movementPlacement struct {
	min  geom.BlockPos
	max  geom.BlockPos
	name string
}

// cells walks every block a placement covers.
func (p movementPlacement) cells(visit func(geom.BlockPos)) {
	for x := p.min.X; x <= p.max.X; x++ {
		for y := p.min.Y; y <= p.max.Y; y++ {
			for z := p.min.Z; z <= p.max.Z; z++ {
				visit(geom.BlockPos{X: x, Y: y, Z: z})
			}
		}
	}
}

// buildMovementRun generates one world and the intents run against it.
func buildMovementRun(scenario movementScenario, room movementRoom, seed uint64) movementRun {
	random := rand.New(rand.NewPCG(seed, 0))

	run := movementRun{scenario: scenario, room: room, seed: seed}
	// Facings spread over a full turn including negative ones.
	run.yaw = float32(random.Float64()*720 - 360)

	span := func(from, to geom.BlockPos, name string) {
		run.placed = append(run.placed, movementPlacement{min: from, max: to, name: name})
	}
	place := func(pos geom.BlockPos, name string) {
		span(pos, pos, name)
	}

	// The floor.
	span(
		geom.BlockPos{X: -room.radius, Y: 0, Z: -room.radius},
		geom.BlockPos{X: room.radius, Y: 0, Z: room.radius},
		"stone",
	)

	// Walls, so that a run of sprinting stays inside the described region.
	for _, wall := range []struct{ min, max geom.BlockPos }{
		{geom.BlockPos{X: -room.wall, Y: 1, Z: -room.wall}, geom.BlockPos{X: room.wall, Y: 6, Z: -room.wall}},
		{geom.BlockPos{X: -room.wall, Y: 1, Z: room.wall}, geom.BlockPos{X: room.wall, Y: 6, Z: room.wall}},
		{geom.BlockPos{X: -room.wall, Y: 1, Z: -room.wall}, geom.BlockPos{X: -room.wall, Y: 6, Z: room.wall}},
		{geom.BlockPos{X: room.wall, Y: 1, Z: -room.wall}, geom.BlockPos{X: room.wall, Y: 6, Z: room.wall}},
	} {
		span(wall.min, wall.max, "stone")
	}

	// Surfaces with different frictions, which is what makes the friction phase's
	// answer vary within one run rather than between runs.
	//
	// Slime is deliberately absent. The vertical clamp is not a rule of the tick
	// at all: the game calls the landing behaviour of the block under the body's
	// feet, whose default is to zero the vertical motion, and slime's overrides it
	// to negate it instead. This milestone implements the default and has no
	// per-block landing hook, so a bounce is a divergence it cannot express and a
	// world with slime in it would be testing a rule nobody has written. The
	// oracle found the bounce within two ticks; the divergence is recorded, and
	// slime's slipperiness is consequently unchecked against the game.
	surfaces := []string{"ice", "packed_ice", "soul_sand"}
	inside := func() int32 {
		return int32(random.IntN(int(2*room.wall-1))) - room.wall + 1
	}
	for range room.surfaces {
		place(geom.BlockPos{X: inside(), Y: 0, Z: inside()}, surfaces[random.IntN(len(surfaces))])
	}

	// Obstacles to walk into and to step onto. A slab is half a block, which the
	// step height clears, and a cube is not.
	for range room.obstacles {
		pos := geom.BlockPos{X: inside(), Y: int32(random.IntN(2)) + 1, Z: inside()}
		// The spawn column stays clear, so that a body never starts inside a
		// block: what the game does then is a separate question from movement.
		if pos.X >= -1 && pos.X <= 1 && pos.Z >= -1 && pos.Z <= 1 {
			continue
		}
		name := "stone"
		if random.IntN(2) == 0 {
			name = "stone_slab"
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

// commands renders this run as the harness protocol. The answers it produces are
// one for the spawn and one per tick.
func (r movementRun) commands() []string {
	commands := []string{"C"}
	for _, block := range r.placed {
		block.cells(func(pos geom.BlockPos) {
			commands = append(commands, fmt.Sprintf("B %d %d %d %s",
				pos.X, pos.Y, pos.Z, block.name))
		})
	}

	commands = append(commands, fmt.Sprintf("S %s %s %s %s %s %t %s %s",
		hex(r.scenario.spawn.X), hex(r.scenario.spawn.Y), hex(r.scenario.spawn.Z),
		single(r.yaw), single(0), !r.scenario.airborne,
		single(r.scenario.moveSpeed), single(r.scenario.jumpFactor)))

	for _, input := range r.inputs {
		strafe, forward := harnessInput(input)
		commands = append(commands, fmt.Sprintf("T %s %s %s %s %t %t",
			single(strafe), single(forward), single(input.Yaw), single(input.Pitch),
			input.Jump, input.Sprint))
	}

	return commands
}

// answers is how many lines this run's commands produce.
func (r movementRun) answers() int { return 1 + len(r.inputs) }

// harnessInput turns a controller's intent into the two axes the entity tick
// receives. Sneaking is scaled here because the client scales it before it
// leaves: see sneakFactor.
func harnessInput(input movement.Input) (strafe, forward float32) {
	if !input.Sneak {
		return input.Strafe, input.Forward
	}

	return float32(float64(input.Strafe) * sneakFactor), float32(float64(input.Forward) * sneakFactor)
}

// single renders a float the way the harness parses it: the shortest decimal
// that reads back as the same float.
func single(value float32) string {
	return strconv.FormatFloat(float64(value), 'g', -1, 32)
}

// movementProfile builds the profile under test.
func movementProfile(t *testing.T) sim.Profile {
	t.Helper()

	set, err := gen.Data()
	if err != nil {
		t.Fatalf("load the 1.8.9 data set: %v", err)
	}

	profile, err := v1_8.New(set)
	if err != nil {
		t.Fatalf("build the 1.8.9 profile: %v", err)
	}

	return profile
}

// TestOneTickOfStandingMatchesTheGame is the smoke test: the smallest trajectory
// there is, checked before the differential test runs a hundred of them.
//
// A body handed no motion is airborne for its first tick, because gravity is
// applied after the move, and is clamped by the floor on every tick after it.
// That is what the game does, and it is worth pinning on its own: if this
// disagrees, nothing the larger test reports will be easy to read.
func TestOneTickOfStandingMatchesTheGame(t *testing.T) {
	jar := newOracle(t)
	profile := movementProfile(t)

	run := movementRun{
		room: movementRoom{radius: 2, wall: 2, floor: -1, ceiling: 4, ticks: 5},
		scenario: movementScenario{
			name:  "stand",
			spawn: geom.Vec3{X: 0.5, Y: 1, Z: 0.5},
			input: func(int, float32) movement.Input {
				return movement.Input{Entity: movementEntity}
			},
			moveSpeed: walkMoveSpeed, jumpFactor: walkJumpFactor,
		},
	}
	run.placed = append(run.placed, movementPlacement{
		min:  geom.BlockPos{X: -2, Y: 0, Z: -2},
		max:  geom.BlockPos{X: 2, Y: 0, Z: 2},
		name: "stone",
	})
	for tick := range 5 {
		run.inputs = append(run.inputs, run.scenario.input(tick, 0))
	}

	answers := jar.run(t, "MovementOracle", run.commands(), run.answers())
	compareMovementRun(t, profile, run, answers)
}

// TestMovementMatchesTheGame is the milestone's gate.
//
// Six scenarios over four random worlds each, a hundred ticks apiece, compared
// every tick rather than at the end. The first differing tick names the phase
// that diverged; an endpoint comparison would name only the scenario.
func TestMovementMatchesTheGame(t *testing.T) {
	jar := newOracle(t)
	profile := movementProfile(t)

	var runs []movementRun
	var commands []string
	want := 0
	for _, scenario := range movementScenarios() {
		for _, seed := range movementSeeds {
			run := buildMovementRun(scenario, wideRoom, seed)
			runs = append(runs, run)
			commands = append(commands, run.commands()...)
			want += run.answers()
		}
	}

	answers := jar.run(t, "MovementOracle", commands, want)

	at := 0
	var total movementCoverage
	for _, run := range runs {
		total = total.add(compareMovementRun(t, profile, run, answers[at:at+run.answers()]))
		at += run.answers()
	}

	t.Logf("checked %d ticks against the game over %d worlds: %+v",
		want-len(runs), len(runs), total)

	// A trajectory that never collides, never leaves the ground, and never steps
	// up agrees with the game about walking in a straight line and says nothing
	// about the rest of the tick. The worlds are random, so what they contain is
	// asserted rather than assumed.
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

// movementCoverage counts what the random worlds actually made the tick do, so
// that a gate which quietly stopped exercising a rule fails instead of passing.
type movementCoverage struct {
	collided int
	airborne int
	landed   int
	rose     int
	stepped  int
}

func (c movementCoverage) add(other movementCoverage) movementCoverage {
	return movementCoverage{
		collided: c.collided + other.collided,
		airborne: c.airborne + other.airborne,
		landed:   c.landed + other.landed,
		rose:     c.rose + other.rose,
		stepped:  c.stepped + other.stepped,
	}
}

// compareMovementRun simulates one run and compares every tick against the
// game's answer for it.
func compareMovementRun(
	t *testing.T, profile sim.Profile, run movementRun, answers []string,
) movementCoverage {
	t.Helper()

	store := movementStore(t, profile, run)

	state, loco, ok := v1_8.Spawn(profile, run.scenario.spawn, run.yaw, 0)
	if !ok {
		t.Fatal("Spawn did not recognize its own profile")
	}
	state.OnGround = !run.scenario.airborne
	loco.MoveSpeed = run.scenario.moveSpeed
	loco.JumpFactor = run.scenario.jumpFactor
	store.SetEntity(movementEntity, state)
	store.SetLocomotion(movementEntity, loco)

	label := fmt.Sprintf("%s/seed %d", run.scenario.name, run.seed)
	spawned := parseMovementState(t, answers[0])
	if !identicalBox(state.Box, spawned.box) {
		t.Fatalf("%s: the spawned body is %+v, the game says %+v",
			label, state.Box, spawned.box)
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

// movementStore describes the run's world to a store, air included, so that no
// sweep can reach a cell nobody described.
func movementStore(t *testing.T, profile sim.Profile, run movementRun) *runtime.Memory {
	t.Helper()

	air, ok := v1_8.Ref(profile, "air")
	if !ok {
		t.Fatal("the profile does not know air")
	}

	store := runtime.NewMemory(profile)
	for x := -run.room.radius; x <= run.room.radius; x++ {
		for y := run.room.floor; y <= run.room.ceiling; y++ {
			for z := -run.room.radius; z <= run.room.radius; z++ {
				if err := store.SetBlock(geom.BlockPos{X: x, Y: y, Z: z}, air); err != nil {
					t.Fatalf("SetBlock: %v", err)
				}
			}
		}
	}

	for _, block := range run.placed {
		ref, ok := v1_8.Ref(profile, block.name)
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

// hasEvent reports whether the tick emitted a domain event of a kind.
func hasEvent(events []sim.DomainEvent, kind string) bool {
	for _, event := range events {
		if event.Kind == kind {
			return true
		}
	}

	return false
}

// parseMovementState reads one answer line.
func parseMovementState(t *testing.T, text string) movementState {
	t.Helper()

	fields := strings.Fields(text)
	if len(fields) != 12 {
		t.Fatalf("the movement oracle returned %d fields: %q", len(fields), text)
	}

	return movementState{
		box: geom.AABB{
			MinX: parseHex(t, fields[0]),
			MinY: parseHex(t, fields[1]),
			MinZ: parseHex(t, fields[2]),
			MaxX: parseHex(t, fields[3]),
			MaxY: parseHex(t, fields[4]),
			MaxZ: parseHex(t, fields[5]),
		},
		motion: geom.Vec3{
			X: parseHex(t, fields[6]),
			Y: parseHex(t, fields[7]),
			Z: parseHex(t, fields[8]),
		},
		onGround:             fields[9] == "true",
		collidedHorizontally: fields[10] == "true",
		collidedVertically:   fields[11] == "true",
	}
}

// identicalVec compares two vectors by their bits.
func identicalVec(a, b geom.Vec3) bool {
	return identical(a.X, b.X) && identical(a.Y, b.Y) && identical(a.Z, b.Z)
}

// formatBox and formatVec print every component at full precision, because a
// failure here is usually a difference in the last bits and a rounded printout
// would hide it.
func formatBox(b geom.AABB) string {
	return fmt.Sprintf("[%s %s %s .. %s %s %s]",
		full(b.MinX), full(b.MinY), full(b.MinZ), full(b.MaxX), full(b.MaxY), full(b.MaxZ))
}

func formatVec(v geom.Vec3) string {
	return fmt.Sprintf("(%s %s %s)", full(v.X), full(v.Y), full(v.Z))
}

func full(value float64) string {
	return strconv.FormatFloat(value, 'g', 17, 64)
}
