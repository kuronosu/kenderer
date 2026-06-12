//go:build sdl

package platform

// The SPIR-V blobs are compiled OFFLINE from the GLSL sources in shaders/src
// and committed, so building kenderer never requires a shader compiler. To
// regenerate after editing a source (glslc ships with the Vulkan SDK):
//
//go:generate glslc shaders/src/lambert.vert -o shaders/lambert.vert.spv
//go:generate glslc shaders/src/lambert.frag -o shaders/lambert.frag.spv

import (
	_ "embed"
	"fmt"
	"math"
	"time"
	"unsafe"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/kuronosu/kenderer/geometry"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/scene"
	"github.com/kuronosu/kenderer/shading"
	"github.com/kuronosu/kenderer/texture"
)

var (
	//go:embed shaders/lambert.vert.spv
	lambertVertSPV []byte
	//go:embed shaders/lambert.frag.spv
	lambertFragSPV []byte
)

// gpuBackend renders with SDL_GPU. It requests SPIR-V shaders, which pins the
// Vulkan driver on every platform SDL supports it on (D3D12 and Metal take
// DXIL/MSL; cross-compiling for those is documented future work, F4.4). The
// swapchain uses the SDR_LINEAR composition: the fragment shader writes linear
// RGB and the sRGB swapchain encodes once at output — the hardware analog of
// the CPU path's shading.ToRGBA.
//
// Scene rendering itself lives in gpuRenderer, which draws to any color
// target; gpuBackend is the swapchain glue. The CPU rasterizer is the
// reference: TestGPUSoftwareParity holds both backends to the same image
// within a small tolerance.
type gpuBackend struct {
	device     *sdl.GPUDevice
	window     *sdl.Window
	renderer   *gpuRenderer
	objectAxes bool
}

// NewGPUBackend returns the SDL_GPU Backend. The device is created at Init.
func NewGPUBackend() Backend {
	return &gpuBackend{}
}

func (b *gpuBackend) Init(window *sdl.Window, w, h int) error {
	device, err := sdl.CreateGPUDevice(sdl.GPU_SHADERFORMAT_SPIRV, false, "")
	if err != nil {
		return fmt.Errorf("create GPU device: %w", err)
	}
	b.device = device
	b.window = window

	if device.ShaderFormats()&sdl.GPU_SHADERFORMAT_SPIRV == 0 {
		b.Close()
		return fmt.Errorf("GPU driver %q does not accept SPIR-V shaders", device.Driver())
	}
	if err := device.ClaimWindow(window); err != nil {
		b.Close()
		return fmt.Errorf("claim window: %w", err)
	}
	// One sRGB encode at output, applied by hardware. Without SDR_LINEAR the
	// shader would have to encode instead — two code paths to keep in parity —
	// so the backend requires it (universally available on Vulkan drivers).
	if !device.WindowSupportsSwapchainComposition(window, sdl.GPU_SWAPCHAINCOMPOSITION_SDR_LINEAR) {
		b.Close()
		return fmt.Errorf("GPU driver %q lacks an sRGB (SDR_LINEAR) swapchain, which the GPU backend requires", device.Driver())
	}
	if err := device.SetSwapchainParameters(window, sdl.GPU_SWAPCHAINCOMPOSITION_SDR_LINEAR, sdl.GPU_PRESENTMODE_VSYNC); err != nil {
		b.Close()
		return fmt.Errorf("set swapchain parameters: %w", err)
	}

	renderer, err := newGPURenderer(device, device.SwapchainTextureFormat(window))
	if err != nil {
		b.Close()
		return err
	}
	b.renderer = renderer
	return nil
}

func (b *gpuBackend) RenderFrame(s *scene.Scene, _ string) (time.Duration, error) {
	cmdbuf, err := b.device.AcquireCommandBuffer()
	if err != nil {
		return 0, fmt.Errorf("acquire command buffer: %w", err)
	}
	swapchain, err := cmdbuf.WaitAndAcquireGPUSwapchainTexture(b.window)
	if err != nil {
		return 0, fmt.Errorf("acquire swapchain texture: %w", err)
	}

	// The blocking swapchain wait above plays the role vsync plays for the
	// software backend, so busy measures only the work recorded after it.
	start := time.Now()
	if swapchain != nil && swapchain.Texture != nil {
		if err := b.renderer.renderScene(cmdbuf, swapchain.Texture, int(swapchain.Width), int(swapchain.Height), s); err != nil {
			return 0, err
		}
	}
	busy := time.Since(start)

	// Submit presents asynchronously; a minimized window (nil swapchain
	// texture) still must submit the acquired command buffer.
	if err := cmdbuf.Submit(); err != nil {
		return 0, fmt.Errorf("submit frame: %w", err)
	}
	return busy, nil
}

