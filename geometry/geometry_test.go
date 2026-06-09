package geometry

import (
	"math"
	"testing"

	"github.com/kuronosu/kenderer/math3d"
)

func TestNewCubeCounts(t *testing.T) {
	m := NewCube(2)
	if got := len(m.Vertices); got != 24 {
		t.Errorf("vertices = %d, want 24", got)
	}
	if got := len(m.Indices); got != 36 {
		t.Errorf("indices = %d, want 36", got)
	}
	if got := m.NumTriangles(); got != 12 {
		t.Errorf("triangles = %d, want 12", got)
	}
}

func TestNewCubeNormalsUnit(t *testing.T) {
	m := NewCube(2)
	for i, v := range m.Vertices {
		if d := math.Abs(v.Normal.Length() - 1); d > 1e-9 {
			t.Errorf("vertex %d normal length = %v, want 1", i, v.Normal.Length())
		}
	}
}

func TestNewCubeWindingOutward(t *testing.T) {
	m := NewCube(2)
	// The geometric normal from the CCW winding must agree with the stored
	// outward vertex normal for every triangle.
	for i := 0; i < m.NumTriangles(); i++ {
		a, b, c := m.Triangle(i)
		geo := b.Position.Sub(a.Position).Cross(c.Position.Sub(a.Position)).Normalize()
		if dot := geo.Dot(a.Normal); dot < 0.999 {
			t.Errorf("triangle %d winds inward: geo·normal = %v", i, dot)
		}
	}
}

func TestMeshBounds(t *testing.T) {
	lo, hi := NewCube(2).Bounds() // side 2, centered → half-extent 1
	if lo != math3d.V3(-1, -1, -1) || hi != math3d.V3(1, 1, 1) {
		t.Errorf("cube bounds = (%v, %v), want (-1,-1,-1)..(1,1,1)", lo, hi)
	}

	if lo, hi := (&Mesh{}).Bounds(); lo != (math3d.Vec3{}) || hi != (math3d.Vec3{}) {
		t.Errorf("empty mesh bounds = (%v, %v), want zero box", lo, hi)
	}
}

func TestNewCubeExtent(t *testing.T) {
	m := NewCube(2) // half-extent 1
	for _, v := range m.Vertices {
		for _, c := range [3]float64{v.Position.X, v.Position.Y, v.Position.Z} {
			if math.Abs(c)-1 > 1e-9 {
				t.Errorf("position component %v lies outside half-extent 1", c)
			}
		}
	}
}
