package math3d

import "math"

// Vec2 is a 2-component vector, used for screen-space coordinates and texture UVs.
type Vec2 struct{ X, Y float64 }

// V2 returns the vector (x, y).
func V2(x, y float64) Vec2 { return Vec2{x, y} }

// Add returns v + u.
func (v Vec2) Add(u Vec2) Vec2 { return Vec2{v.X + u.X, v.Y + u.Y} }

// Sub returns v - u.
func (v Vec2) Sub(u Vec2) Vec2 { return Vec2{v.X - u.X, v.Y - u.Y} }

// Scale returns v * s.
func (v Vec2) Scale(s float64) Vec2 { return Vec2{v.X * s, v.Y * s} }

// Lerp linearly interpolates between v (t=0) and u (t=1).
func (v Vec2) Lerp(u Vec2, t float64) Vec2 { return v.Add(u.Sub(v).Scale(t)) }

// Vec3 is a 3-component vector (positions, directions, normals, linear RGB).
type Vec3 struct{ X, Y, Z float64 }

// V3 returns the vector (x, y, z).
func V3(x, y, z float64) Vec3 { return Vec3{x, y, z} }

// Add returns v + u.
func (v Vec3) Add(u Vec3) Vec3 { return Vec3{v.X + u.X, v.Y + u.Y, v.Z + u.Z} }

// Sub returns v - u.
func (v Vec3) Sub(u Vec3) Vec3 { return Vec3{v.X - u.X, v.Y - u.Y, v.Z - u.Z} }

// Scale returns v * s.
func (v Vec3) Scale(s float64) Vec3 { return Vec3{v.X * s, v.Y * s, v.Z * s} }

// Mul returns the component-wise (Hadamard) product, handy for modulating colors.
func (v Vec3) Mul(u Vec3) Vec3 { return Vec3{v.X * u.X, v.Y * u.Y, v.Z * u.Z} }

// Neg returns -v.
func (v Vec3) Neg() Vec3 { return Vec3{-v.X, -v.Y, -v.Z} }

// Dot returns the dot product v · u.
func (v Vec3) Dot(u Vec3) float64 { return v.X*u.X + v.Y*u.Y + v.Z*u.Z }

// Cross returns the right-handed cross product v × u.
func (v Vec3) Cross(u Vec3) Vec3 {
	return Vec3{
		v.Y*u.Z - v.Z*u.Y,
		v.Z*u.X - v.X*u.Z,
		v.X*u.Y - v.Y*u.X,
	}
}

// Length returns the Euclidean norm of v.
func (v Vec3) Length() float64 { return math.Sqrt(v.Dot(v)) }

// Normalize returns v scaled to unit length. A zero vector is returned unchanged.
func (v Vec3) Normalize() Vec3 {
	l := v.Length()
	if l == 0 {
		return v
	}
	return v.Scale(1 / l)
}

// Lerp linearly interpolates between v (t=0) and u (t=1).
func (v Vec3) Lerp(u Vec3, t float64) Vec3 { return v.Add(u.Sub(v).Scale(t)) }

// XY drops the Z component.
func (v Vec3) XY() Vec2 { return Vec2{v.X, v.Y} }

// Vec4 promotes v to homogeneous coordinates with the given w.
func (v Vec3) Vec4(w float64) Vec4 { return Vec4{v.X, v.Y, v.Z, w} }

// Vec4 is a homogeneous 4-component vector (e.g. clip-space positions).
type Vec4 struct{ X, Y, Z, W float64 }

// V4 returns the vector (x, y, z, w).
func V4(x, y, z, w float64) Vec4 { return Vec4{x, y, z, w} }

// Add returns v + u.
func (v Vec4) Add(u Vec4) Vec4 { return Vec4{v.X + u.X, v.Y + u.Y, v.Z + u.Z, v.W + u.W} }

// Sub returns v - u.
func (v Vec4) Sub(u Vec4) Vec4 { return Vec4{v.X - u.X, v.Y - u.Y, v.Z - u.Z, v.W - u.W} }

// Scale returns v * s.
func (v Vec4) Scale(s float64) Vec4 { return Vec4{v.X * s, v.Y * s, v.Z * s, v.W * s} }

// Lerp linearly interpolates between v (t=0) and u (t=1), including w.
func (v Vec4) Lerp(u Vec4, t float64) Vec4 { return v.Add(u.Sub(v).Scale(t)) }

// XYZ drops the W component.
func (v Vec4) XYZ() Vec3 { return Vec3{v.X, v.Y, v.Z} }
