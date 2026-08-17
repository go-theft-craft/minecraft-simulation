// Package replay records a run as its inputs and the digest of every tick, and
// replays it against a profile.
//
// A recording holds the world it started from, the bodies in it, and per tick
// the commands and the digest the tick produced. It does not hold whole results.
// That is what makes the cross-platform claim sharp: when a platform disagrees,
// it disagrees about a hash, and the hash covers every field of a result. A
// recording of whole results would let a reviewer skim a diff and believe they
// had checked something they had not, and would be too large for anyone to read.
//
// One tick of detail arrives on demand instead. Verify stops at the first
// differing tick — a run that diverges at tick 12 differs at every tick after
// it, and reporting eighty-eight consequences of one cause is noise — and
// renders that tick's expected and actual results for a human, so a failure on a
// runner nobody has access to is still actionable from its log.
//
// The recordings in testdata exist to reach the float32 arithmetic in movement
// and the sine-table indexing, because those are the only places here where a
// compiler, an architecture, or a fused multiply-add could plausibly change a
// result. A matrix over empty ticks would pass on six platforms and prove
// nothing.
package replay
