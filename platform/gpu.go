//go:build sdl

package platform

// The SPIR-V blobs are compiled OFFLINE from the GLSL sources in shaders/src
// and committed, so building kenderer never requires a shader compiler. To
// regenerate after editing a source (glslc ships with the Vulkan SDK):
//
//go:generate glslc shaders/src/triangle.vert -o shaders/triangle.vert.spv
//go:generate glslc shaders/src/triangle.frag -o shaders/triangle.frag.spv

import (
	_ "embed"
	"fmt"
	"time"
	"unsafe"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/kuronosu/kenderer/scene"
)

var (
	//go:embed shaders/triangle.vert.spv
	triangleVertSPV []byte
	//go:embed shaders/triangle.frag.spv
	triangleFragSPV []byte
)

// gpuBackend renders with SDL_GPU. It requests SPIR-V shaders, which pins the
// Vulkan driver on every platform SDL supports it on (D3D12 and Metal take
// DXIL/MSL; cross-compiling for those is documented future work, F4.4).
//
// F4.1 scope: device + swapchain + a minimal graphics pipeline drawing one
// hardcoded clip-space triangle. The scene is ignored until F4.2.
type gpuBackend struct {
	device     *sdl.GPUDevice
	window     *sdl.Window
	pipeline   *sdl.GPUGraphicsPipeline
	vertexBuf  *sdl.GPUBuffer
	objectAxes bool
}

// NewGPUBackend returns the SDL_GPU Backend. The device is created at Init.
func NewGPUBackend() Backend {
	return &gpuBackend{}
}

// triangleVertex matches the vertex layout the bring-up pipeline declares: one
// FLOAT3 clip-space position per vertex.
type triangleVertex struct {
	X, Y, Z float32
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

	if err := b.createPipeline(); err != nil {
		b.Close()
		return err
	}
	if err := b.uploadTriangle(); err != nil {
		b.Close()
		return err
	}
	return nil
}

// createPipeline builds the bring-up graphics pipeline: SPIR-V passthrough
// vertex + constant-color fragment, one FLOAT3 vertex attribute, rendering to
// the swapchain format. Cull and depth state stay at their defaults until the
// real mesh pipeline lands (F4.2).
func (b *gpuBackend) createPipeline() error {
	vert, err := b.createShader(triangleVertSPV, sdl.GPU_SHADERSTAGE_VERTEX)
	if err != nil {
		return fmt.Errorf("create vertex shader: %w", err)
	}
	defer b.device.ReleaseShader(vert)
	frag, err := b.createShader(triangleFragSPV, sdl.GPU_SHADERSTAGE_FRAGMENT)
	if err != nil {
		return fmt.Errorf("create fragment shader: %w", err)
	}
	defer b.device.ReleaseShader(frag)

	pipeline, err := b.device.CreateGraphicsPipeline(&sdl.GPUGraphicsPipelineCreateInfo{
		TargetInfo: sdl.GPUGraphicsPipelineTargetInfo{
			ColorTargetDescriptions: []sdl.GPUColorTargetDescription{
				{Format: b.device.SwapchainTextureFormat(b.window)},
			},
		},
		VertexInputState: sdl.GPUVertexInputState{
			VertexBufferDescriptions: []sdl.GPUVertexBufferDescription{{
				Slot:      0,
				InputRate: sdl.GPU_VERTEXINPUTRATE_VERTEX,
				Pitch:     uint32(unsafe.Sizeof(triangleVertex{})),
			}},
			VertexAttributes: []sdl.GPUVertexAttribute{{
				BufferSlot: 0,
				Format:     sdl.GPU_VERTEXELEMENTFORMAT_FLOAT3,
				Location:   0,
				Offset:     0,
			}},
		},
		PrimitiveType:  sdl.GPU_PRIMITIVETYPE_TRIANGLELIST,
		VertexShader:   vert,
		FragmentShader: frag,
	})
	if err != nil {
		return fmt.Errorf("create graphics pipeline: %w", err)
	}
	b.pipeline = pipeline
	return nil
}

func (b *gpuBackend) createShader(code []byte, stage sdl.GPUShaderStage) (*sdl.GPUShader, error) {
	return b.device.CreateGPUShader(&sdl.GPUShaderCreateInfo{
		Code:       code,
		Entrypoint: "main",
		Format:     sdl.GPU_SHADERFORMAT_SPIRV,
		Stage:      stage,
	})
}

