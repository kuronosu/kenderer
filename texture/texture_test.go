package texture

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"

	"github.com/kuronosu/kenderer/math3d"
)

func vecClose(a, b math3d.Vec3, eps float64) bool {
	return math.Abs(a.X-b.X) < eps && math.Abs(a.Y-b.Y) < eps && math.Abs(a.Z-b.Z) < eps
}

// tex2x2 is a 2x2 texture with a distinct color per texel, origin top-left:
// (0,0)=red (1,0)=green / (0,1)=blue (1,1)=white.
func tex2x2() *Texture {
	t := newTexture(2, 2, KindData)
	t.set(0, 0, math3d.V3(1, 0, 0))
	t.set(1, 0, math3d.V3(0, 1, 0))
	t.set(0, 1, math3d.V3(0, 0, 1))
	t.set(1, 1, math3d.V3(1, 1, 1))
	return t
}

func TestSampleNearestExactTexel(t *testing.T) {
	tx := tex2x2()
	cases := []struct {
		u, v float64
		want math3d.Vec3
	}{
		{0.25, 0.25, math3d.V3(1, 0, 0)}, // top-left
		{0.75, 0.25, math3d.V3(0, 1, 0)}, // top-right
		{0.25, 0.75, math3d.V3(0, 0, 1)}, // bottom-left
		{0.75, 0.75, math3d.V3(1, 1, 1)}, // bottom-right
	}
	for _, c := range cases {
		if got := tx.Sample(c.u, c.v, Nearest, Repeat); got != c.want {
			t.Errorf("Sample(%v,%v) = %v, want %v", c.u, c.v, got, c.want)
		}
	}
}

func TestTexelMatchesNearestSample(t *testing.T) {
	// Texel must expose exactly the stored texels — the same values a nearest
	// sample at the texel center returns — so a consumer re-uploading the image
	// (the GPU backend) reproduces it losslessly.
	tx := tex2x2()
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			got := tx.Texel(x, y)
			want := tx.Sample((float64(x)+0.5)/2, (float64(y)+0.5)/2, Nearest, Repeat)
			if got != want {
				t.Errorf("Texel(%d,%d) = %v, want %v", x, y, got, want)
			}
		}
	}
}

func TestSampleBilinearMidpointAverages(t *testing.T) {
	tx := tex2x2()
	// The exact center is equidistant from all four texels, so bilinear returns
	// their mean: (red+green+blue+white)/4 = (0.5, 0.5, 0.5).
	got := tx.Sample(0.5, 0.5, Bilinear, Repeat)
	if want := math3d.V3(0.5, 0.5, 0.5); !vecClose(got, want, 1e-12) {
		t.Errorf("bilinear center = %v, want %v", got, want)
	}
}

func TestSampleWrapVsClampAtBorder(t *testing.T) {
	tx := tex2x2()
	// At u=v=0 the bilinear footprint reaches texel index -1. Repeat wraps to the
	// opposite edge (averaging all four texels → grey); Clamp pins to the corner
	// texel (red). The two modes must therefore differ.
	rep := tx.Sample(0, 0, Bilinear, Repeat)
	clm := tx.Sample(0, 0, Bilinear, Clamp)
	if vecClose(rep, clm, 1e-9) {
		t.Errorf("repeat (%v) and clamp (%v) should differ at the border", rep, clm)
	}
	if want := math3d.V3(1, 0, 0); !vecClose(clm, want, 1e-12) {
		t.Errorf("clamp at corner = %v, want %v (corner texel)", clm, want)
	}
}

func TestLoadTextureSRGBToLinear(t *testing.T) {
	// A 1x1 opaque PNG of sRGB 188, which linearizes to ≈ 0.5.
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.NRGBA{R: 188, G: 188, B: 188, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	tx, err := LoadTexture(&buf, KindColor)
	if err != nil {
		t.Fatal(err)
	}
	if got := tx.at(0, 0, Clamp); math.Abs(got.X-0.5) > 0.02 {
		t.Errorf("sRGB 188 -> linear %v, want ~0.5", got.X)
	}

	// KindData keeps the raw normalized value (≈ 0.737), proving no linearization.
	buf.Reset()
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	dx, err := LoadTexture(&buf, KindData)
	if err != nil {
		t.Fatal(err)
	}
	if got := dx.at(0, 0, Clamp); math.Abs(got.X-188.0/255.0) > 0.01 {
		t.Errorf("KindData raw value = %v, want ~%v", got.X, 188.0/255.0)
	}
}
