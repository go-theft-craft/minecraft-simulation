package sim

import (
	"bytes"
	"math"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func TestEncoderIsBigEndianAndFixedWidth(t *testing.T) {
	var e encoder
	e.uint32(1)
	if got, want := e.buf, []byte{0, 0, 0, 1}; !bytes.Equal(got, want) {
		t.Fatalf("uint32(1) = %v, want %v", got, want)
	}
}

func TestEncoderWritesNegativeIntegersAsTwosComplement(t *testing.T) {
	var e encoder
	e.int32(-1)
	if got, want := e.buf, []byte{0xFF, 0xFF, 0xFF, 0xFF}; !bytes.Equal(got, want) {
		t.Fatalf("int32(-1) = %v, want %v", got, want)
	}
}

func TestEncoderFoldsNegativeZero(t *testing.T) {
	var positive, negative encoder
	positive.float64(0)
	negative.float64(math.Copysign(0, -1))

	if !bytes.Equal(positive.buf, negative.buf) {
		t.Fatalf("zero encodes differently by sign: %v vs %v", positive.buf, negative.buf)
	}
}

func TestEncoderFoldsEveryNaNToOnePattern(t *testing.T) {
	first := math.Float64frombits(0x7FF8000000000001)
	second := math.Float64frombits(0xFFF8000000000002)

	var a, b encoder
	a.float64(first)
	b.float64(second)

	if !bytes.Equal(a.buf, b.buf) {
		t.Fatalf("two NaNs encode differently: %v vs %v", a.buf, b.buf)
	}
}

func TestEncoderDistinguishesValuesThatSharePrefixes(t *testing.T) {
	// Without a length prefix, "ab" then "c" and "a" then "bc" would encode
	// alike, and a digest could not tell two different results apart.
	var first, second encoder
	first.string("ab")
	first.string("c")
	second.string("a")
	second.string("bc")

	if bytes.Equal(first.buf, second.buf) {
		t.Fatal("two different string sequences encode identically")
	}
}

func TestEncoderTagsSeparateDomains(t *testing.T) {
	var tagged, plain encoder
	tagged.tag(tagVec)
	tagged.float64(1)
	plain.float64(1)

	if bytes.Equal(tagged.buf, plain.buf) {
		t.Fatal("a tagged value encodes the same as an untagged one")
	}
}

func TestEncoderWritesCompositesReproducibly(t *testing.T) {
	build := func() []byte {
		var e encoder
		e.vec(geom.Vec3{X: 1, Y: -2, Z: 0.5})
		e.box(geom.AABB{MinX: -1, MaxY: 3})
		e.blockPos(geom.BlockPos{X: -5, Y: 60, Z: 7})
		e.count(2)

		return e.buf
	}

	if !bytes.Equal(build(), build()) {
		t.Fatal("encoding the same values twice produced different bytes")
	}
	if len(build()) == 0 {
		t.Fatal("encoding produced no bytes")
	}
}
