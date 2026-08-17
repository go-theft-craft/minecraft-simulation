// Package mctest replays recorded trajectories against a profile.
//
// A fixture records what the game did: a world, a body, and a hundred ticks of
// input paired with the position, motion, and flags vanilla produced for them.
// The fixtures in this repository are generated from a real server jar by
// internal/oracle, so a fixture and the differential test cannot disagree about
// what the game does — only about whether this module still matches it.
//
// Replaying needs no JDK, no jar, and no prepared workspace. That is the point:
// the differential test is the milestone's gate and runs only where the game is
// available, while this suite runs everywhere, including CI and a later version's
// development, and fails the moment a rule drifts.
package mctest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// ErrFixture reports a fixture that cannot be read or replayed as written.
var ErrFixture = errors.New("mctest: invalid fixture")

// Fixture is one recorded trajectory.
type Fixture struct {
	// Name labels the scenario, for the failure message a replay produces.
	Name string `json:"name"`
	// Profile is the rules this trajectory was recorded under. A replay against
	// a different profile is refused rather than run: the numbers would be
	// another version's, and a passing comparison would mean nothing.
	Profile sim.ProfileID `json:"profile"`
	// Source says where the expectations came from, so that a reader can tell a
	// recording of the game from a recording of ourselves.
	Source string `json:"source"`
	// World is the blocks the trajectory ran over.
	World World `json:"world"`
	// Body is the state the body started in.
	Body Body `json:"body"`
	// Ticks are the inputs and the results, in order.
	Ticks []Tick `json:"ticks"`
}

// World is a described region with blocks in it.
//
// The region is filled with one named block and the exceptions are listed,
// because the alternative — naming every cell — is thousands of lines of air for
// a trajectory that touches a dozen of them. Every cell in the region is
// described, so a sweep that leaves it is an incomplete tick rather than a
// silent guess.
type World struct {
	// Min and Max bound the described region, inclusive.
	Min geom.BlockPos `json:"min"`
	Max geom.BlockPos `json:"max"`
	// Fill is the block every cell in the region holds unless Blocks says
	// otherwise.
	Fill string `json:"fill"`
	// Blocks are the exceptions, in placement order.
	Blocks []Block `json:"blocks"`
}

// Block is a named cell, or a named box of them.
//
// Blocks are named rather than carrying handles because a handle is an index
// into the table of the profile that minted it: it means nothing to another
// profile, and it means the wrong thing if the table is ever renumbered.
//
// A floor and a wall are each one entry rather than a few hundred, which is
// what keeps a recorded world small enough to read in a diff.
type Block struct {
	// Pos is the cell, or the low corner when To is set.
	Pos geom.BlockPos `json:"pos"`
	// To is the high corner of an inclusive box. It is absent for a single cell.
	To   *geom.BlockPos `json:"to,omitempty"`
	Name string         `json:"name"`
}

// cells walks every cell this entry names.
func (b Block) cells(visit func(geom.BlockPos)) {
	far := b.Pos
	if b.To != nil {
		far = *b.To
	}
	for x := b.Pos.X; x <= far.X; x++ {
		for y := b.Pos.Y; y <= far.Y; y++ {
			for z := b.Pos.Z; z <= far.Z; z++ {
				visit(geom.BlockPos{X: x, Y: y, Z: z})
			}
		}
	}
}

// Body is the state a trajectory starts from.
type Body struct {
	Box        geom.AABB `json:"box"`
	Motion     geom.Vec3 `json:"motion"`
	OnGround   bool      `json:"on_ground"`
	StepHeight float64   `json:"step_height"`
	Yaw        float32   `json:"yaw"`
	Pitch      float32   `json:"pitch"`
	MoveSpeed  float32   `json:"move_speed"`
	JumpFactor float32   `json:"jump_factor"`
}

// Tick is one tick's intent and the result the recording expects from it.
type Tick struct {
	Strafe  float32 `json:"strafe"`
	Forward float32 `json:"forward"`
	Yaw     float32 `json:"yaw"`
	Pitch   float32 `json:"pitch"`
	Jump    bool    `json:"jump"`
	Sprint  bool    `json:"sprint"`
	Sneak   bool    `json:"sneak"`

	// Box, Motion, OnGround, and Collided are what the recording says the tick
	// produced. They are compared bit for bit.
	Box      geom.AABB `json:"box"`
	Motion   geom.Vec3 `json:"motion"`
	OnGround bool      `json:"on_ground"`
	Collided bool      `json:"collided"`
}

// Load reads a fixture from a file.
func Load(path string) (Fixture, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Fixture{}, fmt.Errorf("mctest: read fixture: %w", err)
	}

	var fixture Fixture
	decoder := json.NewDecoder(bytes.NewReader(content))
	// An unknown field is a fixture written against a different format, and
	// replaying it would silently ignore whatever it was trying to say.
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixture); err != nil {
		return Fixture{}, fmt.Errorf("%w %s: %w", ErrFixture, path, err)
	}
	if len(fixture.Ticks) == 0 {
		return Fixture{}, fmt.Errorf("%w %s: it records no ticks", ErrFixture, path)
	}

	return fixture, nil
}

// Save writes a fixture to a file.
//
// The encoding is indented and the field order is the struct's, so that
// regenerating a fixture produces a diff a reader can follow rather than one
// long line.
func (f Fixture) Save(path string) error {
	content, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("mctest: encode fixture: %w", err)
	}
	content = append(content, '\n')

	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("mctest: write fixture: %w", err)
	}

	return nil
}
