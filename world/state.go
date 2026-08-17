package world

import "github.com/go-theft-craft/minecraft-simulation/geom"

// BlockRef is an opaque handle for one block state, minted by a profile.
//
// This package never interprets it. Only the profile that produced a handle can
// say which block and which state it names, which is what keeps world free of
// any particular version's block numbering. A profile answers questions about a
// handle, such as its slipperiness or its collision shape.
//
// The zero handle carries no meaning. It is what an implementation records when
// a caller supplied a shape without a handle, and a profile is free to treat it
// as unknown.
type BlockRef uint32

// StateView answers which block state occupies a cell.
//
// The lookup follows the same three-way rule as CollisionShape: a caller can
// tell known air from a known block from a region nobody has described.
type StateView interface {
	BlockState(pos geom.BlockPos) (BlockRef, Lookup)
}

// View is everything the kernel reads about blocks in one tick.
//
// An implementation must be deterministic and must stay valid for the whole of
// a tick: the same position answers the same way from the first phase to the
// last.
type View interface {
	BlockView
	StateView
}
