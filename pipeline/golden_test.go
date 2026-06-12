package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"

	"github.com/kuronosu/kenderer/geometry"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/scene"
	"github.com/kuronosu/kenderer/shading"
	"github.com/kuronosu/kenderer/texture"
)

// goldenHash pins the exact bytes the renderer produces for goldenScene. The CPU
// rasterizer is the reference oracle for the GPU backend (F4), so refactors along
// that path must not move a single output byte; this test turns "byte-identical"
// from a manual diff into a regression check. If an intentional rendering change
// lands, re-record the constant from the failure message.
const goldenHash = "780c96d8a4a00eb2d82f4045de662438f32038a9a6ebda2c91e97e6d48c69bd5"

// goldenScene exercises every core drawing path in one deterministic 64x64 frame:
// flat and smooth triangles, a perspective-correct textured quad (bilinear,
// repeat), frustum-clipped world axes and per-object axes over the depth buffer.
func goldenScene(t *testing.T) scene.Scene {
	t.Helper()
	return scene.Scene{
		Camera: scene.Camera{
			Eye: math3d.V3(2.6, 2.0, 3.4), Target: math3d.V3(0, 0, 0), Up: math3d.V3(0, 1, 0),
			FOVY: 50 * math.Pi / 180, Near: 0.1, Far: 100,
		},
		Objects: []scene.Object{
			{
				Mesh: geometry.NewCube(2),
				Transform: scene.Transform{
					Rotation: math3d.QuatFromAxisAngle(math3d.V3(1, 0, 0), 0.15).
						Mul(math3d.QuatFromAxisAngle(math3d.V3(0, 1, 0), 0.6)),
					Scale: math3d.V3(1, 1, 1),
				},
				Material: shading.Material{Albedo: math3d.V3(1, 1, 1)},
			},
			{
				Mesh: geometry.NewCube(1),
				Transform: scene.Transform{
					Position: math3d.V3(1.6, 0.9, -0.8),
					Rotation: math3d.QuatFromEuler(0.3, 0.4, 0.1),
					Scale:    math3d.V3(1, 1, 1),
				},
				Material: shading.Material{Albedo: math3d.V3(0.9, 0.4, 0.3)},
				Smooth:   true,
			},
			{
				Mesh: quadMesh(),
				Transform: scene.Transform{
					Position: math3d.V3(-1.7, -0.1, 0.9),
					Rotation: math3d.QuatFromAxisAngle(math3d.V3(0, 1, 0), 0.7),
					Scale:    math3d.V3(1, 1, 1),
				},
				Material: shading.Material{
					Albedo:    math3d.V3(1, 1, 1),
					AlbedoTex: checkerTexture(t),
					Filter:    texture.Bilinear,
					Wrap:      texture.Repeat,
				},
			},
		},
		Light:   shading.DirectionalLight{Direction: math3d.V3(-0.5, -1, -0.7).Normalize(), Color: math3d.V3(1, 1, 1), Intensity: 1},
		Ambient: 0.15,
		Lines:   scene.WorldAxes(),
	}
}

// quadMesh is a unit quad in the z=0 plane facing +Z (CCW front) with UVs
// spanning [0,1]^2, the minimal mesh that interpolates texture coordinates.
func quadMesh() *geometry.Mesh {
	v := func(x, y, u, vv float64) geometry.Vertex {
		return geometry.Vertex{
			Position: math3d.V3(x, y, 0),
			Normal:   math3d.V3(0, 0, 1),
			UV:       math3d.V2(u, vv),
			Color:    math3d.V3(1, 1, 1),
		}
	}
	return &geometry.Mesh{
		Vertices: []geometry.Vertex{v(-1, -1, 0, 1), v(1, -1, 1, 1), v(1, 1, 1, 0), v(-1, 1, 0, 0)},
		Indices:  []uint32{0, 1, 2, 0, 2, 3},
	}
}

// checkerTexture builds a 4x4 two-color checker through the public PNG loading
// path, so the golden also pins the sRGB decode-on-load conversion.
func checkerTexture(t *testing.T) *texture.Texture {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	a := color.RGBA{R: 230, G: 60, B: 40, A: 255}
	b := color.RGBA{R: 40, G: 80, B: 220, A: 255}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if (x+y)%2 == 0 {
				img.SetRGBA(x, y, a)
			} else {
				img.SetRGBA(x, y, b)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode checker: %v", err)
	}
	tex, err := texture.LoadTexture(&buf, texture.KindColor)
	if err != nil {
		t.Fatalf("load checker: %v", err)
	}
	return tex
}

func TestRenderGolden(t *testing.T) {
	r := NewRenderer(Options{
		Width: 64, Height: 64,
		Cull:       CullBack,
		Background: color.RGBA{R: 18, G: 18, B: 24, A: 255},
	})
	r.SetObjectAxes(true)

	img := r.Render(goldenScene(t)).Image()
	sum := sha256.Sum256(img.Pix)
	if got := hex.EncodeToString(sum[:]); got != goldenHash {
		t.Errorf("rendered bytes changed:\n got  %s\n want %s\nif intentional, re-record goldenHash", got, goldenHash)
	}
}
