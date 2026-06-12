package pipeline

import (
	"image/color"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/kuronosu/kenderer/framebuffer"
	"github.com/kuronosu/kenderer/geometry"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/raster"
	"github.com/kuronosu/kenderer/scene"
	"github.com/kuronosu/kenderer/shading"
)

// CullMode selects which triangle facing is discarded before rasterization.
type CullMode int

const (
	CullBack  CullMode = iota // discard back-facing triangles (default)
	CullFront                 // discard front-facing triangles
	CullNone                  // draw both facings
)

// Options configures a Renderer.
type Options struct {
	Width, Height int
	Cull          CullMode
	Background    color.RGBA
	// Workers caps the goroutines used to fill (rasterize) a frame. 0 means auto
	// (runtime.GOMAXPROCS); 1 forces the serial path. The rendered image is
	// identical for any value — only throughput changes.
	Workers int
}

// Renderer renders scenes into a single framebuffer that is reused across frames.
type Renderer struct {
	opts     Options
	fb       *framebuffer.Buffer
	viewport math3d.Mat4

	workers int
	// prepared is the per-frame buffer of screen-space triangles produced by the
	// serial geometry phase and consumed by the parallel fill. It is reused across
	// frames (reset to length 0, grown on demand) so the hot path makes no garbage.
	prepared []raster.Prepared
	// bandCursor hands scanline bands to fill workers (atomic = dynamic schedule).
	bandCursor atomic.Int64
	// objectAxes draws each object's local coordinate frame in the line pass; see
	// SetObjectAxes. World axes (and any other lines) ride along in Scene.Lines.
	objectAxes bool
}

// SetObjectAxes enables or disables drawing each object's local coordinate frame
// (X red, Y green, Z blue) in the line pass. It is off by default; a viewer can
// toggle it at runtime between frames. World axes are not controlled here — they
// are caller-provided segments in Scene.Lines (see scene.WorldAxes).
func (r *Renderer) SetObjectAxes(on bool) { r.objectAxes = on }

// NewRenderer creates a Renderer with the given options. Width and Height must be
// positive.
func NewRenderer(opts Options) *Renderer {
	return &Renderer{
		opts:     opts,
		fb:       framebuffer.New(opts.Width, opts.Height),
		viewport: math3d.Viewport(0, 0, float64(opts.Width), float64(opts.Height)),
		workers:  opts.Workers,
	}
}

// Resize reallocates the framebuffer to w x h pixels and recomputes the viewport.
// The projection follows automatically: Render derives the aspect ratio from the
// current size on every call. Width and Height must be positive.
func (r *Renderer) Resize(w, h int) {
	r.opts.Width, r.opts.Height = w, h
	r.fb = framebuffer.New(w, h)
	r.viewport = math3d.Viewport(0, 0, float64(w), float64(h))
}

// Render draws the scene into the Renderer's framebuffer and returns it.
//
// The returned *framebuffer.Buffer (and the image from its Image method) is
// reused on every call and is valid only until the next call to Render. A caller
// that needs to keep or share a frame must copy or encode it first; the GIF
// presenter is safe because it quantizes each frame before requesting the next.
func (r *Renderer) Render(s scene.Scene) *framebuffer.Buffer {
	r.fb.Clear(r.opts.Background)

	aspect := float64(r.opts.Width) / float64(r.opts.Height)
	viewProj := s.Camera.Projection(aspect).Mul(s.Camera.View())

	// Phase 1 (serial): transform, clip, cull and set up every triangle into the
	// reused prepared buffer, in the exact order the serial path would rasterize
	// them (object, then triangle, then clip-fan order). This ordering is what
	// makes the parallel fill bit-identical: each pixel sees its covering
	// triangles in list order regardless of which worker fills it.
	r.prepared = r.prepared[:0]
	for _, obj := range s.Objects {
		model := obj.Transform.Matrix()
		mvp := viewProj.Mul(model)
		normalMat := math3d.NormalMatrix(model)
		// Box the shader once per object; the interface value is then copied (not
		// re-boxed) into each prepared triangle and only ever read during fill.
		var shader shading.Shader = shading.Lambert{
			Light:    s.Light,
			Ambient:  s.Ambient,
			Material: obj.Material,
		}
		mesh := obj.Mesh
		for i := 0; i < mesh.NumTriangles(); i++ {
			a, b, c := mesh.Triangle(i)
			r.prepareTriangle(a, b, c, model, mvp, normalMat, shader, obj.Smooth)
		}
	}

	// Phase 2 (parallel): fill the prepared triangles across disjoint scanline bands.
	r.fill()

	// Line pass (serial): drawn after the fill barrier, reading the now-complete
	// z-buffer so lines are correctly occluded. It is a no-op (and the framebuffer
	// is untouched) when there are no lines to draw, keeping the output identical.
	r.drawLines(s, viewProj)
	return r.fb
}

