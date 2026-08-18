package movement

import (
	"fmt"
	"slices"
)

// TableSize is how many entries the game's trigonometry table holds.
const TableSize = 65536

// tableScale converts an angle in radians to a table index. It is a float32
// because the multiply that uses it is a float multiply in the game.
const tableScale float32 = 10430.378

// cosineOffset is the quarter turn that turns a sine read into a cosine read:
// 65536 / 4.
const cosineOffset int32 = 16384

// Table is the game's trigonometry table.
//
// The game builds a 65536-entry table of floats at class initialization and
// never calls a sine function during a tick, so reproducing the formula at
// runtime would risk last-place divergence for no benefit. The profile supplies
// the table from the dataset instead.
type Table struct {
	entries []float32
}

// NewTable returns a table over a copy of entries.
//
// It requires exactly TableSize entries. A short table would still index, wrap,
// and return plausible wrong angles, which is the kind of failure that shows up
// as a trajectory that drifts rather than as an error.
func NewTable(entries []float32) (Table, error) {
	if len(entries) != TableSize {
		return Table{}, fmt.Errorf(
			"movement: trigonometry table has %d entries, want %d", len(entries), TableSize,
		)
	}

	return Table{entries: slices.Clone(entries)}, nil
}

// Len returns how many entries the table holds. A zero table holds none.
func (t Table) Len() int { return len(t.entries) }

// Sin returns the game's sine: a table read, not a computation.
//
// Two details are load-bearing. The multiply is float32, and the conversion
// truncates toward zero rather than rounding, which for a negative angle picks a
// different entry than rounding would — for roughly half of all angles. The mask
// then wraps the signed index, which is what makes the table periodic.
func (t Table) Sin(angle float32) float32 {
	return t.entries[int32(angle*tableScale)&(TableSize-1)]
}

// Cos returns the game's cosine, which is the same table read a quarter turn
// along.
//
// The quarter turn is added as a float, inside the conversion, and not to the
// truncated index. The two agree for a positive angle and disagree for a
// negative one, because truncation toward zero rounds the sum differently than
// it rounds the product.
func (t Table) Cos(angle float32) float32 {
	// The product is converted before the quarter turn is added, for the reason
	// ApplyHeading spells out: a multiply feeding an add is fusable, arm64 fuses
	// it, and a fused index is a different table entry.
	return t.entries[int32(float32(angle*tableScale)+float32(cosineOffset))&(TableSize-1)]
}

// At returns the entry at an index, wrapped into the table.
//
// It exists because the two versions this module carries compute the index
// differently and read the same table: 1.8.9 multiplies at single width and
// truncates through an int, and 26.1.2 multiplies at double width and truncates
// through a long. The mask is the one part they share, so the index is the
// caller's and the wrap is here.
//
// The table itself is measured per version rather than shared. That it is
// byte-identical between these two is a fact about both of them, not a promise
// about the next one.
func (t Table) At(index int64) float32 {
	return t.entries[index&(TableSize-1)]
}
