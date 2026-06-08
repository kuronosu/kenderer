package pipeline

import (
	"testing"

	"github.com/kuronosu/kenderer/math3d"
)

func cv(x, y, z, w float64) clipVertex {
	return clipVertex{Pos: math3d.V4(x, y, z, w)}
}

func insideFrustum(p math3d.Vec4) bool {
	for _, dist := range clipPlanes {
		if dist(p) < -1e-9 {
			return false
		}
	}
	return true
}

func TestClipFullyInside(t *testing.T) {
	tris := clipTriangle(cv(-0.5, -0.5, 0, 1), cv(0.5, -0.5, 0, 1), cv(0, 0.5, 0, 1))
	if len(tris) != 1 {
		t.Fatalf("got %d triangles, want 1", len(tris))
	}
	want := [3]math3d.Vec4{math3d.V4(-0.5, -0.5, 0, 1), math3d.V4(0.5, -0.5, 0, 1), math3d.V4(0, 0.5, 0, 1)}
	for i := range want {
		if tris[0][i].Pos != want[i] {
			t.Errorf("vertex %d = %v, want %v (should be unchanged)", i, tris[0][i].Pos, want[i])
		}
	}
}

func TestClipFullyOutside(t *testing.T) {
	// Entirely to the right of the frustum: x > w for every vertex.
	tris := clipTriangle(cv(2, -0.5, 0, 1), cv(3, -0.5, 0, 1), cv(2.5, 0.5, 0, 1))
	if tris != nil {
		t.Errorf("expected nil (fully clipped), got %d triangles", len(tris))
	}
}

func TestClipNearStraddleExpandsToFan(t *testing.T) {
	// One vertex sits behind the near plane (w+z < 0), so the triangle becomes a
	// quad and is re-triangulated into two triangles. The introduced vertices
	// must land on the near plane (w+z == 0) and everything must end up inside.
	tris := clipTriangle(
		cv(-0.5, -0.5, 0, 1), // inside
		cv(0.5, -0.5, 0, 1),  // inside
		cv(0, 0.5, -2, 1),    // behind near
	)
	if len(tris) != 2 {
		t.Fatalf("got %d triangles, want 2", len(tris))
	}
	onNear := 0
	for _, tri := range tris {
		for _, v := range tri {
			if !insideFrustum(v.Pos) {
				t.Errorf("clipped vertex outside frustum: %v", v.Pos)
			}
			if d := v.Pos.W + v.Pos.Z; d > -1e-9 && d < 1e-9 {
				onNear++
			}
		}
	}
	if onNear < 2 {
		t.Errorf("expected the new vertices to lie on the near plane, found %d", onNear)
	}
}
