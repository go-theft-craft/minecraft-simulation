package sim

import (
	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// DomainEvent reports a simulation fact: a collision, a landing, a spawn, a
// removal. A server can turn these into packets and a client into observations.
//
// Kind is a namespaced string, such as "movement.collided". A numeric enum
// invented before any rule emits an event would be renamed by the first
// milestone that does; a string costs nothing at this scale and survives being
// added to.
type DomainEvent struct {
	Kind   string
	Entity entity.ID
	Block  geom.BlockPos
}

// PresentationEvent requests a particle, a sound, or an animation. It carries
// no simulation meaning: a consumer may ignore every one of them and still hold
// correct state.
type PresentationEvent struct {
	Kind   string
	Entity entity.ID
	Block  geom.BlockPos
}
