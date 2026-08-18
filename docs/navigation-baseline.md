# Navigation baseline

The numbers the `Planner` memo is measured against, and the profile that
decided whether building it was worth doing at all.

## What was measured, and where

| | |
| --- | --- |
| Commit | `32d753c`, the admissible heuristic, one commit before the memo work |
| Go | `go1.26.6 linux/amd64`, from `openserbia/go-flake` via Devbox |
| Machine | AMD Ryzen 9 9950X, 16 cores / 32 threads, linux/amd64 |
| Package | `navigation` |

The heuristic fix landed first on purpose. A lower per-block floor makes the
search expand more nodes, so measuring before it would have recorded a baseline
no later commit could be compared against.

## Commands

```bash
devbox run -- go test ./navigation/ -bench . -benchmem -run '^$' -count 5
devbox run -- go test ./navigation/ -bench BenchmarkFindLong -run '^$' -cpuprofile /tmp/nav-cpu.out
devbox run -- go tool pprof -top -nodecount=25 /tmp/nav-cpu.out
devbox run -- go tool pprof -top -cum -nodecount=25 /tmp/nav-cpu.out
```

## Benchmarks, five runs

```
goos: linux
goarch: amd64
pkg: github.com/go-theft-craft/minecraft-simulation/navigation
cpu: AMD Ryzen 9 9950X 16-Core Processor            
BenchmarkFindShort-32    	   54466	     21925 ns/op	   20120 B/op	     151 allocs/op
BenchmarkFindShort-32    	   52032	     21978 ns/op	   20120 B/op	     151 allocs/op
BenchmarkFindShort-32    	   54505	     21517 ns/op	   20120 B/op	     151 allocs/op
BenchmarkFindShort-32    	   55274	     21023 ns/op	   20120 B/op	     151 allocs/op
BenchmarkFindShort-32    	   58569	     20935 ns/op	   20120 B/op	     151 allocs/op
BenchmarkFindLong-32     	     302	   3985687 ns/op	 2094666 B/op	   21263 allocs/op
BenchmarkFindLong-32     	     302	   3904708 ns/op	 2094648 B/op	   21263 allocs/op
BenchmarkFindLong-32     	     307	   3857592 ns/op	 2094648 B/op	   21263 allocs/op
BenchmarkFindLong-32     	     309	   3842682 ns/op	 2094648 B/op	   21263 allocs/op
BenchmarkFindLong-32     	     303	   3902953 ns/op	 2094648 B/op	   21263 allocs/op
BenchmarkFindMaze-32     	    6012	    191865 ns/op	  167304 B/op	    1970 allocs/op
BenchmarkFindMaze-32     	    6190	    190943 ns/op	  167304 B/op	    1970 allocs/op
BenchmarkFindMaze-32     	    6549	    185300 ns/op	  167304 B/op	    1970 allocs/op
BenchmarkFindMaze-32     	    6386	    189100 ns/op	  167304 B/op	    1970 allocs/op
BenchmarkFindMaze-32     	    6085	    184535 ns/op	  167304 B/op	    1970 allocs/op
```

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| `BenchmarkFindShort` | ~21,500 | 20,120 | 151 |
| `BenchmarkFindLong` | ~3,900,000 | 2,094,650 | 21,263 |
| `BenchmarkFindMaze` | ~188,000 | 167,304 | 1,970 |

`FindShort` crosses four blocks of open floor. `FindLong` crosses sixty, which
is where the search does enough work for a profile to say anything. `FindMaze`
is the property suite's pillar fixture, so it measures a search that branches
rather than one walking a straight line.

## Profile: `BenchmarkFindLong`

Flat:

