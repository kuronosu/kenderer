//go:build sdl

package platform

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/kuronosu/kenderer/geometry"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/pipeline"
	"github.com/kuronosu/kenderer/scene"
	"github.com/kuronosu/kenderer/shading"
	"github.com/kuronosu/kenderer/texture"
)

// Parity tolerances. The CPU rasterizer is the oracle; the GPU image must
// match it within a per-channel tolerance on almost every pixel. The slack
// absorbs float32-vs-float64 shading, 8-bit-weight hardware filtering and the
// half-step rounding they cause; the bad-pixel budget absorbs triangle edges,
// where the two rasterizers legitimately disagree about coverage by one pixel.
const (
	parityTolerance = 6     // max per-channel |difference| for a pixel to count as equal
	parityBadFrac   = 0.015 // fraction of pixels allowed beyond the tolerance
)

// TestGPUSoftwareParity renders one deterministic scene with both backends —
// the CPU pipeline.Renderer and the SDL_GPU gpuRenderer drawing into an
// offscreen sRGB target — and compares the images pixel by pixel. The scene
// covers every shading path: flat (face normals via derivatives on the GPU),
// smooth (interpolated normals), constant albedo and a bilinear repeated
// texture. It skips when no SPIR-V-capable GPU device is available.
func TestGPUSoftwareParity(t *testing.T) {
	const w, h = 256, 192

	runtime.LockOSThread()
	defer binsdl.Load().Unload()
	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		t.Skipf("SDL init failed (headless?): %v", err)
	}
	defer sdl.Quit()

	device, err := sdl.CreateGPUDevice(sdl.GPU_SHADERFORMAT_SPIRV, true, "")
	if err != nil {
		t.Skipf("no SPIR-V capable GPU device: %v", err)
	}
	defer device.Destroy()
	t.Logf("GPU driver: %s", device.Driver())

	scn := parityScene(t)

	cpuImg := pipeline.NewRenderer(pipeline.Options{
		Width: w, Height: h,
		Cull:       pipeline.CullBack,
		Background: color.RGBA{R: bgR, G: bgG, B: bgB, A: bgA},
	}).Render(scn).Image()

	gpuPix, err := renderGPUOffscreen(device, w, h, &scn)
	if err != nil {
		t.Fatalf("GPU render: %v", err)
	}

	comparePixels(t, w, h, cpuImg.Pix, gpuPix)
}

// TestGPULines exercises the GPU line pass: with world axes in Scene.Lines and
// the object-axes toggle on, the render must contain saturated red, green and
// blue axis pixels (unlit linear 0/1 colors encode to exact 255s, which no
// shaded surface reaches); with both toggles off the same scene must render
// byte-identically to one that never had lines — the feature is inert, like
// the CPU pipeline's.
func TestGPULines(t *testing.T) {
	const w, h = 256, 192

	runtime.LockOSThread()
	defer binsdl.Load().Unload()
	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		t.Skipf("SDL init failed (headless?): %v", err)
	}
	defer sdl.Quit()
	device, err := sdl.CreateGPUDevice(sdl.GPU_SHADERFORMAT_SPIRV, true, "")
	if err != nil {
		t.Skipf("no SPIR-V capable GPU device: %v", err)
	}
	defer device.Destroy()

	render := func(lines bool) []byte {
		t.Helper()
		scn := parityScene(t)
		r, err := newGPURenderer(device, sdl.GPU_TEXTUREFORMAT_R8G8B8A8_UNORM_SRGB)
		if err != nil {
			t.Fatalf("create renderer: %v", err)
		}
		if lines {
			scn.Lines = scene.WorldAxes()
			r.objectAxes = true
		}
		pix, err := renderGPUWith(device, r, w, h, &scn)
		if err != nil {
			t.Fatalf("GPU render: %v", err)
		}
		return pix
	}

	countSaturated := func(pix []byte) (red, green, blue int) {
		for i := 0; i < len(pix); i += 4 {
			r, g, b := pix[i], pix[i+1], pix[i+2]
			switch {
			case r >= 250 && g < 10 && b < 10:
				red++
			case g >= 250 && r < 10 && b < 10:
				green++
			case b >= 250 && r < 10 && g < 10:
				blue++
			}
		}
		return
	}

	withLines := render(true)
	red, green, blue := countSaturated(withLines)
	t.Logf("axis pixels: %d red, %d green, %d blue", red, green, blue)
	// The world axes alone cross most of the frame; tens of pixels per color is
	// a conservative floor that still catches a missing/mistransformed pass.
	if red < 20 || green < 20 || blue < 20 {
		dir := dumpParityImages(t, w, h, withLines, withLines)
		t.Errorf("GPU line pass missing axis pixels (r=%d g=%d b=%d); image dumped to %s", red, green, blue, dir)
	}

	withoutLines := render(false)
	if r0, g0, b0 := countSaturated(withoutLines); r0+g0+b0 != 0 {
		t.Errorf("toggles off must draw no axis pixels, found r=%d g=%d b=%d", r0, g0, b0)
	}
	if !bytes.Equal(withoutLines, renderBaseline(t, device, w, h)) {
		t.Error("toggles off must render byte-identically to a scene without lines")
	}
}

