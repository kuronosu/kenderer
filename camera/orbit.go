// Package camera provides interactive cameras driven by input.Frame. It is part
// of the core: it has no SDL or windowing dependency, consuming the
// backend-agnostic input types and writing a pose into a scene.Camera.
package camera

import (
	"math"

	"github.com/kuronosu/kenderer/input"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/scene"
)

// OrbitCamera orbits a target point using spherical coordinates: yaw around the
// world +Y axis and pitch up/down. Left-drag orbits, the wheel zooms, and
// middle-drag (or shift+left-drag) pans the target. Quaternions are reserved for
// a future fly camera; orbit is naturally expressed in spherical angles.
type OrbitCamera struct {
	Target   math3d.Vec3
	Distance float64
	Yaw      float64 // radians, around +Y
	Pitch    float64 // radians, + is up; clamped to ±MaxPitch

	OrbitSens float64 // radians per pixel of drag
	PanSens   float64 // world units per pixel, per unit of distance
	ZoomStep  float64 // fractional distance change per wheel notch

	MinDistance, MaxDistance float64
	MaxPitch                 float64 // radians
}

// NewOrbitCamera returns an OrbitCamera looking at target from the given initial
// distance, with sensible defaults for sensitivity and limits. MinDistance is
// kept above the typical near plane so zooming never enters the object.
func NewOrbitCamera(target math3d.Vec3, distance float64) OrbitCamera {
	return OrbitCamera{
		Target:      target,
		Distance:    distance,
		OrbitSens:   0.01,
		PanSens:     0.0015,
		ZoomStep:    0.1,
		MinDistance: 1.5,
		MaxDistance: 100,
		MaxPitch:    89 * math.Pi / 180,
	}
}

// Update advances the camera from one frame of input. The signs here are a
// matter of feel; flip them if a drag or the wheel feels inverted.
func (c *OrbitCamera) Update(in input.Frame) {
	switch {
	case in.Middle || (in.Left && in.Shift): // pan
		right, up := c.basis()
		k := c.PanSens * c.Distance
		c.Target = c.Target.
			Add(right.Scale(-in.MouseDX * k)).
			Add(up.Scale(in.MouseDY * k))
	case in.Left: // orbit
		c.Yaw -= in.MouseDX * c.OrbitSens
		c.Pitch -= in.MouseDY * c.OrbitSens
		c.Pitch = clamp(c.Pitch, -c.MaxPitch, c.MaxPitch)
	}

	if in.Wheel != 0 {
		c.Distance = clamp(c.Distance*math.Pow(1-c.ZoomStep, in.Wheel), c.MinDistance, c.MaxDistance)
	}
}

// Eye returns the world-space camera position.
func (c OrbitCamera) Eye() math3d.Vec3 {
	cp := math.Cos(c.Pitch)
	dir := math3d.V3(cp*math.Sin(c.Yaw), math.Sin(c.Pitch), cp*math.Cos(c.Yaw))
	return c.Target.Add(dir.Scale(c.Distance))
}

// Apply writes the camera's pose (Eye, Target, Up) into cam, leaving the
// projection parameters (FOVY, Near, Far) untouched.
func (c OrbitCamera) Apply(cam *scene.Camera) {
	cam.Eye = c.Eye()
	cam.Target = c.Target
	cam.Up = math3d.V3(0, 1, 0)
}

// basis returns the camera's right and up vectors in world space.
func (c OrbitCamera) basis() (right, up math3d.Vec3) {
	worldUp := math3d.V3(0, 1, 0)
	forward := c.Target.Sub(c.Eye()).Normalize()
	right = forward.Cross(worldUp).Normalize()
	up = right.Cross(forward)
	return right, up
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
