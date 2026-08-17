package collision

import (
	"math"
	"slices"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// The tolerances Java Edition added when it reworked collision around shapes.
//
// The 1.8.9 algorithm has none of these: it compares exactly, and M8.2 recorded
// that absence as deliberate after checking it against the game. They are the
// clearest single signal that these are two algorithms rather than one with a
// parameter.
const (
	// tolerance is the slack the shape clamp works to. A motion smaller than it
	// is treated as no motion at all, and a face within it of the moving box is
	// treated as touching rather than overlapping.
	tolerance = 1.0e-7
	// horizontalTolerance is how far a horizontal axis may fall short of its
	// intent before the move counts as having collided. The game writes it as a
	// float literal and compares doubles against it, so it is the widened float
	// rather than the round decimal.
	horizontalTolerance = float64(float32(1.0e-5))
	// airborneStepProbe is the extra reach downward the step-up probe gets when
	// the body was not already grounded. Also a float literal in the game.
	airborneStepProbe = float64(float32(-1.0e-5))
)

// Axis names a coordinate axis, so that one clamp can serve all three.
//
// The 1.8.9 resolve can write out its three passes because their order is
// fixed. This one cannot: the order depends on the motion, so the passes have
// to be a loop over a value.
type Axis uint8

// The three axes, in the order the coordinates are written everywhere else.
const (
	AxisX Axis = iota
	AxisY
	AxisZ
)

// String returns the axis's name.
func (a Axis) String() string {
	switch a {
	case AxisX:
		return "X"
	case AxisY:
		return "Y"
	case AxisZ:
		return "Z"
	default:
		return "unknown"
	}
}

// ResolveVoxel applies move the way Java Edition does from 1.13 onward.
//
// It is a separate function from Resolve rather than a flag on it, because the
// two are different algorithms and the differences are not parameters:
//
//   - The axis order depends on the motion. Y is always first, and then the
//     horizontal axis with the larger magnitude. 1.8.9 always runs Y, X, Z.
//   - Every comparison works to a tolerance. 1.8.9 compares exactly.
//   - The step-up tries a list of heights collected from the faces of whatever
//     is about to be climbed, in ascending order, and takes the first that
//     beats the flat move. 1.8.9 tries exactly two heights, both the full step
//     height, and takes whichever travels further.
//   - A horizontal axis counts as collided only if it fell short by more than
//     a hundred-thousandth. The vertical axis still compares exactly.
//
// Sharing one function between the two would mean four flags, and a caller
// would have to know all four to know which game it was simulating.
func ResolveVoxel(view world.BlockView, move Move) (Result, error) {
	candidates, err := Gather(view, move.Body.Stretch(move.Motion), move.CandidateLimit)
	if err != nil {
		return Result{}, err
	}
	if len(candidates.Unknown) != 0 {
		return Result{Body: move.Body, Unknown: candidates.Unknown}, nil
	}

	applied := ResolveShapes(candidates.Boxes, move.Body, move.Motion)

	// The step-up conditions, in the game's order: a body may step if it has a
	// step height, if it either landed this tick or was already standing, and if
	// a horizontal axis was actually blocked.
	blockedX := applied.X != move.Motion.X
	blockedZ := applied.Z != move.Motion.Z
	landed := applied.Y != move.Motion.Y && move.Motion.Y < 0

	stepped := false
	if move.StepHeight > 0 && (landed || move.OnGround) && (blockedX || blockedZ) {
		climbed, ok, err := stepUpVoxel(view, move, applied)
		if err != nil {
			return Result{}, err
		}
		if len(climbed.Unknown) != 0 {
			return Result{Body: move.Body, Unknown: climbed.Unknown}, nil
		}
		if ok {
			applied = climbed.Applied
			stepped = true
		}
	}

	return Result{
		Body:    move.Body.Offset(applied),
		Applied: applied,
		// The horizontal flags forgive a shortfall smaller than the tolerance;
		// the vertical one does not. That asymmetry is the game's.
		CollidedX: math.Abs(move.Motion.X-applied.X) >= horizontalTolerance,
		CollidedY: applied.Y != move.Motion.Y,
		CollidedZ: math.Abs(move.Motion.Z-applied.Z) >= horizontalTolerance,
		OnGround:  applied.Y != move.Motion.Y && move.Motion.Y < 0,
		Stepped:   stepped,
	}, nil
}

// stepUpVoxel retries a blocked move over each height the obstacle offers.
//
// The heights come from the obstacle rather than from the entity: every face of
// every shape in reach, measured from the body's feet, is a candidate, and the
// first one that carries the body further horizontally than the flat move wins.
// A staircase of eighths therefore climbs an eighth at a time rather than
// jumping the whole step height, which is what makes modern stairs and slabs
// feel different underfoot from 1.8.9's.
func stepUpVoxel(view world.BlockView, move Move, flat geom.Vec3) (Result, bool, error) {
	landed := flat.Y != move.Motion.Y && move.Motion.Y < 0

	// The body the climb starts from: where the flat move's vertical clamp left
	// it if it landed this tick, and where it began if it was already standing.
	grounded := move.Body
	if landed {
		grounded = move.Body.Offset(geom.Vec3{Y: flat.Y})
	}

	probe := grounded.Stretch(geom.Vec3{X: move.Motion.X, Y: move.StepHeight, Z: move.Motion.Z})
	if !landed {
		// A body already standing reaches a hair below itself as well, so that a
		// face exactly at its feet is still a candidate.
		probe = probe.Stretch(geom.Vec3{Y: airborneStepProbe})
	}

	candidates, coords, err := GatherWithSteps(view, probe, move.CandidateLimit)
	if err != nil {
		return Result{}, false, err
	}
	if len(candidates.Unknown) != 0 {
		return Result{Unknown: candidates.Unknown}, false, nil
	}

	heights := StepHeights(grounded, coords, float32(move.StepHeight), float32(flat.Y))
	for _, height := range heights {
		climbed := ResolveShapes(
			candidates.Boxes, grounded,
			geom.Vec3{X: move.Motion.X, Y: float64(height), Z: move.Motion.Z},
		)
		if climbed.HorizontalLengthSquared() <= flat.HorizontalLengthSquared() {
			continue
		}

		// The climb was measured from the grounded body, so the drop back to the
		// original body's feet comes off the reported vertical motion. This is
		// why a step reports the settle rather than the climb, the same way
		// 1.8.9 does by a different route.
		return Result{Applied: climbed.Sub(geom.Vec3{Y: move.Body.MinY - grounded.MinY})}, true, nil
	}

	return Result{}, false, nil
}

// StepHeights returns the rises worth trying, in ascending order.
//
// The rises come from the coordinates the shapes offer rather than from the
// faces of their boxes, and the difference is real: a shape stores a grid, so a
// plate an eighth thick offers all eight eighth-lines and not just its top. Each
// is measured from the body's feet and narrowed to single width, because the
// game holds a step height as a float.
//
// Rises below the feet, above the step height, or exactly equal to the vertical
// motion the flat move already applied are dropped. The last of those is what
// stops the retry from re-running the move it is trying to improve on.
func StepHeights(body geom.AABB, coords []float64, limit, skip float32) []float32 {
	var heights []float32
	for _, coord := range coords {
		height := float32(coord - body.MinY)
		if height < 0 || height > limit || height == skip {
			continue
		}
		if !slices.Contains(heights, height) {
			heights = append(heights, height)
		}
	}
	slices.Sort(heights)

	return heights
}

// ResolveShapes clamps a motion against boxes, one axis at a time.
//
// The body is offset by what has resolved so far before each axis, so a later
// axis tests a box that has already moved. The order is Y, then the larger
// horizontal axis, then the smaller: a body moving mostly along X resolves X
// before Z, and one moving mostly along Z resolves Z first.
func ResolveShapes(boxes []geom.AABB, body geom.AABB, motion geom.Vec3) geom.Vec3 {
	if len(boxes) == 0 {
		return motion
	}

	var resolved geom.Vec3
	for _, along := range axisOrder(motion) {
		distance := component(motion, along)
		if distance == 0 {
			continue
		}
		clamped := ClampAxis(boxes, body.Offset(resolved), along, distance)
		resolved = withComponent(resolved, along, clamped)
	}

	return resolved
}

// axisOrder returns the order the axes resolve in for a motion.
func axisOrder(motion geom.Vec3) [3]Axis {
	if math.Abs(motion.X) < math.Abs(motion.Z) {
		return [3]Axis{AxisY, AxisZ, AxisX}
	}

	return [3]Axis{AxisY, AxisX, AxisZ}
}

// ClampAxis reduces a distance until it no longer carries the moving box into
// any of the boxes.
//
// A distance that has already shrunk below the tolerance is reported as zero
// rather than carried, which is the game's way of stopping a body that is
// pressed against a wall from creeping by fractions of a millionth of a block.
func ClampAxis(boxes []geom.AABB, moving geom.AABB, along Axis, distance float64) float64 {
	for _, box := range boxes {
		if math.Abs(distance) < tolerance {
			return 0
		}
		distance = clampShape(box, moving, along, distance)
	}

	return distance
}

// clampShape is one box's clamp along one axis.
//
// The overlap test on the other two axes shrinks the moving box by the
// tolerance on each side, so a body sliding exactly along a face is not
// considered to be inside it. The clamp itself allows the same tolerance in the
// other direction, so a face the body is already fractionally past still stops
// it rather than letting it through.
func clampShape(box, moving geom.AABB, along Axis, distance float64) float64 {
	if math.Abs(distance) < tolerance {
		return 0
	}

	first, second := otherAxes(along)
	if !overlaps(box, moving, first) || !overlaps(box, moving, second) {
		return distance
	}

	if distance > 0 {
		if gap := minOf(box, along) - maxOf(moving, along); gap >= -tolerance {
			return math.Min(distance, gap)
		}

		return distance
	}

	if gap := maxOf(box, along) - minOf(moving, along); gap <= tolerance {
		return math.Max(distance, gap)
	}

	return distance
}

// overlaps reports whether the moving box, shrunk by the tolerance, overlaps
// the box along one axis.
func overlaps(box, moving geom.AABB, along Axis) bool {
	return minOf(moving, along)+tolerance < maxOf(box, along) &&
		maxOf(moving, along)-tolerance > minOf(box, along)
}

// otherAxes returns the two axes a clamp tests overlap on.
func otherAxes(along Axis) (Axis, Axis) {
	switch along {
	case AxisX:
		return AxisY, AxisZ
	case AxisY:
		return AxisX, AxisZ
	case AxisZ:
		return AxisX, AxisY
	default:
		return AxisX, AxisY
	}
}

func component(vector geom.Vec3, along Axis) float64 {
	switch along {
	case AxisX:
		return vector.X
	case AxisY:
		return vector.Y
	case AxisZ:
		return vector.Z
	default:
		return vector.Z
	}
}

func withComponent(vector geom.Vec3, along Axis, value float64) geom.Vec3 {
	switch along {
	case AxisX:
		vector.X = value
	case AxisY:
		vector.Y = value
	case AxisZ:
		vector.Z = value
	default:
		vector.Z = value
	}

	return vector
}

func minOf(box geom.AABB, along Axis) float64 {
	switch along {
	case AxisX:
		return box.MinX
	case AxisY:
		return box.MinY
	case AxisZ:
		return box.MinZ
	default:
		return box.MinZ
	}
}

func maxOf(box geom.AABB, along Axis) float64 {
	switch along {
	case AxisX:
		return box.MaxX
	case AxisY:
		return box.MaxY
	case AxisZ:
		return box.MaxZ
	default:
		return box.MaxZ
	}
}
