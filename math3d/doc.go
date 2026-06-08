// Package math3d provides the linear-algebra primitives for the renderer:
// 2-, 3- and 4-component vectors and 3x3 / 4x4 matrices, plus the standard
// model/view/projection and viewport transforms.
//
// Conventions (shared by the whole pipeline):
//
//   - Right-handed coordinate system, OpenGL style: +X right, +Y up, and the
//     camera looks down its local -Z axis.
//   - Matrices are stored row-major in a [4][4]float64 (element (row, col) is
//     m[row][col]) and use the column-vector convention: a point is transformed
//     as v' = m.MulVec4(v), and composite transforms are built right-to-left,
//     e.g. MVP = P.Mul(V).Mul(M).
//   - Perspective maps view-space z in [-near, -far] to clip/NDC z in [-1, 1]
//     (with w_clip = -z_view). Viewport then maps NDC z in [-1, 1] to window z
//     in [0, 1] (0 = near, 1 = far) and flips Y so screen rows grow downward.
//   - Normals are transformed by NormalMatrix(model), the inverse transpose of
//     the model matrix' upper-left 3x3, so they stay perpendicular to their
//     surface under non-uniform scaling.
//   - Rotations are also available as quaternions (Quat), in the same
//     right-handed convention: QuatFromAxisAngle(V3(0,1,0), t).Mat4() == RotateY(t)
//     and QuatFromEuler(x,y,z) == Rz·Ry·Rx. Quat.Mat4 is zero-value-safe, so a
//     zero quaternion yields the identity.
//
// The package is free of global state and depends only on the standard library.
package math3d