// uploadTriangle stages one clip-space triangle into a vertex buffer through a
// transfer buffer (SDL_GPU's only upload path) and submits the copy.
func (b *gpuBackend) uploadTriangle() error {
	// CCW in SDL_GPU NDC (+Y up): bottom-left, bottom-right, apex.
	vertices := []triangleVertex{
		{-0.6, -0.6, 0},
		{0.6, -0.6, 0},
		{0, 0.6, 0},
	}
	size := uint32(len(vertices)) * uint32(unsafe.Sizeof(triangleVertex{}))

	buf, err := b.device.CreateBuffer(&sdl.GPUBufferCreateInfo{
		Usage: sdl.GPU_BUFFERUSAGE_VERTEX,
		Size:  size,
	})
	if err != nil {
		return fmt.Errorf("create vertex buffer: %w", err)
	}
	b.vertexBuf = buf

	transfer, err := b.device.CreateTransferBuffer(&sdl.GPUTransferBufferCreateInfo{
		Usage: sdl.GPU_TRANSFERBUFFERUSAGE_UPLOAD,
		Size:  size,
	})
	if err != nil {
		return fmt.Errorf("create transfer buffer: %w", err)
	}
	defer b.device.ReleaseTransferBuffer(transfer)

	ptr, err := b.device.MapTransferBuffer(transfer, false)
	if err != nil {
		return fmt.Errorf("map transfer buffer: %w", err)
	}
	// Element-wise instead of copy(): inlining copy into the reinterpreted
	// destination trips an SSA rewrite bug in the Go 1.26.4 compiler (panic in
	// disjointTypes during lower).
	dst := unsafe.Slice((*triangleVertex)(mappedPointer(ptr)), len(vertices))
	//nolint:staticcheck // S1001: copy() here ICEs the Go 1.26.4 compiler, see above.
	for i := range vertices {
		dst[i] = vertices[i]
	}
	b.device.UnmapTransferBuffer(transfer)

	cmdbuf, err := b.device.AcquireCommandBuffer()
	if err != nil {
		return fmt.Errorf("acquire command buffer: %w", err)
	}
	pass := cmdbuf.BeginCopyPass()
	pass.UploadToGPUBuffer(
		&sdl.GPUTransferBufferLocation{TransferBuffer: transfer, Offset: 0},
		&sdl.GPUBufferRegion{Buffer: buf, Offset: 0, Size: size},
		false,
	)
	pass.End()
	if err := cmdbuf.Submit(); err != nil {
		return fmt.Errorf("submit upload: %w", err)
	}
	return nil
}

func (b *gpuBackend) RenderFrame(_ *scene.Scene, _ string) (time.Duration, error) {
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
		pass := cmdbuf.BeginRenderPass([]sdl.GPUColorTargetInfo{{
			Texture: swapchain.Texture,
			// Same background as the software backend: sRGB bytes 18,18,24
			// written verbatim to the UNORM swapchain.
			ClearColor: sdl.FColor{R: float32(bgR) / 255, G: float32(bgG) / 255, B: float32(bgB) / 255, A: 1},
			LoadOp:     sdl.GPU_LOADOP_CLEAR,
			StoreOp:    sdl.GPU_STOREOP_STORE,
		}}, nil)
		pass.BindGraphicsPipeline(b.pipeline)
		pass.BindVertexBuffers([]sdl.GPUBufferBinding{{Buffer: b.vertexBuf, Offset: 0}})
		pass.DrawPrimitives(3, 1, 0, 0)
		pass.End()
	}
	busy := time.Since(start)

	// Submit presents asynchronously; a minimized window (nil swapchain
	// texture) still must submit the acquired command buffer.
	if err := cmdbuf.Submit(); err != nil {
		return 0, fmt.Errorf("submit frame: %w", err)
	}
	return busy, nil
}

// Resize is a no-op: the swapchain tracks the window size on acquire.
func (b *gpuBackend) Resize(int, int) error { return nil }

// SetObjectAxes stores the toggle; the GPU line pass consumes it from F4.3 on.
func (b *gpuBackend) SetObjectAxes(on bool) { b.objectAxes = on }

func (b *gpuBackend) Close() {
	if b.device == nil {
		return
	}
	_ = b.device.WaitForIdle()
	if b.vertexBuf != nil {
		b.device.ReleaseBuffer(b.vertexBuf)
		b.vertexBuf = nil
	}
	if b.pipeline != nil {
		b.device.ReleaseGraphicsPipeline(b.pipeline)
		b.pipeline = nil
	}
	if b.window != nil {
		b.device.ReleaseWindow(b.window)
		b.window = nil
	}
	b.device.Destroy()
	b.device = nil
}
