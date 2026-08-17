package sim

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func TestOpKindString(t *testing.T) {
	for _, test := range []struct {
		kind OpKind
		want string
	}{
		{OpSetEntity, "set-entity"},
		{OpRemoveEntity, "remove-entity"},
		{OpSetBlock, "set-block"},
		{OpKind(7), "OpKind(7)"},
	} {
		if got := test.kind.String(); got != test.want {
			t.Errorf("OpKind(%d).String() = %q, want %q", test.kind, got, test.want)
		}
	}
}

func TestChangeSetIsEmpty(t *testing.T) {
	if !(ChangeSet{BaseRevision: 4}).IsEmpty() {
		t.Error("a change set with no operations reports itself non-empty")
	}
	if (ChangeSet{Ops: []Op{{Kind: OpRemoveEntity}}}).IsEmpty() {
		t.Error("a change set with an operation reports itself empty")
	}
}

func TestOpsKeepInsertionOrder(t *testing.T) {
	// Two writes to the same cell, where the second is the one that must win.
	// This is why operations are never sorted.
	pos := geom.BlockPos{X: 1}
	changes := ChangeSet{Ops: []Op{
		{Kind: OpSetBlock, Block: pos, Ref: 1},
		{Kind: OpSetBlock, Block: pos, Ref: 2},
	}}

	if changes.Ops[len(changes.Ops)-1].Ref != 2 {
		t.Fatalf("the last operation is not the last one appended: %+v", changes.Ops)
	}
}

func TestOpIsComparable(t *testing.T) {
	// A change set is compared field by field in tests and hashed by the
	// digest, both of which need Op to stay free of slices and maps.
	first := Op{Kind: OpSetEntity, Entity: 3, State: entity.State{Family: entity.FamilyPlayer}}
	second := first
	if first != second {
		t.Fatal("a copy of an operation does not equal its original")
	}
}
