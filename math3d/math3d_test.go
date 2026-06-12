package math3d

import (
	"math"
	"testing"
)

const eps = 1e-9

func approx(a, b float64) bool { return math.Abs(a-b) <= eps }

func vec3Approx(a, b Vec3) bool {
	return approx(a.X, b.X) && approx(a.Y, b.Y) && approx(a.Z, b.Z)
}

func vec4Approx(a, b Vec4) bool {
	return approx(a.X, b.X) && approx(a.Y, b.Y) && approx(a.Z, b.Z) && approx(a.W, b.W)
}

func mat3Approx(a, b Mat3) bool {
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if !approx(a[i][j], b[i][j]) {
				return false
			}
		}
	}
	return true
}

func mat4Approx(a, b Mat4) bool {
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			if !approx(a[i][j], b[i][j]) {
				return false
			}
		}
	}
	return true
}

func TestMat4MulIdentity(t *testing.T) {
	a := Mat4{
		{1, 2, 3, 4},
		{5, 6, 7, 8},
		{9, 10, 11, 12},
		{13, 14, 15, 16},
	}
	id := Identity()
	if !mat4Approx(a.Mul(id), a) {
		t.Errorf("a·I != a")
	}
	if !mat4Approx(id.Mul(a), a) {
		t.Errorf("I·a != a")
	}
}