```
      flat  flat%   sum%        cum   cum%
     0.14s 11.02% 11.02%      0.37s 29.13%  github.com/go-theft-craft/minecraft-simulation/terrain.Query.Ground
     0.14s 11.02% 22.05%      0.48s 37.80%  runtime.mapaccess2
     0.12s  9.45% 31.50%      0.12s  9.45%  aeshashbody
     0.10s  7.87% 39.37%      0.47s 37.01%  github.com/go-theft-craft/minecraft-simulation/collision.gather
     0.10s  7.87% 47.24%      0.10s  7.87%  internal/runtime/maps.ctrlGroup.matchH2 (inline)
     0.08s  6.30% 53.54%      0.08s  6.30%  memeqbody
     0.07s  5.51% 59.06%      0.34s 26.77%  github.com/go-theft-craft/minecraft-simulation/terrain.Query.Fits
     0.04s  3.15% 62.20%      0.04s  3.15%  github.com/go-theft-craft/minecraft-simulation/navigation.(*queue).Push
     0.03s  2.36% 64.57%      0.89s 70.08%  github.com/go-theft-craft/minecraft-simulation/navigation.Capability.expand
     0.03s  2.36% 66.93%      0.03s  2.36%  internal/runtime/maps.(*groupReference).key (inline)
     0.02s  1.57% 68.50%      0.04s  3.15%  container/heap.down
     0.02s  1.57% 70.08%      0.49s 38.58%  github.com/go-theft-craft/minecraft-simulation/collision.Gather (inline)
     0.02s  1.57% 71.65%      0.02s  1.57%  github.com/go-theft-craft/minecraft-simulation/navigation.queue.Less
     0.02s  1.57% 73.23%      0.12s  9.45%  github.com/go-theft-craft/minecraft-simulation/world.(*Blocks).BlockState
     0.02s  1.57% 74.80%      0.02s  1.57%  internal/runtime/maps.(*Map).directoryAt (inline)
     0.02s  1.57% 76.38%      0.02s  1.57%  internal/runtime/maps.(*groupsReference).group (inline)
     0.02s  1.57% 77.95%      0.06s  4.72%  runtime.mapassign
     0.02s  1.57% 79.53%      0.02s  1.57%  runtime.memhash
     0.02s  1.57% 81.10%      0.03s  2.36%  runtime.scanObject
     0.01s  0.79% 81.89%      0.02s  1.57%  github.com/go-theft-craft/minecraft-simulation/collision.touches (inline)
     0.01s  0.79% 82.68%      0.01s  0.79%  github.com/go-theft-craft/minecraft-simulation/geom.AABB.Intersects (inline)
     0.01s  0.79% 83.46%      0.01s  0.79%  github.com/go-theft-craft/minecraft-simulation/geom.Floor (inline)
     0.01s  0.79% 84.25%      0.01s  0.79%  github.com/go-theft-craft/minecraft-simulation/geom.Shape.BoxesAt (inline)
     0.01s  0.79% 85.04%      0.13s 10.24%  github.com/go-theft-craft/minecraft-simulation/navigation.Capability.arriveAt
     0.01s  0.79% 85.83%      0.03s  2.36%  github.com/go-theft-craft/minecraft-simulation/navigation.Capability.heuristic
```

Cumulative:

```
      flat  flat%   sum%        cum   cum%
         0     0%     0%      1.17s 92.13%  github.com/go-theft-craft/minecraft-simulation/navigation.BenchmarkFindLong
         0     0%     0%      1.17s 92.13%  github.com/go-theft-craft/minecraft-simulation/navigation.Find
         0     0%     0%      1.17s 92.13%  testing.(*B).run1.func1
         0     0%     0%      1.17s 92.13%  testing.(*B).runN
     0.03s  2.36%  2.36%      0.89s 70.08%  github.com/go-theft-craft/minecraft-simulation/navigation.Capability.expand
     0.01s  0.79%  3.15%      0.73s 57.48%  github.com/go-theft-craft/minecraft-simulation/terrain.Query.Passable
     0.02s  1.57%  4.72%      0.49s 38.58%  github.com/go-theft-craft/minecraft-simulation/collision.Gather (inline)
     0.14s 11.02% 15.75%      0.48s 37.80%  runtime.mapaccess2
     0.10s  7.87% 23.62%      0.47s 37.01%  github.com/go-theft-craft/minecraft-simulation/collision.gather
     0.14s 11.02% 34.65%      0.37s 29.13%  github.com/go-theft-craft/minecraft-simulation/terrain.Query.Ground
     0.07s  5.51% 40.16%      0.34s 26.77%  github.com/go-theft-craft/minecraft-simulation/terrain.Query.Fits
         0     0% 40.16%      0.33s 25.98%  github.com/go-theft-craft/minecraft-simulation/world.(*Blocks).CollisionShape
     0.01s  0.79% 40.94%      0.13s 10.24%  github.com/go-theft-craft/minecraft-simulation/navigation.Capability.arriveAt
         0     0% 40.94%      0.13s 10.24%  github.com/go-theft-craft/minecraft-simulation/navigation.Capability.enter
     0.12s  9.45% 50.39%      0.12s  9.45%  aeshashbody
     0.02s  1.57% 51.97%      0.12s  9.45%  github.com/go-theft-craft/minecraft-simulation/world.(*Blocks).BlockState
     0.01s  0.79% 52.76%      0.12s  9.45%  runtime.memhash_varlen
     0.10s  7.87% 60.63%      0.10s  7.87%  internal/runtime/maps.ctrlGroup.matchH2 (inline)
     0.08s  6.30% 66.93%      0.08s  6.30%  memeqbody
         0     0% 66.93%      0.08s  6.30%  runtime.systemstack
         0     0% 66.93%      0.07s  5.51%  github.com/go-theft-craft/minecraft-simulation/terrain.Query.HazardAt
         0     0% 66.93%      0.06s  4.72%  runtime.gcBgMarkWorker
         0     0% 66.93%      0.06s  4.72%  runtime.gcBgMarkWorker.func2
     0.01s  0.79% 67.72%      0.06s  4.72%  runtime.gcDrain
     0.02s  1.57% 69.29%      0.06s  4.72%  runtime.mapassign
```

