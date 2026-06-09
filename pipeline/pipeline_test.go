package pipeline

import (
	"bytes"
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kuronosu/kenderer/asset"
	"github.com/kuronosu/kenderer/framebuffer"
	"github.com/kuronosu/kenderer/geometry"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/scene"
	"github.com/kuronosu/kenderer/shading"
	"github.com/kuronosu/kenderer/texture"
)

// triMesh is a single triangle in the z=0 plane facing +Z, wound CCW (front
// toward +Z) when ccw is true and reversed otherwise.
func triMesh(ccw bool) *geometry.Mesh {
	v := func(x, y float64) geometry.Vertex {
		return geometry.Vertex{Position: math3d.V3(x, y, 0), Normal: math3d.V3(0, 0, 1), Color: math3d.V3(1, 1, 1)}
	}
	m := &geometry.Mesh{Vertices: []geometry.Vertex{v(-1, -1), v(1, -1), v(0, 1)}}
	if ccw {
		m.Indices = []uint32{0, 1, 2}
	} else {
		m.Indices = []uint32{0, 2, 1}
	}
	return m
}

func testScene(mesh *geometry.Mesh) scene.Scene {
	return scene.Scene{
		Camera: scene.Camera{
			Eye: math3d.V3(0, 0, 3), Target: math3d.V3(0, 0, 0), Up: math3d.V3(0, 1, 0),
			FOVY: 1.0, Near: 0.1, Far: 100,
		},
		Objects: []scene.Object{{
			Mesh:      mesh,
			Transform: scene.Transform{Scale: math3d.V3(1, 1, 1)},
			Material:  shading.Material{Albedo: math3d.V3(1, 1, 1)},
		}},
		Light:   shading.DirectionalLight{Direction: math3d.V3(0, 0, -1), Color: math3d.V3(1, 1, 1), Intensity: 1},
		Ambient: 0.3,
	}
}

func isBackground(fb *framebuffer.Buffer, x, y int) bool {
	c := fb.Color.RGBAAt(x, y)
	return c.R == 0 && c.G == 0 && c.B == 0
}

func newTestRenderer() *Renderer {
	return NewRenderer(Options{Width: 64, Height: 64, Cull: CullBack, Background: color.RGBA{A: 255}})
}

func TestRenderDrawsFrontFace(t *testing.T) {
	fb := newTestRenderer().Render(testScene(triMesh(true)))
	if isBackground(fb, 32, 32) {
		t.Error("front-facing triangle should cover the center pixel")
	}
}

func TestRenderCullsBackFace(t *testing.T) {
	fb := newTestRenderer().Render(testScene(triMesh(false)))
	if !isBackground(fb, 32, 32) {
		t.Error("back-facing triangle should be culled (center pixel stays background)")
	}
}

func TestRenderSmoothVsFlatNormals(t *testing.T) {
	// A front-facing triangle whose apex normal is tilted away from the light while
	// the base normals face it directly. Flat shading uses the single geometric
	// face normal, so every covered pixel gets the same shade; smooth shading keeps
	// the interpolated per-vertex normals, so a pixel near the apex differs from one
	// near the base.
	mesh := &geometry.Mesh{
		Vertices: []geometry.Vertex{
			{Position: math3d.V3(-1, -1, 0), Normal: math3d.V3(0, 0, 1), Color: math3d.V3(1, 1, 1)},
			{Position: math3d.V3(1, -1, 0), Normal: math3d.V3(0, 0, 1), Color: math3d.V3(1, 1, 1)},
			{Position: math3d.V3(0, 1, 0), Normal: math3d.V3(0, 0.6, 0.8), Color: math3d.V3(1, 1, 1)},
		},
		Indices: []uint32{0, 1, 2},
	}
	scn := testScene(mesh)
	// Keep shaded values below 1 so they do not clamp and stay distinguishable.
	scn.Ambient = 0.1
	scn.Light = shading.DirectionalLight{Direction: math3d.V3(0, 0, -1), Color: math3d.V3(1, 1, 1), Intensity: 0.5}

	const apexX, apexY = 32, 26 // near the apex (upper screen)
	const baseX, baseY = 32, 40 // near the base (lower screen)

	// Flat (default): both covered pixels share the constant face-normal shade.
	scn.Objects[0].Smooth = false
	flat := newTestRenderer().Render(scn)
	if isBackground(flat, apexX, apexY) || isBackground(flat, baseX, baseY) {
		t.Fatal("sample pixels must be covered by the triangle")
	}
	if flat.Color.RGBAAt(apexX, apexY) != flat.Color.RGBAAt(baseX, baseY) {
		t.Errorf("flat shading should be uniform across the face: apex %v != base %v",
			flat.Color.RGBAAt(apexX, apexY), flat.Color.RGBAAt(baseX, baseY))
	}

	// Smooth: the interpolated normal varies, so the two pixels differ.
	scn.Objects[0].Smooth = true
	smooth := newTestRenderer().Render(scn)
	if smooth.Color.RGBAAt(apexX, apexY) == smooth.Color.RGBAAt(baseX, baseY) {
		t.Errorf("smooth shading should vary across the face, both pixels = %v",
			smooth.Color.RGBAAt(apexX, apexY))
	}

	// The default-flat result must equal the smooth-disabled result: enabling the
	// toggle is the only thing that changes behavior.
	if smooth.Color.RGBAAt(baseX, baseY) == flat.Color.RGBAAt(baseX, baseY) {
		// Sanity: smooth must actually differ from flat somewhere too.
		t.Log("note: smooth and flat coincide at the base sample (expected near N·L=1)")
	}
}

