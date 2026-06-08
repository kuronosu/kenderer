package present

import (
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGIFRunProducesAnimatedGIF(t *testing.T) {
	cfg := Config{Width: 4, Height: 4, FPS: 10, Duration: 300 * time.Millisecond}

	calls := 0
	frame := func(time.Duration) image.Image {
		calls++
		img := image.NewRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))
		draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 200, G: 50, B: 50, A: 255}}, image.Point{}, draw.Src)
		return img
	}

	path := filepath.Join(t.TempDir(), "out.gif")
	if err := (GIF{Path: path, Loop: 0}).Run(frame, cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}

	const wantFrames = 3 // 10 fps * 0.3 s
	if calls != wantFrames {
		t.Errorf("FrameFunc called %d times, want %d", calls, wantFrames)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	decoded, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}
	if len(decoded.Image) != wantFrames {
		t.Errorf("decoded frames = %d, want %d", len(decoded.Image), wantFrames)
	}
	if decoded.LoopCount != 0 {
		t.Errorf("loop count = %d, want 0 (forever)", decoded.LoopCount)
	}
	if b := decoded.Image[0].Bounds(); b.Dx() != cfg.Width || b.Dy() != cfg.Height {
		t.Errorf("frame bounds = %v, want %dx%d", b, cfg.Width, cfg.Height)
	}
	for i, d := range decoded.Delay {
		if d != 10 { // round(100/10)
			t.Errorf("delay[%d] = %d, want 10", i, d)
		}
	}
}

func TestGIFRunRejectsBadConfig(t *testing.T) {
	frame := func(time.Duration) image.Image { return image.NewRGBA(image.Rect(0, 0, 1, 1)) }
	path := filepath.Join(t.TempDir(), "bad.gif")

	if err := (GIF{Path: path}).Run(frame, Config{FPS: 0, Duration: time.Second}); err == nil {
		t.Error("expected error for FPS <= 0")
	}
	if err := (GIF{Path: path}).Run(frame, Config{FPS: 10, Duration: 0}); err == nil {
		t.Error("expected error for Duration <= 0")
	}
}
