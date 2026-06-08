package geometry

import "github.com/kuronosu/kenderer/math3d"

// NewCube returns an axis-aligned cube of the given side length, centered at the
// origin. It is built as 24 vertices (four per face, so each face carries its
// own outward normal and is flat-shaded crisply) and 36 indices (two
// counter-clockwise triangles per face). Faces use the standard Rubik's Cube
// colors: +X red, -X orange, +Y white, -Y yellow, +Z green, -Z blue (opposite
// pairs red/orange, white/yellow, green/blue).
func NewCube(size float64) *Mesh {
	h := size / 2
	v := math3d.V3 // local alias to keep the face table compact

	type face struct {
		normal  math3d.Vec3
		color   math3d.Vec3
		corners [4]math3d.Vec3 // counter-clockwise as seen from outside
	}

	// Corners are listed CCW from outside, so the triangulation (0,1,2),(0,2,3)
	// produces outward-facing geometric normals matching the stored normal.
	faces := []face{
		{v(1, 0, 0), v(0.85, 0.05, 0.05), [4]math3d.Vec3{v(h, -h, h), v(h, -h, -h), v(h, h, -h), v(h, h, h)}},      // +X
		{v(-1, 0, 0), v(1.00, 0.30, 0.00), [4]math3d.Vec3{v(-h, -h, -h), v(-h, -h, h), v(-h, h, h), v(-h, h, -h)}}, // -X
		{v(0, 1, 0), v(1.00, 1.00, 1.00), [4]math3d.Vec3{v(-h, h, h), v(h, h, h), v(h, h, -h), v(-h, h, -h)}},      // +Y
		{v(0, -1, 0), v(1.00, 0.80, 0.00), [4]math3d.Vec3{v(-h, -h, -h), v(h, -h, -h), v(h, -h, h), v(-h, -h, h)}}, // -Y
		{v(0, 0, 1), v(0.00, 0.50, 0.12), [4]math3d.Vec3{v(-h, -h, h), v(h, -h, h), v(h, h, h), v(-h, h, h)}},      // +Z
		{v(0, 0, -1), v(0.00, 0.12, 0.70), [4]math3d.Vec3{v(h, -h, -h), v(-h, -h, -h), v(-h, h, -h), v(h, h, -h)}}, // -Z
	}

	uv := [4]math3d.Vec2{math3d.V2(0, 0), math3d.V2(1, 0), math3d.V2(1, 1), math3d.V2(0, 1)}

	m := &Mesh{
		Vertices: make([]Vertex, 0, 24),
		Indices:  make([]uint32, 0, 36),
	}
	for _, f := range faces {
		base := uint32(len(m.Vertices))
		for i, c := range f.corners {
			m.Vertices = append(m.Vertices, Vertex{
				Position: c,
				Normal:   f.normal,
				UV:       uv[i],
				Color:    f.color,
			})
		}
		m.Indices = append(m.Indices, base, base+1, base+2, base, base+2, base+3)
	}
	return m
}