// Resize is a no-op: the swapchain tracks the window on acquire, and the depth
// texture follows the render target's size lazily inside renderScene.
func (b *gpuBackend) Resize(int, int) error { return nil }

// SetObjectAxes stores the toggle; the GPU line pass consumes it from F4.3 on.
func (b *gpuBackend) SetObjectAxes(on bool) { b.objectAxes = on }

func (b *gpuBackend) Close() {
	if b.device == nil {
		return
	}
	_ = b.device.WaitForIdle()
	if b.renderer != nil {
		b.renderer.destroy()
		b.renderer = nil
	}
	if b.window != nil {
		b.device.ReleaseWindow(b.window)
		b.window = nil
	}
	b.device.Destroy()
	b.device = nil
}

// gpuRenderer draws scenes to an arbitrary color target (the swapchain for the
// live backend, an offscreen texture for the parity test). It mirrors the CPU
// pipeline stage by stage: PerspectiveZO projection (z in [0,1], SDL_GPU's
// normalized NDC needs no Y handling), CCW front faces with back culling, a
// z-buffer cleared to the far plane with a strict less depth test, and the
// Lambert shader pair, which documents the shading parity contract.
//
// Mesh and texture resources upload once on first sight and are cached by
// pointer identity; per frame only the uniforms change.
type gpuRenderer struct {
	device       *sdl.GPUDevice
	pipeline     *sdl.GPUGraphicsPipeline
	targetFormat sdl.GPUTextureFormat
	depthFormat  sdl.GPUTextureFormat

	// depth is the depth target, recreated whenever the color target size
	// changes (resize), so renderScene never sees a mismatched attachment.
	depth          *sdl.GPUTexture
	depthW, depthH int

	meshes   map[*geometry.Mesh]gpuMesh
	textures map[*texture.Texture]*sdl.GPUTexture
	samplers map[samplerKey]*sdl.GPUSampler
	// white is the 1x1 white albedo bound for untextured objects, whose real
	// albedo then rides in the albedoFactor uniform (the CPU semantics: a
	// material's constant Albedo is used only when it has no map).
	white *sdl.GPUTexture
}

// gpuMesh is the uploaded form of a geometry.Mesh.
type gpuMesh struct {
	vertices   *sdl.GPUBuffer
	indices    *sdl.GPUBuffer
	numIndices uint32
}

// samplerKey caches one GPU sampler per kenderer filter/wrap combination.
type samplerKey struct {
	filter texture.Filter
	wrap   texture.Wrap
}

// vertexFloats is the number of float32 per uploaded vertex: position (3),
// normal (3), UV (2), color (3) — the attributes shading.Fragment interpolates.
const vertexFloats = 11

func newGPURenderer(device *sdl.GPUDevice, targetFormat sdl.GPUTextureFormat) (*gpuRenderer, error) {
	r := &gpuRenderer{
		device:       device,
		targetFormat: targetFormat,
		depthFormat:  pickDepthFormat(device),
		meshes:       make(map[*geometry.Mesh]gpuMesh),
		textures:     make(map[*texture.Texture]*sdl.GPUTexture),
		samplers:     make(map[samplerKey]*sdl.GPUSampler),
	}
	if err := r.createPipeline(); err != nil {
		r.destroy()
		return nil, err
	}
	white, err := uploadGPUTexture(device, 1, 1, []byte{255, 255, 255, 255})
	if err != nil {
		r.destroy()
		return nil, fmt.Errorf("create white texture: %w", err)
	}
	r.white = white
	return r, nil
}

// pickDepthFormat returns the best depth-only format the device supports as a
// depth target. D32_FLOAT and D24_UNORM are optional in Vulkan, but at least
// one of them is always available; D16_UNORM is the universal fallback.
func pickDepthFormat(device *sdl.GPUDevice) sdl.GPUTextureFormat {
	for _, format := range []sdl.GPUTextureFormat{
		sdl.GPU_TEXTUREFORMAT_D32_FLOAT,
		sdl.GPU_TEXTUREFORMAT_D24_UNORM,
	} {
		if device.TextureSupportsFormat(format, sdl.GPU_TEXTURETYPE_2D, sdl.GPU_TEXTUREUSAGE_DEPTH_STENCIL_TARGET) {
			return format
		}
	}
	return sdl.GPU_TEXTUREFORMAT_D16_UNORM
}

