package combat_test

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/combat"
	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/runtime"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// attacker and target are the two bodies every phase test stages.
const (
	attacker entity.ID = 1
	target   entity.ID = 2
)

// combatOnly narrows a profile to the attack phase, so a movement rule cannot
// move a body a combat test never asked to move. The Fighter methods are
// forwarded because an embedded interface does not carry the optional ones the
// concrete profile satisfies, and a phase that cannot see them would reject
// every attack.
type combatOnly struct{ sim.Profile }

func (combatOnly) Phases() []sim.Phase { return []sim.Phase{combat.Phase()} }

func (c combatOnly) Reach() combat.Reach         { return c.Profile.(combat.Fighter).Reach() }
func (c combatOnly) CreativeReach() combat.Reach { return c.Profile.(combat.Fighter).CreativeReach() }
func (c combatOnly) Cooldown() combat.Cooldown   { return c.Profile.(combat.Fighter).Cooldown() }
func (c combatOnly) BareHandDamage() float64     { return c.Profile.(combat.Fighter).BareHandDamage() }
func (c combatOnly) AttackSpeed() float64        { return c.Profile.(combat.Fighter).AttackSpeed() }
func (c combatOnly) EyeHeight() float64          { return c.Profile.(combat.Fighter).EyeHeight() }

// harness is a kernel, its store, and the tick counter the cooldown needs.
type harness struct {
	kernel  sim.Kernel
	store   *runtime.Memory
	profile sim.Profile
	tick    sim.Tick
}

// playerBoxAt returns a player-sized box standing at (x, y, z).
func playerBoxAt(x, y, z float64) geom.AABB {
	return geom.AABB{
		MinX: x - 0.3, MinY: y, MinZ: z - 0.3,
		MaxX: x + 0.3, MaxY: y + 1.8, MaxZ: z + 0.3,
	}
}

// stage builds a kernel with the attacker at the origin and, when health is
// positive, a target with that much health at (x, y, z).
func stage(t *testing.T, version string, health float32, x, y, z float64) *harness {
	t.Helper()

	profile := combatOnly{Profile: simProfileFor(t, version)}

	kernel, err := sim.NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}

	store := runtime.NewMemory(profile)
	store.SetEntity(attacker, entity.State{
		Family:   entity.FamilyPlayer,
		Box:      playerBoxAt(0, 0, 0),
		OnGround: true,
		Vitals:   entity.Vitals{Health: 20, Tracked: true},
	})
	if health > 0 {
		store.SetEntity(target, entity.State{
			Family:   entity.FamilyPlayer,
			Box:      playerBoxAt(x, y, z),
			OnGround: true,
			Vitals:   entity.Vitals{Health: health, Tracked: true},
		})
	}

	return &harness{kernel: kernel, store: store, profile: profile}
}

// step runs one tick with the given commands and applies a complete result.
// The tick number advances per call, which is what the cooldown charges over.
func step(t *testing.T, h *harness, commands ...sim.Command) sim.TickResult {
	t.Helper()

	h.tick++
	result, err := h.kernel.Step(t.Context(), sim.TickInput{
		Profile:  h.profile,
		Revision: h.store.Revision(),
		Tick:     h.tick,
		Blocks:   h.store.Blocks(),
		Entities: h.store.Entities(),
		Scope:    sim.Scope{Entities: []entity.ID{attacker, target}},
		Commands: commands,
	})
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if result.Completeness.Complete {
		if err := h.store.Apply(result.Changes); err != nil {
			t.Fatalf("Apply: %v", err)
		}
	}

	return result
}

// onlyOutcome returns the result's single command outcome.
func onlyOutcome(t *testing.T, result sim.TickResult) sim.CommandOutcome {
	t.Helper()

	if len(result.Outcomes) != 1 {
		t.Fatalf("the tick recorded %d outcomes, want 1", len(result.Outcomes))
	}

	return result.Outcomes[0]
}

// stateFrom returns the last body state a result wrote for one entity.
func stateFrom(t *testing.T, result sim.TickResult, id entity.ID) (entity.State, bool) {
	t.Helper()

	var state entity.State
	var found bool
	for _, op := range result.Changes.Ops {
		if op.Kind == sim.OpSetEntity && op.Entity == id {
			state, found = op.State, true
		}
	}

	return state, found
}

// countDeaths counts the death events a result carries for one entity.
func countDeaths(result sim.TickResult, id entity.ID) int {
	var deaths int
	for _, event := range result.Domain {
		if event.Kind == combat.EventDied && event.Entity == id {
			deaths++
		}
	}

	return deaths
}

// damageDealt reads how much health one tick took from the target: the store's
// number before the result was applied, less the number the result wrote.
func damageDealt(t *testing.T, before entity.State, result sim.TickResult) float64 {
	t.Helper()

	after, ok := stateFrom(t, result, target)
	if !ok {
		t.Fatal("the tick wrote no state for the target")
	}

	return float64(before.Vitals.Health - after.Vitals.Health)
}

