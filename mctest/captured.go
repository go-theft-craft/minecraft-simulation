package mctest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/movement"
	"github.com/go-theft-craft/minecraft-simulation/runtime"
	"github.com/go-theft-craft/minecraft-simulation/scene"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// Captured is a trajectory a real server sent, extracted from a recording.
//
// It is not a Fixture and cannot be one. A Fixture records what the game
// computed for every tick, because the jar was asked directly. A server states
// an entity's position on its own schedule: an item and an arrow are tracked
// once every twenty ticks, so a capture holds checkpoints rather than ticks and
// says nothing about what happened between two of them.
//
// That makes this the weaker of M9.2's two gates and the only one that runs
// against the game as it is actually played — through a real server's own
// encoder, over a real connection. Twenty ticks of accumulated error is a
// demanding check on gravity and drag even though it cannot name a phase.
type Captured struct {
	// Name labels the scenario for a failure message.
	Name string `json:"name"`
	// Profile is the rules the server was playing by. A replay against another
	// profile is refused rather than run.
	Profile sim.ProfileID `json:"profile"`
	// Family is the body this trajectory is of.
	Family entity.Family `json:"family"`
	// Source says where the numbers came from: the recording's digest, the
	// server's digest, and the date. A trajectory nobody can trace back to a
	// session is a claim rather than evidence.
	Source string `json:"source"`
	// World is the ground the body fell onto.
	World scene.World `json:"world"`
	// Spawn is the state the server said the body started in.
	Spawn CapturedBody `json:"spawn"`
	// Interval is how many ticks the server left between the samples, measured
	// from the recording rather than assumed: the arrival times divided by the
	// game's fifty-millisecond tick and rounded. It is recorded for a reader
	// and checked against the samples, which carry their own tick offsets
	// because a tracker's first update does not fall on its own period.
	Interval int `json:"interval"`
	// Tolerance is how far a simulated position may sit from a captured one, in
	// blocks, and Because says where the number comes from.
	Tolerance float64 `json:"tolerance"`
	Because   string  `json:"because"`
	// Samples are the checkpoints, in order, each at its own tick offset from
	// the spawn.
	Samples []CapturedSample `json:"samples"`
	// Absent, when non-empty, declares that this version cannot be checked this
	// way and says why. A capture with neither samples nor a reason is refused.
	Absent string `json:"absent,omitempty"`
}

// CapturedSample is one position a server stated, and when it stated it.
type CapturedSample struct {
	// Tick is how many ticks after the spawn this position is for, measured
	// from the recording's own arrival times.
	Tick int `json:"tick"`
	// Position is what the server sent, at the resolution it sends: 1/32 of a
	// block on protocol 47, and exactly on 775.
	Position geom.Vec3 `json:"position"`
}

// CapturedBody is where a captured trajectory starts.
type CapturedBody struct {
	Position geom.Vec3 `json:"position"`
	Motion   geom.Vec3 `json:"motion"`
	Width    float32   `json:"width"`
	Height   float32   `json:"height"`
}

// LoadCaptured reads one captured trajectory.
func LoadCaptured(path string) (Captured, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Captured{}, fmt.Errorf("mctest: read capture: %w", err)
	}

	var captured Captured
	decoder := json.NewDecoder(bytes.NewReader(content))
	// An unknown field is a capture written against a different format, and
	// replaying it would silently ignore whatever it was trying to say.
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&captured); err != nil {
		return Captured{}, fmt.Errorf("%w %s: %w", ErrFixture, path, err)
	}
	if captured.Absent == "" {
		switch {
		case len(captured.Samples) == 0:
			return Captured{}, fmt.Errorf(
				"%w %s: it holds no samples and no reason for having none", ErrFixture, path,
			)
		case captured.Interval <= 0:
			return Captured{}, fmt.Errorf(
				"%w %s: its sample interval is %d ticks", ErrFixture, path, captured.Interval,
			)
		case captured.Because == "":
			return Captured{}, fmt.Errorf(
				"%w %s: its tolerance has no recorded derivation", ErrFixture, path,
			)
		}
	}

	return captured, nil
}

