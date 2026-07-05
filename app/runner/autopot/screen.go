package autopot

import (
	"fmt"
	"image"
)

// ScreenSize returns the primary monitor dimensions (width, height).
// Defaults to 0,0 with an error; the real implementation is wired via
// init() in screen_windows.go.
var ScreenSize = func() (int, int) {
	return 0, 0
}

// CaptureScreenRegion captures the given screen rectangle into an RGBA image.
// Defaults to nil with an error; the real implementation is wired via
// init() in screen_windows.go.
var CaptureScreenRegion = func(roi Rect) (*image.RGBA, error) {
	return nil, fmt.Errorf("CaptureScreenRegion: not available on this platform")
}

// CaptureFullScreen captures the entire primary monitor into an RGBA image.
// Defaults to nil with an error; the real implementation is wired via
// init() in screen_windows.go.
var CaptureFullScreen = func() (*image.RGBA, error) {
	return nil, fmt.Errorf("CaptureFullScreen: not available on this platform")
}
