package replay_test

import (
	"path/filepath"
	"testing"

	gen26 "github.com/go-theft-craft/minecraft-protocol/generated/java/v26_1"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	v26_1 "github.com/go-theft-craft/minecraft-simulation/profile/java/v26_1"
	"github.com/go-theft-craft/minecraft-simulation/replay"
	"github.com/go-theft-craft/minecraft-simulation/scene"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// The movement-speed attribute the two moving runs carry. Sprinting raises it by
// thirty percent through a modifier the game computes as a double and narrows
// where the tick reads it.
var (
	walkSpeed26   = float32(0.1)
	sprintSpeed26 = float32(0.1 * 1.3)
)

// profile26 builds the rules this version's recordings were made under.
func profile26(t *testing.T) sim.Profile {
	t.Helper()

	set, err := gen26.Data()
	if err != nil {
		t.Fatalf("load the 26.1.2 data set: %v", err)
	}
	built, err := v26_1.New(set)
	if err != nil {
		t.Fatalf("build the 26.1.2 profile: %v", err)
	}

	return built
}

// recordingSpecs26 are the runs this version contributes to the matrix.
//
// They are not the other version's five with a different profile. Each one
// exists to reach arithmetic this version has and 1.8.9 does not: a double-width
// input normalization feeding a single-width table read, an acceleration that
// divides by the cube of the raw block friction, a heading whose table index is a
// double truncated through a long, and a step-up that climbs whatever heights a
// shape offers rather than a fixed two.
func recordingSpecs26(t *testing.T, built sim.Profile) []replay.Recording {
	t.Helper()

	return []replay.Recording{
		sprintDiagonal26(t, built),
		ice26(t, built),
		jumpAndFall26(t, built),
		stepUp26(t, built),
	}
}

// walkingRoom26 returns a floor of one block, walls around it, and a player
// standing in the middle.
//
// The described region reaches a cell below the floor, because this version's
// friction reads the block a body stands on through a probe that sweeps the
// cells around its feet with the game's own margin.
func walkingRoom26(t *testing.T, built sim.Profile, name, note, surface string) replay.Recording {
	t.Helper()

	recording := blankRecording26(t, built, name, note, geom.Vec3{X: 0.5, Y: 1, Z: 0.5})
	recording.Bodies[0].Locomotion.MoveSpeed = walkSpeed26
	recording.World = scene.World{
		Min:  geom.BlockPos{X: -roomRadius, Y: -2, Z: -roomRadius},
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

// blankRecording26 returns a recording with a player and no world and no ticks.
func blankRecording26(t *testing.T, built sim.Profile, name, note string, at geom.Vec3) replay.Recording {
	t.Helper()

	state, locomotion, ok := v26_1.Spawn(built, at, 0, 0)
	if !ok {
		t.Fatal("Spawn did not recognize its own profile")
	}

	return replay.Recording{
		Name:       name,
		Note:       note,
		Profile:    built.ID(),
		DataDigest: built.(sim.DataDigest).DataDigest().String(),
		Bodies: []replay.Body{{
			ID:     1,
			Family: state.Family,
			Box:    state.Box,
			// Recorded as well as the box, because this version's move rebuilds
			// the box around the position rather than offsetting it, and a run
			// that started from a box alone would move a body standing at the
			// origin.
			Position:   state.Position,
			Motion:     state.Motion,
			OnGround:   state.OnGround,
			StepHeight: state.StepHeight,
			Locomotion: locomotion,
		}},
	}
}

func sprintDiagonal26(t *testing.T, built sim.Profile) replay.Recording {
	t.Helper()

	recording := walkingRoom26(t, built, "sprint-diagonal",
		"Both axes at full input, sprinting, through a full turn of yaw. The densest "+
			"arithmetic in this profile, and it is not the other version's: the input is "+
			"stretched onto the unit square at single width, normalized at double width, "+
			"and rotated by a sine and a cosine read from the table at an index this "+
			"version computes as a double truncated through a long.",
		"stone")
	recording.Bodies[0].Locomotion.MoveSpeed = sprintSpeed26

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

func ice26(t *testing.T, built sim.Profile) replay.Recording {
	t.Helper()

	recording := walkingRoom26(t, built, "ice",
		"The same walk over ice, with packed ice and blue ice in stripes. This version "+
			"divides 0.21600002 by the cube of the raw block friction where 1.8.9 divides "+
			"0.16277136 by the cube of the product, so three different frictions put that "+
			"division on values far from the default — and blue ice, at 0.989, is a value "+
			"1.8.9 has no block for.",
		"ice")

	// Stripes rather than one surface, so the friction changes underfoot mid-run
	// and the recomputation from the pre-move support is exercised.
	for x := int32(-8); x <= 8; x += 3 {
		recording.World.Blocks = append(recording.World.Blocks, span(
			geom.BlockPos{X: x, Y: 0, Z: -roomRadius},
			geom.BlockPos{X: x, Y: 0, Z: roomRadius},
			"packed_ice",
		))
	}
	for z := int32(-7); z <= 7; z += 5 {
		recording.World.Blocks = append(recording.World.Blocks, span(
			geom.BlockPos{X: -roomRadius, Y: 0, Z: z},
			geom.BlockPos{X: roomRadius, Y: 0, Z: z},
			"blue_ice",
		))
	}

	return drive(recording, 240, func(tick int) replay.Command {
		return replay.Command{
			Strafe: 1, Forward: 1,
			Yaw: float32(tick) * 3.7,
		}
	})
}

func jumpAndFall26(t *testing.T, built sim.Profile) replay.Recording {
	t.Helper()

	// A shaft, so the fall is long enough for gravity and the vertical drag to
	// reach terminal speed. Accumulation is where a one-bit difference becomes
	// visible, and a fall of four blocks accumulates nothing.
	recording := blankRecording26(t, built, "jump-and-fall",
		"A hundred and twenty blocks of falling, then repeated jumps. The fall runs "+
			"gravity and the vertical drag over enough ticks to reach terminal speed. The "+
			"jumps run this version's own rule, which takes the larger of the jump power "+
			"and the motion already there rather than assigning over it, and adds a "+
			"sprinting body's impulse through a double multiply where 1.8.9 uses a float.",
		geom.Vec3{X: 0.5, Y: 130, Z: 0.5})
	recording.Bodies[0].Locomotion.MoveSpeed = walkSpeed26

	recording.World = scene.World{
		// A cell wider than the shaft on every side, for the probe that reads
		// the block a body stands on: it sweeps the cells around the feet with
		// the game's own margin, which reaches past a wall.
		Min:  geom.BlockPos{X: -4, Y: -2, Z: -4},
		Max:  geom.BlockPos{X: 4, Y: 140, Z: 4},
		Fill: "air",
		Blocks: []scene.Block{
			span(geom.BlockPos{X: -3, Y: 0, Z: -3}, geom.BlockPos{X: 3, Y: 0, Z: 3}, "stone"),
			span(geom.BlockPos{X: -3, Y: 1, Z: -3}, geom.BlockPos{X: -3, Y: 140, Z: 3}, "stone"),
			span(geom.BlockPos{X: 3, Y: 1, Z: -3}, geom.BlockPos{X: 3, Y: 140, Z: 3}, "stone"),
			span(geom.BlockPos{X: -3, Y: 1, Z: -3}, geom.BlockPos{X: 3, Y: 140, Z: -3}, "stone"),
			span(geom.BlockPos{X: -3, Y: 1, Z: 3}, geom.BlockPos{X: 3, Y: 140, Z: 3}, "stone"),
		},
	}
	recording.Bodies[0].OnGround = false

	return drive(recording, 260, func(tick int) replay.Command {
		return replay.Command{
			Forward: 1,
			Yaw:     float32(tick) * 2.3,
			Jump:    tick > 180,
			Sprint:  tick > 200,
		}
	})
}

func stepUp26(t *testing.T, built sim.Profile) replay.Recording {
	t.Helper()

	recording := walkingRoom26(t, built, "step-up",
		"A walk into a field of slabs and stairs. The step-up this version runs climbs "+
			"whatever heights the obstacle's own shape offers, ascending, rather than "+
			"trying a fixed height — so a stair offers one rise per grid line and a slab "+
			"offers halves, and the choice of the first improving candidate is what this "+
			"records.",
		"stone")

	for x := int32(-7); x <= 7; x += 2 {
		for z := int32(-7); z <= 7; z += 3 {
			name := "smooth_stone_slab"
			if (x+z)%3 == 0 {
				name = "stone_brick_stairs"
			}
			recording.World.Blocks = append(recording.World.Blocks, scene.Block{
				Pos: geom.BlockPos{X: x, Y: 1, Z: z}, Name: name,
			})
		}
	}

	return drive(recording, 240, func(tick int) replay.Command {
		return replay.Command{
			Strafe: -1, Forward: 1,
			Yaw: 180 - float32(tick)*1.3,
		}
	})
}

// TestGenerateRecordings26 writes this version's committed recordings, behind
// the same flag the other version's generator uses.
func TestGenerateRecordings26(t *testing.T) {
	if !*writeRecordings {
		t.Skip("pass -write-recordings to rewrite the committed recordings")
	}

	built := profile26(t)
	for _, spec := range recordingSpecs26(t, built) {
		recorded, err := replay.Record(built, spec)
		if err != nil {
			t.Fatalf("%s: Record: %v", spec.Name, err)
		}

		path := filepath.Join("testdata", "26_1", spec.Name+".json")
		if err := recorded.Save(path); err != nil {
			t.Fatalf("%s: Save: %v", spec.Name, err)
		}
		t.Logf("wrote %s: %d ticks", path, recorded.Covers())
	}
}
