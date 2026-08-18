package v26_1

import (
	"github.com/go-theft-craft/minecraft-protocol/data"

	"github.com/go-theft-craft/minecraft-simulation/combat"
)

// This file is the 26.1.2 half of M9.6: the combat numbers, and the attack
// cooldown 1.8.9 lacks.
//
// Unlike 1.8.9's, most of these numbers are attributes and therefore dumpable:
// the reach distances and the attack speed come from the generated attribute
// registry rather than from a literal. What stays transcribed is what the
// attribute system does not carry — the creative-mode modifiers and the
// player's own attack-damage base — from the deobfuscated 26.1.2 server jar
// minecraft-reference pins, on 2026-08-18.

// creativeAttackBonus is the creative entity-interaction modifier, from
// ServerPlayer's creative_mode_entity_range attribute modifier (+2.0).
const creativeAttackBonus = 2.0

// creativeInteractBonus is the creative block-interaction modifier, from
// ServerPlayer's creative_mode_block_range attribute modifier (+0.5).
const creativeInteractBonus = 0.5

// bareHandDamage is the player's attack-damage attribute base, set in
// Player.createAttributes. The generated attribute default is 2 — that is the
// generic default, and the player overrides it.
const bareHandDamage = 1.0

// playerEyeHeight is a standing player's eye height, from Player's entity
// dimensions (DEFAULT_EYE_HEIGHT, 1.62F).
const playerEyeHeight = 1.62

// The class constants Player.java carries beside the attributes, used when a
// hand-built set has no attribute registry: DEFAULT_ENTITY_INTERACTION_RANGE,
// DEFAULT_BLOCK_INTERACTION_RANGE, and Attributes.ATTACK_SPEED's default.
const (
	defaultAttackReach   = 3.0
	defaultInteractReach = 4.5
	defaultAttackSpeed   = 4.0
)

// fighting is the combat table New builds from the attribute registry.
type fighting struct {
	reach       combat.Reach
	attackSpeed float64
}

// newFighting reads the combat attributes out of the set.
//
// The registry is authoritative when the set carries one, so a corrected
// attribute dump moves these numbers without an edit here. A hand-built set
// with no registry — the profile tests construct several — falls back to the
// class constants, which are the same numbers stated a second time in
// Player.java. The same shape newMiningTable uses for a set without
// materials.
func newFighting(set *data.Set) fighting {
	built := fighting{
		reach:       combat.Reach{Attack: defaultAttackReach, Interact: defaultInteractReach},
		attackSpeed: defaultAttackSpeed,
	}

	registry := set.Attributes()
	if registry == nil {
		return built
	}
	for _, want := range []struct {
		name string
		into *float64
	}{
		{"entityInteractionRange", &built.reach.Attack},
		{"blockInteractionRange", &built.reach.Interact},
		{"attackSpeed", &built.attackSpeed},
	} {
		if attribute, ok := registry.ByName(want.name); ok {
			*want.into = attribute.Default
		}
	}

	return built
}

// Reach implements combat.Fighter, from the entity- and block-interaction
// range attributes.
func (p *profile) Reach() combat.Reach { return p.fighting.reach }

// CreativeReach implements combat.Fighter: the survival attributes plus the
// two creative modifiers.
func (p *profile) CreativeReach() combat.Reach {
	return combat.Reach{
		Attack:   p.fighting.reach.Attack + creativeAttackBonus,
		Interact: p.fighting.reach.Interact + creativeInteractBonus,
	}
}

// Cooldown implements combat.Fighter. This is the version the mechanic exists
// on.
func (p *profile) Cooldown() combat.Cooldown { return combat.TickedCooldown() }

// BareHandDamage implements combat.Fighter.
func (p *profile) BareHandDamage() float64 { return bareHandDamage }

// AttackSpeed implements combat.Fighter, from the attack-speed attribute.
func (p *profile) AttackSpeed() float64 { return p.fighting.attackSpeed }

// EyeHeight implements combat.Fighter.
func (p *profile) EyeHeight() float64 { return playerEyeHeight }
