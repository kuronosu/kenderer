package scene

import (
	"math"
	"testing"

	"github.com/kuronosu/kenderer/math3d"
)

func TestTransformMatrixOrder(t *testing.T) {
	// Scale by 2, rotate +90 deg about Y, then translate by (10,0,0).
	// (1,0,0) -> scale (2,0,0) -> RotateY(90) (0,0,-2) -> translate (10,0,-2).
	tr := Transform{
		Position: math3d.V3(10, 0, 0),
		Rotation: math3d.V3(0, math.Pi/2, 0),
		Scale:    math3d.V3(2, 2, 2),
	}
	got := tr.Matrix().MulVec4(math3d.V4(1, 0, 0, 1))
	want := math3d.V4(10, 0, -2, 1)
	if math.Abs(got.X-want.X) > 1e-9 || math.Abs(got.Y-want.Y) > 1e-9 || math.Abs(got.Z-want.Z) > 1e-9 {
		t.Errorf("Matrix·(1,0,0) = %v, want %v", got, want)
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
