// Package sim owns the tick contract: what one simulation step consumes, what
// it produces, how a result is canonically encoded, and the kernel that runs a
// profile's phases.
//
// Nothing here implements a game rule. A profile supplies the rules; this
// package supplies the shape they run in, the record they produce, and the
// guarantees that record carries. Nothing in this package reads the clock, uses
// global random state, or depends on map iteration order.
package sim

import (
	"fmt"
	"strings"
)

// Revision counts change sets a store has applied. A change set names the
// revision it was computed against and applies only to a store still holding
// it.
type Revision uint64

// Tick counts simulated ticks. It is not a Revision: an incomplete, cancelled,
// or rejected tick advances this counter and produces no revision at all.
type Tick uint64

// ProfileID names a set of rules. The game version and the rules revision are
// separate fields because a fix to our implementation of 1.8.9 changes the
// second without touching the first, and a replay must be able to tell those
// apart.
type ProfileID struct {
	// Edition is the game edition, such as "java".
	Edition string
	// GameVersion is the version whose behaviour is reproduced, such as "1.8.9".
	GameVersion string
	// RulesRevision is our implementation revision of those rules.
	RulesRevision string
}

// String returns the identity as edition/version@rules.
func (p ProfileID) String() string {
	return fmt.Sprintf("%s/%s@%s", p.Edition, p.GameVersion, p.RulesRevision)
}

// IsZero reports whether every field is empty.
func (p ProfileID) IsZero() bool {
	return p.Edition == "" && p.GameVersion == "" && p.RulesRevision == ""
}

// validate reports why the identity cannot name a profile.
func (p ProfileID) validate() error {
	var missing []string
	if p.Edition == "" {
		missing = append(missing, "edition")
	}
	if p.GameVersion == "" {
		missing = append(missing, "game version")
	}
	if p.RulesRevision == "" {
		missing = append(missing, "rules revision")
	}
	if len(missing) != 0 {
		return fmt.Errorf("sim: profile identity is missing its %s", strings.Join(missing, ", "))
	}

	return nil
}
