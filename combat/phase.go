package combat

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// Attack asks the kernel to swing at a target.
//
// It carries nothing but the two identities. The reach comes from the
// attacker's eye and the target's box, the charge from the attacker's own
// last-attack record, the sprint from its locomotion, and the damage from the
// profile — a command that restated any of those would let a caller disagree
// with the state the kernel already holds.
type Attack struct {
	Attacker entity.ID
	Target   entity.ID
}

// CommandKind implements sim.Command.
func (Attack) CommandKind() string { return "combat.attack" }

// EventStruck is the domain event a landed hit emits, on the target.
const EventStruck = "combat.struck"

// EventDied is the domain event a target emits when a hit takes its health to
// zero. It is emitted exactly once per death: the body is removed in the same
// tick, so a later attack on it finds no entity rather than a corpse that
// dies again.
const EventDied = "combat.died"

// phaseID names the attack phase.
const phaseID = "combat.attack"

// ErrNoCombatRules reports a profile that cannot answer combat questions.
var ErrNoCombatRules = errors.New("combat: this profile declares no combat rules")

// neverAttacked is the tick count handed to the cooldown for a body with no
// last-attack record. Any real delay saturates the charge long before this.
const neverAttacked = math.MaxInt32

// Phase returns the kernel phase that applies attacks: reach, cooldown,
// damage, knockback, death.
//
// It computes rather than accumulates: whether a swing lands and how hard is a
// function of the two bodies and the tick number alone, so the phase needs no
// memory and two kernels stepped with the same input agree — which is what
// the digest is for.
func Phase() sim.Phase { return attackPhase{} }

type attackPhase struct{}

// ID implements sim.Phase.
func (attackPhase) ID() string { return phaseID }

// Run implements sim.Phase.
func (attackPhase) Run(_ context.Context, tick *sim.TickState) error {
	for index, command := range tick.Commands() {
		attack, ok := command.(Attack)
		if !ok {
			continue
		}

		outcome := apply(tick, attack)
		outcome.Index, outcome.Kind = index, attack.CommandKind()
		tick.RecordOutcome(outcome)
	}

	return nil
}

// apply answers one attack.
func apply(tick *sim.TickState, attack Attack) sim.CommandOutcome {
	fighter, ok := tick.Profile().(Fighter)
	if !ok {
		return rejected(ErrNoCombatRules.Error())
	}

	attacker, ok := tick.Entity(attack.Attacker)
	if !ok {
		// Not a rejection. The command names a body the caller believes in,
		// so a body the tick cannot see is a tick that could not be computed:
		// the caller's move is to load the entity and step again, not to stop
		// attacking.
		tick.MissingEntities(attack.Attacker)

		return rejected(fmt.Sprintf("attacker %d has not been described", attack.Attacker))
	}
	target, ok := tick.Entity(attack.Target)
	if !ok {
		tick.MissingEntities(attack.Target)

		return rejected(fmt.Sprintf("target %d has not been described", attack.Target))
	}

	// Reach is measured from the eye to the nearest point of the target's
	// box, against the survival number: the kernel does not model game modes,
	// and survival is the stricter one.
	eye := eyeOf(attacker, fighter.EyeHeight())
	reach := fighter.Reach().Attack
	if !InReach(eye, target.Box, reach) {
		return rejected(fmt.Sprintf(
			"target %d is out of reach: the attack reach is %.2f", attack.Target, reach,
		))
	}

	charge := chargeOf(tick, attacker, fighter)
	sprinting := false
	if locomotion, ok := tick.Locomotion(attack.Attacker); ok {
		sprinting = locomotion.Sprinting
	}

	strike := Strike{
		Base:      fighter.BareHandDamage(),
		Charge:    charge,
		Sprinting: sprinting,
	}

	// The attacker's swing is spent whether or not the target survives it.
	attacker.Vitals.LastAttack = uint64(tick.Tick())
	attacker.Vitals.Attacked = true
	tick.SetEntity(attack.Attacker, attacker)

	strike.applyTo(tick, attack.Target, target, eyeOf(attacker, 0))

	return sim.CommandOutcome{Accepted: true}
}

// applyTo lands one strike on one body.
func (s Strike) applyTo(tick *sim.TickState, id entity.ID, target entity.State, from geom.Vec3) {
	target.Motion = Knockback(from, centreOf(target.Box), s, target.Motion)
	tick.EmitDomain(sim.DomainEvent{Kind: EventStruck, Entity: id})

	if target.Vitals.Tracked {
		target.Vitals.Health -= float32(Damage(s))
		if target.Vitals.Health <= 0 {
			// Once, and the body goes with it: a death emitted per tick while
			// the entity is still being removed makes every caller that
			// counts deaths wrong.
			tick.EmitDomain(sim.DomainEvent{Kind: EventDied, Entity: id})
			tick.RemoveEntity(id)

			return
		}
	}

	tick.SetEntity(id, target)
}

// chargeOf resolves the attacker's cooldown charge at this tick.
func chargeOf(tick *sim.TickState, attacker entity.State, fighter Fighter) float64 {
	since := neverAttacked
	if attacker.Vitals.Attacked {
		since = int(uint64(tick.Tick()) - attacker.Vitals.LastAttack)
	}

	return fighter.Cooldown().Charge(since, fighter.AttackSpeed())
}

// eyeOf is the point a body measures reach from: the horizontal centre of its
// box, raised off its bottom by the profile's eye height.
func eyeOf(body entity.State, eyeHeight float64) geom.Vec3 {
	return geom.Vec3{
		X: (body.Box.MinX + body.Box.MaxX) / 2,
		Y: body.Box.MinY + eyeHeight,
		Z: (body.Box.MinZ + body.Box.MaxZ) / 2,
	}
}

// centreOf is the centre of a box, which is where a knockback pushes away
// from.
func centreOf(box geom.AABB) geom.Vec3 {
	return geom.Vec3{
		X: (box.MinX + box.MaxX) / 2,
		Y: (box.MinY + box.MaxY) / 2,
		Z: (box.MinZ + box.MaxZ) / 2,
	}
}

// rejected builds a refusal that names its reason.
func rejected(reason string) sim.CommandOutcome {
	return sim.CommandOutcome{Accepted: false, Reason: reason}
}
