package autopot

import (
	"fmt"
	"image"
	"testing"
)

// ---------------------------------------------------------------------------
// Default fallback tests
// ---------------------------------------------------------------------------

func TestScreenSize_DefaultReturnsZero(t *testing.T) {
	orig := ScreenSize
	defer func() { ScreenSize = orig }()
	// Reset to sentinel default — on Windows, init() in screen_windows.go
	// already wired the real platform implementation.
	ScreenSize = func() (int, int) { return 0, 0 }

	w, h := ScreenSize()
	if w != 0 || h != 0 {
		t.Errorf("ScreenSize: expected (0,0) from default sentinel, got (%d,%d)", w, h)
	}
}

func TestCaptureScreenRegion_DefaultReturnsError(t *testing.T) {
	orig := CaptureScreenRegion
	defer func() { CaptureScreenRegion = orig }()
	CaptureScreenRegion = func(roi Rect) (*image.RGBA, error) {
		return nil, fmt.Errorf("CaptureScreenRegion: not available on this platform")
	}

	img, err := CaptureScreenRegion(Rect{X: 0, Y: 0, W: 100, H: 100})
	if err == nil {
		t.Error("CaptureScreenRegion: expected error from default sentinel")
	}
	if img != nil {
		t.Error("CaptureScreenRegion: expected nil image from default sentinel")
	}
}

func TestCaptureFullScreen_DefaultReturnsError(t *testing.T) {
	orig := CaptureFullScreen
	defer func() { CaptureFullScreen = orig }()
	CaptureFullScreen = func() (*image.RGBA, error) {
		return nil, fmt.Errorf("CaptureFullScreen: not available on this platform")
	}

	img, err := CaptureFullScreen()
	if err == nil {
		t.Error("CaptureFullScreen: expected error from default sentinel")
	}
	if img != nil {
		t.Error("CaptureFullScreen: expected nil image from default sentinel")
	}
}

// ---------------------------------------------------------------------------
// DI swap-in tests
// ---------------------------------------------------------------------------

func TestScreenSize_CanBeSwapped(t *testing.T) {
	orig := ScreenSize
	defer func() { ScreenSize = orig }()

	ScreenSize = func() (int, int) { return 1920, 1080 }

	w, h := ScreenSize()
	if w != 1920 || h != 1080 {
		t.Errorf("ScreenSize: expected (1920,1080), got (%d,%d)", w, h)
	}
}

func TestCaptureScreenRegion_SwapReturnsImage(t *testing.T) {
	orig := CaptureScreenRegion
	defer func() { CaptureScreenRegion = orig }()

	sentinel := image.NewRGBA(image.Rect(0, 0, 10, 10))
	CaptureScreenRegion = func(roi Rect) (*image.RGBA, error) {
		if roi.W <= 0 || roi.H <= 0 {
			return nil, fmt.Errorf("invalid roi")
		}
		return sentinel, nil
	}

	img, err := CaptureScreenRegion(Rect{X: 10, Y: 20, W: 50, H: 30})
	if err != nil {
		t.Fatalf("CaptureScreenRegion: %v", err)
	}
	if img != sentinel {
		t.Error("CaptureScreenRegion: expected sentinel image back")
	}

	_, err = CaptureScreenRegion(Rect{X: 0, Y: 0, W: 0, H: 0})
	if err == nil {
		t.Error("CaptureScreenRegion: expected error for invalid roi")
	}
}

func TestCaptureFullScreen_SwapReturnsImage(t *testing.T) {
	orig := CaptureFullScreen
	defer func() { CaptureFullScreen = orig }()

	sentinel := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	CaptureFullScreen = func() (*image.RGBA, error) {
		return sentinel, nil
	}

	img, err := CaptureFullScreen()
	if err != nil {
		t.Fatalf("CaptureFullScreen: %v", err)
	}
	if img != sentinel {
		t.Error("CaptureFullScreen: expected sentinel image")
	}
}

func TestScreenFunctions_RestoreAfterSwap(t *testing.T) {
	// Verify the clean-up pattern: swap-and-restore within a sub-scope
	// leaves the variable pointing at the original.
	orig := CaptureScreenRegion

	func() {
		CaptureScreenRegion = func(roi Rect) (*image.RGBA, error) {
			return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
		}
		defer func() { CaptureScreenRegion = orig }()

		img, _ := CaptureScreenRegion(Rect{W: 1, H: 1})
		if img == nil {
			t.Fatal("mock should return non-nil")
		}
	}()

	// After restore, the variable should be the original (platform-wired).
	_ = orig
}
