package replay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/movement"
	"github.com/go-theft-craft/minecraft-simulation/scene"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

var (
	// ErrRecording reports a recording that cannot be read or replayed as
	// written.
	ErrRecording = errors.New("replay: invalid recording")
	// ErrProfileMismatch reports a recording made under different rules.
	ErrProfileMismatch = errors.New("replay: the recording was made under a different profile")
	// ErrDataMismatch reports a recording made against different game data.
	ErrDataMismatch = errors.New("replay: the recording was made against different game data")
)

// commandMovementInput is the only command kind a recording carries so far.
const commandMovementInput = "movement.input"

// Recording is one run: where it started, what it was asked to do, and the
// digest every tick produced.
type Recording struct {
	// Name labels the run, for the failure a replay reports.
	Name string `json:"name"`
	// Note says what this recording is for, since the point of these files is
	// which arithmetic they reach rather than what a body ends up doing.
	Note string `json:"note,omitempty"`
	// Profile is the rules that produced the digests.
	Profile sim.ProfileID `json:"profile"`
	// DataDigest pins the numbers those rules ran on. A recording made before a
	// dataset correction disagrees with the build afterwards, and without this
	// the disagreement arrives as an unexplained digest mismatch.
	DataDigest string `json:"data_digest"`
	// World is the blocks the run happened in.
	World scene.World `json:"world"`
	// Bodies are the entities the run simulated, in scope order. The order is
	// part of the input: the kernel walks the scope in the order it is given and
	// the digest covers it.
	Bodies []Body `json:"bodies"`
	// Limits bounds the work each tick may do. The zero value means the
	// kernel's defaults, and it is recorded because a tick that hit a limit
	// produced a different result than one that did not.
	Limits sim.Limits `json:"limits"`
	// Random is the state the first tick draws from.
	Random sim.RandomState `json:"random"`
	// Ticks are the inputs and the digests, in order.
	Ticks []Tick `json:"ticks"`
}

// Body is one entity's starting state.
type Body struct {
	ID     entity.ID     `json:"id"`
	Family entity.Family `json:"family"`
	Box    geom.AABB     `json:"box"`
	// Position is where the body stands, for a version whose move rebuilds the
	// box around it. A recording made under a version that keeps no position
	// omits this, so every recording written before one did is unchanged.
	Position   geom.Vec3           `json:"position,omitzero"`
	Motion     geom.Vec3           `json:"motion"`
	OnGround   bool                `json:"on_ground"`
	StepHeight float64             `json:"step_height"`
	Locomotion movement.Locomotion `json:"locomotion"`
}

// Tick is one tick's commands and the digest it produced.
type Tick struct {
	Input []Command `json:"input"`
	// Digest is the tick result's canonical hash, in lowercase hexadecimal. It
	// is the whole expectation: the hash covers every field of a result, so a
	// platform that disagrees disagrees about all of it at once.
	Digest string `json:"digest"`
}

// Command is a serialized sim.Command.
//
// An interface does not round-trip through JSON, so a recording carries a kind
// and the fields that kind needs. When later mechanics add commands this type
// grows; what it must never do is skip a kind it does not know. A tick replayed
// with its input dropped still produces a digest, and that digest would be a
// plausible wrong answer rather than an error.
type Command struct {
	Kind    string    `json:"kind"`
	Entity  entity.ID `json:"entity"`
	Strafe  float32   `json:"strafe,omitempty"`
	Forward float32   `json:"forward,omitempty"`
	Yaw     float32   `json:"yaw,omitempty"`
	Pitch   float32   `json:"pitch,omitempty"`
	Jump    bool      `json:"jump,omitempty"`
	Sprint  bool      `json:"sprint,omitempty"`
	Sneak   bool      `json:"sneak,omitempty"`
}

// command turns a recorded command back into the one the kernel takes.
func (c Command) command() (sim.Command, error) {
	if c.Kind != commandMovementInput {
		return nil, fmt.Errorf("%w: unknown command kind %q", ErrRecording, c.Kind)
	}

	return movement.Input{
		Entity:  c.Entity,
		Strafe:  c.Strafe,
		Forward: c.Forward,
		Yaw:     c.Yaw,
		Pitch:   c.Pitch,
		Jump:    c.Jump,
		Sprint:  c.Sprint,
		Sneak:   c.Sneak,
	}, nil
}

// Load reads a recording from a file.
func Load(path string) (Recording, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Recording{}, fmt.Errorf("replay: read recording: %w", err)
	}

	var recording Recording
	decoder := json.NewDecoder(bytes.NewReader(content))
	// An unknown field is a recording written against a different format, and
	// replaying it would ignore whatever it was trying to say.
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&recording); err != nil {
		return Recording{}, fmt.Errorf("%w %s: %w", ErrRecording, path, err)
	}

	// Every command is resolved at load, not at the tick that uses it. A
	// recording that names a kind this build does not know must fail before it
	// simulates anything, or the first ninety ticks pass and the failure arrives
	// somewhere unrelated to the cause.
	for index, tick := range recording.Ticks {
		for _, command := range tick.Input {
			if _, err := command.command(); err != nil {
				return Recording{}, fmt.Errorf("%s tick %d: %w", path, index, err)
			}
		}
	}

	return recording, nil
}

// Save writes a recording to a file.
//
// The encoding is indented and the field order is the struct's, so that
// re-recording produces a diff a reviewer can follow. Saving the same recording
// twice produces the same bytes, which is what makes such a diff meaningful.
func (r Recording) Save(path string) error {
	content, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("replay: encode recording: %w", err)
	}
	content = append(content, '\n')

	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("replay: write recording: %w", err)
	}

	return nil
}

// Covers reports how many ticks this recording is evidence about.
//
// An empty recording is not an error — it is a legal, useless file — so the way
// to notice one is to ask. The suite that gates on these files asks.
func (r Recording) Covers() int { return len(r.Ticks) }

// check refuses a recording that is not evidence about this build.
//
// Both checks happen before anything simulates. A recording made under other
// rules or against other numbers will disagree, and letting it run first would
// report the disagreement as a digest mismatch at tick zero — true, and useless
// for working out why.
func (r Recording) check(profile sim.Profile) error {
	if got := profile.ID(); got != r.Profile {
		return fmt.Errorf("%w: recorded under %s, replaying against %s",
			ErrProfileMismatch, r.Profile, got)
	}
	if r.DataDigest == "" {
		return nil
	}

	reporter, ok := profile.(sim.DataDigest)
	if !ok {
		return fmt.Errorf("%w: the recording pins data %s and this profile reports none",
			ErrDataMismatch, r.DataDigest)
	}
	if got := reporter.DataDigest().String(); got != r.DataDigest {
		return fmt.Errorf("%w: recorded against %s, replaying against %s",
			ErrDataMismatch, r.DataDigest, got)
	}

	return nil
}
