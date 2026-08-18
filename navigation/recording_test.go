package navigation

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/terrain"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

func TestRecordingViewLogsEveryCellRead(t *testing.T) {
	blocks := flat(-1, -1, 1, 1)
	recorder := &recordingView{view: blocks}

	recorder.CollisionShape(geom.BlockPos{X: 0, Y: 0, Z: 0})
	recorder.BlockState(geom.BlockPos{X: 1, Y: 0, Z: 0})

	read := recorder.read()
	if len(read) != 2 {
		t.Fatalf("read %d cells, want 2: %v", len(read), read)
	}
}

// A cell read twice is recorded once. Without this the dependency set of one
// Passable answer grows with the body's height for no benefit.
func TestRecordingViewDeduplicates(t *testing.T) {
	blocks := flat(-1, -1, 1, 1)
	recorder := &recordingView{view: blocks}

	cell := geom.BlockPos{X: 0, Y: 0, Z: 0}
	recorder.CollisionShape(cell)
	recorder.BlockState(cell)
	recorder.CollisionShape(cell)

	if read := recorder.read(); len(read) != 1 {
		t.Fatalf("read %d cells, want 1: %v", len(read), read)
	}
}

func TestRecordingViewResetClearsTheLog(t *testing.T) {
	blocks := flat(-1, -1, 1, 1)
	recorder := &recordingView{view: blocks}

	recorder.CollisionShape(geom.BlockPos{X: 0, Y: 0, Z: 0})
	recorder.reset()

	if read := recorder.read(); len(read) != 0 {
		t.Fatalf("read %d cells after reset, want 0", len(read))
	}
}

// The decorator must answer exactly what it wraps. A recorder that changed an
// answer would corrupt every cached result built on it.
func TestRecordingViewAnswersAsItsWrappedView(t *testing.T) {
	blocks := flat(-1, -1, 1, 1)
	blocks.Set(geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.FullCube())
	recorder := &recordingView{view: blocks}

	for _, cell := range []geom.BlockPos{{X: 0, Y: 0, Z: 0}, {X: 1, Y: 0, Z: 0}, {X: 9, Y: 9, Z: 9}} {
		wantShape, wantLookup := blocks.CollisionShape(cell)
		gotShape, gotLookup := recorder.CollisionShape(cell)
		if gotLookup != wantLookup || gotShape.Len() != wantShape.Len() {
			t.Fatalf("CollisionShape(%v) = %v/%v, want %v/%v", cell, gotShape.Len(), gotLookup, wantShape.Len(), wantLookup)
		}

		wantRef, wantLookup := blocks.BlockState(cell)
		gotRef, gotLookup := recorder.BlockState(cell)
		if gotRef != wantRef || gotLookup != wantLookup {
			t.Fatalf("BlockState(%v) = %v/%v, want %v/%v", cell, gotRef, gotLookup, wantRef, wantLookup)
		}
	}
}

// It must satisfy world.View, since terrain.Query takes one.
var _ world.View = (*recordingView)(nil)

// And a terrain.Query built on it must work unchanged.
func TestRecordingViewDrivesATerrainQuery(t *testing.T) {
	blocks := flat(-1, -1, 1, 1)
	recorder := &recordingView{view: blocks}
	query := terrain.Query{View: recorder, Body: walker.Body}

	got, err := query.Passable(geom.BlockPos{X: 0, Y: 0, Z: 0})
	if err != nil {
		t.Fatalf("Passable returned an error: %v", err)
	}
	if got != terrain.Clear {
		t.Fatalf("Passable = %v, want Clear", got)
	}
	if len(recorder.read()) == 0 {
		t.Fatal("a Passable query recorded no reads")
	}
}