// LoadCapturedDir reads every capture in a directory, sorted by name.
func LoadCapturedDir(dir string) ([]Captured, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("mctest: list captures: %w", err)
	}

	captures := make([]Captured, 0, len(paths))
	for _, path := range paths {
		captured, err := LoadCaptured(path)
		if err != nil {
			return nil, err
		}
		captures = append(captures, captured)
	}

	return captures, nil
}

// Save writes a capture.
func (c Captured) Save(path string) error {
	content, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("mctest: encode capture: %w", err)
	}

	if err := os.WriteFile(path, append(content, '\n'), 0o600); err != nil {
		return fmt.Errorf("mctest: write capture: %w", err)
	}

	return nil
}

// ReplayCaptured runs the trajectory through a profile and reports the largest
// deviation from what the server sent.
//
// It returns an error for a mismatch of rules — a capture of another version, a
// world the profile cannot describe, a tick that left the described region — and
// a deviation for a difference of physics. A caller that could not tell those
// apart would report the wrong thing.
func ReplayCaptured(profile sim.Profile, captured Captured) (float64, error) {
	if captured.Absent != "" {
		return 0, fmt.Errorf("%w %s: it declares itself absent: %s",
			ErrFixture, captured.Name, captured.Absent)
	}
	if captured.Profile != profile.ID() {
		return 0, fmt.Errorf("%w %s: it was captured under %s and is being replayed under %s",
			ErrFixture, captured.Name, captured.Profile, profile.ID())
	}

	store := runtime.NewMemory(profile)
	if err := captured.World.Describe(profile, store.SetBlock); err != nil {
		return 0, fmt.Errorf("mctest: %s: %w", captured.Name, err)
	}

	const body = entity.ID(1)
	// Both the box and the position are set, because the two versions disagree
	// about which is the original: 1.8.9 moves the box and derives the position
	// from it, and 26.1.2 moves the position and rebuilds the box around it. A
	// capture states a position, so this hands each profile the field it reads.
	store.SetEntity(body, entity.State{
		Family:   captured.Family,
		Box:      movement.Box(captured.Spawn.Position, captured.Spawn.Width, captured.Spawn.Height),
		Position: captured.Spawn.Position,
		Motion:   captured.Spawn.Motion,
	})

	kernel, err := sim.NewKernel(profile)
	if err != nil {
		return 0, fmt.Errorf("mctest: %s: %w", captured.Name, err)
	}
	runner := runtime.NewRunner(store, kernel)
	runner.SetScope(sim.Scope{Entities: []entity.ID{body}})

	ctx := context.Background()
	worst := 0.0
	at := 0
	for index, want := range captured.Samples {
		if want.Tick <= at {
			return worst, fmt.Errorf("%w %s: sample %d is at tick %d, which is not after %d",
				ErrFixture, captured.Name, index, want.Tick, at)
		}

		for ; at < want.Tick; at++ {
			result, err := runner.Step(ctx, nil)
			if err != nil {
				return worst, fmt.Errorf("mctest: %s: tick %d: %w", captured.Name, at, err)
			}
			if !result.Completeness.Complete {
				return worst, fmt.Errorf("%w %s: tick %d left the described world: %+v",
					ErrFixture, captured.Name, at, result.Completeness.Missing)
			}
		}

		state, ok := store.Entities().Entity(body)
		if !ok {
			return worst, fmt.Errorf("%w %s: the body is gone at tick %d",
				ErrFixture, captured.Name, at)
		}

		worst = math.Max(worst, deviation(position(state.Box), want.Position))
	}

	return worst, nil
}

// deviation is the largest per-axis difference, so a number can be read against
// the wire's own per-axis resolution without a square root in the way.
func deviation(got, want geom.Vec3) float64 {
	return math.Max(math.Abs(got.X-want.X),
		math.Max(math.Abs(got.Y-want.Y), math.Abs(got.Z-want.Z)))
}

// position is where a body stands: the middle of its box horizontally and its
// bottom vertically, which is the point every protocol sends.
func position(box geom.AABB) geom.Vec3 {
	return geom.Vec3{
		X: (box.MinX + box.MaxX) / 2,
		Y: box.MinY,
		Z: (box.MinZ + box.MaxZ) / 2,
	}
}