## The verdict: terrain reading dominates

Yes, and by a wide margin.

`Capability.expand` accounts for 70.08% of the search. Nearly all of that is
terrain: `terrain.Query.Passable` alone is **57.48% cumulative**, and
`Capability.arriveAt` adds another **10.24%** through `HazardAt` (5.51%) and
`FluidAt` (3.94%). Together the two questions the search asks about a cell are
about **68% of its running time**.

Underneath `Passable`, the cost splits between `terrain.Query.Ground` (29.13%)
and `terrain.Query.Fits` (26.77%), which between them run `collision.Gather`
(38.58%). The `runtime.mapaccess2` entry at 37.80% flat-heavy looks like a map
problem and is not one: it is almost entirely the fake world's own storage,
reached through `world.(*Blocks).CollisionShape` (25.98%) and `BlockState`
(9.45%). It is a cost of *reading terrain*, not of the search's bookkeeping.

Against that, the two things a memo would not help:

- **The frontier** — `frontier.push` and `container/heap.Push` at 3.94%
  cumulative, `heap.down` at 3.15%, `queue.Less` at 1.57%. Call it 6–8% in
  total. The heap does `log n` work on a slice and it shows.
- **The search's own maps** — `cameFrom` and `cost`, visible as
  `runtime.mapassign` (4.72%) and `runtime.mapaccess1` (3.94%), and even those
  figures include calls made from inside `collision` and `world`. Under 9%.

So the design's premise holds: roughly ten times more time goes into asking
terrain about a cell than into the machinery that decides which cell to ask
about next. Caching the answers is the right optimization, and the plan
proceeds to Task 3.

One number to carry forward as the honest ceiling: even a memo with a perfect
hit rate cannot remove more than the ~68% that terrain reading costs, and it
adds a map lookup and a dependency list of its own on every answer. The claim to
test in Task 8 is a large constant-factor win on a warm cache, not an order of
magnitude.

---

# After: the Planner

Same machine, same Go, same commands, measured on the commit that added the
planner benchmarks.

