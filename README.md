# kenderer

A 3D **software** rendering pipeline in pure Go — it rasterizes triangles on the
CPU through all the classic stages and ships a demo that renders a rotating,
flat-shaded cube to an animated GIF. It has since grown into a small interactive
3D tool: an optional, build-tag-gated viewer (`-tags sdl`) adds a real-time SDL3
window with an orbit camera, OBJ/glTF model loading, textures, and a second
**GPU** backend (SDL_GPU / Vulkan) held to the CPU rasterizer by parity tests.

The renderer core and the GIF path stay **zero third-party imports** (standard
library only). The only outside dependencies are confined to where they are
imported: SDL3 behind the `sdl` build tag (`platform`, `cmd/viewer`) and the
pure-Go glTF reader (`qmuntal/gltf`) in `asset/gltf` alone — so the default
`go build ./...` pulls glTF into the module graph, but the core and GIF packages
import nothing third-party.

## Quick start

```sh
# Render the demo (writes cube.gif in the current directory)
go run ./cmd/cube -w 480 -h 480 -fps 30 -dur 4s -out cube.gif

# Run the test suite
go test ./...
```

Flags: `-w`, `-h` (size), `-fps`, `-dur` (duration / one full turn), `-fov`
(vertical FOV, degrees), `-out` (GIF path), `-workers` (fill goroutines; 0 = auto
= GOMAXPROCS, 1 = serial), `-axes` (draw world + object axes: X red, Y green,
Z blue).

## Interactive viewer (optional, SDL3)

A real-time, resizable window with an orbit camera that can render the built-in
cube or a loaded model. It lives behind the `sdl` build tag, so SDL3 (a cgo-free
purego binding, `github.com/Zyko0/go-sdl3`) is pulled only here; the default
build and the GIF exporter stay free of it.

```sh
# Built-in cube
go run -tags sdl ./cmd/viewer -w 800 -h 600 -fps 60 -fov 50 -stats

# Load a model (OBJ, glTF or GLB); the camera auto-frames it
go run -tags sdl ./cmd/viewer -model model.obj

# Supply an albedo texture for an OBJ that has no material
go run -tags sdl ./cmd/viewer -model mesh.obj -texture albedo.png

# Render with the GPU backend (SDL_GPU/Vulkan) instead of the CPU rasterizer
go run -tags sdl ./cmd/viewer -backend gpu -model model.obj
```

Flags: `-model` (`.obj`/`.gltf`/`.glb`; empty = cube), `-texture` (albedo for an
OBJ without a material), `-backend` (`software` = CPU rasterizer, `gpu` =
SDL_GPU/Vulkan), `-stats`, `-axes` (draw axes at startup), `-fullscreen`,
`-workers` (software backend only).

Controls: **drag** to orbit, **scroll** to zoom, **middle-drag** (or
**shift+drag**) to pan, **F1** toggles the FPS/frame-time overlay, **F2** toggles
the world axes, **F3** toggles each object's local axes, **Escape** or the close
button quits. The framebuffer tracks the window's pixel size, so resizing stays
crisp and undistorted (HiDPI included).

### Stats overlay

The `-stats` overlay (on by default) shows two numbers, e.g. `60 FPS  3.0 ms`. The
**FPS** is wall-clock; the **ms** is the per-frame *work* time, so it reflects the
real rasterizer cost — the approximate uncapped throughput is `1000/ms`, even when
the FPS is pinned by something else.

In a window the FPS is usually pinned by the desktop compositor pacing `Present` to
the display's refresh (often a dynamic 40–60 Hz on laptops), not by `-fps` — so the
FPS reads the refresh while the **ms** stays the true measurement. To see raw
throughput in the FPS number too, run with `-fullscreen`: it bypasses the compositor.
The framebuffer then covers the whole screen, so the per-frame **ms** rises with the
larger pixel count.

The software backend draws the overlay onto the framebuffer; the GPU backend cannot
mix the 2D debug-text API with its swapchain, so it shows the same numbers in the
window title instead.

### GPU backend

`-backend gpu` renders through **SDL_GPU**, requesting SPIR-V so SDL selects its
Vulkan driver on every OS. It needs a Vulkan driver **at runtime** (building never
does — the GLSL shaders in `platform/shaders/src/` are compiled offline to the
committed, embedded `.spv` blobs). The GPU path mirrors the CPU rasterizer's
conventions exactly — same projection, winding, culling, and a single sRGB encode
at output in linear-working shaders — and is pinned to the CPU **oracle** by parity
tests (`go test -tags sdl ./platform/`, which skip when no device is present).

### SDL3 is not a system install

- **Embedded (recommended):** the viewer imports
  `github.com/Zyko0/go-sdl3/bin/binsdl`, which writes the bundled SDL3 library at
  runtime — `defer binsdl.Load().Unload()`.
- **From a path:** load a system/local SDL3 instead with
  `sdl.LoadLibrary(sdl.Path())` (needs `SDL3.dll` / `libSDL3.so` / `.dylib`
  reachable).

## Pipeline stages

`pipeline.Renderer.Render` runs, per object:

1. **Vertex transform** — model, view (`LookAt`) and perspective projection
   combined into MVP; positions go to clip space, normals via the normal matrix.
2. **Clipping** — Sutherland–Hodgman against all six clip-space frustum planes
   (`w ± x`, `w ± y`, `w ± z ≥ 0`); the result is re-triangulated as a fan.
