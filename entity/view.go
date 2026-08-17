package entity

import "slices"

// View is everything the kernel reads about entities in one tick.
//
// An implementation must be deterministic and must stay valid for the whole of
// a tick.
type View interface {
	// Entity returns the body with the given identifier.
	Entity(id ID) (State, bool)
	// IDs returns every identifier the view holds, in ascending order.
	//
	// The order is part of the contract. A tick that walked entities in map
	// order would emit its operations in a different order on every run, and no
	// result digest could ever be stable.
	IDs() []ID
}

// Bodies is an in-memory View.
//
// Bodies is not safe for concurrent modification. Build it, then read it.
type Bodies struct {
	states map[ID]State
}

// NewBodies returns an empty view.
func NewBodies() *Bodies {
	return &Bodies{states: make(map[ID]State)}
}

// Set records a body.
func (b *Bodies) Set(id ID, state State) {
	b.states[id] = state
}

// Remove drops a body.
func (b *Bodies) Remove(id ID) {
	delete(b.states, id)
}

// Entity implements View.
func (b *Bodies) Entity(id ID) (State, bool) {
	state, ok := b.states[id]

	return state, ok
}

// IDs implements View, in ascending order.
func (b *Bodies) IDs() []ID {
	ids := make([]ID, 0, len(b.states))
	for id := range b.states {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	return ids
}

// Clone returns a view that does not alias this one, which is what lets a store
// hand out a snapshot a later tick cannot change underneath a reader.
func (b *Bodies) Clone() *Bodies {
	clone := &Bodies{states: make(map[ID]State, len(b.states))}
	for id, state := range b.states {
		clone.states[id] = state
	}

	return clone
}