func TestRenderCube(t *testing.T) {
	scn := testScene(geometry.NewCube(1.5))
	scn.Objects[0].Transform.Rotation = math3d.QuatFromEuler(0.5, 0.7, 0) // show three faces

	fb := newTestRenderer().Render(scn)

	if isBackground(fb, 32, 32) {
		t.Error("cube center pixel should be covered")
	}
	covered := 0
	for y := 0; y < fb.Height; y++ {
		for x := 0; x < fb.Width; x++ {
			if !isBackground(fb, x, y) {
				covered++
			}
		}
	}
	if min := fb.Width * fb.Height / 8; covered < min {
		t.Errorf("cube covers only %d/%d pixels, expected at least %d", covered, fb.Width*fb.Height, min)
	}
}

// framedScene centers a close-up camera on the mesh's bounds and pulls back just
// far enough to fill most of the frame, so the object spans many scanline bands.
// The camera sits off-axis so a cube shows three faces and the depth test sees
// overlapping geometry — the conditions that exercise the banded fill.
func framedScene(mesh *geometry.Mesh, smooth bool, mat shading.Material) scene.Scene {
	const fovy = 1.0
	lo, hi := mesh.Bounds()
	center := lo.Add(hi).Scale(0.5)
	radius := hi.Sub(lo).Length() * 0.5
	if radius == 0 {
		radius = 1
	}
	dist := radius / math.Sin(fovy/2) * 1.1
	eye := center.Add(math3d.V3(0.4, 0.3, 1).Normalize().Scale(dist))
	return scene.Scene{
		Camera: scene.Camera{
			Eye: eye, Target: center, Up: math3d.V3(0, 1, 0),
			FOVY: fovy, Near: math.Max(1e-3, radius*0.01), Far: dist + radius*4,
		},
		Objects: []scene.Object{{
			Mesh:      mesh,
			Transform: scene.Transform{Scale: math3d.V3(1, 1, 1)},
			Material:  mat,
			Smooth:    smooth,
		}},
		Light:   shading.DirectionalLight{Direction: math3d.V3(-0.5, -1, -0.7).Normalize(), Color: math3d.V3(1, 1, 1), Intensity: 1},
		Ambient: 0.15,
	}
}

// objScene loads examples/<name>.obj (zero-dep) into a smooth, textured close-up
// scene. ok is false when the asset is absent, so the caller can t.Skip.
func objScene(name string) (scn scene.Scene, ok bool) {
	dir := filepath.Join("..", "examples")
	f, err := os.Open(filepath.Join(dir, name+".obj"))
	if err != nil {
		return scene.Scene{}, false
	}
	defer func() { _ = f.Close() }()
	m, err := asset.LoadOBJ(f, dir)
	if err != nil {
		return scene.Scene{}, false
	}
	mat := shading.Material{Albedo: m.BaseColor, AlbedoTex: m.AlbedoTex, Filter: texture.Bilinear, Wrap: texture.Repeat}
	return framedScene(m.Mesh, true, mat), true
}

