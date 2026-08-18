package v1_8

import "github.com/go-theft-craft/minecraft-simulation/combat"

// This file is the 1.8.9 half of M9.6: the combat numbers, and the recorded
// absence of the attack cooldown.
//
// The reach constants are transcribed from the deobfuscated 1.8.9 jars
// minecraft-reference pins, on 2026-08-18. They are the client's numbers, not
// the server's: the server tolerates 6 blocks feet-to-feet on a use-entity
// packet (NetHandlerPlayServer.processUseEntity, 36.0 squared), and a client
// that used that slack would be sending attacks a vanilla client cannot —
// which is the first thing an anti-cheat notices. The client's numbers are
// the strict end, and they were only measurable in its jar, because the
// server never states them.

// attackReach is how far the 1.8.9 client lets a survival player hit an
// entity, from EntityRenderer.getMouseOver: the pointed entity is discarded
// beyond 3.0 blocks whenever the block reach exceeds it.
const attackReach = 3.0

// interactReach is the survival block reach, from
// PlayerControllerMP.getBlockReachDistance.
const interactReach = 4.5

// creativeAttackReach is the extended entity reach creative grants, from
// EntityRenderer.getMouseOver via PlayerControllerMP.extendedReach.
const creativeAttackReach = 6.0

// creativeInteractReach is the creative block reach, from
// PlayerControllerMP.getBlockReachDistance.
const creativeInteractReach = 5.0

// bareHandDamage is the player's attack-damage attribute base, set in
// EntityPlayer.applyEntityAttributes. The generated attribute default is 2 —
// that is the generic mob default, and the player overrides it.
const bareHandDamage = 1.0

// playerEyeHeight is a standing player's eye height, from
// EntityPlayer.getEyeHeight.
const playerEyeHeight = 1.62

// noCooldownReason is the recorded absence Task 2 exists for. It is a value
// the conformance report prints, which is what separates "verified absent"
// from "never checked".
const noCooldownReason = "the attack cooldown arrived in 1.9; " +
	"every 1.8.9 swing deals full damage regardless of timing"

// Reach implements combat.Fighter.
func (p *profile) Reach() combat.Reach {
	return combat.Reach{Attack: attackReach, Interact: interactReach}
}

// CreativeReach implements combat.Fighter.
func (p *profile) CreativeReach() combat.Reach {
	return combat.Reach{Attack: creativeAttackReach, Interact: creativeInteractReach}
}

// Cooldown implements combat.Fighter.
func (p *profile) Cooldown() combat.Cooldown {
	return combat.NoCooldown(noCooldownReason)
}

// BareHandDamage implements combat.Fighter.
func (p *profile) BareHandDamage() float64 { return bareHandDamage }

// AttackSpeed implements combat.Fighter.
//
// 1.8.9 has no attack-speed attribute — the mechanic it feeds arrived in 1.9
// — and this profile's cooldown never reads the value.
func (p *profile) AttackSpeed() float64 { return 0 }

// EyeHeight implements combat.Fighter.
func (p *profile) EyeHeight() float64 { return playerEyeHeight }
