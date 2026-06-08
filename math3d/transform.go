package math3d

import "math"

// Translate returns a matrix that translates by t.
func Translate(t Vec3) Mat4 {
	m := Identity()
	m[0][3] = t.X
	m[1][3] = t.Y
	m[2][3] = t.Z
	return m
}

// Scale returns a matrix that scales by s along each axis.
func Scale(s Vec3) Mat4 {
	return Mat4{
		{s.X, 0, 0, 0},
		{0, s.Y, 0, 0},
		{0, 0, s.Z, 0},
		{0, 0, 0, 1},
	}
}

// RotateX returns a right-handed rotation of rad radians about the X axis.
func RotateX(rad float64) Mat4 {
	c, s := math.Cos(rad), math.Sin(rad)
	m := Identity()
	m[1][1], m[1][2] = c, -s
	m[2][1], m[2][2] = s, c
	return m
}

// RotateY returns a right-handed rotation of rad radians about the Y axis.
func RotateY(rad float64) Mat4 {
	c, s := math.Cos(rad), math.Sin(rad)
	m := Identity()
	m[0][0], m[0][2] = c, s
	m[2][0], m[2][2] = -s, c
	return m
}

// RotateZ returns a right-handed rotation of rad radians about the Z axis.
func RotateZ(rad float64) Mat4 {
	c, s := math.Cos(rad), math.Sin(rad)
	m := Identity()
	m[0][0], m[0][1] = c, -s
	m[1][0], m[1][1] = s, c
	return m
}

// Perspective returns a right-handed perspective projection matrix. fovy is the
// vertical field of view in radians and aspect is width/height. View-space z in
// [-near, -far] maps to NDC z in [-1, 1], with w_clip = -z_view.
func Perspective(fovy, aspect, near, far float64) Mat4 {
	f := 1 / math.Tan(fovy/2)
	var m Mat4
	m[0][0] = f / aspect
	m[1][1] = f
	m[2][2] = (far + near) / (near - far)
	m[2][3] = (2 * far * near) / (near - far)
	m[3][2] = -1
	return m
}

// LookAt returns a right-handed view matrix for a camera positioned at eye,
// looking at target, with the given approximate up vector. The camera looks
// down its local -Z axis.
func LookAt(eye, target, up Vec3) Mat4 {
	f := target.Sub(eye).Normalize() // forward
	s := f.Cross(up).Normalize()     // right
	u := s.Cross(f)                  // recomputed (orthonormal) up
	return Mat4{
		{s.X, s.Y, s.Z, -s.Dot(eye)},
		{u.X, u.Y, u.Z, -u.Dot(eye)},
		{-f.X, -f.Y, -f.Z, f.Dot(eye)},
		{0, 0, 0, 1},
	}
}

// Viewport returns the matrix mapping NDC coordinates to the screen rectangle
// [x, x+w] x [y, y+h]. Y is flipped (NDC +1 maps to the top of the rectangle)
// and NDC z in [-1, 1] maps to window z in [0, 1].
func Viewport(x, y, w, h float64) Mat4 {
	return Mat4{
		{w / 2, 0, 0, x + w/2},
		{0, -h / 2, 0, y + h/2},
		{0, 0, 0.5, 0.5},
		{0, 0, 0, 1},
	}
}

// NormalMatrix returns the matrix used to transform normals: the inverse
// transpose of the model matrix' upper-left 3x3. For a rigid transform this
// equals the rotation itself; for non-uniform scaling it keeps normals
// perpendicular to their surface. If the linear part is singular, the
// uncorrected upper-left 3x3 is returned.
func NormalMatrix(model Mat4) Mat3 {
	linear := model.UpperLeft3()
	inv, ok := linear.Inverse()
	if !ok {
		return linear
	}
	return inv.Transpose()
}
