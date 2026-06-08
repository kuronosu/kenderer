package shading

import (
	"image/color"
	"math"
	"testing"

	"github.com/kuronosu/kenderer/math3d"
)

func TestLambertFacingAndAway(t *testing.T) {
	sh := Lambert{
		Light:    DirectionalLight{Direction: math3d.V3(0, 0, -1), Color: math3d.V3(1, 1, 1), Intensity: 1},
		Ambient:  0.2,
		Material: Material{Albedo: math3d.V3(1, 1, 1)},
	}
	front := sh.Shade(Fragment{Normal: math3d.V3(0, 0, 1), Color: math3d.V3(1, 1, 1)})
	back := sh.Shade(Fragment{Normal: math3d.V3(0, 0, -1), Color: math3d.V3(1, 1, 1)})

	if front.X <= back.X {
		t.Errorf("front (%v) should be brighter than back (%v)", front.X, back.X)
	}
	if math.Abs(back.X-0.2) > 1e-9 {
		t.Errorf("back face should be ambient only (0.2), got %v", back.X)
	}
	if want := 0.2 + 1.0; math.Abs(front.X-want) > 1e-9 {
		t.Errorf("front face = %v, want %v (ambient + diffuse)", front.X, want)
	}
}

func TestCombineFragmentPerspective(t *testing.T) {
	// A tilted triangle: vertices with distinct clip-space w, hence distinct invW.
	f0 := Fragment{UV: math3d.V2(0, 0)}
	f1 := Fragment{UV: math3d.V2(1, 0)}
	f2 := Fragment{UV: math3d.V2(0, 1)}
	invW0, invW1, invW2 := 1.0, 0.5, 0.25 // w = 1, 2, 4
	b0, b1, b2 := 1.0/3, 1.0/3, 1.0/3     // centroid

	got := CombineFragment(f0, f1, f2, b0, b1, b2, invW0, invW1, invW2)

	// Analytic perspective-correct value: sum(bi*invWi*UVi) / sum(bi*invWi).
	wantU, wantV := 2.0/7, 1.0/7
	if math.Abs(got.UV.X-wantU) > 1e-12 || math.Abs(got.UV.Y-wantV) > 1e-12 {
		t.Errorf("perspective-correct UV = %v, want (%v, %v)", got.UV, wantU, wantV)
	}

	// The result MUST differ from the affine (no 1/w) interpolation, so this
	// test fails if the perspective correction is ever removed.
	affineU := b0*f0.UV.X + b1*f1.UV.X + b2*f2.UV.X // = 1/3
	if math.Abs(got.UV.X-affineU) < 1e-3 {
		t.Errorf("perspective UV (%v) must differ from affine (%v)", got.UV.X, affineU)
	}
}

func TestLerpFragment(t *testing.T) {
	a := Fragment{WorldPos: math3d.V3(0, 0, 0), UV: math3d.V2(0, 0), Color: math3d.V3(0, 0, 0)}
	b := Fragment{WorldPos: math3d.V3(2, 4, 6), UV: math3d.V2(1, 1), Color: math3d.V3(1, 1, 1)}
	m := LerpFragment(a, b, 0.5)
	if m.WorldPos != math3d.V3(1, 2, 3) {
		t.Errorf("lerp worldpos = %v, want (1,2,3)", m.WorldPos)
	}
	if m.UV != math3d.V2(0.5, 0.5) {
		t.Errorf("lerp uv = %v, want (0.5,0.5)", m.UV)
	}
}

func TestToRGBA(t *testing.T) {
	if got := ToRGBA(math3d.V3(0, 0, 0)); got != (color.RGBA{R: 0, G: 0, B: 0, A: 255}) {
		t.Errorf("black -> %v", got)
	}
	if got := ToRGBA(math3d.V3(1, 1, 1)); got != (color.RGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Errorf("white -> %v", got)
	}
	// Out-of-range components clamp.
	if got := ToRGBA(math3d.V3(2, -1, 0.5)); got.R != 255 || got.G != 0 {
		t.Errorf("clamp failed: %v", got)
	}
	// sRGB encoding brightens midtones: linear 0.5 maps above the naive 0.5*255.
	if got := ToRGBA(math3d.V3(0.5, 0.5, 0.5)); got.R <= 127 {
		t.Errorf("sRGB(0.5) = %d, want > 127 (gamma brightening)", got.R)
	}
}
