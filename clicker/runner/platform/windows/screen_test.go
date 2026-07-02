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

func TestPlayerBarSearchROI(t *testing.T) {
	sw, sh := 1920, 1080 // Typical screen size
	roi := PlayerBarSearchROI(sw, sh)
	if roi.W <= 0 || roi.H <= 0 {
		t.Errorf("PlayerBarSearchROI() returned invalid dimensions: %v", roi)
	}
	// Check that the region is in the lower half of the screen
	if roi.Y < sh/2 {
		t.Errorf("PlayerBarSearchROI() Y coordinate (%d) should be in lower half of screen (%d)", roi.Y, sh)
	}
	t.Logf("Player bar search ROI: %v", roi)
}