// createPipeline builds the mesh pipeline around the Lambert shader pair. The
// fixed-function state mirrors the CPU rasterizer: CCW = front with back
// culling (kenderer's winding convention in SDL_GPU's GL-like NDC), depth test
// "pass if closer" with writes on (the z-buffer contract), no blending.
func (r *gpuRenderer) createPipeline() error {
	vert, err := r.createShader(lambertVertSPV, sdl.GPU_SHADERSTAGE_VERTEX, 0, 1)
	if err != nil {
		return fmt.Errorf("create vertex shader: %w", err)
	}
	defer r.device.ReleaseShader(vert)
	frag, err := r.createShader(lambertFragSPV, sdl.GPU_SHADERSTAGE_FRAGMENT, 1, 1)
	if err != nil {
		return fmt.Errorf("create fragment shader: %w", err)
	}
	defer r.device.ReleaseShader(frag)

	pipeline, err := r.device.CreateGraphicsPipeline(&sdl.GPUGraphicsPipelineCreateInfo{
		TargetInfo: sdl.GPUGraphicsPipelineTargetInfo{
			ColorTargetDescriptions: []sdl.GPUColorTargetDescription{{Format: r.targetFormat}},
			HasDepthStencilTarget:   true,
			DepthStencilFormat:      r.depthFormat,
		},
		DepthStencilState: sdl.GPUDepthStencilState{
			CompareOp:        sdl.GPU_COMPAREOP_LESS,
			EnableDepthTest:  true,
			EnableDepthWrite: true,
		},
		RasterizerState: sdl.GPURasterizerState{
			FillMode:  sdl.GPU_FILLMODE_FILL,
			CullMode:  sdl.GPU_CULLMODE_BACK,
			FrontFace: sdl.GPU_FRONTFACE_COUNTER_CLOCKWISE,
		},
		VertexInputState: sdl.GPUVertexInputState{
			VertexBufferDescriptions: []sdl.GPUVertexBufferDescription{{
				Slot:      0,
				InputRate: sdl.GPU_VERTEXINPUTRATE_VERTEX,
				Pitch:     vertexFloats * 4,
			}},
			VertexAttributes: []sdl.GPUVertexAttribute{
				{BufferSlot: 0, Location: 0, Format: sdl.GPU_VERTEXELEMENTFORMAT_FLOAT3, Offset: 0},  // position
				{BufferSlot: 0, Location: 1, Format: sdl.GPU_VERTEXELEMENTFORMAT_FLOAT3, Offset: 12}, // normal
				{BufferSlot: 0, Location: 2, Format: sdl.GPU_VERTEXELEMENTFORMAT_FLOAT2, Offset: 24}, // uv
				{BufferSlot: 0, Location: 3, Format: sdl.GPU_VERTEXELEMENTFORMAT_FLOAT3, Offset: 32}, // color
			},
		},
		PrimitiveType:  sdl.GPU_PRIMITIVETYPE_TRIANGLELIST,
		VertexShader:   vert,
		FragmentShader: frag,
	})
	if err != nil {
		return fmt.Errorf("create graphics pipeline: %w", err)
	}
	r.pipeline = pipeline
	return nil
}

func (r *gpuRenderer) createShader(code []byte, stage sdl.GPUShaderStage, numSamplers, numUniformBuffers uint32) (*sdl.GPUShader, error) {
	return r.device.CreateGPUShader(&sdl.GPUShaderCreateInfo{
		Code:              code,
		Entrypoint:        "main",
		Format:            sdl.GPU_SHADERFORMAT_SPIRV,
		Stage:             stage,
		NumSamplers:       numSamplers,
		NumUniformBuffers: numUniformBuffers,
	})
}

