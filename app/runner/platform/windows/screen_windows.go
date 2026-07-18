//go:build windows

package runner

import (
	"fmt"
	"image"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	gdi32                = windows.NewLazySystemDLL("gdi32.dll")
	procCreateCompatDC   = gdi32.NewProc("CreateCompatibleDC")
	procCreateDIBSection = gdi32.NewProc("CreateDIBSection")
	procSelectObject     = gdi32.NewProc("SelectObject")
	procBitBlt           = gdi32.NewProc("BitBlt")
	procDeleteDC         = gdi32.NewProc("DeleteDC")
	procDeleteObject     = gdi32.NewProc("DeleteObject")
)

const (
	gdiSrcCopy     = 0x00CC0020
	dibRGBColors   = 0
	biRGB          = 0
)

// ScreenSize returns the primary monitor dimensions.
func ScreenSize() (w, h int) {
	sw, _, _ := procGetSystemMetrics.Call(0)
	sh, _, _ := procGetSystemMetrics.Call(1)
	return int(sw), int(sh)
}

// CaptureScreenRegion grabs a screen rectangle into an RGBA image.
//
// Uses CreateDIBSection + BitBlt so pixels land directly in a GDI-owned
// buffer. This avoids CreateCompatibleBitmap + GetDIBits, which fails on
// some GPU drivers / HDR / 10-bit displays even when the bitmap is correctly
// deselected from its DC (MSDN requirement).
func CaptureScreenRegion(roi ScreenROI) (*image.RGBA, error) {
	if roi.W <= 0 || roi.H <= 0 {
		return nil, fmt.Errorf("invalid capture roi %v", roi)
	}

	hdcScreen, _, err := procGetDC.Call(0)
	if hdcScreen == 0 {
		return nil, fmt.Errorf("GetDC failed: %w", err)
	}
	defer procReleaseDC.Call(0, hdcScreen)

	hdcMem, _, err := procCreateCompatDC.Call(hdcScreen)
	if hdcMem == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed: %w", err)
	}
	defer procDeleteDC.Call(hdcMem)

	var bmi bitmapInfo
	bmi.Header.Size = uint32(unsafe.Sizeof(bmi.Header))
	bmi.Header.Width = int32(roi.W)
	bmi.Header.Height = -int32(roi.H) // top-down
	bmi.Header.Planes = 1
	bmi.Header.BitCount = 32
	bmi.Header.Compression = biRGB

	var bits unsafe.Pointer
	hbmp, _, err := procCreateDIBSection.Call(
		hdcMem,
		uintptr(unsafe.Pointer(&bmi)),
		dibRGBColors,
		uintptr(unsafe.Pointer(&bits)),
		0,
		0,
	)
	if hbmp == 0 || bits == nil {
		return nil, fmt.Errorf("CreateDIBSection failed: %w", err)
	}
	defer procDeleteObject.Call(hbmp)

	old, _, err := procSelectObject.Call(hdcMem, hbmp)
	if old == 0 {
		return nil, fmt.Errorf("SelectObject failed: %w", err)
	}
	defer procSelectObject.Call(hdcMem, old)

	r, _, err := procBitBlt.Call(
		hdcMem, 0, 0, uintptr(roi.W), uintptr(roi.H),
		hdcScreen, uintptr(roi.X), uintptr(roi.Y), gdiSrcCopy,
	)
	if r == 0 {
		return nil, fmt.Errorf("BitBlt failed: %w", err)
	}

	n := roi.W * roi.H * 4
	src := unsafe.Slice((*byte)(bits), n)
	out := image.NewRGBA(image.Rect(0, 0, roi.W, roi.H))
	// DIB section is BGRA; convert to RGBA.
	for i := 0; i < n; i += 4 {
		out.Pix[i+0] = src[i+2]
		out.Pix[i+1] = src[i+1]
		out.Pix[i+2] = src[i+0]
		out.Pix[i+3] = src[i+3]
	}
	return out, nil
}

// CaptureFullScreen grabs the entire primary monitor area into an RGBA
// image. Used by the status-panel recognition path: FindStatusPanel needs
// the whole screen so it can locate the panel via template matching even
// when the in-game UI has drifted off its default top-left position.
//
// Returns an error if the screen size can't be queried or the BitBlt copy
// fails. Callers should treat failures as transient and retry on the next
// tick — the capture loop is upstream, not this helper.
func CaptureFullScreen() (*image.RGBA, error) {
	sw, sh := ScreenSize()
	if sw <= 0 || sh <= 0 {
		return nil, fmt.Errorf("invalid screen size %dx%d", sw, sh)
	}
	return CaptureScreenRegion(ScreenROI{X: 0, Y: 0, W: sw, H: sh})
}

// ScreenROI defines a rectangular region for screen capture.
type ScreenROI struct {
	X, Y, W, H int
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
}
