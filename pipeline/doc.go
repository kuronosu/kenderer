// Package pipeline orchestrates the software render pipeline, turning a
// scene.Scene into a rendered framebuffer. For each object it runs the classic
// stages: vertex transform (model/view/projection into clip space), frustum
// clipping, perspective division, viewport mapping, backface culling and
// rasterization.
//
// Conventions it enforces:
//
//   - Clipping is Sutherland-Hodgman against all six clip-space frustum planes
//     (w+x, w-x, w+y, w-y, w+z, w-z >= 0), so triangles crossing the near plane
//     never reach the perspective divide with a non-positive w.
//   - Flat shading: a single world-space face normal is computed once on the
//     original (pre-clip) triangle and carried to every clipped fragment.
//   - Backface culling uses the screen-space signed area; with the Y-flipping
//     viewport, a front face (CCW in NDC) has negative area.
//
// Render runs in two phases. The geometry phase is serial: it transforms, clips,
// culls and sets up every triangle into a reused buffer of screen-space
// "prepared" triangles, in the order the triangles would be rasterized. The fill
// phase is parallel: the framebuffer rows are split into scanline bands handed to
// a pool of workers by an atomic counter (dynamic scheduling), and each worker
// rasterizes the prepared triangles clamped to the bands it claims. Bands are
// disjoint row ranges, so workers never touch the same pixel or depth cell and
// the framebuffer needs no locking. Because each pixel is owned by one worker and
// every worker walks the prepared list in the same order, the depth test and
// top-left fill rule resolve ties exactly as in serial: the image is bit-identical
// for any worker count (Options.Workers; 0 = GOMAXPROCS, 1 = the serial path).
//
// After the fill, a serial line pass draws any colored world-space segments
// (Scene.Lines, e.g. scene.WorldAxes) and, when SetObjectAxes is on, each object's
// local coordinate frame (scene.ObjectAxes). It runs after the fill barrier so it
// reads the complete
// z-buffer (lines are correctly occluded) and is deterministic regardless of worker
// count. Segments are clipped against the same six frustum planes as triangles —
// parametrically (Liang-Barsky) before the perspective divide, so a segment
// crossing the near plane is never divided by a non-positive w. Lines depth-test
// but write no depth, and the pass is a no-op when there is nothing to draw, so the
// feature is inert and the output unchanged unless a caller opts in.
//
// A Renderer reuses one framebuffer across frames; see Render for the returned
// buffer's lifetime.
package pipeline
