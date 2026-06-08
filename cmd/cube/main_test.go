package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunWritesGIF(t *testing.T) {
	out := filepath.Join(t.TempDir(), "cube.gif")

	if err := run(32, 32, 8, 250*time.Millisecond, out, 50); err != nil {
		t.Fatalf("run: %v", err)
	}

	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output GIF is empty")
	}
}

func TestRunRejectsBadSize(t *testing.T) {
	out := filepath.Join(t.TempDir(), "cube.gif")
	if err := run(0, 32, 8, time.Second, out, 50); err == nil {
		t.Error("expected error for non-positive width")
	}
}
