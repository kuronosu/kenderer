// Package scene describes what to render: a perspective Camera, Objects (each a
// mesh with a translate-rotate-scale transform whose rotation is a quaternion,
// plus a material), and a single directional light with an ambient term. It is
// plain data consumed by the pipeline package and holds no rendering logic of
// its own.
package scene
