# kenderer

A 3D **software** rendering pipeline in pure Go — no GPU, no third-party
dependencies in the core. It rasterizes triangles on the CPU through all the
classic stages and ships a demo that renders a rotating, flat-shaded cube to an
animated GIF.

## Quick start

```sh
# Render the demo (writes cube.gif in the current directory)
go run ./cmd/cube -w 480 -h 480 -fps 30 -dur 4s -out cube.gif

# Run the test suite
go test ./...
```

Flags: `-w`, `-h` (size), `-fps`, `-dur` (duration / one full turn), `-fov`
(vertical FOV, degrees), `-out` (GIF path).

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
| `pipeline`    | stage orchestration + frustum clipping                      |
| `present`     | `Presenter` interface + stdlib animated-GIF backend         |
| `cmd/cube`    | demo entry point                                            |

Dependencies form a DAG with `math3d` at the bottom and `cmd/cube` at the top;
nothing in the core imports a presentation backend.

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