// renderBaseline renders the plain parity scene (no lines) on a fresh renderer.
func renderBaseline(t *testing.T, device *sdl.GPUDevice, w, h int) []byte {
	t.Helper()
	scn := parityScene(t)
	r, err := newGPURenderer(device, sdl.GPU_TEXTUREFORMAT_R8G8B8A8_UNORM_SRGB)
	if err != nil {
		t.Fatalf("create renderer: %v", err)
	}
	pix, err := renderGPUWith(device, r, w, h, &scn)
	if err != nil {
		t.Fatalf("GPU render: %v", err)
	}
	return pix
}

// parityScene mirrors the golden-test scene: a flat white cube, a smooth tinted
// cube and a bilinear-textured quad under one directional light.
func parityScene(t *testing.T) scene.Scene {
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
				Mesh: parityQuad(),
				Transform: scene.Transform{
					Position: math3d.V3(-1.7, -0.1, 0.9),
					Rotation: math3d.QuatFromAxisAngle(math3d.V3(0, 1, 0), 0.7),
					Scale:    math3d.V3(1, 1, 1),
				},
				Material: shading.Material{
					Albedo:    math3d.V3(1, 1, 1),
					AlbedoTex: parityTexture(t),
					Filter:    texture.Bilinear,
					Wrap:      texture.Repeat,
				},
			},
		},
		Light:   shading.DirectionalLight{Direction: math3d.V3(-0.5, -1, -0.7).Normalize(), Color: math3d.V3(1, 1, 1), Intensity: 1},
		Ambient: 0.15,
	}
}

