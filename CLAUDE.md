# CLAUDE.md

Guidance for working in this repo. Per-package detail lives in each package's
`doc.go`; this file is the map plus the rules that change how you write code.

## 1. What it is
kenderer: a 3D **software** rendering pipeline in pure Go (CPU rasterizer, no
GPU), evolving into a small interactive 3D engine / tool. Module
`github.com/kuronosu/kenderer`, Go 1.26.4.

## 2. Hard constraints
- **Renderer core + GIF path = ZERO third-party imports** (stdlib only): math3d,
  framebuffer, geometry, texture, shading, raster, scene, pipeline, present and
  `cmd/cube` import no third-party code.
- Only allowed deps, strictly confined to where they are imported:
  - **SDL3** (`Zyko0/go-sdl3` + `ebitengine/purego`, zero-cgo — `CGO_ENABLED=0`
    builds both backends) behind `//go:build sdl`, only in `platform/` and
    `cmd/viewer`. The SDL_GPU backend uses this same binding: no new dep.
  - **glTF** (`qmuntal/gltf`, pure Go — no native lib) only in `asset/gltf`. It is
    *not* build-tag gated, so `go build ./...` does pull it into the module graph;
    the core/GIF packages above still import nothing third-party.

## 3. Architecture (DAG, `math3d` at the bottom)
- Bottom (stdlib-only): `math3d` (vec/mat/transform/quat), `framebuffer` (color +
  depth target).
- `geometry`→math3d · `texture`→math3d (stdlib image) · `shading`→math3d,texture ·
  `raster`→math3d,framebuffer,shading · `scene`→math3d,geometry,shading ·
  `pipeline`→all of those · `asset`→geometry,texture,math3d ·
  `asset/gltf`→asset,geometry,texture,math3d,**qmuntal/gltf** · `present` (stdlib)
  and `cmd/cube` consume pipeline/framebuffer/present.
- Interactive: `input` (stdlib) → `camera` (→math3d,scene,input). SDL-gated:
  `platform` → sdl,input,scene,pipeline (loop + `Backend` seam: `software`
  wraps `pipeline.Renderer`, `gpu` is SDL_GPU; each backend owns its
  presentation) ; `cmd/viewer` (`-backend software|gpu`) →
  platform,pipeline,scene,camera,asset,asset/gltf,texture.
- **Flex points:** input is decoupled — `input.Frame` is our own type; camera and
  the loop depend on it, never on SDL. Rendering sits behind `platform.Backend`
  (F4): the CPU rasterizer is the reference/oracle backend, the SDL_GPU backend
  is held to it by parity tests; `pipeline.Renderer` stays a concrete struct
  that `cmd/cube` uses headless.

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
- Normals: inverse-transpose of the model's upper-left 3×3 (`NormalMatrix`). Shading
  mode is **per-object** (`scene.Object.Smooth`): flat (default) replaces the vertex
  normals with the face normal computed **once** on the original pre-clip triangle;
  smooth keeps the interpolated per-vertex normals → Phong (Lambert normalizes the
  fragment normal, since interpolation denormalizes it). Imported meshes load smooth;
  the procedural cube stays flat.
- Rotation: `scene.Transform.Rotation` is `math3d.Quat`; `QuatFromEuler == Rz·Ry·Rx`;
  `Quat.Mat4` is zero-value-safe (zero `Quat{}` → identity).
- Color: shade in **linear** RGB; `shading.ToRGBA` encodes linear→sRGB once, at
  output. Albedo textures are decoded sRGB→linear on load (`texture.KindColor`);
  glTF `baseColorFactor` and `COLOR_0` vertex colors are already linear. Loaders set
  `Vertex.Color` to white so the shaded base `albedo ⊙ vertexColor` is never zeroed.
- Clipping: Sutherland–Hodgman in clip space vs the 6 frustum planes, inside
  `pipeline` as pure functions. Lines reuse the **same** planes parametrically
  (Liang–Barsky, `clipSegment`) before the divide, so a near-crossing segment never
  divides by w≈0.
- Lines: `raster.DrawLine` is an unlit, constant-color, depth-**tested but not
  depth-written** screen-space DDA (the line analog of the triangle fill). A serial
  `pipeline` line pass after the fill barrier draws `scene.Scene.Lines` (caller
  world segments, e.g. `scene.WorldAxes`) and per-object local axes (gated by
  `Renderer.SetObjectAxes`; endpoints from `scene.ObjectAxes`, the single source
  both backends share). Axis colors live once in `scene.AxisColor{X,Y,Z}`
  (linear: +X red, +Y green, +Z blue). Inert when off ⇒ output byte-identical.
