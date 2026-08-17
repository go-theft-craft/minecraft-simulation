# Can M8.7 gate on the game? Four questions, answered

Written 2026-08-17, as M8.7 Task 1. The plan branches on this note: if a
jar-backed oracle is possible, the milestone follows M8.4's shape and the gate is
the game; if it is not, the gate degrades to hand-derived fixtures and M8.8's
live server, which is the weaker gate M8.4 deliberately avoided.

**The answer is that a jar-backed oracle is possible, and cheaper than 1.8.9's.**
What is expensive here is something else, and the second half of this note is
about that.

Every claim below was produced by running something, and the command is given.

## 1. Can a named 26.1.2 server jar be produced?

**The question is obsolete: no deobfuscation is needed.** Mojang ships this
version unobfuscated.

The M8.7 plan says "there is no deobfuscated 26.1.2 jar in the workspace ...
whether a named jar can be produced, and by which path, is the first thing to
find out". The workspace has no `named.jar` for this version because none is
required, and `minecraft-reference` already knows this: its version catalogue
records 26.1.2 and 26.2 with a naming strategy of `identity`, against `mcp` for
1.8.9 and `mojang` for the versions between. An `identity` version returns its
analysis jar unremapped.

The evidence is in the jar itself:

```bash
unzip -l reference/work/versions/26.1.2/server/executable.jar \
  | grep -E 'LivingEntity|Bootstrap'
```

which lists `net/minecraft/world/entity/LivingEntity.class` and
`net/minecraft/server/Bootstrap.class` under their real names, among 7,351
classes. The analysis jar for this version is `executable.jar` — the bundled
server, not the `original.jar` bundler wrapper, which holds only 170 entries and
Mojang's loader.

Two consequences worth carrying:

- The 1.8.9 oracle's compile step points at `named.jar`. A 26.1.2 harness points
  at `executable.jar` plus the version's `libraries` tree, which the workspace
  already holds. `minecraft-reference`'s block dumper resolves this per version
  by reading the workspace's own compatibility report rather than guessing, and
  the movement oracle should do the same rather than hard-coding either name.
- Compilation is *typed* rather than reflective, which is strictly better. The
  1.8.9 physics dumper reaches values reflectively because that version keeps
  them private; here the compiler checks the harness against the jar it will run
  on, so a renamed method fails at `javac` instead of throwing halfway through a
  run.

## 2. Does the movement code initialize in a headless process?

**Yes, in about a second.** The sequence is
`SharedConstants.tryDetectVersion()`, then `Bootstrap.bootStrap()`, then a
class load of `Blocks` to run the static initializer that fills the state
registry. This is the same shape as 1.8.9's single `Bootstrap.register()` call,
with two extra steps.

Proven by compiling a probe against the jar and running it:

```
bootstrap ok
stone friction 0.6
ice friction 0.98
slime friction 0.8
player type entity.minecraft.player
```

Reading `BuiltInRegistries.BLOCK` before that class load finds it empty, and
touching `Level` before bootstrapping throws `Not bootstrapped (called from
registry minecraft:game_event)` — which this note's own first probe did, and is
worth stating because the failure names a registry rather than the missing call.

**One trap the harness protocol must handle.** Bootstrapping installs a logging
framework that takes `System.out` over. Every line the probe above printed came
back wrapped as `[21:02:56] [main/INFO]: [STDOUT]: bootstrap ok`. A harness that
prints its answers after bootstrapping and parses them on the Go side will parse
log decoration. `minecraft-reference`'s 26.1.2 block dumper already solves this
by capturing the `PrintStream` before the game starts, and the movement harness
must do the same.

## 3. Is the entity movement method reachable the same way?

**Yes for the entity; the cost has moved to the world.**

The 1.8.9 harness subclasses `EntityLivingBase` and overrides four equipment
accessors plus the three `Entity` abstracts. Here `LivingEntity` declares one
abstract method of its own — the main-arm accessor — and `Entity` declares four:
synched-data definition, server-side hurt, and the two save-data methods. So the
entity subclass is *smaller* than 1.8.9's.

The movement entry point is a `travel` method taking one input vector, which
dispatches to a fluid branch, a fall-flying branch, and a land branch. The land
branch is the one this milestone reproduces, and it is private — so the harness
drives `travel`, exactly as the 1.8.9 harness drives the whole living update
rather than the pieces.

**The world stub is the real work.** `Level` in 1.8.9 could be subclassed with
six overrides. This version's `Level` declares roughly twenty abstract methods
of its own and inherits more from its accessor interfaces, and its constructor
takes level data, a dimension key, a registry access, and a dimension-type
holder rather than the four loose arguments 1.8.9 took. None of that is
blocked — every one of those objects is reachable after bootstrap — but it is a
few hundred lines of stub rather than the thirty M8.2 needed, and it is the
first task of the oracle work rather than a detail inside it.

Counted with a reflection probe over the class hierarchy:

