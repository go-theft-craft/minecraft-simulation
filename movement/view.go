package movement

import "github.com/go-theft-craft/minecraft-simulation/entity"

// Bodies is an in-memory LocomotionView.
//
// Bodies is not safe for concurrent modification. Build it, then read it.
type Bodies struct {
	states map[entity.ID]Locomotion
}

// NewBodies returns an empty view.
func NewBodies() *Bodies {
	return &Bodies{states: make(map[entity.ID]Locomotion)}
}

// Set records locomotion state.
func (b *Bodies) Set(id entity.ID, state Locomotion) {
	b.states[id] = state
}

// Remove drops a body's locomotion state.
func (b *Bodies) Remove(id entity.ID) {
	delete(b.states, id)
}

// Locomotion implements LocomotionView.
func (b *Bodies) Locomotion(id entity.ID) (Locomotion, bool) {
	state, ok := b.states[id]

	return state, ok
}

// Clone returns a view that does not alias this one, which is what lets a store
// fork a snapshot a later tick cannot change underneath a reader.
func (b *Bodies) Clone() *Bodies {
	clone := &Bodies{states: make(map[entity.ID]Locomotion, len(b.states))}
	for id, state := range b.states {
		clone.states[id] = state
	}

	return clone
}
