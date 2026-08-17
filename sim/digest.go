package sim

import (
	"crypto/sha256"
	"encoding/hex"
)

// Digest is a canonical hash of one tick result.
//
// Two results with the same digest describe the same tick: the same changes in
// the same order, the same events, the same outcomes, the same random state,
// the same data read, and the same completeness, under the same profile. That
// is the property M8.6 gates on across operating systems and architectures, and
// every fixture that compares runs depends on it.
type Digest [32]byte

// String returns the digest in lowercase hexadecimal.
func (d Digest) String() string {
	return hex.EncodeToString(d[:])
}

// IsZero reports whether the digest was never computed.
func (d Digest) IsZero() bool {
	return d == Digest{}
}

// computeDigest hashes the canonical encoding of the result under a profile
// identity.
//
// The identity is included so that a digest from one profile cannot collide
// with a digest from another. The result's own Digest field is excluded, since
// it is zero while this runs and filled from the return value.
func (r TickResult) computeDigest(id ProfileID) Digest {
	var e encoder
	e.tag(tagResult)

	e.tag(tagProfileID)
	e.string(id.Edition)
	e.string(id.GameVersion)
	e.string(id.RulesRevision)

	e.uint64(uint64(r.Revision))
	e.uint64(uint64(r.Tick))

	e.tag(tagChangeSet)
	e.uint64(uint64(r.Changes.BaseRevision))
	e.count(len(r.Changes.Ops))
	for _, op := range r.Changes.Ops {
		e.tag(tagOp)
		e.uint8(uint8(op.Kind))
		e.int32(int32(op.Entity))
		e.tag(tagEntityState)
		e.uint8(uint8(op.State.Family))
		e.box(op.State.Box)
		e.vec(op.State.Motion)
		e.bool(op.State.OnGround)
		e.float64(op.State.StepHeight)
		e.blockPos(op.Block)
		e.uint32(uint32(op.Ref))
	}

	e.count(len(r.Domain))
	for _, event := range r.Domain {
		e.tag(tagDomainEvent)
		e.string(event.Kind)
		e.int32(int32(event.Entity))
		e.blockPos(event.Block)
	}

	e.count(len(r.Presentation))
	for _, event := range r.Presentation {
		e.tag(tagPresentationEvent)
		e.string(event.Kind)
		e.int32(int32(event.Entity))
		e.blockPos(event.Block)
	}

	e.count(len(r.Outcomes))
	for _, outcome := range r.Outcomes {
		e.tag(tagCommandOutcome)
		e.count(outcome.Index)
		e.string(outcome.Kind)
		e.bool(outcome.Accepted)
		e.string(outcome.Reason)
	}

	e.tag(tagRandomState)
	e.count(len(r.Random.Streams))
	for _, stream := range r.Random.Streams {
		e.tag(tagRandomStream)
		e.string(stream.Name)
		e.uint64(stream.State)
	}

	e.count(len(r.Read))
	for _, dependency := range r.Read {
		encodeDependency(&e, dependency)
	}

	e.tag(tagCompleteness)
	e.bool(r.Completeness.Complete)
	e.count(len(r.Completeness.Missing))
	for _, dependency := range r.Completeness.Missing {
		encodeDependency(&e, dependency)
	}

	return sha256.Sum256(e.buf)
}

// encodeDependency writes one dependency. Both the read set and the missing set
// use it, so the two can never drift apart.
func encodeDependency(e *encoder, dependency Dependency) {
	e.tag(tagDependency)
	e.uint8(uint8(dependency.Kind))
	e.blockPos(dependency.Block)
	e.int32(int32(dependency.Entity))
	e.string(dependency.Name)
}
