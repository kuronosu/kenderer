package pipeline

import (
	"image/color"
	"testing"

	"github.com/kuronosu/kenderer/framebuffer"
	"github.com/kuronosu/kenderer/geometry"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/scene"
	"github.com/kuronosu/kenderer/shading"
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

func TestRenderCube(t *testing.T) {
	scn := testScene(geometry.NewCube(1.5))
	scn.Objects[0].Transform.Rotation = math3d.V3(0.5, 0.7, 0) // show three faces

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
