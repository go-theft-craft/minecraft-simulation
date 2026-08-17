package sim

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// DependencyKind says which kind of data a dependency names.
type DependencyKind uint8

const (
	// DependencyBlock names one block cell.
	DependencyBlock DependencyKind = iota + 1
	// DependencyEntity names one entity.
	DependencyEntity
	// DependencyRegistry names a whole registry, such as the block registry.
	DependencyRegistry
)

// String returns the kind's name.
func (d DependencyKind) String() string {
	switch d {
	case DependencyBlock:
		return "block"
	case DependencyEntity:
		return "entity"
	case DependencyRegistry:
		return "registry"
	default:
		return fmt.Sprintf("DependencyKind(%d)", uint8(d))
	}
}

// Dependency names one piece of data a tick read, or one it needed and could
// not get.
//
// The same type serves both because an incomplete tick's missing set is a
// subset of what it tried to read, and a caller that wants to load the gap and
// try again needs the two described the same way.
//
// The struct is a flat union rather than an interface so that it stays
// comparable and canonically encodable. Fields that a kind does not use are
// zero.
type Dependency struct {
	Kind   DependencyKind
	Block  geom.BlockPos
	Entity entity.ID
	Name   string
}

// String names only the field the kind uses.
func (d Dependency) String() string {
	switch d.Kind {
	case DependencyBlock:
		return fmt.Sprintf("block(%d,%d,%d)", d.Block.X, d.Block.Y, d.Block.Z)
	case DependencyEntity:
		return fmt.Sprintf("entity(%d)", d.Entity)
	case DependencyRegistry:
		return fmt.Sprintf("registry(%s)", d.Name)
	default:
		return d.Kind.String()
	}
}

// Completeness reports whether a tick had everything it needed.
//
// A tick that did not is not an error: the caller is expected to load the named
// data and run the tick again. The result of an incomplete tick carries no
// applicable change set and no events, so applying it is impossible rather than
// merely discouraged.
type Completeness struct {
	// Complete is true when every rule in scope had the data it asked for.
	Complete bool
	// Missing names the data that was not available, sorted and deduplicated.
	Missing []Dependency
}

// sortDependencies returns a deduplicated copy in a total order.
//
// The order is by kind, then name, then block coordinate, then entity, and it
// exists so that a digest cannot depend on the order in which a rule happened
// to walk the world. The argument is not modified.
func sortDependencies(in []Dependency) []Dependency {
	if len(in) == 0 {
		return nil
	}

	out := slices.Clone(in)
	slices.SortFunc(out, func(a, b Dependency) int {
		return cmp.Or(
			cmp.Compare(a.Kind, b.Kind),
			cmp.Compare(a.Name, b.Name),
			cmp.Compare(a.Block.X, b.Block.X),
			cmp.Compare(a.Block.Y, b.Block.Y),
			cmp.Compare(a.Block.Z, b.Block.Z),
			cmp.Compare(a.Entity, b.Entity),
		)
	})

	return slices.Compact(out)
}
