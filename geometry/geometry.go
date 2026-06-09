package geometry

import "github.com/kuronosu/kenderer/math3d"

// Vertex is a single mesh vertex carrying the attributes the pipeline
// interpolates across a triangle.
type Vertex struct {
	Position math3d.Vec3
	Normal   math3d.Vec3
	UV       math3d.Vec2
	Color    math3d.Vec3 // linear-RGB base color, modulates the shaded result
}

// Mesh is an indexed triangle mesh. Each consecutive triple in Indices selects
// three vertices forming one triangle, wound counter-clockwise as seen from the
// front.
type Mesh struct {
	Vertices []Vertex
	Indices  []uint32
}

// NumTriangles reports how many triangles the index buffer describes.
func (m *Mesh) NumTriangles() int { return len(m.Indices) / 3 }

// Bounds returns the axis-aligned bounding box of the mesh's vertex positions as
// its minimum and maximum corners. An empty mesh returns the zero box. Callers
// use it to frame a loaded model (center plus radius) for the camera.
func (m *Mesh) Bounds() (min, max math3d.Vec3) {
	if len(m.Vertices) == 0 {
		return math3d.Vec3{}, math3d.Vec3{}
	}
	min = m.Vertices[0].Position
	max = min
	for _, v := range m.Vertices[1:] {
		min = min.Min(v.Position)
		max = max.Max(v.Position)
	}
	return min, max
}

// Triangle returns the three vertices of triangle i, where 0 <= i < NumTriangles.
func (m *Mesh) Triangle(i int) (a, b, c Vertex) {
	j := i * 3
	return m.Vertices[m.Indices[j]], m.Vertices[m.Indices[j+1]], m.Vertices[m.Indices[j+2]]
}
