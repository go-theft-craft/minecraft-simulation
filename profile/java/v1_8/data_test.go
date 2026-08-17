package v1_8

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

func TestTheProfileReportsADataDigest(t *testing.T) {
	reporter, ok := built(t).(sim.DataDigest)
	if !ok {
		t.Fatal("the profile does not report a data digest")
	}
	if reporter.DataDigest().IsZero() {
		t.Fatal("the data digest is zero, which is what a profile that computed none would report")
	}
}

func TestTheDataDigestIsStableAcrossBuilds(t *testing.T) {
	// Two profiles from the same dataset must agree, or every recording would
	// fail its data check on the next process that loaded it.
	first := built(t).(sim.DataDigest).DataDigest()
	second := built(t).(sim.DataDigest).DataDigest()

	if first != second {
		t.Fatalf("two builds of one dataset digest differently: %s and %s", first, second)
	}
}

func TestTheDataDigestNoticesEveryKindOfNumber(t *testing.T) {
	profile := built(t).(*profile)
	base := profile.dataDigest

	// Each case perturbs one kind of value the profile holds. A digest that
	// missed any of them would let a corrected dataset replay as if nothing had
	// changed, which is the failure this pin exists to prevent.
	cases := []struct {
		name  string
		alter func(*blockTable, *[]float32, map[entity.Family]sim.MotionConstants)
	}{
		{"a block name", func(blocks *blockTable, _ *[]float32, _ map[entity.Family]sim.MotionConstants) {
			blocks.names[1] += "_renamed"
		}},
		{"a slipperiness", func(blocks *blockTable, _ *[]float32, _ map[entity.Family]sim.MotionConstants) {
			blocks.frictions[1] += 0.01
		}},
		{"a collision shape", func(blocks *blockTable, _ *[]float32, _ map[entity.Family]sim.MotionConstants) {
			blocks.shapes[1] = geom.NewShape(geom.AABB{MaxX: 1, MaxY: 0.5, MaxZ: 1})
		}},
		{"a trigonometry entry", func(_ *blockTable, table *[]float32, _ map[entity.Family]sim.MotionConstants) {
			(*table)[17] += 1e-7
		}},
		{
			"a motion constant in its last bits",
			func(_ *blockTable, _ *[]float32, motion map[entity.Family]sim.MotionConstants) {
				constants := motion[entity.FamilyPlayer]
				// The round decimal rather than the widened float, which is the
				// difference the whole milestone is about and the one a digest
				// over decimal text would miss.
				constants.StepHeight = 0.6
				motion[entity.FamilyPlayer] = constants
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			blocks := blockTable{
				names:     append([]string(nil), profile.blocks.names...),
				shapes:    append([]geom.Shape(nil), profile.blocks.shapes...),
				frictions: append([]float32(nil), profile.blocks.frictions...),
				byName:    profile.blocks.byName,
			}
			table := append([]float32(nil), dataset(t).Physics().SinTable...)
			motion := make(map[entity.Family]sim.MotionConstants, len(profile.motion))
			for family, constants := range profile.motion {
				motion[family] = constants
			}

			test.alter(&blocks, &table, motion)

			if got := computeDataDigest(blocks, table, motion); got == base {
				t.Fatalf("changing %s left the digest at %s", test.name, got)
			}
		})
	}
}