```
BenchmarkFindShort-32          	   54958	     22668 ns/op	   20264 B/op	     152 allocs/op
BenchmarkFindShort-32          	   58054	     20176 ns/op	   20264 B/op	     152 allocs/op
BenchmarkFindShort-32          	   61866	     19570 ns/op	   20264 B/op	     152 allocs/op
BenchmarkFindShort-32          	   62317	     19426 ns/op	   20264 B/op	     152 allocs/op
BenchmarkFindShort-32          	   62042	     19443 ns/op	   20264 B/op	     152 allocs/op
BenchmarkFindLong-32           	     312	   3772904 ns/op	 2094809 B/op	   21264 allocs/op
BenchmarkFindLong-32           	     320	   3753762 ns/op	 2094792 B/op	   21264 allocs/op
BenchmarkFindLong-32           	     316	   3771895 ns/op	 2094792 B/op	   21264 allocs/op
BenchmarkFindLong-32           	     318	   3769645 ns/op	 2094792 B/op	   21264 allocs/op
BenchmarkFindLong-32           	     316	   3774741 ns/op	 2094792 B/op	   21264 allocs/op
BenchmarkFindMaze-32           	    6560	    178084 ns/op	  167448 B/op	    1971 allocs/op
BenchmarkFindMaze-32           	    6889	    180835 ns/op	  167448 B/op	    1971 allocs/op
BenchmarkFindMaze-32           	    6570	    178393 ns/op	  167448 B/op	    1971 allocs/op
BenchmarkFindMaze-32           	    6374	    179559 ns/op	  167448 B/op	    1971 allocs/op
BenchmarkFindMaze-32           	    6656	    179455 ns/op	  167448 B/op	    1971 allocs/op
BenchmarkPlanCold-32           	     247	   4916001 ns/op	 6775296 B/op	   60417 allocs/op
BenchmarkPlanCold-32           	     231	   5080928 ns/op	 6775272 B/op	   60417 allocs/op
BenchmarkPlanCold-32           	     240	   4917664 ns/op	 6775319 B/op	   60417 allocs/op
BenchmarkPlanCold-32           	     234	   5011198 ns/op	 6775764 B/op	   60417 allocs/op
BenchmarkPlanCold-32           	     248	   4890483 ns/op	 6775272 B/op	   60417 allocs/op
BenchmarkPlanWarm-32           	     658	   1787718 ns/op	 1518648 B/op	    9263 allocs/op
BenchmarkPlanWarm-32           	     667	   1794637 ns/op	 1518648 B/op	    9263 allocs/op
BenchmarkPlanWarm-32           	     668	   1785400 ns/op	 1518648 B/op	    9263 allocs/op
BenchmarkPlanWarm-32           	     669	   1777192 ns/op	 1518648 B/op	    9263 allocs/op
BenchmarkPlanWarm-32           	     657	   1781360 ns/op	 1518648 B/op	    9263 allocs/op
BenchmarkPlanAfterChange-32    	     664	   1763192 ns/op	 1519332 B/op	    9271 allocs/op
BenchmarkPlanAfterChange-32    	     679	   1763106 ns/op	 1519328 B/op	    9271 allocs/op
BenchmarkPlanAfterChange-32    	     676	   1755719 ns/op	 1519572 B/op	    9271 allocs/op
BenchmarkPlanAfterChange-32    	     674	   1775525 ns/op	 1519330 B/op	    9271 allocs/op
BenchmarkPlanAfterChange-32    	     637	   1803153 ns/op	 1519340 B/op	    9271 allocs/op
```

| Benchmark | ns/op | B/op | allocs/op | vs `FindLong` |
| --- | --- | --- | --- | --- |
| `BenchmarkFindLong` | ~3,770,000 | 2,094,792 | 21,264 | — |
| `BenchmarkPlanCold` | ~4,930,000 | 6,775,300 | 60,417 | **1.31× slower** |
| `BenchmarkPlanWarm` | ~1,785,000 | 1,518,648 | 9,263 | **2.11× faster** |
| `BenchmarkPlanAfterChange` | ~1,772,000 | 1,519,340 | 9,271 | **2.13× faster** |

`FindShort` and `FindMaze` are unchanged from the baseline within noise, which
is the point: nothing about `Find` moved.

## The assessment

**A warm plan is 2.1× faster than a fresh `Find`**, on half the allocations and
27% less memory. That is a real win and it is the case a routing body is
actually in: it plans, walks a few blocks, and plans again through terrain it
has already asked about.

**A cold plan is 1.3× *slower*.** Filling the memo costs 39,000 allocations that
`Find` never makes — one dependency slice per answer, plus the reverse index
entries. A caller that plans once and throws the planner away should call `Find`
and skip all of it. This is worth stating plainly because the API makes the
wrong choice easy: `NewPlanner` followed by one `Plan` is strictly worse than
`Find`, and only the second search onward pays the first one back.

**Invalidation is not the expensive part.** `BenchmarkPlanAfterChange` reports
one changed cell to `Observe` on every iteration and comes out level with the
warm case — 1.77 ms against 1.79 ms, inside the noise. Walking the reverse index
from one cell to the handful of answers that read it, and recomputing exactly
those, costs nothing measurable against the search around it. The design's claim
that dropping precisely what changed beats dropping everything holds: a `Reset`
in that benchmark would have cost the full cold 4.9 ms.

## Against the ceiling the baseline predicted

The baseline said terrain reading was about 68% of the search and that no memo
could remove more than that. A perfect cache would therefore reach about
1/(1−0.68) ≈ **3.1×**, and the measured warm figure is **2.1×** — roughly 70% of
the theoretical maximum.

The gap is where it should be. The memo replaces a `collision.Gather` sweep with
a map lookup, but every hit still pays that lookup, and the search's own
machinery is untouched: `cameFrom`, `cost`, the frontier heap, and the edge
slices are still built from scratch on every `Plan`. Warm planning still
allocates 1.5 MB and 9,263 times per search, and none of that is terrain. That
residue is what the cluster graph is for — it attacks the number of cells
examined, which is the term this memo deliberately does not touch.
