package raster

import (
	"image/color"
	"math"
	"testing"

	"github.com/kuronosu/kenderer/framebuffer"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/shading"
)

type constShader struct{ c math3d.Vec3 }

func (s constShader) Shade(shading.Fragment) math3d.Vec3 { return s.c }

func TestSignedArea(t *testing.T) {
	a, b, c := math3d.V2(0, 0), math3d.V2(1, 0), math3d.V2(0, 1)
	if got := SignedArea(a, b, c); math.Abs(got-1) > 1e-12 {
		t.Errorf("CCW area = %v, want 1", got)
	}
	if got := SignedArea(a, c, b); math.Abs(got+1) > 1e-12 {
		t.Errorf("CW area = %v, want -1", got)
	}
}

func TestBarycentric(t *testing.T) {
	a, b, c := math3d.V2(0, 0), math3d.V2(6, 0), math3d.V2(0, 6)

	b0, b1, b2 := Barycentric(a, b, c, math3d.V2(2, 2)) // centroid
	for _, w := range []float64{b0, b1, b2} {
		if math.Abs(w-1.0/3) > 1e-12 {
			t.Errorf("centroid weight = %v, want 1/3", w)
		}
	}

	b0, b1, b2 = Barycentric(a, b, c, a) // at vertex a -> (1,0,0)
	if math.Abs(b0-1) > 1e-12 || math.Abs(b1) > 1e-12 || math.Abs(b2) > 1e-12 {
		t.Errorf("vertex a weights = (%v,%v,%v), want (1,0,0)", b0, b1, b2)
	}

	b0, b1, b2 = Barycentric(a, b, c, math3d.V2(-1, -1)) // outside
	if b0 >= 0 && b1 >= 0 && b2 >= 0 {
		t.Errorf("outside point should have a negative weight: (%v,%v,%v)", b0, b1, b2)
	}
}

func TestDrawTriangleDepthTest(t *testing.T) {
	fb := framebuffer.New(4, 4)
	fb.Clear(color.RGBA{A: 255})

	tri := func(z float64) (Vertex, Vertex, Vertex) {
		mk := func(x, y float64) Vertex { return Vertex{Pos: math3d.V3(x, y, z), InvW: 1} }
		return mk(0, 0), mk(4, 0), mk(0, 4)
	}

	a, b, c := tri(0.9)
	DrawTriangle(fb, a, b, c, constShader{math3d.V3(1, 0, 0)}) // far, red
	a, b, c = tri(0.1)
	DrawTriangle(fb, a, b, c, constShader{math3d.V3(0, 1, 0)}) // near, green -> wins

	if got := fb.Color.RGBAAt(1, 1); got.G <= got.R {
		t.Errorf("near triangle should win at (1,1): %v", got)
	}
	if d := fb.DepthAt(1, 1); math.Abs(d-0.1) > 1e-9 {
		t.Errorf("depth at (1,1) = %v, want 0.1", d)
	}

	// A farther triangle must not overwrite the nearer pixel.
	a, b, c = tri(0.5)
	DrawTriangle(fb, a, b, c, constShader{math3d.V3(0, 0, 1)})
	if got := fb.Color.RGBAAt(1, 1); got.B > 0 {
		t.Errorf("farther triangle wrongly overwrote (1,1): %v", got)
	}
}

func TestDrawTriangleWatertightQuad(t *testing.T) {
	fb := framebuffer.New(8, 8)
	bg := color.RGBA{A: 255}
	fb.Clear(bg)

	white := constShader{math3d.V3(1, 1, 1)}
	q0 := Vertex{Pos: math3d.V3(1, 1, 0.5), InvW: 1}
	q1 := Vertex{Pos: math3d.V3(7, 1, 0.5), InvW: 1}
	q2 := Vertex{Pos: math3d.V3(7, 7, 0.5), InvW: 1}
	q3 := Vertex{Pos: math3d.V3(1, 7, 0.5), InvW: 1}

	// Two triangles sharing diagonal q0-q2 tile the quad with no gap nor overlap.
	DrawTriangle(fb, q0, q1, q2, white)
	DrawTriangle(fb, q0, q2, q3, white)

	for y := 2; y < 7; y++ {
		for x := 2; x < 7; x++ {
			if got := fb.Color.RGBAAt(x, y); got == bg {
				t.Errorf("gap at interior pixel (%d,%d)", x, y)
			}
		}
	}
}
