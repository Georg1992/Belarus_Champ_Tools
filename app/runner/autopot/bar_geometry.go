package autopot

import (
	"image"
	"image/color"
)

// Rect is a simple integer rectangle used for bar detection and geometry.
type Rect struct {
	X, Y, W, H int
}

// clampROI clamps the ROI rect to fit within the image bounds.
func clampROI(bounds image.Rectangle, roi Rect) Rect {
	maxX := bounds.Max.X
	maxY := bounds.Max.Y
	if roi.X < bounds.Min.X {
		roi.W -= bounds.Min.X - roi.X
		roi.X = bounds.Min.X
	}
	if roi.Y < bounds.Min.Y {
		roi.H -= bounds.Min.Y - roi.Y
		roi.Y = bounds.Min.Y
	}
	if roi.X+roi.W > maxX {
		roi.W = maxX - roi.X
	}
	if roi.Y+roi.H > maxY {
		roi.H = maxY - roi.Y
	}
	if roi.W < 1 {
		roi.W = 1
	}
	if roi.H < 1 {
		roi.H = 1
	}
	return roi
}

// unionRect returns the bounding rectangle that contains both rects.
func unionRect(a, b Rect) Rect {
	x1 := a.X
	if b.X < x1 {
		x1 = b.X
	}
	y1 := a.Y
	if b.Y < y1 {
		y1 = b.Y
	}
	x2 := a.X + a.W
	if b.X+b.W > x2 {
		x2 = b.X + b.W
	}
	y2 := a.Y + a.H
	if b.Y+b.H > y2 {
		y2 = b.Y + b.H
	}
	return Rect{X: x1, Y: y1, W: x2 - x1, H: y2 - y1}
}

// pixelAt returns the R, G, B components of the pixel at (x, y).
// Fast path: *image.RGBA (produced by CaptureScreenRegion) reads Pix directly
// instead of going through image.At() interface dispatch, which is ~10× faster.
func pixelAt(img image.Image, x, y int) (r, g, b uint8) {
	if rgba, ok := img.(*image.RGBA); ok {
		off := rgba.PixOffset(x, y)
		return rgba.Pix[off], rgba.Pix[off+1], rgba.Pix[off+2]
	}
	c := img.At(x, y)
	rgba := color.RGBAModel.Convert(c).(color.RGBA)
	return rgba.R, rgba.G, rgba.B
}

// absInt returns the absolute value of v.
func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
