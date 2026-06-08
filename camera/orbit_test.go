package camera

import (
	"math"
	"testing"

	"github.com/kuronosu/kenderer/input"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/scene"
)

const eps = 1e-9

func TestOrbitDrag(t *testing.T) {
	c := NewOrbitCamera(math3d.V3(0, 0, 0), 5)
	y0, p0 := c.Yaw, c.Pitch
	c.Update(input.Frame{Left: true, MouseDX: 10, MouseDY: 4})

	if want := y0 - 10*c.OrbitSens; math.Abs(c.Yaw-want) > eps {
		t.Errorf("yaw = %v, want %v", c.Yaw, want)
	}
	if want := p0 - 4*c.OrbitSens; math.Abs(c.Pitch-want) > eps {
		t.Errorf("pitch = %v, want %v", c.Pitch, want)
	}
}

func TestOrbitPitchClamp(t *testing.T) {
	c := NewOrbitCamera(math3d.V3(0, 0, 0), 5)
	c.Update(input.Frame{Left: true, MouseDY: -100000}) // hard pitch up
	if c.Pitch > c.MaxPitch+eps {
		t.Errorf("pitch %v exceeds +MaxPitch %v", c.Pitch, c.MaxPitch)
	}
	c.Update(input.Frame{Left: true, MouseDY: 100000}) // hard pitch down
	if c.Pitch < -c.MaxPitch-eps {
		t.Errorf("pitch %v below -MaxPitch %v", c.Pitch, -c.MaxPitch)
	}
}

func TestZoomDirectionAndClamps(t *testing.T) {
	c := NewOrbitCamera(math3d.V3(0, 0, 0), 5)

	c.Update(input.Frame{Wheel: 1}) // wheel up -> zoom in -> smaller distance
	if c.Distance >= 5 {
		t.Errorf("wheel up should reduce distance, got %v", c.Distance)
	}
	c.Update(input.Frame{Wheel: 100000}) // clamp to min
	if math.Abs(c.Distance-c.MinDistance) > eps {
		t.Errorf("distance = %v, want MinDistance %v", c.Distance, c.MinDistance)
	}
	c.Update(input.Frame{Wheel: -100000}) // clamp to max
	if math.Abs(c.Distance-c.MaxDistance) > eps {
		t.Errorf("distance = %v, want MaxDistance %v", c.Distance, c.MaxDistance)
	}
}

func TestEyeAndApply(t *testing.T) {
	c := NewOrbitCamera(math3d.V3(0, 0, 0), 5) // yaw = pitch = 0
	if got := c.Eye(); math.Abs(got.X) > eps || math.Abs(got.Y) > eps || math.Abs(got.Z-5) > eps {
		t.Errorf("eye = %v, want (0,0,5)", got)
	}

	var cam scene.Camera
	c.Apply(&cam)
	if cam.Eye != c.Eye() || cam.Target != c.Target || cam.Up != math3d.V3(0, 1, 0) {
		t.Errorf("Apply wrote eye=%v target=%v up=%v", cam.Eye, cam.Target, cam.Up)
	}
}

func TestPanMovesTarget(t *testing.T) {
	c := NewOrbitCamera(math3d.V3(0, 0, 0), 5) // yaw=pitch=0 => right=+X, up=+Y
	c.Update(input.Frame{Middle: true, MouseDX: 10})
	if c.Target.X >= 0 {
		t.Errorf("middle-drag right should move the target along -X, got %v", c.Target.X)
	}
	if math.Abs(c.Target.Y) > eps || math.Abs(c.Target.Z) > eps {
		t.Errorf("horizontal pan should not move Y/Z; got target %v", c.Target)
	}
}
