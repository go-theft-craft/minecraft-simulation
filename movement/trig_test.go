package movement

import (
	"math"
	"testing"
)

// identityTable returns a table whose every entry is its own index, so a test
// can name the entry a read chose rather than the value it returned.
func identityTable(t *testing.T) Table {
	t.Helper()

	entries := make([]float32, TableSize)
	for index := range entries {
		entries[index] = float32(index)
	}
	table, err := NewTable(entries)
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}

	return table
}

// gameTable returns the table the game builds, for the range assertions.
func gameTable(t *testing.T) Table {
	t.Helper()

	entries := make([]float32, TableSize)
	for index := range entries {
		entries[index] = float32(math.Sin(float64(index) * math.Pi * 2 / TableSize))
	}
	table, err := NewTable(entries)
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}

	return table
}

func TestNewTableRequiresTheWholeTable(t *testing.T) {
	for _, size := range []int{0, 1, TableSize - 1, TableSize + 1} {
		if _, err := NewTable(make([]float32, size)); err == nil {
			t.Errorf("NewTable accepted %d entries", size)
		}
	}
	if _, err := NewTable(make([]float32, TableSize)); err != nil {
		t.Errorf("NewTable rejected a full table: %v", err)
	}
}

func TestNewTableCopiesItsInput(t *testing.T) {
	entries := make([]float32, TableSize)
	entries[0] = 1
	table, err := NewTable(entries)
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}

	entries[0] = 99
	if got := table.Sin(0); got != 1 {
		t.Fatalf("Sin(0) = %v after the caller mutated its slice, want 1", got)
	}
}

func TestTheIndexArithmeticIsPinned(t *testing.T) {
	table := identityTable(t)

	// Sine reads the first entry at zero, and cosine reads the quarter turn.
	// These are the two indices every other read is measured from.
	if got := table.Sin(0); got != 0 {
		t.Errorf("Sin(0) read entry %v, want 0", got)
	}
	if got := table.Cos(0); got != float32(cosineOffset) {
		t.Errorf("Cos(0) read entry %v, want %d", got, cosineOffset)
	}
}

func TestASmallNegativeAngleTruncatesTowardZero(t *testing.T) {
	table := identityTable(t)

	// -0.0001 · 10430.378 is about -1.043. Truncation gives -1, which the mask
	// wraps to the last entry. Flooring would also give -1 here, but rounding
	// would give -1 as well, so the case that separates them is chosen below.
	if got := table.Sin(-0.0001); got != float32(TableSize-1) {
		t.Errorf("Sin(-0.0001) read entry %v, want %d", got, TableSize-1)
	}

	// -0.00016 · 10430.378 is about -1.669. Truncation gives -1 and rounding
	// would give -2, so this case fails if the conversion ever starts rounding.
	if got := table.Sin(-0.00016); got != float32(TableSize-1) {
		t.Errorf("Sin(-0.00016) read entry %v, want %d (truncation, not rounding)",
			got, TableSize-1)
	}
}

func TestCosineAddsTheQuarterTurnBeforeTruncating(t *testing.T) {
	table := identityTable(t)

	// A negative angle is where adding the offset before the conversion differs
	// from adding it after: -0.00016 · 10430.378 + 16384 is about 16382.33, which
	// truncates to 16382. Adding after truncation would give 16383.
	if got := table.Cos(-0.00016); got != 16382 {
		t.Fatalf("Cos(-0.00016) read entry %v, want 16382", got)
	}
}

func TestSineStaysInRangeOverATurn(t *testing.T) {
	table := gameTable(t)

	for step := range 3600 {
		angle := float32(step) * float32(math.Pi) / 1800
		if got := table.Sin(angle); got < -1 || got > 1 {
			t.Fatalf("Sin(%v) = %v, outside [-1, 1]", angle, got)
		}
		if got := table.Cos(angle); got < -1 || got > 1 {
			t.Fatalf("Cos(%v) = %v, outside [-1, 1]", angle, got)
		}
	}
}

func TestTheMaskMakesTheTablePeriodic(t *testing.T) {
	table := gameTable(t)
	const turn = float32(2 * math.Pi)

	// Not bit-identical: the angle itself rounds differently once a turn is added
	// to it, so the index may land one entry away. Within a table step is what
	// periodicity means here.
	const tolerance = 1e-3
	for _, angle := range []float32{0, 0.5, 1, 2, 3, -1, -2.5} {
		if got, want := table.Sin(angle+turn), table.Sin(angle); math.Abs(float64(got-want)) > tolerance {
			t.Errorf("Sin(%v) = %v but Sin(%v + turn) = %v", angle, want, angle, got)
		}
		if got, want := table.Cos(angle+turn), table.Cos(angle); math.Abs(float64(got-want)) > tolerance {
			t.Errorf("Cos(%v) = %v but Cos(%v + turn) = %v", angle, want, angle, got)
		}
	}
}

func TestAReadDoesNotMutateTheTable(t *testing.T) {
	table := identityTable(t)
	before := table.Sin(1)
	for range 10 {
		table.Sin(1)
		table.Cos(1)
	}
	if got := table.Sin(1); got != before {
		t.Fatalf("Sin(1) = %v after repeated reads, want %v", got, before)
	}
}
