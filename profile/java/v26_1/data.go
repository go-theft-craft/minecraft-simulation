package v26_1

import (
	"crypto/sha256"
	"encoding/binary"
	"math"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// DataDigest implements sim.DataDigest.
//
// It hashes the numbers this profile was built from, not the file they came
// from: the block table, the motion constants, and the trigonometry table, each
// at the width the rules use. A recording that pins this can say which dataset
// it ran against, and a dataset corrected without a version bump — which is
// exactly what a physics correction looks like — stops being an unexplained
// digest mismatch and becomes a named one.
//
// The digest is computed once, when the profile is built, because a replay asks
// for it per recording and this version's block table is thirty thousand states
// deep.
func (p *profile) DataDigest() sim.Digest { return p.dataDigest }

// computeDataDigest hashes every number the profile holds.
//
// Floats are hashed by their bits rather than their decimal text. That is the
// whole point: the values this milestone cares about differ from their round
// decimals in the last bits, and a digest over "0.6" would be blind to the
// difference between 0.6 and float64(float32(0.6)).
//
// The block table is walked by handle, which is by state. A shape shared between
// two states is hashed twice, deliberately: the digest describes what the
// profile answers, and it answers per state.
func computeDataDigest(blocks blockTable, table []float32, motion map[entity.Family]sim.MotionConstants) sim.Digest {
	hash := sha256.New()

	var scratch [8]byte
	writeUint := func(value uint64) {
		binary.BigEndian.PutUint64(scratch[:], value)
		hash.Write(scratch[:])
	}
	writeFloat := func(value float64) { writeUint(math.Float64bits(value)) }
	writeSingle := func(value float32) { writeUint(uint64(math.Float32bits(value))) }
	writeString := func(value string) {
		writeUint(uint64(len(value)))
		hash.Write([]byte(value))
	}
	writeBox := func(box geom.AABB) {
		writeFloat(box.MinX)
		writeFloat(box.MinY)
		writeFloat(box.MinZ)
		writeFloat(box.MaxX)
		writeFloat(box.MaxY)
		writeFloat(box.MaxZ)
	}

	// The blocks, in the order the dataset listed them, with the friction each
	// one answers with.
	writeUint(uint64(len(blocks.names)))
	for index, name := range blocks.names {
		writeString(name)
		writeSingle(blocks.frictions[index])
	}

	// The states, in handle order, each with the block it belongs to and the
	// shape it stands in.
	writeUint(uint64(len(blocks.owner)))
	for handle := range blocks.owner {
		writeUint(uint64(blocks.owner[handle]))

		// BoxesAt at the origin is the shape's own boxes: translating by a zero
		// vector changes no value, and there is no other accessor.
		boxes := blocks.shapes[handle].BoxesAt(geom.BlockPos{}, nil)
		writeUint(uint64(len(boxes)))
		for _, box := range boxes {
			writeBox(box)
		}
	}

	// The motion constants, walked over the families this profile knows rather
	// than over the map, because map order is not an order.
	//
	// A family the profile carries no constants for is skipped rather than
	// hashed as zeroes. The digest says what the profile was built from, and a
	// family that arrived in the code without arriving in the dataset changes
	// nothing about the numbers a tick runs on — hashing it would invalidate
	// every recording for a change that could not move a body.
	for _, family := range []entity.Family{
		entity.FamilyUnknown, entity.FamilyPlayer, entity.FamilyItem, entity.FamilyArrow,
	} {
		constants, carried := motion[family]
		if !carried {
			continue
		}

		writeUint(uint64(family))
		writeFloat(constants.Gravity)
		writeFloat(constants.HorizontalDrag)
		writeFloat(constants.VerticalDrag)
		writeFloat(constants.StepHeight)
	}

	// The trigonometry table. It is large and it is data: a dataset that
	// regenerated it differently would change every trajectory in the module.
	writeUint(uint64(len(table)))
	for _, entry := range table {
		writeSingle(entry)
	}

	var digest sim.Digest
	copy(digest[:], hash.Sum(nil))

	return digest
}