// renderScene records one frame into cmdbuf: it first ensures every mesh and
// texture the scene needs is resident (first-sight uploads submit their own
// command buffers, which SDL_GPU executes in submission order, i.e. before
// cmdbuf), then records the render pass against the given color target. The
// caller owns cmdbuf and submits it.
func (r *gpuRenderer) renderScene(cmdbuf *sdl.GPUCommandBuffer, target *sdl.GPUTexture, w, h int, s *scene.Scene) error {
	if err := r.ensureDepth(w, h); err != nil {
		return err
	}
	for i := range s.Objects {
		if _, err := r.ensureMesh(s.Objects[i].Mesh); err != nil {
			return err
		}
		if tex := s.Objects[i].Material.AlbedoTex; tex != nil {
			if _, err := r.ensureTexture(tex); err != nil {
				return err
			}
		}
	}

	aspect := float64(w) / float64(h)
	viewProj := math3d.PerspectiveZO(s.Camera.FOVY, aspect, s.Camera.Near, s.Camera.Far).Mul(s.Camera.View())

	pass := cmdbuf.BeginRenderPass(
		[]sdl.GPUColorTargetInfo{{
			Texture: target,
			// The shader works in linear RGB and the sRGB target encodes at
			// output, so the clear is the *decoded* background — it encodes
			// back to exactly the software backend's background bytes.
			ClearColor: sdl.FColor{R: bgLinear[0], G: bgLinear[1], B: bgLinear[2], A: 1},
			LoadOp:     sdl.GPU_LOADOP_CLEAR,
			StoreOp:    sdl.GPU_STOREOP_STORE,
		}},
		&sdl.GPUDepthStencilTargetInfo{
			Texture:        r.depth,
			ClearDepth:     1, // the far plane, like the CPU z-buffer's +Inf clear
			LoadOp:         sdl.GPU_LOADOP_CLEAR,
			StoreOp:        sdl.GPU_STOREOP_DONT_CARE,
			StencilLoadOp:  sdl.GPU_LOADOP_DONT_CARE,
			StencilStoreOp: sdl.GPU_STOREOP_DONT_CARE,
		},
	)
	pass.BindGraphicsPipeline(r.pipeline)

	for i := range s.Objects {
		obj := &s.Objects[i]
		mesh := r.meshes[obj.Mesh]
		if mesh.numIndices == 0 {
			continue
		}

		model := obj.Transform.Matrix()
		vu := packVertexUniforms(viewProj.Mul(model), model, math3d.NormalMatrix(model))
		cmdbuf.PushVertexUniformData(0, f32Bytes(vu[:]))
		fu := packFragmentUniforms(s, obj)
		cmdbuf.PushFragmentUniformData(0, f32Bytes(fu[:]))

		albedo := r.white
		if obj.Material.AlbedoTex != nil {
			albedo = r.textures[obj.Material.AlbedoTex]
		}
		sampler, err := r.sampler(obj.Material.Filter, obj.Material.Wrap)
		if err != nil {
			pass.End()
			return err
		}
		pass.BindFragmentSamplers([]sdl.GPUTextureSamplerBinding{{Texture: albedo, Sampler: sampler}})

		pass.BindVertexBuffers([]sdl.GPUBufferBinding{{Buffer: mesh.vertices}})
		pass.BindIndexBuffer(&sdl.GPUBufferBinding{Buffer: mesh.indices}, sdl.GPU_INDEXELEMENTSIZE_32BIT)
		pass.DrawIndexedPrimitives(mesh.numIndices, 1, 0, 0, 0)
	}
	pass.End()
	return nil
}

// ensureDepth (re)creates the depth target at the given size. It is lazy so a
// swapchain resize between events never produces a mismatched attachment.
func (r *gpuRenderer) ensureDepth(w, h int) error {
	if r.depth != nil && r.depthW == w && r.depthH == h {
		return nil
	}
	if r.depth != nil {
		r.device.ReleaseTexture(r.depth)
		r.depth = nil
	}
	depth, err := r.device.CreateTexture(&sdl.GPUTextureCreateInfo{
		Type:              sdl.GPU_TEXTURETYPE_2D,
		Format:            r.depthFormat,
		Usage:             sdl.GPU_TEXTUREUSAGE_DEPTH_STENCIL_TARGET,
		Width:             uint32(w),
		Height:            uint32(h),
		LayerCountOrDepth: 1,
		NumLevels:         1,
	})
	if err != nil {
		return fmt.Errorf("create depth texture: %w", err)
	}
	r.depth = depth
	r.depthW, r.depthH = w, h
	return nil
}

