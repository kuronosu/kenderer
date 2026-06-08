package math3d

// Mat4 is a 4x4 matrix stored row-major: element (row, col) is m[row][col].
// It follows the column-vector convention, so transforms apply as
// v' = m.MulVec4(v) and compose right-to-left (MVP = P.Mul(V).Mul(M)).
type Mat4 [4][4]float64

// Identity returns the 4x4 identity matrix.
func Identity() Mat4 {
	return Mat4{
		{1, 0, 0, 0},
		{0, 1, 0, 0},
		{0, 0, 1, 0},
		{0, 0, 0, 1},
	}
}

// Mul returns the matrix product m·n.
func (m Mat4) Mul(n Mat4) Mat4 {
	var out Mat4
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			out[i][j] = m[i][0]*n[0][j] + m[i][1]*n[1][j] + m[i][2]*n[2][j] + m[i][3]*n[3][j]
		}
	}
	return out
}

// MulVec4 returns m·v with v treated as a column vector.
func (m Mat4) MulVec4(v Vec4) Vec4 {
	return Vec4{
		m[0][0]*v.X + m[0][1]*v.Y + m[0][2]*v.Z + m[0][3]*v.W,
		m[1][0]*v.X + m[1][1]*v.Y + m[1][2]*v.Z + m[1][3]*v.W,
		m[2][0]*v.X + m[2][1]*v.Y + m[2][2]*v.Z + m[2][3]*v.W,
		m[3][0]*v.X + m[3][1]*v.Y + m[3][2]*v.Z + m[3][3]*v.W,
	}
}

// Transpose returns the transpose of m.
func (m Mat4) Transpose() Mat4 {
	var out Mat4
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			out[i][j] = m[j][i]
		}
	}
	return out
}

// UpperLeft3 returns the upper-left 3x3 block of m (its linear part).
func (m Mat4) UpperLeft3() Mat3 {
	return Mat3{
		{m[0][0], m[0][1], m[0][2]},
		{m[1][0], m[1][1], m[1][2]},
		{m[2][0], m[2][1], m[2][2]},
	}
}

// Mat3 is a 3x3 matrix stored row-major, used mainly for transforming normals.
type Mat3 [3][3]float64

// MulVec3 returns m·v.
func (m Mat3) MulVec3(v Vec3) Vec3 {
	return Vec3{
		m[0][0]*v.X + m[0][1]*v.Y + m[0][2]*v.Z,
		m[1][0]*v.X + m[1][1]*v.Y + m[1][2]*v.Z,
		m[2][0]*v.X + m[2][1]*v.Y + m[2][2]*v.Z,
	}
}

// Transpose returns the transpose of m.
func (m Mat3) Transpose() Mat3 {
	return Mat3{
		{m[0][0], m[1][0], m[2][0]},
		{m[0][1], m[1][1], m[2][1]},
		{m[0][2], m[1][2], m[2][2]},
	}
}

// Determinant returns det(m).
func (m Mat3) Determinant() float64 {
	return m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1]) -
		m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0]) +
		m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0])
}

// Inverse returns the inverse of m and true, or the zero matrix and false when
// m is singular.
func (m Mat3) Inverse() (Mat3, bool) {
	det := m.Determinant()
	if det == 0 {
		return Mat3{}, false
	}
	inv := 1 / det
	return Mat3{
		{
			(m[1][1]*m[2][2] - m[1][2]*m[2][1]) * inv,
			(m[0][2]*m[2][1] - m[0][1]*m[2][2]) * inv,
			(m[0][1]*m[1][2] - m[0][2]*m[1][1]) * inv,
		},
		{
			(m[1][2]*m[2][0] - m[1][0]*m[2][2]) * inv,
			(m[0][0]*m[2][2] - m[0][2]*m[2][0]) * inv,
			(m[0][2]*m[1][0] - m[0][0]*m[1][2]) * inv,
		},
		{
			(m[1][0]*m[2][1] - m[1][1]*m[2][0]) * inv,
			(m[0][1]*m[2][0] - m[0][0]*m[2][1]) * inv,
			(m[0][0]*m[1][1] - m[0][1]*m[1][0]) * inv,
		},
	}, true
}