```
net.minecraft.world.level.Level needs 58 methods implemented
net.minecraft.world.entity.LivingEntity needs 30 methods implemented
```

Both numbers are upper bounds — the probe counts an interface method again even
where a superclass already implements it — but the ratio between them is the
finding, and it is the opposite of what M8.4's experience would suggest.

## 4. Which constants are `float` and which are `double`?

M8.2's lesson, applied before writing code. **They do not match 1.8.9, and the
differences are not a rounding detail — several are a different formula.**

Read from the version's own land-movement branch and the shared entity helpers:

| Quantity | 1.8.9 | 26.1.2 |
| --- | --- | --- |
| Block friction | `float`, from the block | `float`, from the block |
| Friction product | `slipperiness * 0.91F`, `float` | same shape, `float` |
| Ground acceleration | `0.16277136F / friction³`, dividing by the **friction product** | `0.216_000_02F / blockFriction³`, dividing by the **raw block friction** |
| Gravity | a `double` literal, `0.08` | an **attribute**, read as a `double` per entity |
| Vertical drag | `0.98F`, applied to a `double` motion | `0.98F`, applied to a `double` motion |
| Horizontal drag | the tick's friction, `float`, applied to a `double` motion | same shape |
| Input threshold | `1.0E-4F` on the squared `float` magnitude | `1.0E-7` on the squared **`double`** magnitude |
| Input normalization | `float` throughout, widened once at the end | a `double` vector normalize, scaled by a `float` speed |
| Degrees to radians | multiply by `float` pi, then divide by 180 | multiply by a single **pre-divided** `float` constant |
| Sine index | `int(angle * 10430.378F) & 65535`, `float` multiplier | `(int)((long)(angle * 10430.378350470453) & 65535L)`, **`double`** multiplier through a `long` |
| Step height | a field, `float`, widened where applied | an **attribute**, `float` |

Four of these are behavioural changes rather than width changes:

- **The acceleration divides by a different number.** 1.8.9 divides by the
  friction product; this version divides by the block's own friction. On stone
  the two differ by a factor of `0.91³`, and the numerator changed to match.
- **Gravity and step height are attributes.** A profile cannot hold them as
  version constants for the family; it holds the default and the consumer
  supplies modifiers, the way M8.4 already handles movement speed.
- **The input vector is a `double` normalize.** M8.4's heading is the most
  width-sensitive rule in the module and it is `float32` throughout. Its
  counterpart here is mostly `double`, with only the sine and cosine at single
  width. Porting M8.4's rule and adjusting constants would be wrong in a way no
  fixture generated from the same assumption would catch.
- **The sine table is still a 65,536-entry lookup, and its index is computed
  differently.** The multiplier is a `double` and the truncation goes through a
  `long`. `movement.Table` therefore needs a second index rule rather than a
  second table, and the dumped table itself may well be identical — Task 2
  should compare rather than assume.

## The finding the plan called its second risk, confirmed

**26.1.2's collision is a different algorithm, and `collision.Resolve` cannot be
reused.**

M8.2 reproduces 1.8.9 exactly: candidates gathered once as boxes, motion
resolved along Y then X then Z, and a blocked horizontal move retried with two
step-up attempts whose winner is whichever travels further horizontally. This
version gathers voxel shapes rather than boxes, and its step-up collects a list
of *candidate* heights from the shapes it is about to climb, tries each in turn,
and takes the first whose horizontal distance beats the flat move. It also
expands the step-up probe by a small negative epsilon when the body was not
already grounded — the kind of constant M8.2 established 1.8.9 does not have.

So the plan's contingency is the actual path: a version-selected collision
variant behind the same interface, with 1.8.9's committed recordings proving the
original path unchanged. That is a task of its own, and it is closer in size to
M8.2 than to a phase.

## Consequence for this milestone

**A jar-backed oracle is possible.** M8.7 follows M8.4's shape: the differential
test against the game's own `travel` first, fixtures generated from the harness
second, and the gate is the game rather than a reading of it. The weaker
fixtures-only branch in the plan is not taken, and M8.8's live check stays a
check on the adapter and the packet cadence rather than the first verification of
these constants.

What this note changes about the plan's shape:

1. **Task 2's dumper is easier than expected.** No deobfuscation, typed rather
   than reflective compilation, and `minecraft-reference` already has a
   multi-version dumper with exactly this structure for blocks — a version map,
   a per-version program, and an analysis jar resolved from the workspace's
   compatibility report. The physics dumper's single-version rejection is the
   thing to replace, and the block dumper is the template.
2. **A collision variant is a prerequisite task, not a contingency.** It belongs
   before the phase list, because the phase list calls it.
3. **The world stub is the oracle's first task and its largest.** Budget it
   accordingly; it is not the afterthought M8.2's six overrides were.
4. **Nothing may be ported from `profile/java/v1_8` by analogy.** Four of the
   eleven quantities above differ in formula rather than in width. The phase
   order must be established from this version's own movement method, as the
   plan already requires, and now there is a concrete reason to expect it to
   come out differently.