3. **Perspective divide** — clip → NDC.
4. **Viewport** — NDC → screen (Y flipped, depth → `[0,1]`).
5. **Backface culling** — by screen-space signed area.
6. **Rasterization** — edge functions with a top-left fill rule
   (`raster.DrawTriangle`).
7. **Z-buffer** — perspective-correct (linear-in-screen) depth test.
8. **Shading** — directional Lambert in linear RGB, with an optional albedo
   texture; per object, flat (face normal) or smooth (interpolated normals →
   Phong). After the fill, a serial line pass draws `Scene.Lines` and each
   object's local axes (depth-tested, not depth-written).

## Packages

| Package       | Responsibility                                              |
|---------------|-------------------------------------------------------------|
| `math3d`      | `Vec2/3/4`, `Mat3/4`, transforms, quaternions, projection, viewport |
| `geometry`    | `Vertex`, `Mesh`, `NewCube`                                 |
| `framebuffer` | color + depth render target (display-agnostic)             |
| `texture`     | `Texture` (linear-RGB sampling: Nearest/Bilinear × Repeat/Clamp) |
| `shading`     | `Shader`, Lambert, `Material`, perspective-correct combine, sRGB |
| `raster`      | triangle + line rasterization, coverage, depth test         |
| `scene`       | `Camera`, `Transform`, `Object`, `Scene`, `Lines`/axes      |
| `pipeline`    | stage orchestration + frustum clipping (+ `Resize`)         |
| `present`     | `Presenter` interface + stdlib animated-GIF backend         |
| `asset`       | `Model`, zero-dep OBJ + MTL loader (`LoadOBJ`)              |
| `asset/gltf`  | glTF/GLB loader via `qmuntal/gltf` (`LoadGLTF`)            |
| `input`       | backend-agnostic real-time input state (no SDL)             |
| `camera`      | `OrbitCamera` (drag/zoom/pan) writing a `scene.Camera`      |
| `cmd/cube`    | GIF demo entry point                                        |
| `platform`    | SDL3 window + frame loop + `Backend` seam (software / gpu) — build tag `sdl` |
| `cmd/viewer`  | interactive viewer entry point — build tag `sdl`            |

Dependencies form a DAG with `math3d` at the bottom. The `sdl`-tagged packages
(`platform`, `cmd/viewer`) are the only ones that pull SDL3 and are excluded from
the default build; only `asset/gltf` pulls the glTF reader. Nothing in the core
imports SDL or a presentation backend. Rendering sits behind `platform.Backend`:
the CPU rasterizer is the reference/oracle, the SDL_GPU backend is held to it by
parity tests.

## Conventions (the non-obvious decisions)

- **Handedness:** right-handed, OpenGL-style. +X right, +Y up, camera looks down
  −Z.
- **Matrices:** row-major storage (`[4][4]float64`, `m[row][col]`),
  column-vector convention (`v' = M·v`), composed right-to-left
  (`MVP = P·V·M`).
- **Rotation:** `scene.Transform.Rotation` is a `math3d.Quat`
  (`QuatFromEuler == Rz·Ry·Rx`); the zero value is identity-safe.
- **Depth:** projection maps view z `[-near,-far] → NDC [-1,1]`; the viewport
  maps that to window z `[0,1]` (0 = near). The Z-buffer clears to `+Inf` and a
  fragment passes when `z < stored`. Window z is linear in screen space, so its
  barycentric interpolation is exact — no perspective correction for depth.
- **Perspective-correct attributes:** UV/normal/color/world-pos are interpolated
  with `1/w` (`shading.CombineFragment`), unlike depth.
- **Winding / culling:** CCW = front. After the Y-flipping viewport, front faces
  have negative screen-space signed area; the top-left fill rule keeps shared
  edges watertight.
- **Normals & shading mode:** normals are transformed by the inverse-transpose of
  the model's upper-left 3×3. Shading is per object (`Object.Smooth`): flat
  replaces the vertex normals with one world-space face normal computed on the
  original pre-clip triangle; smooth keeps the interpolated normals (Phong).
  Imported meshes load smooth; the procedural cube stays flat.
- **Color:** all shading is done in linear RGB; `shading.ToRGBA` applies the
  sRGB encode once, at output. Albedo textures are decoded sRGB→linear on load;
  glTF base-color factors and vertex colors are already linear.
- **GPU parity:** the SDL_GPU backend uses `math3d.PerspectiveZO` (z∈[0,1],
  provably equal to the CPU's window z) and lets SDL normalize NDC and do the
  sRGB encode in hardware, so it matches the CPU oracle pixel-for-pixel within
  tolerance (pinned by `platform` tests under `-tags sdl`).

## Extending it

The architecture is built to grow without touching the core:

- **More meshes:** add constructors to `geometry` or loaders to `asset`.
- **More shading:** implement new `shading.Shader`s; the varyings (UV, normal,
  world position) are already interpolated perspective-correctly.
- **New backends:** implement `platform.Backend`; the CPU rasterizer is the
  parity oracle a new backend is measured against.
- **New output formats:** add a `present.Presenter` (e.g. a PNG-sequence or
  raw-framebuffer writer) that consumes the same `FrameFunc`; the core never
  imports it.

Next on the roadmap (F5): a live scene graph, frustum culling, and multi-light
support.
