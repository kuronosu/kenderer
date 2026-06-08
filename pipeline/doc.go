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
// A Renderer reuses one framebuffer across frames; see Render for the returned
// buffer's lifetime.
package pipeline