func TestAnOutOfReachAttackIsRefusedWithAReason(t *testing.T) {
	t.Parallel()

	h := stage(t, "1_8_9", 20, 20, 0, 0)
	result := step(t, h, combat.Attack{Attacker: attacker, Target: target})

	outcome := onlyOutcome(t, result)
	if outcome.Accepted {
		t.Fatal("an attack twenty blocks away was accepted")
	}
	if outcome.Reason == "" {
		t.Fatal("a refused attack gave no reason")
	}
	if !result.Changes.IsEmpty() {
		t.Fatal("a refused attack emitted a change set")
	}
}

func TestAnAttackOnAnUnknownEntityIsIncomplete(t *testing.T) {
	t.Parallel()

	h := stage(t, "1_8_9", 0, 0, 0, 0)
	result := step(t, h, combat.Attack{Attacker: attacker, Target: 99})

	if result.Completeness.Complete {
		t.Fatal("an attack on an entity the kernel has never seen reported a " +
			"complete tick")
	}
	missing := result.Completeness.Missing
	if len(missing) == 0 || missing[0].Kind != sim.DependencyEntity {
		t.Fatalf("Missing = %+v, want an entity dependency", missing)
	}
}

func TestKnockbackAppearsAsMotionNotPosition(t *testing.T) {
	t.Parallel()

	h := stage(t, "1_8_9", 20, 1, 0, 0)
	before, _ := h.store.Entities().Entity(target)
	result := step(t, h, combat.Attack{Attacker: attacker, Target: target})

	after, ok := stateFrom(t, result, target)
	if !ok {
		t.Fatal("the attack wrote no state for the target")
	}
	if after.Box != before.Box {
		t.Fatal("the attack moved the target's box directly; knockback must be " +
			"motion that the next movement tick resolves through collision")
	}
	if after.Motion == before.Motion {
		t.Fatal("the attack changed no motion")
	}
}

func TestDeathIsEmittedOnceWhenHealthReachesZero(t *testing.T) {
	t.Parallel()

	// Once. A death emitted per tick while the entity is still being removed
	// makes every caller that counts deaths wrong.
	h := stage(t, "1_8_9", 0.5, 1, 0, 0)
	var deaths int
	for range 5 {
		result := step(t, h, combat.Attack{Attacker: attacker, Target: target})
		deaths += countDeaths(result, target)
	}
	if deaths != 1 {
		t.Fatalf("emitted %d death events for one death", deaths)
	}
}

func TestADeadTargetIsRemovedFromTheWorld(t *testing.T) {
	t.Parallel()

	h := stage(t, "1_8_9", 0.5, 1, 0, 0)
	step(t, h, combat.Attack{Attacker: attacker, Target: target})

	if _, alive := h.store.Entities().Entity(target); alive {
		t.Fatal("a target whose health reached zero is still in the store")
	}
}

func TestTheAttackerSpendsItsSwingWhetherOrNotTheTargetDies(t *testing.T) {
	t.Parallel()

	h := stage(t, "1_8_9", 20, 1, 0, 0)
	result := step(t, h, combat.Attack{Attacker: attacker, Target: target})

	state, ok := stateFrom(t, result, attacker)
	if !ok {
		t.Fatal("the attack wrote no state for the attacker")
	}
	if !state.Vitals.Attacked || state.Vitals.LastAttack != uint64(result.Tick) {
		t.Fatalf("the attacker's swing was not recorded: %+v", state.Vitals)
	}
}

func TestTheCooldownGatesRepeatedAttacksOn26_1_2Only(t *testing.T) {
	t.Parallel()

	// Two swings in consecutive ticks: full damage twice on 1.8.9, reduced on
	// the second on 26.1.2. This single test states the divergence rather
	// than two tests that could drift apart.
	for _, version := range versions {
		t.Run(version, func(t *testing.T) {
			t.Parallel()

			h := stage(t, version, 20, 1, 0, 0)

			before, _ := h.store.Entities().Entity(target)
			first := damageDealt(t, before,
				step(t, h, combat.Attack{Attacker: attacker, Target: target}))
			before, _ = h.store.Entities().Entity(target)
			second := damageDealt(t, before,
				step(t, h, combat.Attack{Attacker: attacker, Target: target}))

			if version == "1_8_9" && second != first {
				t.Fatalf("1.8.9 dealt %v then %v; it has no attack cooldown",
					first, second)
			}
			if version == "26_1_2" && second >= first {
				t.Fatalf("26.1.2 dealt %v then %v; the second swing was not "+
					"reduced by the cooldown", first, second)
			}
		})
	}
}
