package mining

import (
	"errors"

	"github.com/go-theft-craft/minecraft-protocol/data"

	"github.com/go-theft-craft/minecraft-simulation/world"
)

// ErrUnknownBlock reports a handle the classifier's profile did not mint.
var ErrUnknownBlock = errors.New("mining: no such block")

// Held is the item in the player's hand, as a version's own item id and the
// enchantment levels that change a break time.
//
// An id rather than a modelled stack, because that is all this question needs.
// Modelling an inventory item here would put M9.7's data model in M9.4's path.
// The zero value is a bare hand: no version numbers an item zero.
type Held struct {
	Item       data.ItemID
	Efficiency int
}

// Effects are the status effects that change a break time.
//
// Amplifiers, not levels: haste I is amplifier 0, as the protocol sends it,
// which is why the presence of an effect is a separate field from its strength.
type Effects struct {
	Haste, MiningFatigue int
	HasHaste, HasFatigue bool
}

// Classifier is a profile that can say how one block breaks under one held item.
//
// It is optional for the same reason sim.BlockNames is: nothing inside a tick
// reads it — the dig phase is handed conditions rather than resolving them — and
// a profile assembled in a test has no registries to answer from. A caller that
// needs it asserts for it and reports a profile that cannot answer, rather than
// every profile being obliged to carry block data.
//
// It is declared here rather than in sim because mining imports sim, so a sim
// interface returning a mining.Conditions would be an import cycle. The
// vocabulary belongs to the package that owns it either way.
//
// It is per-version because the two editions classify blocks by different
// vocabularies. 1.8.9's material for stone is "rock"; 26.1.2's is
// "mineable/pickaxe", and 26.1.2 additionally encodes tool correctness as
// materials named "incorrect_for_<tier>_tool" that 1.8.9 has no counterpart
// for. A shared lookup keyed by material name would miss on every 26.1 block.
type Classifier interface {
	// Conditions resolves everything version-specific about breaking one block
	// with one held item.
	Conditions(ref world.BlockRef, held Held, effects Effects, underwater, airborne bool) (Conditions, error)
	// Hardness returns the block's hardness, or nil when it has none.
	Hardness(ref world.BlockRef) *float64
}
