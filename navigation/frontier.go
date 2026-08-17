package navigation

import (
	"container/heap"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// node is one search state: where the body is and how it is occupying that
// place. Mutations are not part of it. A version that adds digging records
// them on edges, because keying a node by the set of blocks removed to reach
// it makes the state space explode.
type node struct {
	Pos     geom.BlockPos
	Posture Posture
}

// nodeLess is a total order over nodes.
//
// It exists so that two equal priorities resolve the same way on every run and
// every platform. Without it the frontier's order depends on the heap's
// incidental arrangement, two identical searches return different paths, and
// the digest comparison the replay gate performs fails for a reason nothing in
// the recording explains.
func nodeLess(a, b node) bool {
	if a.Pos.X != b.Pos.X {
		return a.Pos.X < b.Pos.X
	}
	if a.Pos.Y != b.Pos.Y {
		return a.Pos.Y < b.Pos.Y
	}
	if a.Pos.Z != b.Pos.Z {
		return a.Pos.Z < b.Pos.Z
	}

	return a.Posture < b.Posture
}

// entry is one queued node and its priority.
type entry struct {
	node     node
	priority float64
}

// queue is the heap.Interface implementation. It is separate from frontier so
// that the exported-looking heap methods are not part of frontier's surface.
type queue []entry

func (q queue) Len() int { return len(q) }

func (q queue) Less(i, j int) bool {
	if q[i].priority != q[j].priority {
		return q[i].priority < q[j].priority
	}

	return nodeLess(q[i].node, q[j].node)
}

func (q queue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }

func (q *queue) Push(value any) { *q = append(*q, value.(entry)) }

func (q *queue) Pop() any {
	old := *q
	last := old[len(old)-1]
	*q = old[:len(old)-1]

	return last
}

// frontier is the search's open set: lowest priority first, ties broken on the
// node order.
type frontier struct {
	queue queue
}

// push queues a node.
func (f *frontier) push(n node, priority float64) {
	heap.Push(&f.queue, entry{node: n, priority: priority})
}

// pop removes and returns the next node, reporting false when empty.
func (f *frontier) pop() (node, bool) {
	if f.queue.Len() == 0 {
		return node{}, false
	}

	return heap.Pop(&f.queue).(entry).node, true
}
