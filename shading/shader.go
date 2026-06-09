package shading

import (
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/texture"
)

// Shader maps a Fragment to a linear-RGB color whose components are nominally in
// [0, 1] (ToRGBA clamps anything outside that range). Implementing this
// interface is the extension point for new lighting models (Gouraud, Phong,
// textured, ...).
type Shader interface {
	Shade(f Fragment) math3d.Vec3
}

// Material describes a surface's reflectance. When AlbedoTex is non-nil the base
// reflectance is sampled from it (in linear RGB) at the fragment UV using Filter
// and Wrap; otherwise the constant Albedo is used. The sampled UV is already
// perspective-correct (see CombineFragment), so texturing needs no rasterizer
// change.
type Material struct {
	Albedo    math3d.Vec3      // linear-RGB base reflectance, used when AlbedoTex is nil
	AlbedoTex *texture.Texture // optional albedo map (nil = untextured)
	Filter    texture.Filter   // sampling filter for AlbedoTex
	Wrap      texture.Wrap     // wrap mode for AlbedoTex
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

// Shade implements Shader. In linear RGB the shaded color is
//
//	base * (ambient + lightColor * intensity * max(0, dot(N, L)))
//
// where base = albedo * Fragment.Color (componentwise), albedo is sampled from
// Material.AlbedoTex when present (else Material.Albedo), N is the normalized
// fragment normal and L points toward the light. The surface is thus modulated by
// the albedo/vertex color and the light color, not only by the light intensity.
//
// N is normalized here, which is a no-op for flat shading (constant face normal)
// but required for smooth shading, where the interpolated per-fragment normal is
// generally not unit length. sRGB encoding happens later, in ToRGBA.
func (s Lambert) Shade(f Fragment) math3d.Vec3 {
	n := f.Normal.Normalize()
	toLight := s.Light.Direction.Normalize().Neg()
	lambert := n.Dot(toLight)
	if lambert < 0 {
		lambert = 0
	}
	albedo := s.Material.Albedo
	if s.Material.AlbedoTex != nil {
		albedo = s.Material.AlbedoTex.Sample(f.UV.X, f.UV.Y, s.Material.Filter, s.Material.Wrap)
	}
	base := albedo.Mul(f.Color)
	ambient := base.Scale(s.Ambient)
	diffuse := base.Mul(s.Light.Color).Scale(s.Light.Intensity * lambert)
	return ambient.Add(diffuse)
}