// ensureMesh uploads the mesh's vertex and index buffers on first sight.
func (r *gpuRenderer) ensureMesh(mesh *geometry.Mesh) (gpuMesh, error) {
	if m, ok := r.meshes[mesh]; ok {
		return m, nil
	}
	data := make([]float32, 0, len(mesh.Vertices)*vertexFloats)
	for _, v := range mesh.Vertices {
		data = append(data,
			float32(v.Position.X), float32(v.Position.Y), float32(v.Position.Z),
			float32(v.Normal.X), float32(v.Normal.Y), float32(v.Normal.Z),
			float32(v.UV.X), float32(v.UV.Y),
			float32(v.Color.X), float32(v.Color.Y), float32(v.Color.Z))
	}
	vertices, err := uploadGPUBuffer(r.device, sdl.GPU_BUFFERUSAGE_VERTEX, data)
	if err != nil {
		return gpuMesh{}, fmt.Errorf("upload vertices: %w", err)
	}
	indices, err := uploadGPUBuffer(r.device, sdl.GPU_BUFFERUSAGE_INDEX, mesh.Indices)
	if err != nil {
		r.device.ReleaseBuffer(vertices)
		return gpuMesh{}, fmt.Errorf("upload indices: %w", err)
	}
	m := gpuMesh{vertices: vertices, indices: indices, numIndices: uint32(len(mesh.Indices))}
	r.meshes[mesh] = m
	return m, nil
}

// ensureTexture uploads the texture on first sight. Texels are stored linear
// on the CPU (decoded from sRGB on load), so they are re-encoded to sRGB bytes
// — a lossless round-trip back to the source image — and uploaded into an
// sRGB-format texture the sampler decodes in hardware, keeping filtering in
// linear space exactly like texture.Sample.
func (r *gpuRenderer) ensureTexture(tex *texture.Texture) (*sdl.GPUTexture, error) {
	if t, ok := r.textures[tex]; ok {
		return t, nil
	}
	pix := make([]byte, 0, tex.Width*tex.Height*4)
	for y := 0; y < tex.Height; y++ {
		for x := 0; x < tex.Width; x++ {
			c := shading.ToRGBA(tex.Texel(x, y))
			pix = append(pix, c.R, c.G, c.B, c.A)
		}
	}
	t, err := uploadGPUTexture(r.device, tex.Width, tex.Height, pix)
	if err != nil {
		return nil, fmt.Errorf("upload texture: %w", err)
	}
	r.textures[tex] = t
	return t, nil
}

// sampler returns the cached GPU sampler for a kenderer filter/wrap pair.
func (r *gpuRenderer) sampler(filter texture.Filter, wrap texture.Wrap) (*sdl.GPUSampler, error) {
	key := samplerKey{filter: filter, wrap: wrap}
	if s, ok := r.samplers[key]; ok {
		return s, nil
	}
	info := sdl.GPUSamplerCreateInfo{MipmapMode: sdl.GPU_SAMPLERMIPMAPMODE_NEAREST}
	if filter == texture.Bilinear {
		info.MinFilter, info.MagFilter = sdl.GPU_FILTER_LINEAR, sdl.GPU_FILTER_LINEAR
	} else {
		info.MinFilter, info.MagFilter = sdl.GPU_FILTER_NEAREST, sdl.GPU_FILTER_NEAREST
	}
	mode := sdl.GPU_SAMPLERADDRESSMODE_REPEAT
	if wrap == texture.Clamp {
		mode = sdl.GPU_SAMPLERADDRESSMODE_CLAMP_TO_EDGE
	}
	info.AddressModeU, info.AddressModeV, info.AddressModeW = mode, mode, mode

	s, err := r.device.CreateSampler(&info)
	if err != nil {
		return nil, fmt.Errorf("create sampler: %w", err)
	}
	r.samplers[key] = s
	return s, nil
}

// destroy releases every GPU resource the renderer owns. The caller must have
// waited for the device to go idle.
func (r *gpuRenderer) destroy() {
	for _, m := range r.meshes {
		r.device.ReleaseBuffer(m.vertices)
		r.device.ReleaseBuffer(m.indices)
	}
	r.meshes = nil
	for _, t := range r.textures {
		r.device.ReleaseTexture(t)
	}
	r.textures = nil
	for _, s := range r.samplers {
		r.device.ReleaseSampler(s)
	}
	r.samplers = nil
	if r.white != nil {
		r.device.ReleaseTexture(r.white)
		r.white = nil
	}
	if r.depth != nil {
		r.device.ReleaseTexture(r.depth)
		r.depth = nil
	}
	if r.pipeline != nil {
		r.device.ReleaseGraphicsPipeline(r.pipeline)
		r.pipeline = nil
	}
}

