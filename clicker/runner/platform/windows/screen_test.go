//go:build windows

package runner

import (
	"image"
	"testing"
)

func TestScreenSize(t *testing.T) {
	w, h := ScreenSize()
	if w <= 0 || h <= 0 {
		t.Errorf("ScreenSize() = %dx%d; want positive dimensions", w, h)
	}
	t.Logf("Screen size: %dx%d", w, h)
}

func TestCaptureFullScreen(t *testing.T) {
	img, err := CaptureFullScreen()
	if err != nil {
		t.Fatalf("CaptureFullScreen() error = %v", err)
	}
	if img == nil {
		t.Fatal("CaptureFullScreen() = nil; want image")
	}
	if img.Bounds().Empty() {
		t.Error("CaptureFullScreen() returned empty image")
	}
	t.Logf("Full screen capture: %v", img.Bounds())
}

func TestCaptureScreenRegion(t *testing.T) {
	// Test with a small region
	roi := ScreenROI{X: 0, Y: 0, W: 100, H: 100}
	img, err := CaptureScreenRegion(roi)
	if err != nil {
		t.Fatalf("CaptureScreenRegion() error = %v", err)
	}
	if img == nil {
		t.Fatal("CaptureScreenRegion() = nil; want image")
	}
	expectedBounds := image.Rect(0, 0, 100, 100)
	if img.Bounds() != expectedBounds {
		t.Errorf("CaptureScreenRegion() bounds = %v; want %v", img.Bounds(), expectedBounds)
	}
	t.Logf("Screen region capture: %v", img.Bounds())
}


