package sim

import (
	"encoding/binary"
	"math"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// The canonical encoding is fixed-width and big-endian, and every composite
// value is preceded by a one-byte tag.
//
// The tags are domain separation. Without them two different structures could
// produce the same bytes, and a digest that cannot tell them apart is worse
// than no digest, because it would report agreement between results that
// disagree.
//
// Tags are append-only. Changing an existing tag's value changes every digest
// ever recorded, so a new kind of value takes a new tag.
const (
	tagResult uint8 = iota + 1
	tagChangeSet
	tagOp
	tagEntityState
	tagDomainEvent
	tagPresentationEvent
	tagCommandOutcome
	tagRandomState
	tagRandomStream
	tagDependency
	tagCompleteness
	tagProfileID
	tagVec
	tagBox
	tagBlockPos
	// tagLocomotion arrived with M8.4 and is appended rather than inserted. A
	// tag's value is part of every digest ever recorded, so the list grows at
	// the end and never in the middle.
	tagLocomotion
	// tagPosition arrived with M8.7, for a version that keeps a body's position
	// as well as its box. It is appended for the same reason, and it is written
	// only when the position is non-zero — so a profile that has none encodes
	// exactly the bytes it encoded before this tag existed, and every recording
	// made before M8.7 still verifies.
	tagPosition
	// tagSupport arrived with M8.7 too, for the block a version records as
	// holding a body up. Written only when there is one, for the same reason
	// tagPosition is.
	tagSupport
	// tagVitals arrived with M9.6, for a body whose health or last attack is
	// tracked. Written only when the record is non-zero, so a body nobody
	// fights encodes exactly the bytes it encoded before combat existed, and
	// every recording made before M9.6 still verifies.
	tagVitals
)

// canonicalNaN is the single pattern every NaN encodes as. NaN payloads are not
// portable across platforms, and M8.6 gates on digests matching across six of
// them.
const canonicalNaN uint64 = 0x7FF8000000000000

// canonicalNaN32 is the same fold at single width.
const canonicalNaN32 uint32 = 0x7FC00000

// encoder builds a canonical byte string. It never fails: every value this
// package holds has an encoding, and a value that did not would be a compile
// error rather than a runtime one.
type encoder struct {
	buf []byte
}

// tag writes a domain separator.
func (e *encoder) tag(value uint8) {
	e.buf = append(e.buf, value)
}

// bool writes one byte.
func (e *encoder) bool(value bool) {
	if value {
		e.buf = append(e.buf, 1)

		return
	}
	e.buf = append(e.buf, 0)
}

// uint8 writes one byte.
func (e *encoder) uint8(value uint8) {
	e.buf = append(e.buf, value)
}

// uint32 writes four bytes, big-endian.
func (e *encoder) uint32(value uint32) {
	e.buf = binary.BigEndian.AppendUint32(e.buf, value)
}

// int32 writes four bytes of two's complement, big-endian.
func (e *encoder) int32(value int32) {
	e.uint32(uint32(value))
}

// uint64 writes eight bytes, big-endian.
func (e *encoder) uint64(value uint64) {
	e.buf = binary.BigEndian.AppendUint64(e.buf, value)
}

// float64 writes the IEEE 754 bits, big-endian, after folding negative zero to
// zero and every NaN to one pattern. See canonicalNaN and the package
// documentation for why.
func (e *encoder) float64(value float64) {
	switch {
	case math.IsNaN(value):
		e.uint64(canonicalNaN)
	case value == 0:
		// Catches both zeros: -0.0 == 0.0 is true.
		e.uint64(0)
	default:
		e.uint64(math.Float64bits(value))
	}
}

// float32 writes a float at the width the game computed it, folding negative
// zero and NaN by the same rules float64 uses.
//
// A float32 is written as four bytes rather than widened to eight, so that the
// encoding records the width the value actually has. Widening here would let two
// different float32 values that share a float64 rounding encode alike.
func (e *encoder) float32(value float32) {
	switch {
	case value != value: // NaN, without importing math for one comparison.
		e.uint32(canonicalNaN32)
	case value == 0:
		e.uint32(0)
	default:
		e.uint32(math.Float32bits(value))
	}
}

// count writes a length as four bytes. Lengths are encoded so that two
// sequences cannot run together into the same bytes.
func (e *encoder) count(value int) {
	e.uint32(uint32(value))
}

// string writes a length-prefixed UTF-8 string.
func (e *encoder) string(value string) {
	e.count(len(value))
	e.buf = append(e.buf, value...)
}

// vec writes a vector.
func (e *encoder) vec(value geom.Vec3) {
	e.tag(tagVec)
	e.float64(value.X)
	e.float64(value.Y)
	e.float64(value.Z)
}

// box writes an axis-aligned box.
func (e *encoder) box(value geom.AABB) {
	e.tag(tagBox)
	e.float64(value.MinX)
	e.float64(value.MinY)
	e.float64(value.MinZ)
	e.float64(value.MaxX)
	e.float64(value.MaxY)
	e.float64(value.MaxZ)
}

// blockPos writes a block position.
func (e *encoder) blockPos(value geom.BlockPos) {
	e.tag(tagBlockPos)
	e.int32(value.X)
	e.int32(value.Y)
	e.int32(value.Z)
}