- **GPU backend** (`platform/gpu.go`, SDL_GPU): requests SPIR-V ⇒ SDL picks its
  Vulkan driver on every OS. Projection is `math3d.PerspectiveZO` (z∈[0,1],
  provably equal to the CPU's window z — pinned by test); SDL_GPU normalizes NDC
  across drivers (+Y up, GL-like; SDL does the Vulkan Y-flip internally — do
  **not** flip again); winding stays CCW front + back cull. **One** sRGB encode
  at output, by hardware: SDR_LINEAR swapchain (or sRGB offscreen target);
  shaders work in linear, mirroring `shading.ToRGBA`/`Lambert` exactly (flat =
  `cross(dFdy, dFdx)` of world pos; untextured = 1×1 white map × albedo
  uniform). Meshes/textures upload once, cached by pointer; textures re-encode
  linear→sRGB bytes (lossless round-trip) into sRGB formats so hardware
  filtering stays linear. Stats: the SDL_Renderer 2D API (DebugText) cannot
  coexist with a GPU swapchain ⇒ the GPU backend shows stats in the window
  title; the software backend keeps its overlay. Parity vs the CPU oracle is
  pinned by `platform` tests (`-tags sdl`; skip without a device).
- **Shaders:** GLSL 450 in `platform/shaders/src/`, compiled OFFLINE to SPIR-V
  with `glslc` (Vulkan SDK) via `go:generate` in `platform/gpu.go`; the `.spv`
  blobs are committed and embedded, so builds never need a shader compiler.
  SDL_GPU SPIR-V binding sets: vertex uniforms set 1, fragment samplers set 2,
  fragment uniforms set 3.
- SDL texture: `PIXELFORMAT_ABGR8888` (little-endian byte order = R,G,B,A of
  `image.RGBA.Pix`; do **not** use RGBA8888); update with `img.Stride`; size to the
  drawable pixels via `Renderer.CurrentOutputSize` (HiDPI). [platform]
- Textures/assets: `texture.Texture` samples in linear RGB, origin top-left, with
  Nearest/Bilinear × Repeat/Clamp. Loaders normalize UV to that origin (OBJ flips V;
  glTF passes through). `asset.LoadOBJ` (zero-dep, OBJ+MTL) and
  `asset/gltf.LoadGLTF` (qmuntal) return `asset.Model{Mesh, BaseColor, AlbedoTex}`;
  the caller builds the `shading.Material`. glTF bakes the node transform into
  world-space positions (flat mesh; live scene graph is F5).

## 5. Commands (must all pass)
- `go build ./...` (no SDL) **and** `go build -tags sdl ./...` compile.
- `go test ./...` green. `go vet ./...` and `go vet -tags sdl ./...` clean.
- `gofmt -l .` empty. `golangci-lint run` and `golangci-lint run --build-tags sdl`
  → 0 issues.
- `go mod tidy` adds no external require beyond the confined deps in §2.
- Run: `go run ./cmd/cube -out cube.gif` (GIF, zero-dep) ·
  `go run -tags sdl ./cmd/viewer` (live window; SDL3 embedded via `binsdl`, no
  system install needed) · `-backend gpu` for SDL_GPU/Vulkan (needs a Vulkan
  driver at runtime; building never does). GPU parity tests run under
  `go test -tags sdl ./platform/` and skip without a device. Generated
  `*.gif`/`*.png` are gitignored.

## 6. Roadmap (status)
- **F1** software renderer + GIF — ✅
- **F2** window + input + orbit camera (SDL3) — ✅
- **F3** assets: OBJ + textures (zero-dep) and glTF isolated via `qmuntal/gltf` in
  `asset/gltf`; per-object smooth/flat shading toggle — ✅
- **F4** GPU backend (SDL_GPU/Vulkan/SPIR-V) behind `platform.Backend`, CPU
  rasterizer as the parity oracle — ✅
- **F4.4** (future, documented only): cross-platform shaders — compile the GLSL
  offline to DXIL (D3D12) and MSL (Metal) with SDL_shadercross and pick the
  format by `ShaderFormats()`; needs Windows/macOS toolchains. Also: text
  overlay inside the GPU swapchain (today: window title).
- **F5** scene graph + frustum culling + multi-light — ← **NEXT**
