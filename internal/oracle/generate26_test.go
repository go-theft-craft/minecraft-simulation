package oracle_test

import (
	"path/filepath"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/mctest"
	"github.com/go-theft-craft/minecraft-simulation/scene"
)

// fixtureDirectory26 is where this version's replayable suite lives. The other
// version's stays where it was, because a fixture's path is what a reader looks
// it up by and moving one would be a change to a file this milestone must not
// touch.
const fixtureDirectory26 = "../../mctest/testdata/26_1"

// TestGenerateFixtures26 records the six scenarios from the game.
//
// Every expectation in a fixture is the jar's answer, never ours — the same rule
// M8.4's generator follows, for the same reason: a fixture written from our own
// output would agree with us by construction and would notice nothing.
//
// The flag that makes it write is the other generator's, so one run rewrites both
// versions' suites or neither.
func TestGenerateFixtures26(t *testing.T) {
	jar := newOracle26(t)
	profile := movementProfile26(t)

	for _, scenario := range movementScenarios26() {
		run := buildMovementRun26(scenario, compactRoom, fixtureSeed)
		answers := jar.run(t, "MovementOracle26", commands26(run), run.answers())

		fixture := recordFixture26(t, run, answers)
		fixture.Profile = profile.ID()

		path := filepath.Join(fixtureDirectory26, scenario.name+".json")
		if !*writeFixtures {
			committed, err := mctest.Load(path)
			if err != nil {
				t.Fatalf("%s: %v (pass -write-fixtures to record it)", scenario.name, err)
			}
			compareFixtures(t, scenario.name, committed, fixture)

			continue
		}

		if err := fixture.Save(path); err != nil {
			t.Fatalf("%s: save: %v", scenario.name, err)
		}
		t.Logf("wrote %s: %d blocks, %d ticks", path, len(fixture.World.Blocks), len(fixture.Ticks))
	}
}

// recordFixture26 turns one run and the game's answers to it into a fixture.
func recordFixture26(t *testing.T, run movementRun, answers []string) mctest.Fixture {
	t.Helper()

	spawned := parseMovementState(t, answers[0])

	fixture := mctest.Fixture{
		Name:   run.scenario.name,
		Source: "generated from a Java Edition 26.1.2 server jar by internal/oracle",
		World: scene.World{
			// A cell wider and deeper than the room on every side, because this
			// version's friction probe sweeps the cells around a body's feet with
			// the game's own margin and that margin reaches past the floor.
			Min:  geom.BlockPos{X: -run.room.radius - 1, Y: run.room.floor - 1, Z: -run.room.radius - 1},
			Max:  geom.BlockPos{X: run.room.radius + 1, Y: run.room.ceiling + 1, Z: run.room.radius + 1},
			Fill: "air",
		},
		Body: mctest.Body{
			Box:      spawned.box,
			Position: run.scenario.spawn,
			Motion:   spawned.motion,
			OnGround: spawned.onGround,
			// The step height is the profile's own constant rather than
			// something the harness reports, because the harness takes the
			// entity's default and this records what the body was given.
			StepHeight: float64(float32(0.6)),
			Yaw:        run.yaw,
			MoveSpeed:  run.scenario.moveSpeed,
		},
	}

	for _, block := range run.placed {
		entry := scene.Block{Pos: block.min, Name: block.name}
		if block.max != block.min {
			to := block.max
			entry.To = &to
		}
		fixture.World.Blocks = append(fixture.World.Blocks, entry)
	}

	for index, input := range run.inputs {
		state := parseMovementState(t, answers[1+index])

		fixture.Ticks = append(fixture.Ticks, mctest.Tick{
			// The axes are recorded as the controller meant them, with the sneak
			// flag alongside, because a fixture replays through the same intent a
			// consumer sends. The harness was handed the shaped pair; see
			// commands26.
			Strafe:   input.Strafe,
			Forward:  input.Forward,
			Yaw:      input.Yaw,
			Pitch:    input.Pitch,
			Jump:     input.Jump,
			Sprint:   input.Sprint,
			Sneak:    input.Sneak,
			Box:      state.box,
			Motion:   state.motion,
			OnGround: state.onGround,
			Collided: state.collidedHorizontally,
		})
	}

	return fixture
}