func TestMat4MulComposition(t *testing.T) {
	tr := Translate(Vec3{1, 2, 3})
	sc := Scale(Vec3{2, 2, 2})
	// T·S applies S first (rightmost), then T.
	tests := []struct {
		in   Vec4
		want Vec4
	}{
		{Vec4{0, 0, 0, 1}, Vec4{1, 2, 3, 1}}, // origin -> translation
		{Vec4{1, 1, 1, 1}, Vec4{3, 4, 5, 1}}, // scaled to (2,2,2), then +(1,2,3)
	}
	m := tr.Mul(sc)
	for _, tc := range tests {
		if got := m.MulVec4(tc.in); !vec4Approx(got, tc.want) {
			t.Errorf("T·S·%v = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestRotate(t *testing.T) {
	tests := []struct {
		name string
		m    Mat4
		in   Vec4
		want Vec4
	}{
		{"Z:X->Y", RotateZ(math.Pi / 2), Vec4{1, 0, 0, 1}, Vec4{0, 1, 0, 1}},
		{"Y:Z->X", RotateY(math.Pi / 2), Vec4{0, 0, 1, 1}, Vec4{1, 0, 0, 1}},
		{"X:Y->Z", RotateX(math.Pi / 2), Vec4{0, 1, 0, 1}, Vec4{0, 0, 1, 1}},
	}
	for _, tc := range tests {
		if got := tc.m.MulVec4(tc.in); !vec4Approx(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestPerspective(t *testing.T) {
	near, far := 1.0, 100.0
	p := Perspective(math.Pi/2, 1, near, far)

	// fovy=90, aspect=1 => focal length f=1 on the diagonal.
	if !approx(p[0][0], 1) || !approx(p[1][1], 1) {
		t.Errorf("focal diagonal = (%v,%v), want (1,1)", p[0][0], p[1][1])
	}

	tests := []struct {
		z      float64
		wantZN float64 // expected NDC z
	}{
		{-near, -1},
		{-far, 1},
	}
	for _, tc := range tests {
		c := p.MulVec4(Vec4{0, 0, tc.z, 1})
		if zn := c.Z / c.W; !approx(zn, tc.wantZN) {
			t.Errorf("z_view=%v -> NDC z=%v, want %v", tc.z, zn, tc.wantZN)
		}
	}

	// w_clip must equal -z_view.
	if c := p.MulVec4(Vec4{0, 0, -42, 1}); !approx(c.W, 42) {
		t.Errorf("w_clip = %v, want 42", c.W)
	}
}

func TestPerspectiveZO(t *testing.T) {
	near, far := 1.0, 100.0
	p := PerspectiveZO(math.Pi/2, 1, near, far)
	gl := Perspective(math.Pi/2, 1, near, far)

	// Only the depth rows differ from Perspective: x/y mapping and w_clip match.
	for r := 0; r < 2; r++ {
		for c := 0; c < 4; c++ {
			if p[r][c] != gl[r][c] {
				t.Errorf("row %d differs from Perspective: %v vs %v", r, p[r], gl[r])
			}
		}
	}

	tests := []struct {
		z      float64
		wantZN float64 // expected NDC z (zero-to-one)
	}{
		{-near, 0},
		{-far, 1},
	}
	for _, tc := range tests {
		c := p.MulVec4(Vec4{0, 0, tc.z, 1})
		if zn := c.Z / c.W; !approx(zn, tc.wantZN) {
			t.Errorf("z_view=%v -> NDC z=%v, want %v", tc.z, zn, tc.wantZN)
		}
	}

	// The ZO depth must equal the GL NDC depth remapped to a window z in [0, 1]
	// (0.5z+0.5, the viewport convention) — that identity is what makes the GPU
	// and CPU backends agree on every interpolated depth value, not just at the
	// near and far planes.
	for _, z := range []float64{-1.5, -7, -42.5, -99} {
		zo := p.MulVec4(Vec4{0, 0, z, 1})
		gn := gl.MulVec4(Vec4{0, 0, z, 1})
		if got, want := zo.Z/zo.W, 0.5*(gn.Z/gn.W)+0.5; !approx(got, want) {
			t.Errorf("z_view=%v: ZO depth %v != window-remapped GL depth %v", z, got, want)
		}
	}

	if c := p.MulVec4(Vec4{0, 0, -42, 1}); !approx(c.W, 42) {
		t.Errorf("w_clip = %v, want 42", c.W)
	}
}

func TestLookAt(t *testing.T) {
	eye := Vec3{0, 0, 5}
	view := LookAt(eye, Vec3{0, 0, 0}, Vec3{0, 1, 0})

	if got := view.MulVec4(eye.Vec4(1)); !vec4Approx(got, Vec4{0, 0, 0, 1}) {
		t.Errorf("eye -> %v, want origin", got)
	}
	// The target sits in front of the camera: negative view-space z at distance 5.
	if got := view.MulVec4(Vec4{0, 0, 0, 1}); !approx(got.Z, -5) {
		t.Errorf("target z_view = %v, want -5", got.Z)
	}
}

func TestViewport(t *testing.T) {
	vp := Viewport(0, 0, 800, 600)
	tests := []struct {
		name string
		ndc  Vec4
		want Vec3
	}{
		{"top-left/near", Vec4{-1, 1, -1, 1}, Vec3{0, 0, 0}},
		{"bottom-right/far", Vec4{1, -1, 1, 1}, Vec3{800, 600, 1}},
		{"center/mid", Vec4{0, 0, 0, 1}, Vec3{400, 300, 0.5}},
	}
	for _, tc := range tests {
		if got := vp.MulVec4(tc.ndc).XYZ(); !vec3Approx(got, tc.want) {
			t.Errorf("%s: Viewport·%v = %v, want %v", tc.name, tc.ndc, got, tc.want)
		}
	}
}

func TestNormalMatrix(t *testing.T) {
	// Pure rotation is orthonormal: its inverse transpose equals itself.
	rot := RotateZ(0.7)
	if nm := NormalMatrix(rot); !mat3Approx(nm, rot.UpperLeft3()) {
		t.Errorf("normal matrix of a rotation should equal its 3x3 block")
	}

	// Under non-uniform scaling the inverse transpose keeps a normal
	// perpendicular to a tangent, whereas the naive linear transform does not.
	model := Scale(Vec3{2, 1, 1})
	lin := model.UpperLeft3()
	nm := NormalMatrix(model)

	tangent := Vec3{1, -1, 0}
	normal := Vec3{1, 1, 0} // perpendicular to tangent
	tT := lin.MulVec3(tangent)

	if nT := nm.MulVec3(normal); !approx(nT.Dot(tT), 0) {
		t.Errorf("inverse-transpose normal not perpendicular: dot=%v", nT.Dot(tT))
	}
	if nNaive := lin.MulVec3(normal); approx(nNaive.Dot(tT), 0) {
		t.Errorf("naive normal unexpectedly perpendicular; test would be vacuous")
	}
}

func TestVec3Ops(t *testing.T) {
	if got := (Vec3{1, 0, 0}).Cross(Vec3{0, 1, 0}); !vec3Approx(got, Vec3{0, 0, 1}) {
		t.Errorf("X×Y = %v, want Z", got)
	}
	if got := (Vec3{1, 0, 0}).Dot(Vec3{0, 1, 0}); !approx(got, 0) {
		t.Errorf("X·Y = %v, want 0", got)
	}
	if got := (Vec3{3, 4, 0}).Length(); !approx(got, 5) {
		t.Errorf("|(3,4,0)| = %v, want 5", got)
	}
	if got := (Vec3{0, 5, 0}).Normalize(); !vec3Approx(got, Vec3{0, 1, 0}) {
		t.Errorf("normalize(0,5,0) = %v, want (0,1,0)", got)
	}
	if got := (Vec3{0, 0, 0}).Normalize(); !vec3Approx(got, Vec3{0, 0, 0}) {
		t.Errorf("normalize(0,0,0) = %v, want zero", got)
	}
}
