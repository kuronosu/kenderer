//go:build sdl

// Command viewer renders the kenderer cube in a live, resizable window with an
// orbit camera: drag to orbit, scroll to zoom, middle-drag (or shift+drag) to
// pan, Escape or the close button to quit.
//
// It is built only with the "sdl" tag:
//
//	go build -tags sdl ./cmd/viewer
//
// The default build and the GIF exporter (cmd/cube) stay dependency-free.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"time"

	"github.com/kuronosu/kenderer/camera"
	"github.com/kuronosu/kenderer/geometry"
	"github.com/kuronosu/kenderer/input"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/pipeline"
	"github.com/kuronosu/kenderer/platform"
	"github.com/kuronosu/kenderer/scene"
	"github.com/kuronosu/kenderer/shading"
)

// viewer adapts the renderer, scene and orbit camera to the platform.App loop.
type viewer struct {
	renderer *pipeline.Renderer
	scn      scene.Scene
	cam      camera.OrbitCamera
}

func (v *viewer) Update(_ time.Duration, in input.Frame) {
	v.cam.Update(in)
	v.cam.Apply(&v.scn.Camera)
}

func (v *viewer) Render() *image.RGBA { return v.renderer.Render(v.scn).Image() }

func (v *viewer) Resize(w, h int) { v.renderer.Resize(w, h) }

func main() {
	width := flag.Int("w", 800, "initial window width")
	height := flag.Int("h", 600, "initial window height")
	fps := flag.Int("fps", 60, "target frames per second")
	fovDeg := flag.Float64("fov", 50, "vertical field of view in degrees")
	flag.Parse()

	v := &viewer{
		renderer: pipeline.NewRenderer(pipeline.Options{
			Width:      *width,
			Height:     *height,
			Cull:       pipeline.CullBack,
			Background: color.RGBA{R: 18, G: 18, B: 24, A: 255},
		}),
		scn: scene.Scene{
			Camera: scene.Camera{Up: math3d.V3(0, 1, 0), FOVY: *fovDeg * math.Pi / 180, Near: 0.1, Far: 100},
			Objects: []scene.Object{{
				Mesh:      geometry.NewCube(2),
				Transform: scene.Transform{Rotation: math3d.QuatIdentity(), Scale: math3d.V3(1, 1, 1)},
				Material:  shading.Material{Albedo: math3d.V3(1, 1, 1)},
			}},
			Light:   shading.DirectionalLight{Direction: math3d.V3(-0.5, -1, -0.7).Normalize(), Color: math3d.V3(1, 1, 1), Intensity: 1},
			Ambient: 0.15,
		},
		cam: camera.NewOrbitCamera(math3d.V3(0, 0, 0), 6),
	}
	v.cam.Apply(&v.scn.Camera) // initial pose before the first frame

	cfg := platform.Config{Title: "kenderer viewer", Width: *width, Height: *height, FPS: *fps}
	if err := platform.Run(cfg, v); err != nil {
		fmt.Fprintln(os.Stderr, "viewer:", err)
		os.Exit(1)
	}
}
