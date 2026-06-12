package scene

import (
	"math"

	"github.com/kuronosu/kenderer/geometry"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/shading"
)

// Camera is a perspective camera defined by its pose and frustum parameters.
type Camera struct {
	Eye, Target, Up math3d.Vec3
	FOVY            float64 // vertical field of view, radians
	Near, Far       float64
}

// View returns the world-to-view (LookAt) matrix.
func (c Camera) View() math3d.Mat4 { return math3d.LookAt(c.Eye, c.Target, c.Up) }

// Projection returns the perspective projection matrix for the given aspect
// ratio (width / height).
func (c Camera) Projection(aspect float64) math3d.Mat4 {
	return math3d.Perspective(c.FOVY, aspect, c.Near, c.Far)
}

// Transform is a translate-rotate-scale transform. Rotation is a quaternion; the
// composed matrix is T * Rotation.Mat4() * S, i.e. a vertex is scaled, rotated,
// then translated. The zero-value Rotation (Quat{}) means "no rotation" because
// math3d.Quat.Mat4 is zero-value-safe.
type Transform struct {
	Position math3d.Vec3
	Rotation math3d.Quat
	Scale    math3d.Vec3
}

// Matrix returns the composed model matrix. Note that a zero Scale collapses the
// object; callers typically want Scale = (1, 1, 1).
func (t Transform) Matrix() math3d.Mat4 {
	return math3d.Translate(t.Position).Mul(t.Rotation.Mat4()).Mul(math3d.Scale(t.Scale))
}

// Object is a mesh placed in the world with a material. Smooth selects the
// shading mode: when false (the default) the renderer flat-shades each triangle
// with its geometric face normal; when true it keeps the interpolated per-vertex
// normals, yielding smooth (Phong) shading. Flatness is a property of the model,
// so a flat-shaded cube and a smooth imported mesh can share one scene.
type Object struct {
	Mesh      *geometry.Mesh
	Transform Transform
	Material  shading.Material
	Smooth    bool
}

// Segment is a colored line segment in world space, drawn by the renderer's line
// pass with no lighting and a depth test against the geometry (it is occluded but
// writes no depth). Color is linear RGB, like every other color the pipeline
// shades, and is sRGB-encoded once at output. Segments are the generic line
// primitive: world axes, grids, wireframes and normals are all just lists of them.
type Segment struct {
	A, B  math3d.Vec3 // world-space endpoints
	Color math3d.Vec3 // linear RGB
}

// AxisColorX, AxisColorY and AxisColorZ are the conventional axis colors in linear
// RGB: +X red, +Y green, +Z blue. Being pure 0/1 components, they encode to
// saturated sRGB (e.g. 255,0,0) at output. The world-axis helper and the renderer's
// per-object axes share these so the convention has a single source of truth.
var (
	AxisColorX = math3d.V3(1, 0, 0)
	AxisColorY = math3d.V3(0, 1, 0)
	AxisColorZ = math3d.V3(0, 0, 1)
)

// axisInfinity is the half-length of the "infinite" world-axis segments. The
// frustum clip trims them to the visible portion, so a value this large reads as
// effectively infinite for any reasonable camera while avoiding ±Inf arithmetic.
const axisInfinity = 1e7

// WorldAxes returns the three world coordinate axes as colored segments spanning
// ±axisInfinity along X (red), Y (green) and Z (blue). Drop them into Scene.Lines
// to draw the world frame; the renderer's frustum clip makes them effectively
// infinite lines through the origin.
func WorldAxes() []Segment {
	return []Segment{
		{A: math3d.V3(-axisInfinity, 0, 0), B: math3d.V3(axisInfinity, 0, 0), Color: AxisColorX},
		{A: math3d.V3(0, -axisInfinity, 0), B: math3d.V3(0, axisInfinity, 0), Color: AxisColorY},
		{A: math3d.V3(0, 0, -axisInfinity), B: math3d.V3(0, 0, axisInfinity), Color: AxisColorZ},
	}
}

// Scene bundles everything the renderer needs to draw one frame.
type Scene struct {
	Camera  Camera
	Objects []Object
	Light   shading.DirectionalLight
	Ambient float64
	// Lines are world-space colored segments drawn (depth-tested, unlit) after the
	// triangles, e.g. the world axes from WorldAxes. A nil/empty slice draws nothing.
	Lines []Segment
}

// Bounds returns the combined axis-aligned bounding box of the objects' meshes,
// as its minimum and maximum corners. It unions each mesh's local-space Bounds
// and does not apply per-object Transforms: loaded models are mounted with an
// identity transform (the asset loaders bake world-space geometry), so mesh space
// is world space for them. An empty slice returns the zero box.
func Bounds(objects []Object) (min, max math3d.Vec3) {
	if len(objects) == 0 {
		return math3d.Vec3{}, math3d.Vec3{}
	}
	inf := math.Inf(1)
	min = math3d.V3(inf, inf, inf)
	max = math3d.V3(-inf, -inf, -inf)
	for _, o := range objects {
		lo, hi := o.Mesh.Bounds()
		min = min.Min(lo)
		max = max.Max(hi)
	}
	return min, max
}
