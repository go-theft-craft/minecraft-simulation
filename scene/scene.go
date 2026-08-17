// Package scene describes a world by name, so that something outside the
// simulation can hand a profile a world to run in.
//
// Blocks are named rather than carrying handles because a handle is an index
// into the table of the profile that minted it: it means nothing to another
// profile, and it means the wrong thing if that table is ever renumbered. A
// name survives both.
//
// A description is a filled region plus the exceptions, rather than a list of
// cells. The region matters as much as the blocks in it: the tick reports itself
// incomplete over any cell nobody described, so a description that named only
// its solid blocks would leave a body unable to move through the air around
// them. Writing that air out cell by cell is thousands of lines nobody reads,
// which is why the fill exists.
package scene

import (
	"errors"
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/sim"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// ErrScene reports a description that cannot be turned into a world.
var ErrScene = errors.New("scene: invalid world description")

// Block is a named cell, or a named box of them.
//
// A floor and a wall are each one entry rather than a few hundred, which is what
// keeps a committed description small enough to read in a diff.
type Block struct {
	// Pos is the cell, or the low corner when To is set.
	Pos geom.BlockPos `json:"pos"`
	// To is the high corner of an inclusive box. It is absent for a single cell.
	To   *geom.BlockPos `json:"to,omitempty"`
	Name string         `json:"name"`
}

// Cells walks every cell this entry names, in a fixed order.
//
// A reversed box names nothing rather than looping forever or silently
// swapping its corners: a caller that wrote its corners the wrong way round has
// a bug, and Validate is where it is told so.
func (b Block) Cells(visit func(geom.BlockPos)) {
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

// World is a described region with blocks in it.
type World struct {
	// Min and Max bound the described region, inclusive.
	Min geom.BlockPos `json:"min"`
	Max geom.BlockPos `json:"max"`
	// Fill is the block every cell in the region holds unless Blocks says
	// otherwise.
	Fill string `json:"fill"`
	// Blocks are the exceptions, in placement order. Later entries win, so a
	// description can lay a floor and then replace patches of it.
	Blocks []Block `json:"blocks"`
}

// Describe writes the world through set, which is a store's SetBlock.
//
// It takes a function rather than returning a store so that this package stays
// a description: a caller that keeps its world somewhere other than the
// in-memory store still gets the region, the ordering, and the name resolution.
func (w World) Describe(profile sim.Profile, set func(geom.BlockPos, world.BlockRef) error) error {
	names, ok := profile.(sim.BlockNames)
	if !ok {
		return fmt.Errorf(
			"%w: profile %s cannot resolve block names, and this world is described by name",
			ErrScene, profile.ID(),
		)
	}
	if err := w.Validate(); err != nil {
		return err
	}

	fill, ok := names.Ref(w.Fill)
	if !ok {
		return fmt.Errorf("%w: the profile does not know the fill block %q", ErrScene, w.Fill)
	}

	for x := w.Min.X; x <= w.Max.X; x++ {
		for y := w.Min.Y; y <= w.Max.Y; y++ {
			for z := w.Min.Z; z <= w.Max.Z; z++ {
				if err := set(geom.BlockPos{X: x, Y: y, Z: z}, fill); err != nil {
					return fmt.Errorf("scene: fill the region: %w", err)
				}
			}
		}
	}

	for _, block := range w.Blocks {
		ref, ok := names.Ref(block.Name)
		if !ok {
			return fmt.Errorf("%w: the profile does not know the block %q at %+v",
				ErrScene, block.Name, block.Pos)
		}

		var failure error
		block.Cells(func(pos geom.BlockPos) {
			if failure != nil {
				return
			}
			failure = set(pos, ref)
		})
		if failure != nil {
			return fmt.Errorf("scene: place %q: %w", block.Name, failure)
		}
	}

	return nil
}

// Validate reports a description that cannot mean what it says.
//
// The checks are the ones whose failure is silent rather than loud: a region
// with its corners the wrong way round describes nothing, and every tick over it
// is incomplete for a reason that names cells rather than the description. A
// block outside the region is the same mistake seen from the other side.
func (w World) Validate() error {
	if w.Fill == "" {
		return fmt.Errorf("%w: it names no fill, so the region it claims to describe is empty", ErrScene)
	}
	if w.Max.X < w.Min.X || w.Max.Y < w.Min.Y || w.Max.Z < w.Min.Z {
		return fmt.Errorf("%w: the region runs from %+v to %+v, which is inside out",
			ErrScene, w.Min, w.Max)
	}

	for _, block := range w.Blocks {
		far := block.Pos
		if block.To != nil {
			far = *block.To
		}
		if far.X < block.Pos.X || far.Y < block.Pos.Y || far.Z < block.Pos.Z {
			return fmt.Errorf("%w: %q runs from %+v to %+v, which is inside out",
				ErrScene, block.Name, block.Pos, far)
		}
		if !w.contains(block.Pos) || !w.contains(far) {
			return fmt.Errorf("%w: %q at %+v..%+v is outside the described region %+v..%+v",
				ErrScene, block.Name, block.Pos, far, w.Min, w.Max)
		}
	}

	return nil
}

// Cells reports how many cells the region holds, which is what a caller sizing
// a store wants to know.
func (w World) Cells() int {
	if w.Max.X < w.Min.X || w.Max.Y < w.Min.Y || w.Max.Z < w.Min.Z {
		return 0
	}

	return int(w.Max.X-w.Min.X+1) * int(w.Max.Y-w.Min.Y+1) * int(w.Max.Z-w.Min.Z+1)
}

func (w World) contains(pos geom.BlockPos) bool {
	return pos.X >= w.Min.X && pos.X <= w.Max.X &&
		pos.Y >= w.Min.Y && pos.Y <= w.Max.Y &&
		pos.Z >= w.Min.Z && pos.Z <= w.Max.Z
}
