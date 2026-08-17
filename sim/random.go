package sim

import (
	"cmp"
	"slices"
)

// RandomStream is one named generator's serialized state.
//
// A single uint64 holds Java's 48-bit generator seed with room to spare, and
// keeping the state a plain integer is what lets a result be encoded, hashed,
// stored, and replayed without a custom serializer.
type RandomStream struct {
	Name  string
	State uint64
}

// RandomState is every random stream a tick may draw from.
//
// Streams are named because the parent design requires separate sources to stay
// separate when the game version uses them; folding them into one generator
// would change which numbers a rule sees.
//
// Streams are kept sorted by name so that two states holding the same streams
// encode identically however they were assembled.
type RandomState struct {
	Streams []RandomStream
}

// Clone returns a state that does not alias this one.
func (r RandomState) Clone() RandomState {
	return RandomState{Streams: slices.Clone(r.Streams)}
}

// Stream returns a named stream's state.
func (r RandomState) Stream(name string) (uint64, bool) {
	for _, stream := range r.Streams {
		if stream.Name == name {
			return stream.State, true
		}
	}

	return 0, false
}

// WithStream returns a state with the named stream set, replacing any stream
// that already has the name.
func (r RandomState) WithStream(name string, state uint64) RandomState {
	streams := slices.Clone(r.Streams)
	for index := range streams {
		if streams[index].Name == name {
			streams[index].State = state

			return RandomState{Streams: streams}
		}
	}

	streams = append(streams, RandomStream{Name: name, State: state})
	slices.SortFunc(streams, func(a, b RandomStream) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return RandomState{Streams: streams}
}
