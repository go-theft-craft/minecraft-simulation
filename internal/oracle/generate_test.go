package oracle_test

import (
	"flag"
	"path/filepath"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/mctest"
	"github.com/go-theft-craft/minecraft-simulation/scene"
)

// writeFixtures makes the generator write. Without it the generator only checks
// that the committed fixtures still say what the game says, so an ordinary test
// run can never quietly rewrite an expectation. Regenerating is a deliberate
// act with a diff to read.
var writeFixtures = flag.Bool("write-fixtures", false,
	"rewrite the committed movement fixtures from the game")

// fixtureDirectory is where the replayable suite lives.
const fixtureDirectory = "../../mctest/testdata"

// fixtureSeed is the world every fixture is recorded over. One seed, because a
// fixture suite is a conformance check rather than a search: the differential
// test is what runs the scenarios over many worlds.
const fixtureSeed = 7

// TestGenerateFixtures records the six scenarios from the game.
//
// Every expectation in a fixture is the jar's answer, never ours. That is what
// makes the fixtures worth shipping: a fixture and the differential test cannot
// disagree about what vanilla does, only about whether this module still matches
// it. A fixture written from our own output would agree with us by construction
// and would notice nothing.
func TestGenerateFixtures(t *testing.T) {
	jar := newOracle(t)
	profile := movementProfile(t)

	for _, scenario := range movementScenarios() {
		run := buildMovementRun(scenario, compactRoom, fixtureSeed)
		answers := jar.run(t, "MovementOracle", run.commands(), run.answers())

		fixture := recordFixture(t, run, answers)
		fixture.Profile = profile.ID()

		path := filepath.Join(fixtureDirectory, scenario.name+".json")
		if !*writeFixtures {
			// Not writing: check the committed file still records what the game
			// just said. A fixture that has drifted from the jar is a finding in
			// its own right, and finding it here is better than finding it as an
			// unexplained replay failure with no JDK in sight.
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

// recordFixture turns one run and the game's answers to it into a fixture.
func recordFixture(t *testing.T, run movementRun, answers []string) mctest.Fixture {
	t.Helper()

	spawned := parseMovementState(t, answers[0])

	fixture := mctest.Fixture{
		Name:   run.scenario.name,
		Source: "generated from a Java Edition 1.8.9 server jar by internal/oracle",
		World: scene.World{
			Min:  geom.BlockPos{X: -run.room.radius, Y: run.room.floor, Z: -run.room.radius},
			Max:  geom.BlockPos{X: run.room.radius, Y: run.room.ceiling, Z: run.room.radius},
			Fill: "air",
		},
		Body: mctest.Body{
			Box:      spawned.box,
			Motion:   spawned.motion,
			OnGround: spawned.onGround,
			// The step height is the profile's own constant rather than
			// something the harness reports, because the harness takes the
			// entity's default and this records what the body was given.
			StepHeight: float64(float32(0.6)),
			Yaw:        run.yaw,
			MoveSpeed:  run.scenario.moveSpeed,
			JumpFactor: run.scenario.jumpFactor,
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
			// consumer sends. The harness was handed the scaled pair; see
			// harnessInput.
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

// compareFixtures reports the first difference between a committed fixture and
// a freshly recorded one.
func compareFixtures(t *testing.T, name string, committed, recorded mctest.Fixture) {
	t.Helper()

	if len(committed.Ticks) != len(recorded.Ticks) {
		t.Fatalf("%s: the committed fixture records %d ticks and the game just produced %d",
			name, len(committed.Ticks), len(recorded.Ticks))
	}
	if committed.Body != recorded.Body {
		t.Fatalf("%s: the committed body is %+v, the game says %+v",
			name, committed.Body, recorded.Body)
	}
	for index, tick := range recorded.Ticks {
		if committed.Ticks[index] != tick {
			t.Fatalf("%s tick %d: the committed fixture says %+v, the game says %+v",
				name, index, committed.Ticks[index], tick)
		}
	}
}