// packVertexUniforms lays out the vertex uniform block: mvp, model and the
// normal matrix (a Mat3 widened to a mat4 column layout). Mat4 is row-major
// with column vectors (v' = M·v), while std140 mat4 is column-major, so each
// matrix is transposed as it is packed.
func packVertexUniforms(mvp, model math3d.Mat4, normal math3d.Mat3) [48]float32 {
	var u [48]float32
	packMat4(u[0:16], mvp)
	packMat4(u[16:32], model)
	for c := 0; c < 3; c++ {
		for row := 0; row < 3; row++ {
			u[32+c*4+row] = float32(normal[row][c])
		}
	}
	u[47] = 1
	return u
}

func packMat4(dst []float32, m math3d.Mat4) {
	for c := 0; c < 4; c++ {
		for row := 0; row < 4; row++ {
			dst[c*4+row] = float32(m[row][c])
		}
	}
}

// packFragmentUniforms lays out the fragment uniform block; see lambert.frag
// for the field semantics.
func packFragmentUniforms(s *scene.Scene, obj *scene.Object) [16]float32 {
	var u [16]float32
	u[0], u[1], u[2] = float32(s.Light.Direction.X), float32(s.Light.Direction.Y), float32(s.Light.Direction.Z)
	u[4], u[5], u[6] = float32(s.Light.Color.X), float32(s.Light.Color.Y), float32(s.Light.Color.Z)
	u[7] = float32(s.Light.Intensity)
	albedo := math3d.V3(1, 1, 1) // textured: the map replaces the constant albedo
	if obj.Material.AlbedoTex == nil {
		albedo = obj.Material.Albedo
	}
	u[8], u[9], u[10] = float32(albedo.X), float32(albedo.Y), float32(albedo.Z)
	u[12] = float32(s.Ambient)
	if obj.Smooth {
		u[13] = 1
	}
	return u
}

// bgLinear is the shared background color decoded from its sRGB bytes: the
// linear value whose hardware sRGB encode reproduces bgR/bgG/bgB exactly.
var bgLinear = [3]float32{srgbByteToLinear(bgR), srgbByteToLinear(bgG), srgbByteToLinear(bgB)}

// srgbByteToLinear decodes one 8-bit sRGB component to linear (the inverse of
// shading's output encode, over the byte domain).
func srgbByteToLinear(b uint8) float32 {
	c := float64(b) / 255
	if c <= 0.04045 {
		return float32(c / 12.92)
	}
	return float32(math.Pow((c+0.055)/1.055, 2.4))
}

// f32Bytes reinterprets a float32 slice as its raw bytes for uniform pushes.
func f32Bytes(data []float32) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(data))), len(data)*4)
}

// mappedPointer reinterprets the address MapTransferBuffer returns as a
// pointer. The mapped memory is driver-owned — never Go-managed — so the
// uintptr round-trip is safe; going through a pointer-to-pointer read (the
// same pattern the binding's generated code uses) instead of a direct
// uintptr→unsafe.Pointer conversion keeps go vet's unsafeptr check accurate
// for the rest of the package.
func mappedPointer(addr uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&addr))
}

// writeMapped copies data into the transfer memory mapped at addr. It loops
// element-wise instead of copy(): inlining copy into the reinterpreted
// destination trips an SSA rewrite bug in the Go 1.26.4 compiler (panic in
// disjointTypes during lower).
func writeMapped[T any](addr uintptr, data []T) {
	dst := unsafe.Slice((*T)(mappedPointer(addr)), len(data))
	//nolint:staticcheck // S1001: copy() here ICEs the Go 1.26.4 compiler, see above.
	for i := range data {
		dst[i] = data[i]
	}
}

// readMapped copies n bytes out of the transfer memory mapped at addr — the
// read half of writeMapped, with the same compiler-bug-avoiding loop.
func readMapped(addr uintptr, n int) []byte {
	src := unsafe.Slice((*byte)(mappedPointer(addr)), n)
	out := make([]byte, n)
	//nolint:staticcheck // S1001: copy() here ICEs the Go 1.26.4 compiler, see writeMapped.
	for i := range out {
		out[i] = src[i]
	}
	return out
}

