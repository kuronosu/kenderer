package math3d

import (
	"math"
	"testing"
)

func quatApprox(a, b Quat) bool {
	return approx(a.X, b.X) && approx(a.Y, b.Y) && approx(a.Z, b.Z) && approx(a.W, b.W)
}

func TestQuatFromAxisAngleMatchesRotations(t *testing.T) {
	for _, theta := range []float64{0, 0.1, math.Pi / 4, math.Pi / 2, 2, math.Pi, 4} {
		if !mat4Approx(QuatFromAxisAngle(V3(1, 0, 0), theta).Mat4(), RotateX(theta)) {
			t.Errorf("X theta=%v: Quat.Mat4 != RotateX", theta)
		}
		if !mat4Approx(QuatFromAxisAngle(V3(0, 1, 0), theta).Mat4(), RotateY(theta)) {
			t.Errorf("Y theta=%v: Quat.Mat4 != RotateY", theta)
		}
		if !mat4Approx(QuatFromAxisAngle(V3(0, 0, 1), theta).Mat4(), RotateZ(theta)) {
			t.Errorf("Z theta=%v: Quat.Mat4 != RotateZ", theta)
		}
	}
}

func TestQuatFromEulerMatchesMatrixOrder(t *testing.T) {
	cases := [][3]float64{
		{0, 0, 0}, {0.3, 0, 0}, {0, 0.5, 0}, {0, 0, 0.7}, {0.3, 0.5, 0.7}, {-0.4, 1.1, 0.2},
	}
	for _, c := range cases {
		x, y, z := c[0], c[1], c[2]
		want := RotateZ(z).Mul(RotateY(y)).Mul(RotateX(x))
		if got := QuatFromEuler(x, y, z).Mat4(); !mat4Approx(got, want) {
			t.Errorf("euler %v: Quat.Mat4 != Rz·Ry·Rx", c)
		}
	}
}

func TestQuatMulComposes(t *testing.T) {
	q := QuatFromAxisAngle(V3(0, 1, 0), 0.7)
	id := QuatIdentity()
	if !quatApprox(q.Mul(id), q) || !quatApprox(id.Mul(q), q) {
		t.Errorf("identity must be neutral for Mul")
	}
	// The product corresponds to matrix composition.
	a := QuatFromAxisAngle(V3(1, 0, 0), 0.5)
	b := QuatFromAxisAngle(V3(0, 0, 1), 0.9)
	if !mat4Approx(a.Mul(b).Mat4(), a.Mat4().Mul(b.Mat4())) {
		t.Errorf("Mul does not compose like the matrix product")
	}
}

func TestQuatRotateMatchesMat4(t *testing.T) {
	q := QuatFromEuler(0.3, 0.5, 0.7)
	for _, v := range []Vec3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}, {1, 2, 3}, {-2, 0.5, 1}} {
		got := q.Rotate(v)
		want := q.Mat4().MulVec4(v.Vec4(1)).XYZ()
		if !vec3Approx(got, want) {
			t.Errorf("Rotate(%v) = %v, want %v", v, got, want)
		}
	}
}

func TestSlerpEndpointsAndLength(t *testing.T) {
	a := QuatIdentity()
	b := QuatFromAxisAngle(V3(0, 1, 0), math.Pi/2) // dot(a,b) = cos(pi/4) > 0
	if !quatApprox(Slerp(a, b, 0), a) {
		t.Errorf("Slerp(a,b,0) must equal a")
	}
	if !quatApprox(Slerp(a, b, 1), b) {
		t.Errorf("Slerp(a,b,1) must equal b")
	}
	for _, tt := range []float64{0.25, 0.5, 0.75} {
		q := Slerp(a, b, tt)
		if l := math.Sqrt(q.dot(q)); math.Abs(l-1) > 1e-9 {
			t.Errorf("Slerp length at t=%v = %v, want ~1", tt, l)
		}
	}
}

func TestQuatZeroValueIsIdentity(t *testing.T) {
	if !mat4Approx(Quat{}.Mat4(), Identity()) {
		t.Errorf("zero-value Quat{}.Mat4() must be the identity matrix")
	}
}
