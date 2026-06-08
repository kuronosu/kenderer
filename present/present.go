package present

import (
	"image"
	"time"
)

// Config describes the animation to present.
type Config struct {
	Width, Height int
	FPS           int
	Duration      time.Duration
}

// FrameFunc renders the frame at animation time t. It must be deterministic in t
// so that different presentation backends produce identical motion.
type FrameFunc func(t time.Duration) image.Image

// Presenter consumes frames from a FrameFunc and presents them (for example, to
// a file). The implementation decides how animation time advances.
type Presenter interface {
	Run(frame FrameFunc, cfg Config) error
}
