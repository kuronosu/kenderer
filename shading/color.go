package shading

import (
	"image/color"
	"math"

	"github.com/kuronosu/kenderer/math3d"
)

// ToRGBA converts a linear-RGB color to an 8-bit, sRGB-encoded, opaque
// color.RGBA. Components are clamped to [0, 1] and gamma-encoded so the shaded
// (linear) result is displayed correctly.
func ToRGBA(c math3d.Vec3) color.RGBA {
	return color.RGBA{
		R: encodeSRGB(c.X),
		G: encodeSRGB(c.Y),
		B: encodeSRGB(c.Z),
		A: 255,
	}
}

// encodeSRGB maps a linear component in [0, 1] to an 8-bit sRGB value, clamping
// out-of-range input.
func encodeSRGB(linear float64) uint8 {
	switch {
	case linear <= 0:
		return 0
	case linear >= 1:
		return 255
	}
	var s float64
	if linear <= 0.0031308 {
		s = linear * 12.92
	} else {
		s = 1.055*math.Pow(linear, 1/2.4) - 0.055
	}
	return uint8(s*255 + 0.5)
}