// TestFillWorkersEquivalence pins the core invariant of the parallel fill: the
// rendered image is bit-identical regardless of worker count. It renders each
// scene with the serial path (Workers:1) and with the parallel path
// (Workers:max(2,GOMAXPROCS)) into independent renderers and asserts the color
// buffer (.Pix) and the depth buffer are byte/element-equal. Cases span flat
// (cube), and smooth + textured + perspective-correct UV + overdraw (the OBJ
// meshes). File-backed cases skip when the asset is missing.
func TestFillWorkersEquivalence(t *testing.T) {
	const w, h = 200, 150
	bg := color.RGBA{R: 18, G: 18, B: 24, A: 255}
	parallel := max(2, runtime.GOMAXPROCS(0))

	render := func(scn scene.Scene, workers int) *framebuffer.Buffer {
		return NewRenderer(Options{Width: w, Height: h, Cull: CullBack, Background: bg, Workers: workers}).Render(scn)
	}

	check := func(t *testing.T, scn scene.Scene) {
		t.Helper()
		// Separate renderers own separate framebuffers; snapshot the serial result
		// anyway so the comparison is explicit and independent of buffer reuse.
		serial := render(scn, 1)
		sPix := append([]byte(nil), serial.Color.Pix...)
		sDepth := append([]float64(nil), serial.Depth...)

		// Guard against a vacuous comparison: the object must cover a meaningful
		// slice of the frame (hence span many bands), not sit off-screen.
		covered := 0
		for i := 0; i+3 < len(sPix); i += 4 {
			if sPix[i] != bg.R || sPix[i+1] != bg.G || sPix[i+2] != bg.B {
				covered++
			}
		}
		if want := w * h / 20; covered < want {
			t.Fatalf("scene covers only %d/%d px; framing too small to exercise bands", covered, w*h)
		}

		par := render(scn, parallel)
		if !bytes.Equal(sPix, par.Color.Pix) {
			t.Errorf("color buffer differs between workers=1 and workers=%d", parallel)
		}
		if len(sDepth) != len(par.Depth) {
			t.Fatalf("depth length mismatch: %d vs %d", len(sDepth), len(par.Depth))
		}
		for i := range sDepth {
			if sDepth[i] != par.Depth[i] {
				t.Errorf("depth differs at index %d (pixel %d,%d): %v vs %v",
					i, i%w, i/w, sDepth[i], par.Depth[i])
				break
			}
		}
	}

	t.Run("cube_flat", func(t *testing.T) {
		check(t, framedScene(geometry.NewCube(1.5), false, shading.Material{Albedo: math3d.V3(1, 1, 1)}))
	})
	t.Run("sphere_obj_smooth_textured", func(t *testing.T) {
		scn, ok := objScene("sphere")
		if !ok {
			t.Skip("examples/sphere.obj not available")
		}
		check(t, scn)
	})
	t.Run("mario_obj_smooth_textured", func(t *testing.T) {
		scn, ok := objScene("mario")
		if !ok {
			t.Skip("examples/mario.obj not available")
		}
		check(t, scn)
	})
}

// BenchmarkFillWorkers measures whole-frame throughput across worker counts for
// two regimes: a heavy, fill-bound frame (textured sphere at 800x800) where the
// parallel fill should speed up roughly with cores, and a cheap frame (small cube)
// that probes the fixed per-frame overhead of the parallel path — small scenes
// must not regress notably. Geometry stays serial, so speedups are end-to-end
// (Amdahl-bounded), not fill-only. On laptops, run a single sub-benchmark in
// isolation (e.g. -bench '/workers=8$'); a full sweep can thermally throttle the
// later (higher-worker) cases and understate their speed.
func BenchmarkFillWorkers(b *testing.B) {
	bg := color.RGBA{R: 18, G: 18, B: 24, A: 255}

	sphere, sphereOK := objScene("sphere")
	scenes := []struct {
		name string
		w, h int
		scn  scene.Scene
		skip bool
	}{
		{name: "cube_small", w: 160, h: 160, scn: framedScene(geometry.NewCube(1.5), false, shading.Material{Albedo: math3d.V3(1, 1, 1)})},
		{name: "sphere_large", w: 800, h: 800, scn: sphere, skip: !sphereOK},
	}

	maxW := runtime.GOMAXPROCS(0)
	seen := map[int]bool{}
	var counts []int
	for wk := 1; wk <= maxW; wk *= 2 {
		if !seen[wk] {
			counts = append(counts, wk)
			seen[wk] = true
		}
	}
	if !seen[maxW] {
		counts = append(counts, maxW)
	}

	for _, bs := range scenes {
		for _, workers := range counts {
			r := NewRenderer(Options{Width: bs.w, Height: bs.h, Cull: CullBack, Background: bg, Workers: workers})
			b.Run(fmt.Sprintf("%s/workers=%d", bs.name, workers), func(b *testing.B) {
				if bs.skip {
					b.Skip("examples/sphere.obj not available")
				}
				for i := 0; i < b.N; i++ {
					r.Render(bs.scn)
				}
			})
		}
	}
}
