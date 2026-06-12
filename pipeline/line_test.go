package pipeline

import (
	"bytes"
	"image/color"
	"math"
	"runtime"
	"testing"

	"github.com/kuronosu/kenderer/framebuffer"
	"github.com/kuronosu/kenderer/geometry"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/scene"
	"github.com/kuronosu/kenderer/shading"
)

// TestClipSegmentNearPlane pins the line-clipper's critical property: a segment
// with one endpoint behind the camera (negative clip-space w, crossing the near
// plane) is trimmed in clip space so both surviving endpoints have w > 0. That is
// what lets the perspective divide proceed without dividing by w ≈ 0 — the classic
// line-clipping bug. a is inside the frustum; b is behind the camera.
func TestClipSegmentNearPlane(t *testing.T) {
	a := math3d.V4(0, 0, 0, 1)   // origin, well inside the frustum
	b := math3d.V4(0, 0, -3, -1) // behind the camera: w < 0, outside the near plane

	ca, cb, ok := clipSegment(a, b)
	if !ok {
		t.Fatal("segment crossing the near plane should be partially visible")
	}
	for _, p := range []math3d.Vec4{ca, cb} {
		for _, c := range []float64{p.X, p.Y, p.Z, p.W} {
			if math.IsNaN(c) || math.IsInf(c, 0) {
				t.Fatalf("clipped endpoint has non-finite component: %+v", p)
			}
		}
		if p.W <= 0 {
			t.Errorf("clipped endpoint must have w > 0 for a safe divide, got w = %v (%+v)", p.W, p)
		}
	}
	// The endpoint that started inside is untouched; the trimmed one lands on the
	// near plane (w + z == 0 in this clip convention).
	if ca != a {
		t.Errorf("inside endpoint should be unchanged, got %+v want %+v", ca, a)
	}
	if d := cb.W + cb.Z; math.Abs(d) > 1e-9 {
		t.Errorf("trimmed endpoint should lie on the near plane (w+z=0), got %v (%+v)", d, cb)
	}
}

// TestClipSegmentWhollyOutside checks a segment entirely on the outside of a plane
// is rejected (ok=false), so nothing is drawn for off-screen lines.
func TestClipSegmentWhollyOutside(t *testing.T) {
	a := math3d.V4(0, 0, -2, 1) // w+z = -1 < 0: behind the near plane
	b := math3d.V4(0, 0, -3, 1) // w+z = -2 < 0: also behind it
	if _, _, ok := clipSegment(a, b); ok {
		t.Error("a segment wholly outside the near plane should be rejected")
	}
}

// axesTestScene frames a flat cube close-up (reusing framedScene) and enables the
// world axes via Scene.Lines. Object axes are enabled on the renderer by the caller.
func axesTestScene() scene.Scene {
	scn := framedScene(geometry.NewCube(1.5), false, shading.Material{Albedo: math3d.V3(1, 1, 1)})
	scn.Lines = scene.WorldAxes()
	return scn
}

// TestAxesAdditiveAndDepthInert pins two invariants of the line pass: drawing the
// axes changes the color buffer (the feature is visible) but leaves the depth buffer
// byte-for-byte identical to the axes-off render (lines test depth, never write it).
// The depth equality also shows the triangle path is untouched by the line pass.
func TestAxesAdditiveAndDepthInert(t *testing.T) {
	const w, h = 200, 150
	bg := color.RGBA{R: 18, G: 18, B: 24, A: 255}
	scn := axesTestScene()

	off := NewRenderer(Options{Width: w, Height: h, Cull: CullBack, Background: bg}).Render(
		framedScene(geometry.NewCube(1.5), false, shading.Material{Albedo: math3d.V3(1, 1, 1)}))
	offPix := append([]byte(nil), off.Color.Pix...)
	offDepth := append([]float64(nil), off.Depth...)

	rOn := NewRenderer(Options{Width: w, Height: h, Cull: CullBack, Background: bg})
	rOn.SetObjectAxes(true)
	on := rOn.Render(scn)

	if len(offDepth) != len(on.Depth) {
		t.Fatalf("depth length mismatch: %d vs %d", len(offDepth), len(on.Depth))
	}
	for i := range offDepth {
		if offDepth[i] != on.Depth[i] {
			t.Fatalf("axes ON changed depth at index %d (pixel %d,%d): %v vs %v — lines must not write z",
				i, i%w, i/w, offDepth[i], on.Depth[i])
		}
	}
	if bytes.Equal(offPix, on.Color.Pix) {
		t.Error("axes ON should change the color buffer (the axes are not visible)")
	}
}

// TestAxesFillWorkersEquivalence extends the parallel-fill invariant to the line
// pass: with the axes on, the rendered color and depth are still bit-identical
// between the serial (workers=1) and parallel paths. The line pass runs serially
// after the fill barrier, so it is deterministic regardless of worker count.
func TestAxesFillWorkersEquivalence(t *testing.T) {
	const w, h = 200, 150
	bg := color.RGBA{R: 18, G: 18, B: 24, A: 255}
	scn := axesTestScene()

	render := func(workers int) *framebuffer.Buffer {
		r := NewRenderer(Options{Width: w, Height: h, Cull: CullBack, Background: bg, Workers: workers})
		r.SetObjectAxes(true)
		return r.Render(scn)
	}

	serial := render(1)
	sPix := append([]byte(nil), serial.Color.Pix...)
	sDepth := append([]float64(nil), serial.Depth...)

	par := render(max(2, runtime.GOMAXPROCS(0)))
	if !bytes.Equal(sPix, par.Color.Pix) {
		t.Error("color buffer differs between workers=1 and parallel with axes on")
	}
	for i := range sDepth {
		if sDepth[i] != par.Depth[i] {
			t.Errorf("depth differs at index %d (pixel %d,%d): %v vs %v", i, i%w, i/w, sDepth[i], par.Depth[i])
			break
		}
	}
}
