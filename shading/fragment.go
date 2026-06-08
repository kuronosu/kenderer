package shading

import "github.com/kuronosu/kenderer/math3d"

// Fragment holds the per-pixel surface attributes handed to a Shader. The values
// are the triangle's vertex attributes interpolated to the fragment's position.
type Fragment struct {
	WorldPos math3d.Vec3
	Normal   math3d.Vec3
	UV       math3d.Vec2
	Color    math3d.Vec3 // interpolated linear-RGB vertex color
}

// LerpFragment linearly interpolates every attribute between a (t=0) and b
// (t=1). It is used by the clipper: because clip-space coordinates are a linear
// map of eye space, a plain linear interpolation in the clip-edge parameter
// yields the true attribute value at the clip intersection (the perspective
// correction is applied later, during rasterization).
func LerpFragment(a, b Fragment, t float64) Fragment {
	return Fragment{
		WorldPos: a.WorldPos.Lerp(b.WorldPos, t),
		Normal:   a.Normal.Lerp(b.Normal, t),
		UV:       a.UV.Lerp(b.UV, t),
		Color:    a.Color.Lerp(b.Color, t),
	}
}

// CombineFragment blends the three vertex fragments of a triangle with
// perspective correction. b0, b1, b2 are the geometric (screen-space)
// barycentric weights, which sum to 1; invW0..invW2 are the reciprocals of the
// vertices' clip-space w. Each attribute is combined as
//
//	wi  = bi * invWi
//	attr = sum(wi * attr_i) / sum(wi)
//
// which is the perspective-correct value. (Depth must NOT be combined this way:
// window z is linear in screen space and uses the geometric weights directly.)
func CombineFragment(f0, f1, f2 Fragment, b0, b1, b2, invW0, invW1, invW2 float64) Fragment {
	w0 := b0 * invW0
	w1 := b1 * invW1
	w2 := b2 * invW2
	inv := 1 / (w0 + w1 + w2)
	w0 *= inv
	w1 *= inv
	w2 *= inv
	return Fragment{
		WorldPos: f0.WorldPos.Scale(w0).Add(f1.WorldPos.Scale(w1)).Add(f2.WorldPos.Scale(w2)),
		Normal:   f0.Normal.Scale(w0).Add(f1.Normal.Scale(w1)).Add(f2.Normal.Scale(w2)),
		UV:       f0.UV.Scale(w0).Add(f1.UV.Scale(w1)).Add(f2.UV.Scale(w2)),
		Color:    f0.Color.Scale(w0).Add(f1.Color.Scale(w1)).Add(f2.Color.Scale(w2)),
	}
}
