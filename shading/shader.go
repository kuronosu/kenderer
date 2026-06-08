package shading

import "github.com/kuronosu/kenderer/math3d"

// Shader maps a Fragment to a linear-RGB color whose components are nominally in
// [0, 1] (ToRGBA clamps anything outside that range). Implementing this
// interface is the extension point for new lighting models (Gouraud, Phong,
// textured, ...).
type Shader interface {
	Shade(f Fragment) math3d.Vec3
}

// Material describes a surface's reflectance.
type Material struct {
	Albedo math3d.Vec3 // linear-RGB base reflectance
}

// DirectionalLight is an infinitely distant light. Direction is the direction in
// which the light travels (world space), so the vector pointing toward the light
// is its negation.
type DirectionalLight struct {
	Direction math3d.Vec3
	Color     math3d.Vec3 // linear RGB
	Intensity float64
}

// Lambert is a diffuse shader: ambient term plus Lambertian (cosine) diffuse
// from a single directional light. With a constant per-face normal it produces
// flat shading; with interpolated normals it produces Phong-style smooth diffuse.
type Lambert struct {
	Light    DirectionalLight
	Ambient  float64
	Material Material
}

// Shade implements Shader.
func (s Lambert) Shade(f Fragment) math3d.Vec3 {
	n := f.Normal.Normalize()
	toLight := s.Light.Direction.Normalize().Neg()
	lambert := n.Dot(toLight)
	if lambert < 0 {
		lambert = 0
	}
	base := s.Material.Albedo.Mul(f.Color)
	ambient := base.Scale(s.Ambient)
	diffuse := base.Mul(s.Light.Color).Scale(s.Light.Intensity * lambert)
	return ambient.Add(diffuse)
}
