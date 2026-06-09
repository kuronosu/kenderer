# kenderer

A 3D **software** rendering pipeline in pure Go — no GPU, no third-party
dependencies in the core. It rasterizes triangles on the CPU through all the
classic stages and ships a demo that renders a rotating, flat-shaded cube to an
animated GIF. An optional, build-tag-gated viewer (`-tags sdl`) adds a real-time
SDL3 window with an orbit camera; the default build and the GIF path stay
zero-dependency.

## Quick start

```sh
# Render the demo (writes cube.gif in the current directory)
go run ./cmd/cube -w 480 -h 480 -fps 30 -dur 4s -out cube.gif

# Run the test suite
go test ./...
```

Flags: `-w`, `-h` (size), `-fps`, `-dur` (duration / one full turn), `-fov`
(vertical FOV, degrees), `-out` (GIF path).

## Interactive viewer (optional, SDL3)

A real-time, resizable window with an orbit camera. It lives behind the `sdl`
build tag, so it is the **only** part that pulls a third-party dependency
(`github.com/Zyko0/go-sdl3`, a cgo-free purego binding); the default build and
the GIF exporter above stay zero-dependency.

```sh
# Build/run the viewer (the sdl tag is required)
go run -tags sdl ./cmd/viewer -w 800 -h 600 -fps 60 -fov 50 -stats
```

Controls: **drag** to orbit, **scroll** to zoom, **middle-drag** (or
**shift+drag**) to pan, **F1** to toggle the FPS overlay, **Escape** or the close
button to quit. The framebuffer tracks the window's pixel size, so resizing stays
crisp and undistorted (HiDPI included).

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

SDL3 itself does **not** need to be installed:

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
8. **Shading** — flat shading with a directional Lambert light.

## Packages

| Package       | Responsibility                                              |
|---------------|-------------------------------------------------------------|
| `math3d`      | `Vec2/3/4`, `Mat3/4`, transforms, projection, viewport       |
| `geometry`    | `Vertex`, `Mesh`, `NewCube`                                  |
| `framebuffer` | color + depth render target (display-agnostic)              |
| `shading`     | `Shader` interface, Lambert, perspective-correct combine, sRGB |
| `raster`      | triangle rasterization, coverage, depth test                |
| `scene`       | `Camera`, `Transform`, `Object`, `Scene`                    |
| `pipeline`    | stage orchestration + frustum clipping (+ `Resize`)         |
| `present`     | `Presenter` interface + stdlib animated-GIF backend         |
| `input`       | backend-agnostic real-time input state (no SDL)             |
| `camera`      | `OrbitCamera` (drag/zoom/pan) writing a `scene.Camera`      |
| `cmd/cube`    | GIF demo entry point                                        |
| `platform`    | SDL3 window + frame loop — build tag `sdl`                  |
| `cmd/viewer`  | interactive viewer entry point — build tag `sdl`            |

Dependencies form a DAG with `math3d` at the bottom. The `sdl`-tagged packages
(`platform`, `cmd/viewer`) are the only ones with a third-party dependency and
are excluded from the default build; nothing in the core imports SDL or a
presentation backend.

## Conventions (the non-obvious decisions)

- **Handedness:** right-handed, OpenGL-style. +X right, +Y up, camera looks down
  −Z.
- **Matrices:** row-major storage (`[4][4]float64`, `m[row][col]`),
  column-vector convention (`v' = M·v`), composed right-to-left
  (`MVP = P·V·M`).
- **Depth:** projection maps view z `[-near,-far] → NDC [-1,1]`; the viewport
  maps that to window z `[0,1]` (0 = near). The Z-buffer clears to `+Inf` and a
  fragment passes when `z < stored`. Window z is linear in screen space, so its
  barycentric interpolation is exact — no perspective correction for depth.
- **Perspective-correct attributes:** UV/normal/color/world-pos are interpolated
  with `1/w` (`shading.CombineFragment`), unlike depth.
- **Winding / culling:** CCW = front. After the Y-flipping viewport, front faces
  have negative screen-space signed area; the top-left fill rule keeps shared
  edges watertight.
- **Normals:** transformed by the inverse-transpose of the model's upper-left
  3×3. Flat shading uses a single world-space face normal computed once on the
  original (pre-clip) triangle.
- **Color:** all shading is done in linear RGB; `shading.ToRGBA` applies sRGB
  gamma once, just before quantization.

## Extending it

The architecture is built to grow without touching the core:

- **More meshes:** add constructors/loaders to `geometry`.
- **Textures, Gouraud, Phong:** implement new `shading.Shader`s; the varyings
  (UV, normal, world position) are already interpolated perspective-correctly.
- **New output formats:** add a `present.Presenter` (e.g. a PNG-sequence or
  raw-framebuffer writer) that consumes the same `FrameFunc`; the core never
  imports it.