// parityQuad is a unit quad facing +Z with UVs spanning [0,1]^2.
func parityQuad() *geometry.Mesh {
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

// parityTexture is a 4x4 two-color checker loaded through the public PNG path,
// so the GPU's sRGB-decode-and-filter is held against the CPU's decode-on-load.
func parityTexture(t *testing.T) *texture.Texture {
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

// renderGPUOffscreen draws the scene with a fresh gpuRenderer into an
// offscreen sRGB color target and downloads the resulting RGBA bytes.
func renderGPUOffscreen(device *sdl.GPUDevice, w, h int, s *scene.Scene) ([]byte, error) {
	r, err := newGPURenderer(device, sdl.GPU_TEXTUREFORMAT_R8G8B8A8_UNORM_SRGB)
	if err != nil {
		return nil, err
	}
	return renderGPUWith(device, r, w, h, s)
}

// renderGPUWith renders one frame with the given renderer offscreen, downloads
// the RGBA bytes and destroys the renderer.
func renderGPUWith(device *sdl.GPUDevice, r *gpuRenderer, w, h int, s *scene.Scene) ([]byte, error) {
	defer func() {
		_ = device.WaitForIdle()
		r.destroy()
	}()

	target, err := device.CreateTexture(&sdl.GPUTextureCreateInfo{
		Type:              sdl.GPU_TEXTURETYPE_2D,
		Format:            sdl.GPU_TEXTUREFORMAT_R8G8B8A8_UNORM_SRGB,
		Usage:             sdl.GPU_TEXTUREUSAGE_COLOR_TARGET,
		Width:             uint32(w),
		Height:            uint32(h),
		LayerCountOrDepth: 1,
		NumLevels:         1,
	})
	if err != nil {
		return nil, fmt.Errorf("create target: %w", err)
	}
	defer device.ReleaseTexture(target)

	size := uint32(w * h * 4)
	download, err := device.CreateTransferBuffer(&sdl.GPUTransferBufferCreateInfo{
		Usage: sdl.GPU_TRANSFERBUFFERUSAGE_DOWNLOAD,
		Size:  size,
	})
	if err != nil {
		return nil, fmt.Errorf("create download buffer: %w", err)
	}
	defer device.ReleaseTransferBuffer(download)

	cmdbuf, err := device.AcquireCommandBuffer()
	if err != nil {
		return nil, fmt.Errorf("acquire command buffer: %w", err)
	}
	if err := r.renderScene(cmdbuf, target, w, h, s); err != nil {
		return nil, err
	}
	pass := cmdbuf.BeginCopyPass()
	pass.DownloadFromGPUTexture(
		&sdl.GPUTextureRegion{Texture: target, W: uint32(w), H: uint32(h), D: 1},
		&sdl.GPUTextureTransferInfo{TransferBuffer: download},
	)
	pass.End()

	fence, err := cmdbuf.SubmitAndAcquireFence()
	if err != nil {
		return nil, fmt.Errorf("submit: %w", err)
	}
	if err := device.WaitForFences(true, []*sdl.GPUFence{fence}); err != nil {
		device.ReleaseFence(fence)
		return nil, fmt.Errorf("wait for fence: %w", err)
	}
	device.ReleaseFence(fence)

	ptr, err := device.MapTransferBuffer(download, false)
	if err != nil {
		return nil, fmt.Errorf("map download buffer: %w", err)
	}
	pix := readMapped(ptr, int(size))
	device.UnmapTransferBuffer(download)
	return pix, nil
}

// comparePixels asserts the GPU image matches the CPU oracle within the parity
// tolerances, dumps both images for inspection when it does not, and guards
// against a vacuously empty render.
func comparePixels(t *testing.T, w, h int, cpu, gpu []byte) {
	t.Helper()
	if len(cpu) != len(gpu) {
		t.Fatalf("pixel buffer sizes differ: cpu %d, gpu %d", len(cpu), len(gpu))
	}

	covered, bad, maxDiff := 0, 0, 0
	for i := 0; i < len(cpu); i += 4 {
		if cpu[i] != bgR || cpu[i+1] != bgG || cpu[i+2] != bgB {
			covered++
		}
		pixelBad := false
		for c := 0; c < 3; c++ {
			d := int(cpu[i+c]) - int(gpu[i+c])
			if d < 0 {
				d = -d
			}
			if d > maxDiff {
				maxDiff = d
			}
			if d > parityTolerance {
				pixelBad = true
			}
		}
		if pixelBad {
			bad++
		}
	}

	total := w * h
	if covered < total/10 {
		t.Fatalf("CPU render covers only %d/%d pixels; scene framing is broken", covered, total)
	}
	badFrac := float64(bad) / float64(total)
	t.Logf("parity: %d/%d pixels beyond tolerance %d (%.3f%%), max channel diff %d",
		bad, total, parityTolerance, badFrac*100, maxDiff)
	if badFrac > parityBadFrac {
		dir := dumpParityImages(t, w, h, cpu, gpu)
		t.Errorf("GPU image diverges from the CPU oracle: %.3f%% pixels beyond tolerance (budget %.3f%%); images dumped to %s",
			badFrac*100, parityBadFrac*100, dir)
	}
}

// dumpParityImages writes cpu.png and gpu.png to a temp dir for side-by-side
// inspection and returns the dir.
func dumpParityImages(t *testing.T, w, h int, cpu, gpu []byte) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "kenderer-parity-")
	if err != nil {
		t.Logf("dump: %v", err)
		return ""
	}
	for name, pix := range map[string][]byte{"cpu.png": cpu, "gpu.png": gpu} {
		img := &image.RGBA{Pix: pix, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Logf("dump %s: %v", name, err)
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o644); err != nil {
			t.Logf("dump %s: %v", name, err)
		}
	}
	return dir
}
