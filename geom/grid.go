package geom

import (
	"math"
	"slices"
)

// The constants Java Edition builds a block's collider with, from 1.13 onward.
//
// A shape is not stored as the boxes it was written as. The game snaps each box
// to a power-of-two grid inside its cell and stores which cells of that grid are
// filled, so a plate an eighth of a block thick becomes an eighth-resolution
// grid with one layer set — and its coordinate list is every line of that grid,
// not the two faces it was described by. That distinction is invisible to
// collision, which only asks what is solid, and load-bearing for the step-up,
// which asks the shape what heights it offers.
const (
	// gridTolerance is how close a face has to be to a grid line to count as on
	// it. The game scales it by the number of intervals.
	gridTolerance = 1.0e-7
	// gridMaxBits is the finest grid a box may snap to: three bits, so eighths.
	// A box that does not land on eighths keeps its own two faces instead.
	gridMaxBits = 3
	// gridUpperBound is how far past a cell a face may sit and still be
	// considered block-local. It is not exactly one.
	gridUpperBound = 1.0000001
)

// GridY returns the Y coordinates this shape offers, in block-local space and
// ascending order.
//
// For a full cube that is nothing but its two faces. For a slab it is thirds of
// nothing — halves: zero, a half, and one. For a snow layer an eighth thick it
// is all nine eighth-lines, seven of which are empty air. That is what the game
// stores, and a step-up asks for exactly this list.
//
// A shape whose boxes do not fit the grid — one reaching outside its cell, or
// one whose faces land between eighths — keeps its own faces, because that is
// what the game falls back to.
//
// The union across boxes is the merged grid. Every grid here is a power of two,
// so the union of two of them is the finer of the two, which is what the game's
// own merge produces.
func (s Shape) GridY() []float64 {
	var coords []float64
	for _, box := range s.boxes {
		for _, coord := range boxGridY(box) {
			if !slices.Contains(coords, coord) {
				coords = append(coords, coord)
			}
		}
	}
	slices.Sort(coords)

	return coords
}

// boxGridY returns one box's Y coordinates under the game's rules.
func boxGridY(box AABB) []float64 {
	// A box too thin on any axis is not a collider at all.
	if box.MaxX-box.MinX < gridTolerance ||
		box.MaxY-box.MinY < gridTolerance ||
		box.MaxZ-box.MinZ < gridTolerance {
		return nil
	}

	xBits := gridBits(box.MinX, box.MaxX)
	yBits := gridBits(box.MinY, box.MaxY)
	zBits := gridBits(box.MinZ, box.MaxZ)

	// One axis off the grid drops the whole shape to its own faces: the game
	// stores such a box as its coordinates rather than as a grid.
	if xBits < 0 || yBits < 0 || zBits < 0 {
		return []float64{box.MinY, box.MaxY}
	}
	// A box on the whole cell in every axis is the shared full-cube shape.
	if xBits == 0 && yBits == 0 && zBits == 0 {
		return []float64{0, 1}
	}

	intervals := 1 << yBits
	coords := make([]float64, 0, intervals+1)
	for line := range intervals + 1 {
		coords = append(coords, float64(line)/float64(intervals))
	}

	return coords
}

// gridBits returns how many bits of grid a span fits on, or -1 for a span that
// fits none.
//
// The search runs from the coarsest grid up, so a box on the whole cell answers
// zero and a box on halves answers one. A face more than a tolerance off every
// grid line up to eighths has no grid.
func gridBits(minimum, maximum float64) int {
	if minimum < -gridTolerance || maximum > gridUpperBound {
		return -1
	}

	for bits := range gridMaxBits + 1 {
		intervals := float64(int(1) << bits)
		scaledMin := minimum * intervals
		scaledMax := maximum * intervals
		onMin := math.Abs(scaledMin-math.Round(scaledMin)) < gridTolerance*intervals
		onMax := math.Abs(scaledMax-math.Round(scaledMax)) < gridTolerance*intervals
		if onMin && onMax {
			return bits
		}
	}

	return -1
}
