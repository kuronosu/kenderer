// Command cube renders a rotating, flat-shaded cube with the kenderer software
// pipeline and writes the animation as an animated GIF (standard library only).
//
// Example:
//
//	go run ./cmd/cube -w 480 -h 480 -fps 30 -dur 4s -out cube.gif
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"time"

	"github.com/kuronosu/kenderer/geometry"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/pipeline"
	"github.com/kuronosu/kenderer/present"
	"github.com/kuronosu/kenderer/scene"
	"github.com/kuronosu/kenderer/shading"
)

func main() {
	width := flag.Int("w", 480, "frame width in pixels")
	height := flag.Int("h", 480, "frame height in pixels")
	fps := flag.Int("fps", 30, "frames per second")
	duration := flag.Duration("dur", 4*time.Second, "animation duration (one full rotation)")
	out := flag.String("out", "cube.gif", "output GIF path")
	fovDeg := flag.Float64("fov", 50, "vertical field of view in degrees")
	flag.Parse()

	if err := run(*width, *height, *fps, *duration, *out, *fovDeg); err != nil {
		fmt.Fprintln(os.Stderr, "cube:", err)
		os.Exit(1)
	}
}

func run(width, height, fps int, duration time.Duration, out string, fovDeg float64) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("width and height must be positive (got %dx%d)", width, height)
	}

	cube := geometry.NewCube(2)

	camera := scene.Camera{
		Eye:    math3d.V3(2.6, 2.0, 3.4),
		Target: math3d.V3(0, 0, 0),
		Up:     math3d.V3(0, 1, 0),
		FOVY:   fovDeg * math.Pi / 180,
		Near:   0.1,
		Far:    100,
	}
	light := shading.DirectionalLight{
		Direction: math3d.V3(-0.5, -1, -0.7).Normalize(),
		Color:     math3d.V3(1, 1, 1),
		Intensity: 1,
	}

	renderer := pipeline.NewRenderer(pipeline.Options{
		Width:      width,
		Height:     height,
		Cull:       pipeline.CullBack,
		Background: color.RGBA{R: 18, G: 18, B: 24, A: 255},
	})

	// frame is deterministic in t: the cube makes exactly one turn about Y over
	// the whole duration, so the GIF loops seamlessly. The elevated camera
	// reveals the top face.
	frame := func(t time.Duration) image.Image {
		angle := 2 * math.Pi * t.Seconds() / duration.Seconds()
		scn := scene.Scene{
			Camera: camera,
			Objects: []scene.Object{{
				Mesh: cube,
				Transform: scene.Transform{
					Rotation: math3d.V3(0, angle, 0),
					Scale:    math3d.V3(1, 1, 1),
				},
				Material: shading.Material{Albedo: math3d.V3(1, 1, 1)},
			}},
			Light:   light,
			Ambient: 0.15,
		}
		return renderer.Render(scn).Image()
	}

	cfg := present.Config{Width: width, Height: height, FPS: fps, Duration: duration}
	if err := (present.GIF{Path: out, Loop: 0}).Run(frame, cfg); err != nil {
		return err
	}

	fmt.Printf("wrote %s (%dx%d, %d fps, %s)\n", out, width, height, fps, duration)
	return nil
}
