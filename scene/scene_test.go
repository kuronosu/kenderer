package scene

import (
	"math"
	"testing"

	"github.com/kuronosu/kenderer/geometry"
	"github.com/kuronosu/kenderer/math3d"
)

func TestTransformMatrixOrder(t *testing.T) {
	// Scale by 2, rotate +90 deg about Y, then translate by (10,0,0).
	// (1,0,0) -> scale (2,0,0) -> RotateY(90) (0,0,-2) -> translate (10,0,-2).
	tr := Transform{
		Position: math3d.V3(10, 0, 0),
		Rotation: math3d.QuatFromEuler(0, math.Pi/2, 0),
		Scale:    math3d.V3(2, 2, 2),
	}
	got := tr.Matrix().MulVec4(math3d.V4(1, 0, 0, 1))
	want := math3d.V4(10, 0, -2, 1)
	if math.Abs(got.X-want.X) > 1e-9 || math.Abs(got.Y-want.Y) > 1e-9 || math.Abs(got.Z-want.Z) > 1e-9 {
		t.Errorf("Matrix·(1,0,0) = %v, want %v", got, want)
	}
}

func TestObjectAxes(t *testing.T) {
	approx := func(a, b math3d.Vec3) bool {
		return math.Abs(a.X-b.X) < 1e-9 && math.Abs(a.Y-b.Y) < 1e-9 && math.Abs(a.Z-b.Z) < 1e-9
	}

	// A size-2 cube has extent 2, so the axes are 1.25*2 = 2.5 long in model
	// space; the translation moves the whole frame, and every backend must see
	// these exact world-space endpoints (single source of the gizmo).
	pos := math3d.V3(3, -1, 2)
	obj := Object{
		Mesh:      geometry.NewCube(2),
		Transform: Transform{Position: pos, Rotation: math3d.QuatIdentity(), Scale: math3d.V3(1, 1, 1)},
	}
	axes := ObjectAxes(obj)
	wantTips := [3]math3d.Vec3{
		pos.Add(math3d.V3(2.5, 0, 0)),
		pos.Add(math3d.V3(0, 2.5, 0)),
		pos.Add(math3d.V3(0, 0, 2.5)),
	}
	wantColors := [3]math3d.Vec3{AxisColorX, AxisColorY, AxisColorZ}
	for i, seg := range axes {
		if !approx(seg.A, pos) {
			t.Errorf("axis %d origin = %v, want %v", i, seg.A, pos)
		}
		if !approx(seg.B, wantTips[i]) {
			t.Errorf("axis %d tip = %v, want %v", i, seg.B, wantTips[i])
		}
		if seg.Color != wantColors[i] {
			t.Errorf("axis %d color = %v, want %v", i, seg.Color, wantColors[i])
		}
	}

	// A degenerate (zero-extent) mesh falls back to unit axes: 1.25*1.
	deg := Object{
		Mesh:      &geometry.Mesh{Vertices: []geometry.Vertex{{}}},
		Transform: Transform{Scale: math3d.V3(1, 1, 1)},
	}
	if tip := ObjectAxes(deg)[0].B; !approx(tip, math3d.V3(1.25, 0, 0)) {
		t.Errorf("degenerate-mesh axis tip = %v, want (1.25,0,0)", tip)
	}
}

func TestCameraViewMapsEyeToOrigin(t *testing.T) {
	c := Camera{
		Eye:    math3d.V3(0, 0, 5),
		Target: math3d.V3(0, 0, 0),
		Up:     math3d.V3(0, 1, 0),
		FOVY:   1, Near: 0.1, Far: 100,
	}
	got := c.View().MulVec4(math3d.V4(0, 0, 5, 1))
	if math.Abs(got.X) > 1e-9 || math.Abs(got.Y) > 1e-9 || math.Abs(got.Z) > 1e-9 {
		t.Errorf("eye -> %v, want origin", got)
	}
}
