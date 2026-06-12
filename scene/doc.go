// Package scene describes what to render: a perspective Camera, Objects (each a
// mesh with a translate-rotate-scale transform whose rotation is a quaternion,
// plus a material), a single directional light with an ambient term, and
// colored world-space line Segments (Scene.Lines). It is plain data consumed by
// the rendering backends and holds no rendering logic of its own. The axis
// helpers — WorldAxes for the world frame, ObjectAxes for a per-object gizmo —
// are the single source of those segments, so every backend draws the same
// overlay with the same AxisColor conventions.
package scene
