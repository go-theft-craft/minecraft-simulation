package navigation

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func TestGoalBlockIsReachedAtExactlyItsCell(t *testing.T) {
	goal := GoalBlock{Pos: geom.BlockPos{X: 3, Y: 64, Z: -2}}

	if !goal.Reached(geom.BlockPos{X: 3, Y: 64, Z: -2}) {
		t.Fatal("Reached refused the goal cell itself")
	}
	if goal.Reached(geom.BlockPos{X: 3, Y: 65, Z: -2}) {
		t.Fatal("Reached accepted a cell one block above")
	}
}

func TestGoalBlockHeuristicIsManhattanBlocks(t *testing.T) {
	goal := GoalBlock{Pos: geom.BlockPos{X: 3, Y: 64, Z: -2}}

	if got := goal.Heuristic(geom.BlockPos{X: 0, Y: 64, Z: 0}); got != 5 {
		t.Fatalf("Heuristic = %v, want 5 (|3-0| + |64-64| + |-2-0|)", got)
	}
	if got := goal.Heuristic(goal.Pos); got != 0 {
		t.Fatalf("Heuristic at the goal = %v, want 0", got)
	}
}

func TestGoalXZIgnoresHeight(t *testing.T) {
	goal := GoalXZ{X: 10, Z: -4}

	if !goal.Reached(geom.BlockPos{X: 10, Y: 200, Z: -4}) {
		t.Fatal("Reached refused the column at another height")
	}
	if got := goal.Heuristic(geom.BlockPos{X: 0, Y: 63, Z: 0}); got != 14 {
		t.Fatalf("Heuristic = %v, want 14 (|10| + |-4|, no Y term)", got)
	}
}

func TestGoalYLevelIgnoresColumn(t *testing.T) {
	goal := GoalYLevel{Y: 12}

	if !goal.Reached(geom.BlockPos{X: -100, Y: 12, Z: 40}) {
		t.Fatal("Reached refused the level in another column")
	}
	if got := goal.Heuristic(geom.BlockPos{X: 0, Y: 64, Z: 0}); got != 52 {
		t.Fatalf("Heuristic = %v, want 52", got)
	}
}

func TestGoalNearAcceptsTheRadius(t *testing.T) {
	goal := GoalNear{Pos: geom.BlockPos{}, Radius: 3}

	if !goal.Reached(geom.BlockPos{X: 3}) {
		t.Fatal("Reached refused a cell exactly on the radius")
	}
	if goal.Reached(geom.BlockPos{X: 3, Z: 1}) {
		t.Fatal("Reached accepted a cell beyond the radius")
	}
	if got := goal.Heuristic(geom.BlockPos{X: 10}); got != 7 {
		t.Fatalf("Heuristic = %v, want 7 (10 - radius 3)", got)
	}
	if got := goal.Heuristic(geom.BlockPos{X: 1}); got != 0 {
		t.Fatalf("Heuristic inside the radius = %v, want 0", got)
	}
}

func TestGoalGetToBlockIsReachedBesideNotInside(t *testing.T) {
	goal := GoalGetToBlock{Pos: geom.BlockPos{X: 5, Y: 10, Z: 5}}

	if !goal.Reached(geom.BlockPos{X: 4, Y: 10, Z: 5}) {
		t.Fatal("Reached refused a face neighbour")
	}
	if !goal.Reached(geom.BlockPos{X: 5, Y: 11, Z: 5}) {
		t.Fatal("Reached refused the cell above")
	}
	if goal.Reached(goal.Pos) {
		t.Fatal("Reached accepted the block's own cell")
	}
	if goal.Reached(geom.BlockPos{X: 4, Y: 10, Z: 4}) {
		t.Fatal("Reached accepted a diagonal neighbour")
	}
	if got := goal.Heuristic(geom.BlockPos{X: 5, Y: 10, Z: 8}); got != 2 {
		t.Fatalf("Heuristic = %v, want 2 (3 blocks away, satisfied 1 out)", got)
	}
}

func TestGoalRunAwayIsReachedWhenEverySourceIsFar(t *testing.T) {
	goal := GoalRunAway{
		From:     []geom.BlockPos{{X: 0}, {X: 10}},
		Distance: 5,
	}

	if goal.Reached(geom.BlockPos{X: 4}) {
		t.Fatal("Reached accepted a cell only 4 from the nearer source")
	}
	if !goal.Reached(geom.BlockPos{X: 5}) {
		t.Fatal("Reached refused a cell exactly at Distance from both; the boundary counts, as GoalNear's does")
	}
	if !goal.Reached(geom.BlockPos{X: 20}) {
		t.Fatal("Reached refused a cell far from every source")
	}
	// From X=4: source at 0 is 4 away (needs 1 more), source at 10 is 6 away
	// (satisfied). The bound is the worst violation.
	if got := goal.Heuristic(geom.BlockPos{X: 4}); got != 1 {
		t.Fatalf("Heuristic = %v, want 1", got)
	}
}

func TestGoalCompositeTakesAnyMemberAndTheNearestBound(t *testing.T) {
	goal := GoalComposite{
		GoalBlock{Pos: geom.BlockPos{X: 10}},
		GoalBlock{Pos: geom.BlockPos{X: -2}},
	}

	if !goal.Reached(geom.BlockPos{X: -2}) {
		t.Fatal("Reached refused a member's cell")
	}
	if got := goal.Heuristic(geom.BlockPos{}); got != 2 {
		t.Fatalf("Heuristic = %v, want 2 (the nearer member)", got)
	}
}

func TestGoalInvertedIsNeverReached(t *testing.T) {
	goal := GoalInverted{Goal: GoalBlock{Pos: geom.BlockPos{}}}

	if goal.Reached(geom.BlockPos{X: 1000}) {
		t.Fatal("an inverted goal must never be reached")
	}
	if got := goal.Heuristic(geom.BlockPos{X: 4}); got != -4 {
		t.Fatalf("Heuristic = %v, want -4 (the inner bound, negated)", got)
	}
}

func TestGoalAxisMeasuresTheNearestAxisOrDiagonal(t *testing.T) {
	goal := GoalAxis{Y: 64}

	if !goal.Reached(geom.BlockPos{X: 7, Y: 64, Z: 7}) {
		t.Fatal("Reached refused the x=z diagonal at the right height")
	}
	if goal.Reached(geom.BlockPos{X: 7, Y: 63, Z: 7}) {
		t.Fatal("Reached accepted the diagonal at the wrong height")
	}
	// (6, 64, 8): the x=z diagonal is |6-8| = 2 coordinate changes away,
	// nearer than the x axis (8) or the z axis (6).
	if got := goal.Heuristic(geom.BlockPos{X: 6, Y: 64, Z: 8}); got != 2 {
		t.Fatalf("Heuristic = %v, want 2", got)
	}
}
