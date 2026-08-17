package sim

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/movement"
)

func sampleResult() TickResult {
	return TickResult{
		Revision: 3,
		Tick:     11,
		Changes: ChangeSet{BaseRevision: 3, Ops: []Op{
			{Kind: OpSetEntity, Entity: 1, State: entity.State{
				Family: entity.FamilyPlayer,
				Box:    geom.AABB{MaxX: 0.6, MaxY: 1.8, MaxZ: 0.6},
				Motion: geom.Vec3{Y: -0.0784000015258789},
			}},
			{Kind: OpSetBlock, Block: geom.BlockPos{X: 1, Y: 2, Z: 3}, Ref: 9},
			{Kind: OpSetLocomotion, Entity: 1, Locomotion: movement.Locomotion{
				JumpTicks: 10, Yaw: 90, MoveSpeed: 0.10000000149011612, JumpFactor: 0.02,
			}},
		}},
		Domain:       []DomainEvent{{Kind: "movement.collided", Entity: 1}},
		Presentation: []PresentationEvent{{Kind: "movement.step", Entity: 1}},
		Outcomes:     []CommandOutcome{{Index: 0, Kind: "movement.walk", Accepted: true}},
		Random:       RandomState{}.WithStream("world", 7),
		Read:         []Dependency{{Kind: DependencyBlock, Block: geom.BlockPos{X: 1}}},
		Completeness: Completeness{Complete: true},
	}
}

var sampleProfileID = ProfileID{Edition: "java", GameVersion: "1.8.9", RulesRevision: "1"}

func TestDigestIsStableAcrossRuns(t *testing.T) {
	first := sampleResult().computeDigest(sampleProfileID)
	second := sampleResult().computeDigest(sampleProfileID)

	if first != second {
		t.Fatalf("the same result digested differently: %s vs %s", first, second)
	}
	if first.IsZero() {
		t.Fatal("a non-empty result digested to zero")
	}
}

func TestDigestIgnoresTheDigestField(t *testing.T) {
	result := sampleResult()
	want := result.computeDigest(sampleProfileID)

	result.Digest = want
	if got := result.computeDigest(sampleProfileID); got != want {
		t.Fatalf("digest changed once the field was filled: %s vs %s", got, want)
	}
}

func TestDigestSeparatesProfiles(t *testing.T) {
	other := ProfileID{Edition: "java", GameVersion: "26.1.2", RulesRevision: "1"}
	if sampleResult().computeDigest(sampleProfileID) == sampleResult().computeDigest(other) {
		t.Fatal("two profiles produced the same digest for the same result")
	}
}

func TestDigestNoticesEveryField(t *testing.T) {
	base := sampleResult().computeDigest(sampleProfileID)

	for name, mutate := range map[string]func(*TickResult){
		"revision":      func(r *TickResult) { r.Revision++ },
		"tick":          func(r *TickResult) { r.Tick++ },
		"base revision": func(r *TickResult) { r.Changes.BaseRevision++ },
		"an operation":  func(r *TickResult) { r.Changes.Ops[1].Ref++ },
		"operation order": func(r *TickResult) {
			r.Changes.Ops[0], r.Changes.Ops[1] = r.Changes.Ops[1], r.Changes.Ops[0]
		},
		"a motion bit": func(r *TickResult) {
			r.Changes.Ops[0].State.Motion.Y = -0.078400001525878
		},
		"a domain event":       func(r *TickResult) { r.Domain[0].Kind = "movement.landed" },
		"a presentation event": func(r *TickResult) { r.Presentation[0].Kind = "movement.splash" },
		"an outcome":           func(r *TickResult) { r.Outcomes[0].Accepted = false },
		"random state":         func(r *TickResult) { r.Random = r.Random.WithStream("world", 8) },
		"a read dependency":    func(r *TickResult) { r.Read[0].Block.X = 2 },
		"locomotion": func(r *TickResult) {
			r.Changes.Ops[2].Locomotion.JumpTicks++
		},
		"a locomotion float": func(r *TickResult) {
			// One float32 step away, which a float64 encoding of the same field
			// would also notice; the width is pinned by the encoder's own test.
			r.Changes.Ops[2].Locomotion.Yaw = 90.00001
		},
		"completeness": func(r *TickResult) { r.Completeness = Completeness{} },
	} {
		t.Run(name, func(t *testing.T) {
			result := sampleResult()
			mutate(&result)
			if got := result.computeDigest(sampleProfileID); got == base {
				t.Fatalf("changing %s did not change the digest", name)
			}
		})
	}
}

func TestDigestStringIsHex(t *testing.T) {
	got := sampleResult().computeDigest(sampleProfileID).String()
	if len(got) != 64 {
		t.Fatalf("String returned %d characters, want 64: %q", len(got), got)
	}
}