// uploadGPUBuffer creates a GPU buffer and fills it with data through a
// transfer buffer (SDL_GPU's only upload path), submitting the copy on its own
// command buffer.
func uploadGPUBuffer[T any](device *sdl.GPUDevice, usage sdl.GPUBufferUsageFlags, data []T) (*sdl.GPUBuffer, error) {
	var zero T
	size := uint32(len(data)) * uint32(unsafe.Sizeof(zero))
	buf, err := device.CreateBuffer(&sdl.GPUBufferCreateInfo{Usage: usage, Size: size})
	if err != nil {
		return nil, fmt.Errorf("create buffer: %w", err)
	}
	fail := func(e error) (*sdl.GPUBuffer, error) {
		device.ReleaseBuffer(buf)
		return nil, e
	}

	transfer, err := device.CreateTransferBuffer(&sdl.GPUTransferBufferCreateInfo{
		Usage: sdl.GPU_TRANSFERBUFFERUSAGE_UPLOAD,
		Size:  size,
	})
	if err != nil {
		return fail(fmt.Errorf("create transfer buffer: %w", err))
	}
	defer device.ReleaseTransferBuffer(transfer)

	ptr, err := device.MapTransferBuffer(transfer, false)
	if err != nil {
		return fail(fmt.Errorf("map transfer buffer: %w", err))
	}
	writeMapped(ptr, data)
	device.UnmapTransferBuffer(transfer)

	cmdbuf, err := device.AcquireCommandBuffer()
	if err != nil {
		return fail(fmt.Errorf("acquire command buffer: %w", err))
	}
	pass := cmdbuf.BeginCopyPass()
	pass.UploadToGPUBuffer(
		&sdl.GPUTransferBufferLocation{TransferBuffer: transfer},
		&sdl.GPUBufferRegion{Buffer: buf, Size: size},
		false,
	)
	pass.End()
	if err := cmdbuf.Submit(); err != nil {
		return fail(fmt.Errorf("submit upload: %w", err))
	}
	return buf, nil
}

// uploadGPUTexture creates a sampleable sRGB texture from tightly packed RGBA
// bytes and fills it through a transfer buffer on its own command buffer.
func uploadGPUTexture(device *sdl.GPUDevice, w, h int, pixels []byte) (*sdl.GPUTexture, error) {
	tex, err := device.CreateTexture(&sdl.GPUTextureCreateInfo{
		Type:              sdl.GPU_TEXTURETYPE_2D,
		Format:            sdl.GPU_TEXTUREFORMAT_R8G8B8A8_UNORM_SRGB,
		Usage:             sdl.GPU_TEXTUREUSAGE_SAMPLER,
		Width:             uint32(w),
		Height:            uint32(h),
		LayerCountOrDepth: 1,
		NumLevels:         1,
	})
	if err != nil {
		return nil, fmt.Errorf("create texture: %w", err)
	}
	fail := func(e error) (*sdl.GPUTexture, error) {
		device.ReleaseTexture(tex)
		return nil, e
	}

	transfer, err := device.CreateTransferBuffer(&sdl.GPUTransferBufferCreateInfo{
		Usage: sdl.GPU_TRANSFERBUFFERUSAGE_UPLOAD,
		Size:  uint32(len(pixels)),
	})
	if err != nil {
		return fail(fmt.Errorf("create transfer buffer: %w", err))
	}
	defer device.ReleaseTransferBuffer(transfer)

	ptr, err := device.MapTransferBuffer(transfer, false)
	if err != nil {
		return fail(fmt.Errorf("map transfer buffer: %w", err))
	}
	writeMapped(ptr, pixels)
	device.UnmapTransferBuffer(transfer)

	cmdbuf, err := device.AcquireCommandBuffer()
	if err != nil {
		return fail(fmt.Errorf("acquire command buffer: %w", err))
	}
	pass := cmdbuf.BeginCopyPass()
	pass.UploadToGPUTexture(
		&sdl.GPUTextureTransferInfo{TransferBuffer: transfer},
		&sdl.GPUTextureRegion{Texture: tex, W: uint32(w), H: uint32(h), D: 1},
		false,
	)
	pass.End()
	if err := cmdbuf.Submit(); err != nil {
		return fail(fmt.Errorf("submit upload: %w", err))
	}
	return tex, nil
}
