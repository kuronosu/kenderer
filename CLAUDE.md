# CLAUDE.md

Guidance for working in this repo. Per-package detail lives in each package's
`doc.go`; this file is the map plus the rules that change how you write code.

## 1. What it is
kenderer: a 3D **software** rendering pipeline in pure Go (CPU rasterizer, no
GPU), evolving into a small interactive 3D engine / tool. Module
`github.com/kuronosu/kenderer`, Go 1.26.4.

## 2. Hard constraints
- **Core = ZERO third-party dependencies** (stdlib only). `go build ./...` and the
  GIF export path must stay dependency-free.
- Only allowed deps, strictly confined to where they are imported:
  - **SDL3** (`Zyko0/go-sdl3` + `ebitengine/purego`) behind `//go:build sdl`, only
    in `platform/` and `cmd/viewer`.
  - **glTF** (`qmuntal/gltf`) only in `asset/gltf` (F3 — not created yet).

## 3. Architecture (DAG, `math3d` at the bottom)
- Bottom (stdlib-only): `math3d` (vec/mat/transform/quat), `framebuffer` (color +
  depth target).
- `geometry`,`shading` → math3d · `raster` → math3d,framebuffer,shading ·
  `scene` → math3d,geometry,shading · `pipeline` → all of those · `present`
  (stdlib) and `cmd/cube` consume pipeline/framebuffer.
- Interactive: `input` (stdlib) → `camera` (→math3d,scene,input). SDL-gated:
  `platform` → sdl,input ; `cmd/viewer` → platform,pipeline,scene,camera.
- **Flex points:** input is decoupled — `input.Frame` is our own type; camera and
  the loop depend on it, never on SDL. Rendering is isolated in `pipeline` so a
  GPU backend can later sit behind a `Renderer` interface (F4); today
  `pipeline.Renderer` is a concrete struct.

## 4. Conventions (critical, compact — full detail in each `doc.go`)
- Right-handed, OpenGL-style: +X right, +Y up, camera looks down −Z. [math3d]
- `Mat4` row-major `[4][4]float64` (`m[row][col]`); column-vector `v' = M·v`;
  compose right-to-left, `MVP = P·V·M`.
- Depth: NDC z∈[−1,1] → window z∈[0,1] (0 near, 1 far); z-buffer cleared to `+Inf`,
  fragment passes if `z < stored`. Window z is linear in screen space → geometric
  barycentrics are exact for depth (no perspective correction for depth).
- Perspective-correct attributes via `shading.CombineFragment` (`wi = bi·InvWi`);
  depth is **not** perspective-corrected.
- Winding: CCW = front in NDC; after the Y-flipping viewport, front faces have
  **negative** screen-space signed area → cull positive area; top-left fill rule.
- Normals: inverse-transpose of the model's upper-left 3×3 (`NormalMatrix`). Flat
  shading uses the face normal computed **once** on the original pre-clip triangle.
- Rotation: `scene.Transform.Rotation` is `math3d.Quat`; `QuatFromEuler == Rz·Ry·Rx`;
  `Quat.Mat4` is zero-value-safe (zero `Quat{}` → identity).
- Color: shade in **linear** RGB; `shading.ToRGBA` encodes linear→sRGB once, at
  output. (F3: sample albedo textures sRGB→linear before shading.)
- Clipping: Sutherland–Hodgman in clip space vs the 6 frustum planes, inside
  `pipeline` as pure functions.
- SDL texture: `PIXELFORMAT_ABGR8888` (little-endian byte order = R,G,B,A of
  `image.RGBA.Pix`; do **not** use RGBA8888); update with `img.Stride`; size to the
  drawable pixels via `Renderer.CurrentOutputSize` (HiDPI). [platform]
- Textures/assets (F3 target): sampler origin top-left; loaders normalize UV (OBJ
  flips V, glTF as-is).

## 5. Commands (must all pass)
- `go build ./...` (no SDL) **and** `go build -tags sdl ./...` compile.
- `go test ./...` green. `go vet ./...` and `go vet -tags sdl ./...` clean.
- `gofmt -l .` empty. `golangci-lint run` and `golangci-lint run --build-tags sdl`
  → 0 issues.
- `go mod tidy` adds no external require beyond the confined deps in §2.
- Run: `go run ./cmd/cube -out cube.gif` (GIF, zero-dep) ·
  `go run -tags sdl ./cmd/viewer` (live window; SDL3 embedded via `binsdl`, no
  system install needed). Generated `*.gif`/`*.png` are gitignored.

## 6. Roadmap (status)
- **F1** software renderer + GIF — ✅
- **F2** window + input + orbit camera (SDL3) — ✅
- **F3** assets: OBJ + textures (zero-dep) and glTF isolated via `qmuntal/gltf` in
  `asset/gltf` — ← **NEXT**
- **F4** GPU backend (WebGPU/OpenGL) behind a `Renderer` interface
- **F5** scene graph + frustum culling + multi-light
