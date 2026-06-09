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
	"path/filepath"
	"strings"
	"time"

	"github.com/kuronosu/kenderer/asset"
	assetgltf "github.com/kuronosu/kenderer/asset/gltf"
	"github.com/kuronosu/kenderer/camera"
	"github.com/kuronosu/kenderer/geometry"
	"github.com/kuronosu/kenderer/input"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/pipeline"
	"github.com/kuronosu/kenderer/platform"
	"github.com/kuronosu/kenderer/scene"
	"github.com/kuronosu/kenderer/shading"
	"github.com/kuronosu/kenderer/texture"
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
	modelPath := flag.String("model", "", "model to load (.obj, .gltf, .glb); empty = built-in cube")
	texPath := flag.String("texture", "", "albedo texture for an OBJ without a material (optional)")
	flag.Parse()

	fovy := *fovDeg * math.Pi / 180

	objects, orbit, near, far, err := buildScene(*modelPath, *texPath, fovy)
	if err != nil {
		fmt.Fprintln(os.Stderr, "viewer:", err)
		os.Exit(1)
	}

	v := &viewer{
		renderer: pipeline.NewRenderer(pipeline.Options{
			Width:      *width,
			Height:     *height,
			Cull:       pipeline.CullBack,
			Background: color.RGBA{R: 18, G: 18, B: 24, A: 255},
		}),
		scn: scene.Scene{
			Camera:  scene.Camera{Up: math3d.V3(0, 1, 0), FOVY: fovy, Near: near, Far: far},
			Objects: objects,
			Light:   shading.DirectionalLight{Direction: math3d.V3(-0.5, -1, -0.7).Normalize(), Color: math3d.V3(1, 1, 1), Intensity: 1},
			Ambient: 0.15,
		},
		cam: orbit,
	}
	v.cam.Apply(&v.scn.Camera) // initial pose before the first frame

	cfg := platform.Config{Title: "kenderer viewer", Width: *width, Height: *height, FPS: *fps}
	if err := platform.Run(cfg, v); err != nil {
		fmt.Fprintln(os.Stderr, "viewer:", err)
		os.Exit(1)
	}
}

// buildScene assembles the objects, orbit camera and depth range for the viewer.
// With no model path it returns the built-in cube and its original camera; with a
// model it loads the file and auto-frames the camera to the combined bounds.
func buildScene(modelPath, texPath string, fovy float64) ([]scene.Object, camera.OrbitCamera, float64, float64, error) {
	if modelPath == "" {
		cube := []scene.Object{{
			Mesh:      geometry.NewCube(2),
			Transform: scene.Transform{Rotation: math3d.QuatIdentity(), Scale: math3d.V3(1, 1, 1)},
			Material:  shading.Material{Albedo: math3d.V3(1, 1, 1)},
		}}
		return cube, camera.NewOrbitCamera(math3d.V3(0, 0, 0), 6), 0.1, 100, nil
	}

	objects, err := loadObjects(modelPath, texPath)
	if err != nil {
		return nil, camera.OrbitCamera{}, 0, 0, err
	}
	if len(objects) == 0 {
		return nil, camera.OrbitCamera{}, 0, 0, fmt.Errorf("model %q has no renderable meshes", modelPath)
	}
	orbit, near, far := frameObjects(objects, fovy)
	return objects, orbit, near, far, nil
}

// loadObjects loads the model file and mounts each Model as a scene.Object with a
// linear/textured material and smooth shading (imported meshes carry authored
// normals). An optional override texture supplies an albedo map for OBJ files
// without a material.
func loadObjects(modelPath, texPath string) ([]scene.Object, error) {
	models, err := loadModels(modelPath)
	if err != nil {
		return nil, err
	}

	var override *texture.Texture
	if texPath != "" {
		if override, err = texture.LoadTextureFile(texPath, texture.KindColor); err != nil {
			return nil, err
		}
	}

	objects := make([]scene.Object, 0, len(models))
	for _, m := range models {
		tex := m.AlbedoTex
		if tex == nil {
			tex = override
		}
		objects = append(objects, scene.Object{
			Mesh:      m.Mesh,
			Transform: scene.Transform{Rotation: math3d.QuatIdentity(), Scale: math3d.V3(1, 1, 1)},
			Material:  shading.Material{Albedo: m.BaseColor, AlbedoTex: tex, Filter: texture.Bilinear, Wrap: texture.Repeat},
			Smooth:    true,
		})
	}
	return objects, nil
}

// loadModels dispatches on the file extension: OBJ via the zero-dep loader, glTF
// and GLB via the qmuntal-backed loader.
func loadModels(path string) ([]*asset.Model, error) {
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".obj":
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()
		m, err := asset.LoadOBJ(f, filepath.Dir(path))
		if err != nil {
			return nil, err
		}
		return []*asset.Model{m}, nil
	case ".gltf", ".glb":
		return assetgltf.LoadGLTF(path)
	default:
		return nil, fmt.Errorf("unsupported model extension %q (want .obj, .gltf, .glb)", ext)
	}
}

// frameObjects centers the orbit camera on the combined bounding box and pulls it
// back far enough to fit the bounding sphere in the vertical field of view,
// adjusting the zoom limits and depth range to the model's scale.
func frameObjects(objects []scene.Object, fovy float64) (camera.OrbitCamera, float64, float64) {
	lo, hi := scene.Bounds(objects)
	center := lo.Add(hi).Scale(0.5)
	radius := hi.Sub(lo).Length() * 0.5
	if radius == 0 {
		radius = 1
	}

	dist := radius / math.Sin(fovy/2) * 1.2
	orbit := camera.NewOrbitCamera(center, dist)
	orbit.MinDistance = math.Max(1e-3, radius*0.05)
	orbit.MaxDistance = dist * 10

	near := math.Max(1e-3, radius*0.01)
	far := orbit.MaxDistance + radius
	return orbit, near, far
}