// prepareTriangle runs the per-triangle geometry stages: vertex transform, normal
// selection and frustum clip, appending each resulting sub-triangle to the
// prepared buffer (via prepare). It does no rasterization; the fill phase does.
//
// When smooth is false (flat shading) the per-vertex normals are replaced by the
// triangle's geometric face normal, computed once on the original (pre-clip)
// triangle in world space, so the clipper carries a constant normal to every
// fragment. Recomputing it per clipped sub-triangle would be unstable on the
// near-degenerate slivers clipping can produce. When smooth is true the
// interpolated per-vertex normals (already transformed by the normal matrix) are
// kept, so CombineFragment yields a per-fragment normal and Lambert produces
// Phong shading; the face-normal path also serves as the fallback for meshes
// without usable normals.
func (r *Renderer) prepareTriangle(a, b, c geometry.Vertex, model, mvp math3d.Mat4, normalMat math3d.Mat3, shader shading.Shader, smooth bool) {
	v0 := processVertex(a, model, mvp, normalMat)
	v1 := processVertex(b, model, mvp, normalMat)
	v2 := processVertex(c, model, mvp, normalMat)

	if !smooth {
		n := faceNormal(v0.Frag.WorldPos, v1.Frag.WorldPos, v2.Frag.WorldPos)
		v0.Frag.Normal = n
		v1.Frag.Normal = n
		v2.Frag.Normal = n
	}

	for _, tri := range clipTriangle(v0, v1, v2) {
		r.prepare(tri, shader)
	}
}

// processVertex transforms a mesh vertex into clip space and gathers its
// world-space varyings.
func processVertex(v geometry.Vertex, model, mvp math3d.Mat4, normalMat math3d.Mat3) clipVertex {
	pos4 := v.Position.Vec4(1)
	return clipVertex{
		Pos: mvp.MulVec4(pos4),
		Frag: shading.Fragment{
			WorldPos: model.MulVec4(pos4).XYZ(),
			Normal:   normalMat.MulVec3(v.Normal),
			UV:       v.UV,
			Color:    v.Color,
		},
	}
}

// prepare performs the perspective divide, viewport transform and backface cull
// for one clipped triangle, then appends it — with its rasterization setup — to
// the prepared buffer. Culled and degenerate (zero-area) triangles are dropped,
// exactly as the serial path skipped them, so they never reach the fill phase.
func (r *Renderer) prepare(tri [3]clipVertex, shader shading.Shader) {
	var rv [3]raster.Vertex
	for i, cv := range tri {
		screen, invW := r.toScreen(cv.Pos)
		rv[i] = raster.Vertex{Pos: screen, InvW: invW, Frag: cv.Frag}
	}
	area := raster.SignedArea(rv[0].Pos.XY(), rv[1].Pos.XY(), rv[2].Pos.XY())
	if r.culls(area) {
		return
	}
	if p, ok := raster.Prepare(rv[0], rv[1], rv[2], area, r.opts.Width, r.opts.Height, shader); ok {
		r.prepared = append(r.prepared, p)
	}
}

// bandsPerWorker is the average number of scanline bands handed to each fill
// worker. Using several bands per worker lets the atomic dispatch balance uneven
// triangle coverage (dynamic scheduling); too many would add per-band overhead.
const bandsPerWorker = 8

// fill rasterizes the prepared triangles into the framebuffer. It splits the rows
// into bands handed out by an atomic counter to a pool of workers; each worker
// loops, claiming bands until they run out, and rasterizes every prepared
// triangle clamped to its band's rows. Because bands are disjoint row ranges, no
// two workers ever touch the same pixel or depth cell, so the framebuffer needs
// no locking. With workers == 1 the bands run inline on the caller — exactly the
// serial path. The rendered image is identical for any worker or band count.
func (r *Renderer) fill() {
	workers := r.workers
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	h := r.opts.Height

	if workers == 1 || h <= 1 || len(r.prepared) == 0 {
		for k := range r.prepared {
			raster.DrawBand(r.fb, &r.prepared[k], 0, h)
		}
		return
	}

	bandRows := h / (workers * bandsPerWorker)
	if bandRows < 1 {
		bandRows = 1
	}
	numBands := (h + bandRows - 1) / bandRows // ceil(h / bandRows)

	r.bandCursor.Store(0)
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for {
				b := int(r.bandCursor.Add(1)) - 1
				if b >= numBands {
					return
				}
				y0 := b * bandRows
				y1 := min(y0+bandRows, h)
				for k := range r.prepared {
					raster.DrawBand(r.fb, &r.prepared[k], y0, y1)
				}
			}
		}()
	}
	wg.Wait()
}

// culls reports whether a triangle with the given screen-space signed area is
// removed under the configured cull mode. With the Y-flipping viewport, a
// front-facing (CCW in NDC) triangle has negative screen-space area.
func (r *Renderer) culls(area float64) bool {
	switch r.opts.Cull {
	case CullBack:
		return area > 0
	case CullFront:
		return area < 0
	default:
		return false
	}
}

func faceNormal(a, b, c math3d.Vec3) math3d.Vec3 {
	return b.Sub(a).Cross(c.Sub(a)).Normalize()
}
