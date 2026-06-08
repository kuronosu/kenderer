package scene

import (
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

// Object is a mesh placed in the world with a material.
type Object struct {
	Mesh      *geometry.Mesh
	Transform Transform
	Material  shading.Material
}

// Scene bundles everything the renderer needs to draw one frame.
type Scene struct {
	Camera  Camera
	Objects []Object
	Light   shading.DirectionalLight
	Ambient float64
}
