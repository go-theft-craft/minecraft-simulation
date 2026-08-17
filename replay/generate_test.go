package replay_test

import (
	"flag"
	"path/filepath"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	v1_8 "github.com/go-theft-craft/minecraft-simulation/profile/java/v1_8"
	"github.com/go-theft-craft/minecraft-simulation/replay"
	"github.com/go-theft-craft/minecraft-simulation/scene"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// writeRecordings makes the generator write. Without it nothing is written, so
// an ordinary run can never refresh an expectation.
//
// That matters more here than anywhere else in the repository. These digests are
// this module's own output, so a recording refreshed to make a red matrix go
// green would turn the gate into a rubber stamp — the matrix would then be
// comparing each platform against itself. Regenerating is for a deliberate
// change in behaviour, and its commit message says which.
var writeRecordings = flag.Bool("write-recordings", false,
	"rewrite the committed determinism recordings from this build")

// The room the walking runs happen in: described to its edges, walled inside
// them, so a run cannot reach a cell nobody described. Record refuses an
// incomplete tick, so a room that was too small would fail loudly rather than
// pin the digest of a tick that did nothing.
const (
	roomRadius = 10
	roomWall   = 9
)

// Sprinting raises the movement-speed attribute by 30% and the air factor by the
// same fraction, both computed as doubles and narrowed. This milestone takes
// them as inputs — whatever fills Locomotion owns them — so they are stated here
// rather than derived.
var (
	walkSpeed        = float32(0.1)
	walkJumpFactor   = float32(0.02)
	sprintSpeed      = float32(0.1 * 1.3)
	sprintJumpFactor = float32(float64(float32(0.02)) + float64(float32(0.02))*0.3)
)

// recordingSpecs are the runs the determinism matrix rests on.
//
// Every one of them exists to reach arithmetic, not to be interesting: the
// encoder is integer and byte work and was never plausibly platform-dependent,
// while the float32 products in movement and the truncated sine-table index are
// the only places a compiler, an architecture, or a fused multiply-add could
// change an answer. A matrix over empty ticks would pass on six platforms and
// prove nothing.
func recordingSpecs(t *testing.T, built sim.Profile) []replay.Recording {
	t.Helper()

	return []replay.Recording{
		sprintDiagonal(t, built),
		ice(t, built),
		jumpAndFall(t, built),
		stepUp(t, built),
		slimeAndSoulSand(t, built),
	}
}

// walkingRoom returns a floor of one block, walls around it, and a player
// standing in the middle.
func walkingRoom(t *testing.T, built sim.Profile, name, note, surface string) replay.Recording {
	t.Helper()

	recording := blankRecording(t, built, name, note, geom.Vec3{X: 0.5, Y: 1, Z: 0.5})
	// Stated rather than left to the spawn's defaults, because a recording is
	// read by whoever is investigating a mismatch and every input it ran on
	// should be visible in it.
	recording.Bodies[0].Locomotion.MoveSpeed = walkSpeed
	recording.Bodies[0].Locomotion.JumpFactor = walkJumpFactor
	recording.World = scene.World{
		Min:  geom.BlockPos{X: -roomRadius, Y: -1, Z: -roomRadius},
		Max:  geom.BlockPos{X: roomRadius, Y: 8, Z: roomRadius},
		Fill: "air",
		Blocks: []scene.Block{
			span(geom.BlockPos{X: -roomRadius, Y: 0, Z: -roomRadius},
				geom.BlockPos{X: roomRadius, Y: 0, Z: roomRadius}, surface),
			span(geom.BlockPos{X: -roomWall, Y: 1, Z: -roomWall},
				geom.BlockPos{X: roomWall, Y: 4, Z: -roomWall}, "stone"),
			span(geom.BlockPos{X: -roomWall, Y: 1, Z: roomWall},
				geom.BlockPos{X: roomWall, Y: 4, Z: roomWall}, "stone"),
			span(geom.BlockPos{X: -roomWall, Y: 1, Z: -roomWall},
				geom.BlockPos{X: -roomWall, Y: 4, Z: roomWall}, "stone"),
			span(geom.BlockPos{X: roomWall, Y: 1, Z: -roomWall},
				geom.BlockPos{X: roomWall, Y: 4, Z: roomWall}, "stone"),
		},
	}

	return recording
}

// blankRecording returns a recording with a player and no world and no ticks.
func blankRecording(t *testing.T, built sim.Profile, name, note string, at geom.Vec3) replay.Recording {
	t.Helper()

	state, locomotion, ok := v1_8.Spawn(built, at, 0, 0)
	if !ok {
		t.Fatal("Spawn did not recognize its own profile")
	}

	return replay.Recording{
		Name:       name,
		Note:       note,
		Profile:    built.ID(),
		DataDigest: built.(sim.DataDigest).DataDigest().String(),
		Bodies: []replay.Body{{
			ID:         1,
			Family:     state.Family,
			Box:        state.Box,
			Motion:     state.Motion,
			OnGround:   state.OnGround,
			StepHeight: state.StepHeight,
			Locomotion: locomotion,
		}},
	}
}

func span(from, to geom.BlockPos, name string) scene.Block {
	return scene.Block{Pos: from, To: &to, Name: name}
}

// drive fills a recording's ticks from a function of the tick number.
func drive(recording replay.Recording, ticks int, input func(tick int) replay.Command) replay.Recording {
	recording.Ticks = make([]replay.Tick, 0, ticks)
	for tick := range ticks {
		command := input(tick)
		command.Kind = "movement.input"
		command.Entity = 1
		recording.Ticks = append(recording.Ticks, replay.Tick{Input: []replay.Command{command}})
	}

	return recording
}

func sprintDiagonal(t *testing.T, built sim.Profile) replay.Recording {
	t.Helper()

	recording := walkingRoom(t, built, "sprint-diagonal",
		"Both axes at full input, sprinting, through a full turn of yaw. The densest "+
			"float32 path in the module: the input normalization, the truncated sine-table "+
			"index in every quadrant including negative angles, and the single widening in "+
			"the heading.",
		"stone")
	recording.Bodies[0].Locomotion.MoveSpeed = sprintSpeed
	recording.Bodies[0].Locomotion.JumpFactor = sprintJumpFactor

	// A full turn and a bit, starting below zero, because the table index
	// truncates toward zero and only a negative angle separates truncation from
	// rounding.
	return drive(recording, 220, func(tick int) replay.Command {
		return replay.Command{
			Strafe: 1, Forward: 1, Sprint: true,
			Yaw: -200 + float32(tick)*1.9,
		}
	})
}

func ice(t *testing.T, built sim.Profile) replay.Recording {
	t.Helper()

	recording := walkingRoom(t, built, "ice",
		"The same walk over ice, with packed ice in stripes. Slipperiness 0.98 puts the "+
			"friction product and the acceleration division — 0.16277136 divided by the cube "+
			"of the friction — on values far from the default, and the low friction lets "+
			"speed accumulate across ticks rather than settling.",
		"ice")

	// Stripes rather than a single surface, so the friction changes underfoot
	// mid-run and the recomputation from the pre-move position is exercised.
	for x := int32(-8); x <= 8; x += 3 {
		recording.World.Blocks = append(recording.World.Blocks, span(
			geom.BlockPos{X: x, Y: 0, Z: -roomRadius},
			geom.BlockPos{X: x, Y: 0, Z: roomRadius},
			"packed_ice",
		))
	}

	return drive(recording, 240, func(tick int) replay.Command {
		return replay.Command{
			Strafe: 1, Forward: 1,
			Yaw: float32(tick) * 3.7,
		}
	})
}

func jumpAndFall(t *testing.T, built sim.Profile) replay.Recording {
	t.Helper()

	// A shaft, so the fall is long enough for gravity and the vertical drag to
	// reach terminal speed. Accumulation is where a one-bit difference becomes
	// visible, and a fall of four blocks accumulates nothing.
	recording := blankRecording(t, built, "jump-and-fall",
		"A hundred and twenty blocks of falling, then repeated jumps. The fall runs "+
			"gravity and the vertical drag over enough ticks to reach terminal speed, which "+
			"is where a one-bit difference compounds into a visible one; the jumps then "+
			"exercise the impulse, the counter, and the sprint boost read from the table.",
		geom.Vec3{X: 0.5, Y: 120, Z: 0.5})
	recording.Bodies[0].OnGround = false
	// The shaft is walled for its whole height. A body steering as it falls
	// drifts, and Record refuses a tick that leaves the description, so the
	// alternative to walls is a recording that stops wherever the drift happened
	// to reach.
	const shaft = 6
	recording.World = scene.World{
		Min:  geom.BlockPos{X: -shaft - 1, Y: -1, Z: -shaft - 1},
		Max:  geom.BlockPos{X: shaft + 1, Y: 124, Z: shaft + 1},
		Fill: "air",
		Blocks: []scene.Block{
			span(geom.BlockPos{X: -shaft, Y: 0, Z: -shaft},
				geom.BlockPos{X: shaft, Y: 0, Z: shaft}, "stone"),
			span(geom.BlockPos{X: -shaft, Y: 1, Z: -shaft},
				geom.BlockPos{X: shaft, Y: 123, Z: -shaft}, "stone"),
			span(geom.BlockPos{X: -shaft, Y: 1, Z: shaft},
				geom.BlockPos{X: shaft, Y: 123, Z: shaft}, "stone"),
			span(geom.BlockPos{X: -shaft, Y: 1, Z: -shaft},
				geom.BlockPos{X: -shaft, Y: 123, Z: shaft}, "stone"),
			span(geom.BlockPos{X: shaft, Y: 1, Z: -shaft},
				geom.BlockPos{X: shaft, Y: 123, Z: shaft}, "stone"),
		},
	}

	return drive(recording, 260, func(tick int) replay.Command {
		// Steering while falling, so the airborne branch of the speed rule — the
		// jump movement factor — is not dead code in this run.
		command := replay.Command{Forward: 1, Yaw: float32(tick) * 2}
		if tick > 90 {
			// Held in stretches, so the counter is exercised both while it runs
			// down and after a release zeroes it.
			command.Jump = tick%13 < 9
			command.Sprint = tick%29 < 17
		}

		return command
	})
}

func stepUp(t *testing.T, built sim.Profile) replay.Recording {
	t.Helper()

	recording := walkingRoom(t, built, "step-up",
		"A staircase climbed and walked off. Half-block rises alternate slabs and cubes, "+
			"because a step height of float64(float32(0.6)) clears half a block and not a "+
			"whole one, so every stride reaches the retry the collision oracle checked and "+
			"the settle value it corrected.",
		"stone")

	// Alternating slab and cube, each rising half a block: slab tops at .5, cube
	// tops at whole numbers.
	for step := int32(0); step < 6; step++ {
		x := 2 + step
		if step%2 == 0 {
			recording.World.Blocks = append(recording.World.Blocks, scene.Block{
				Pos: geom.BlockPos{X: x, Y: 1 + step/2, Z: 0}, Name: "stone_slab",
			})

			continue
		}
		recording.World.Blocks = append(recording.World.Blocks, scene.Block{
			Pos: geom.BlockPos{X: x, Y: 1 + step/2, Z: 0}, Name: "stone",
		})
	}

	return drive(recording, 200, func(tick int) replay.Command {
		// Into the staircase, then away from it, so the climb is attempted from a
		// standing start again and again rather than once.
		yaw := float32(-90)
		if tick%50 >= 25 {
			yaw = 90
		}

		return replay.Command{Forward: 1, Yaw: yaw}
	})
}

func slimeAndSoulSand(t *testing.T, built sim.Profile) replay.Recording {
	t.Helper()

	recording := walkingRoom(t, built, "slime-and-soul-sand",
		"The remaining non-default slipperiness values, so the block table is exercised "+
			"beyond stone and ice. Note what this recording is not: vanilla bounces a body "+
			"that lands on slime, because the vertical clamp is a per-block landing callback "+
			"whose slime override negates the motion, and this module implements only the "+
			"default. The digests here are this build's answer, and on slime that answer is "+
			"a known divergence from the game rather than a claim about it.",
		"soul_sand")

	for x := int32(-8); x <= 8; x += 4 {
		recording.World.Blocks = append(recording.World.Blocks, span(
			geom.BlockPos{X: x, Y: 0, Z: -roomRadius},
			geom.BlockPos{X: x, Y: 0, Z: roomRadius},
			"slime",
		))
	}

	return drive(recording, 220, func(tick int) replay.Command {
		return replay.Command{
			Strafe: -1, Forward: 1,
			Yaw:  180 - float32(tick)*1.3,
			Jump: tick%17 < 4,
		}
	})
}

// TestGenerateRecordings writes the committed recordings, and only behind its
// flag.
func TestGenerateRecordings(t *testing.T) {
	if !*writeRecordings {
		t.Skip("pass -write-recordings to rewrite the committed recordings")
	}

	built := profile(t)
	for _, spec := range recordingSpecs(t, built) {
		recorded, err := replay.Record(built, spec)
		if err != nil {
			t.Fatalf("%s: Record: %v", spec.Name, err)
		}

		path := filepath.Join("testdata", spec.Name+".json")
		if err := recorded.Save(path); err != nil {
			t.Fatalf("%s: Save: %v", spec.Name, err)
		}
		t.Logf("wrote %s: %d ticks", path, recorded.Covers())
	}
}
