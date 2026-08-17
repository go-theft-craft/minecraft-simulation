package navigation

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func at(x, y, z int32) node {
	return node{Pos: geom.BlockPos{X: x, Y: y, Z: z}, Posture: PostureStand}
}

func TestFrontierPopsLowestPriorityFirst(t *testing.T) {
	var f frontier
	f.push(at(1, 0, 0), 9)
	f.push(at(2, 0, 0), 1)
	f.push(at(3, 0, 0), 5)

	want := []node{at(2, 0, 0), at(3, 0, 0), at(1, 0, 0)}
	for i, expected := range want {
		got, ok := f.pop()
		if !ok {
			t.Fatalf("pop %d: frontier empty", i)
		}
		if got != expected {
			t.Fatalf("pop %d = %v, want %v", i, got, expected)
		}
	}
	if _, ok := f.pop(); ok {
		t.Fatal("pop from an empty frontier reported ok")
	}
}

// The property the determinism gate rests on: equal priorities pop in the node
// order, never in insertion order and never in a heap's incidental order.
func TestFrontierBreaksEqualPrioritiesOnNodeOrder(t *testing.T) {
	insertions := [][]node{
		{at(2, 0, 0), at(1, 0, 0), at(1, 0, 1), at(1, 1, 0)},
		{at(1, 1, 0), at(1, 0, 1), at(1, 0, 0), at(2, 0, 0)},
		{at(1, 0, 1), at(2, 0, 0), at(1, 1, 0), at(1, 0, 0)},
	}
	want := []node{at(1, 0, 0), at(1, 0, 1), at(1, 1, 0), at(2, 0, 0)}

	for _, order := range insertions {
		var f frontier
		for _, n := range order {
			f.push(n, 1)
		}
		for i, expected := range want {
			got, ok := f.pop()
			if !ok {
				t.Fatalf("pop %d: frontier empty", i)
			}
			if got != expected {
				t.Fatalf("insertion %v: pop %d = %v, want %v", order, i, got, expected)
			}
		}
	}
}

func TestNodeLessOrdersByPositionThenPosture(t *testing.T) {
	stand := node{Pos: geom.BlockPos{X: 1, Y: 1, Z: 1}, Posture: PostureStand}
	swim := node{Pos: geom.BlockPos{X: 1, Y: 1, Z: 1}, Posture: PostureSwim}

	if !nodeLess(stand, swim) {
		t.Fatal("nodeLess did not order PostureStand before PostureSwim")
	}
	if nodeLess(swim, stand) {
		t.Fatal("nodeLess is not antisymmetric on posture")
	}
	if nodeLess(stand, stand) {
		t.Fatal("nodeLess reported a node less than itself")
	}
}
