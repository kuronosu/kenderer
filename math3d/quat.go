package math3d

import "math"

// Quat is a quaternion {X, Y, Z, W} with W the scalar part. It represents a
// rotation in the same right-handed convention as RotateX/Y/Z: for a unit
// quaternion, QuatFromAxisAngle(V3(0,1,0), t).Mat4() equals RotateY(t), and
// QuatFromEuler(x,y,z).Mat4() equals RotateZ(z).Mul(RotateY(y)).Mul(RotateX(x)).
type Quat struct{ X, Y, Z, W float64 }

// QuatIdentity returns the identity rotation.
func QuatIdentity() Quat { return Quat{0, 0, 0, 1} }

// QuatFromAxisAngle returns the rotation of rad radians about axis (which is
// normalized internally).
func QuatFromAxisAngle(axis Vec3, rad float64) Quat {
	n := axis.Normalize()
	h := rad / 2
	s := math.Sin(h)
	return Quat{n.X * s, n.Y * s, n.Z * s, math.Cos(h)}
}

// QuatFromEuler returns the rotation Rz(z)·Ry(y)·Rx(x) (angles in radians),
// matching the Euler order used elsewhere in the pipeline.
func QuatFromEuler(x, y, z float64) Quat {
	return QuatFromAxisAngle(Vec3{0, 0, 1}, z).
		Mul(QuatFromAxisAngle(Vec3{0, 1, 0}, y)).
		Mul(QuatFromAxisAngle(Vec3{1, 0, 0}, x))
}

// Mul returns the Hamilton product q·r. It composes like matrix multiplication:
// q.Mul(r).Rotate(v) == q.Rotate(r.Rotate(v)).
func (q Quat) Mul(r Quat) Quat {
	return Quat{
		q.W*r.X + q.X*r.W + q.Y*r.Z - q.Z*r.Y,
		q.W*r.Y - q.X*r.Z + q.Y*r.W + q.Z*r.X,
		q.W*r.Z + q.X*r.Y - q.Y*r.X + q.Z*r.W,
		q.W*r.W - q.X*r.X - q.Y*r.Y - q.Z*r.Z,
	}
}

// dot returns the 4-component dot product q·r.
func (q Quat) dot(r Quat) float64 { return q.X*r.X + q.Y*r.Y + q.Z*r.Z + q.W*r.W }

// Normalize returns q scaled to unit length. A zero quaternion returns the
// identity, mirroring Mat4's zero-value handling.
func (q Quat) Normalize() Quat {
	n := math.Sqrt(q.dot(q))
	if n == 0 {
		return QuatIdentity()
	}
	inv := 1 / n
	return Quat{q.X * inv, q.Y * inv, q.Z * inv, q.W * inv}
}

// Mat4 returns the rotation matrix for q. It is zero-value-safe: a zero
// quaternion (q·q ≈ 0, including the unset Quat{}) yields the identity matrix,
// so callers may treat the zero value as "no rotation" without a guard. The
// factor s = 2/(q·q) keeps the result correct even for non-unit quaternions.
func (q Quat) Mat4() Mat4 {
	nn := q.dot(q)
	if nn < 1e-12 {
		return Identity()
	}
	s := 2 / nn
	xx, yy, zz := q.X*q.X*s, q.Y*q.Y*s, q.Z*q.Z*s
	xy, xz, yz := q.X*q.Y*s, q.X*q.Z*s, q.Y*q.Z*s
	wx, wy, wz := q.W*q.X*s, q.W*q.Y*s, q.W*q.Z*s
	return Mat4{
		{1 - (yy + zz), xy - wz, xz + wy, 0},
		{xy + wz, 1 - (xx + zz), yz - wx, 0},
		{xz - wy, yz + wx, 1 - (xx + yy), 0},
		{0, 0, 0, 1},
	}
}

// Rotate returns v rotated by q. It assumes q is a unit quaternion.
func (q Quat) Rotate(v Vec3) Vec3 {
	u := Vec3{q.X, q.Y, q.Z}
	t := u.Cross(v).Scale(2)
	return v.Add(t.Scale(q.W)).Add(u.Cross(t))
}

// Slerp returns the spherical linear interpolation between unit quaternions a
// and b at t in [0, 1], following the shorter arc. The result stays unit-length;
// for nearly-parallel inputs it falls back to a normalized linear blend.
func Slerp(a, b Quat, t float64) Quat {
	d := a.dot(b)
	if d < 0 { // take the shorter arc
		b = Quat{-b.X, -b.Y, -b.Z, -b.W}
		d = -d
	}
	const parallel = 0.9995
	if d > parallel {
		r := Quat{
			a.X + (b.X-a.X)*t,
			a.Y + (b.Y-a.Y)*t,
			a.Z + (b.Z-a.Z)*t,
			a.W + (b.W-a.W)*t,
		}
		return r.Normalize()
	}
	theta0 := math.Acos(d)
	theta := theta0 * t
	sin0 := math.Sin(theta0)
	sa := math.Sin(theta0-theta) / sin0
	sb := math.Sin(theta) / sin0
	return Quat{
		a.X*sa + b.X*sb,
		a.Y*sa + b.Y*sb,
		a.Z*sa + b.Z*sb,
		a.W*sa + b.W*sb,
	}
}
