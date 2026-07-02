package autopot

import (
	"testing"
)

func TestPlayerBarSearchROI(t *testing.T) {
	sw, sh := 1920, 1080 // Typical screen size
	roi := PlayerBarSearchROI(sw, sh)
	if roi.W <= 0 || roi.H <= 0 {
		t.Errorf("PlayerBarSearchROI() returned invalid dimensions: %v", roi)
	}
	if roi.W != 110 || roi.H != 60 {
		t.Errorf("PlayerBarSearchROI() = %dx%d; want 110x60", roi.W, roi.H)
	}
	// Centre of ROI should be at (screenW/2, screenH/2 + 15).
	expectedX := sw/2 - 55
	expectedY := sh/2 + 15 - 30
	if roi.X != expectedX || roi.Y != expectedY {
		t.Errorf("PlayerBarSearchROI() = (%d,%d); want (%d,%d)", roi.X, roi.Y, expectedX, expectedY)
	}
	t.Logf("Player bar search ROI: %v", roi)
}
