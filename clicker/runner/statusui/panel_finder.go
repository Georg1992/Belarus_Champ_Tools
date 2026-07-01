package statusui

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/png"
	"sync"
)

//go:embed StatusPanel.png
var statusPanelTemplatePNG []byte

var (
	statusPanelTemplateOnce sync.Once
	statusPanelTemplate     image.Image
)

// DefaultStatusPanelTemplate returns the embedded StatusPanel.png decoded as
// an image.Image. Loaded once on first call; safe for concurrent use.
//
// The template is embedded into the binary so the recognition pipeline
// doesn't depend on a runtime file path. Callers that want to ship their
// own template can still pass any image.Image to FindStatusPanel directly.
func DefaultStatusPanelTemplate() image.Image {
	statusPanelTemplateOnce.Do(func() {
		if len(statusPanelTemplatePNG) == 0 {
			return
		}
		img, err := png.Decode(bytes.NewReader(statusPanelTemplatePNG))
		if err == nil {
			statusPanelTemplate = img
		}
	})
	return statusPanelTemplate
}

// FindStatusPanelOptions configures the panel search.
type FindStatusPanelOptions struct {
	// TopLeftRegion is the first region to scan, in screen coordinates.
	// Defaults to image.Rect(0, 0, 400, 200) when empty — the
	// typical Ragnarok status window location in the upper-left.
	TopLeftRegion image.Rectangle
	// MaxScore is the maximum acceptable SAD score (0 = perfect,
	// 1 = worst). Defaults to 0.30 when zero. 0.30 sits above the
	// worst top-left-panel SAD observed in the regression set
	// (≈0.25 on captures with content drift) and below
	// false-positive SAD observed on flat unrelated regions
	// (≈0.99) — so VerifyPanel's content signals remain the
	// discriminator.
	MaxScore float64
}

// FindStatusPanel searches for the status panel inside img using
// grayscale sum-of-absolute-differences (SAD) against the template.
//
// The search runs in two passes:
//
//  1. The TopLeftRegion (defaults to image.Rect(0, 0, 400, 200)).
//     The Ragnarok status panel normally lives in this region on
//     real captures; if a match below MaxScore exists here, this
//     returns immediately.
//
//  2. The rest of the image (img.Bounds()). If the first pass did
//     not find anything, this passes continues looking for a
//     panel-shaped match anywhere else on screen. It is the
//     continuation of the search, not a fallback.
//
// Both passes use the same single full-accuracy scan algorithm
// (findPanelInRegion); only the search region differs. This is the
// "same behaviour" — a single uniform algorithm applied to two
// candidate regions in a defined order — instead of two different
// implementations trying to do the same thing.
//
// For *image.RGBA (the format produced by the screen-capture path)
// the hot loop accesses the underlying Pix slice directly rather
// than going through image.At(); for any other format it falls
// back to At().
//
// Returns the best match location, the normalized 0..1 score (0 =
// perfect, 1 = worst), and ok=true iff the best score is at or
// below MaxScore. If no match is found in either region, rect is
// the zero Rectangle and ok=false.
func FindStatusPanel(img, template image.Image, opts FindStatusPanelOptions) (image.Rectangle, float64, bool) {
	region := opts.TopLeftRegion
	if region.Empty() {
		region = image.Rect(0, 0, 400, 200)
	}
	maxScore := opts.MaxScore
	if maxScore == 0 {
		maxScore = 0.30
	}

	// Pass 1: top-left region.
	if rect, score, ok := findPanelInRegion(img, template, region, maxScore); ok {
		return rect, score, true
	}
	// Pass 2: continuation across the rest of the screen.
	return findPanelInRegion(img, template, img.Bounds(), maxScore)
}

// findPanelInRegion scans the given region of img at full pixel
// accuracy (stride=1) against the template and returns the
// lowest-SAD match below maxScore, or ok=false if no match qualifies.
//
// One scan algorithm — no strided pre-filter, no refinement window,
// no branch on stride. The same loop is reused for both the
// top-left pass (default 400×200) and the full-screen continuation
// pass (img.Bounds()).
func findPanelInRegion(img, template image.Image, region image.Rectangle, maxScore float64) (image.Rectangle, float64, bool) {
	ib := img.Bounds()
	tb := template.Bounds()
	tw, th := tb.Dx(), tb.Dy()
	if tw <= 0 || th <= 0 {
		return image.Rectangle{}, 0, false
	}

	region = region.Intersect(ib)
	maxX := region.Max.X - tw
	maxY := region.Max.Y - th
	if maxX < region.Min.X || maxY < region.Min.Y {
		return image.Rectangle{}, 0, false
	}

	tplGray := precomputeGrayscale(template, tb)
	maxPossible := float64(tw*th) * 255.0

	// Fast path: precompute the entire image's grayscale once.
	// Falls back to per-pixel At() for non-RGBA formats.
	imgGray, fastOK := precomputeImageGrayscale(img, ib)
	sad := func(x0, y0 int, earlyExit float64) float64 {
		if fastOK {
			return sadOnGrayscale(imgGray, tplGray, x0, y0, tw, th, earlyExit)
		}
		return sadWithEarlyExit(img, tplGray, x0, y0, tw, th, earlyExit)
	}

	bestScore := maxPossible + 1
	bestX, bestY := region.Min.X, region.Min.Y
scan:
	for y := region.Min.Y; y <= maxY; y++ {
		for x := region.Min.X; x <= maxX; x++ {
			score := sad(x, y, bestScore)
			if score < bestScore {
				bestScore = score
				bestX, bestY = x, y
				if bestScore == 0 {
					break scan
				}
			}
		}
	}

	normalized := bestScore / maxPossible
	if normalized > maxScore {
		return image.Rectangle{}, normalized, false
	}
	return image.Rect(bestX, bestY, bestX+tw, bestY+th), normalized, true
}

// precomputeGrayscale returns a 2-D slice of 8-bit luminance values
// for the pixels in the given image bounds.
func precomputeGrayscale(img image.Image, b image.Rectangle) [][]uint8 {
	w, h := b.Dx(), b.Dy()
	out := make([][]uint8, h)
	for y := 0; y < h; y++ {
		row := make([]uint8, w)
		for x := 0; x < w; x++ {
			row[x] = luma8(img.At(b.Min.X+x, b.Min.Y+y))
		}
		out[y] = row
	}
	return out
}

// precomputeImageGrayscale converts an *image.RGBA (the format produced
// by the screen-capture path) to a 2-D slice of 8-bit luminance values
// by reading the Pix slice directly. For other image formats it returns
// ok=false so the caller can fall back to per-pixel At().
//
// Accessing Pix directly is ~10× faster than calling image.At() for
// every pixel, which is the difference between a sub-second full-screen
// scan and a 5-minute timeout.
//
// Note on alpha pre-multiplication: image.At().RGBA() returns
// pre-multiplied 16-bit values, while reading Pix directly gives the
// un-pre-multiplied 8-bit values. For fully-opaque pixels (which is
// true for screen captures from win.CapturePlayerBarSearch) the two
// are identical; if a non-opaque image is ever passed in, the fast
// path and the At() path will disagree.
func precomputeImageGrayscale(img image.Image, b image.Rectangle) ([][]uint8, bool) {
	rgba, ok := img.(*image.RGBA)
	if !ok {
		return nil, false
	}
	w, h := b.Dx(), b.Dy()
	out := make([][]uint8, h)
	stride := rgba.Stride
	pix := rgba.Pix
	for y := 0; y < h; y++ {
		row := make([]uint8, w)
		srcY := b.Min.Y + y
		for x := 0; x < w; x++ {
			srcX := b.Min.X + x
			off := srcY*stride + srcX*4
			row[x] = luma8FromRGBA(pix[off], pix[off+1], pix[off+2])
		}
		out[y] = row
	}
	return out, true
}

// sadOnGrayscale computes the SAD between tpl and the tw×th region
// of imgGray at (x0, y0) using the precomputed grayscale slices.
// Stops as soon as the running sum reaches earlyExit.
func sadOnGrayscale(imgGray, tpl [][]uint8, x0, y0, tw, th int, earlyExit float64) float64 {
	sum := 0.0
	for dy := 0; dy < th; dy++ {
		irow := imgGray[y0+dy]
		trow := tpl[dy]
		for dx := 0; dx < tw; dx++ {
			diff := float64(trow[dx]) - float64(irow[x0+dx])
			if diff < 0 {
				diff = -diff
			}
			sum += diff
			if sum >= earlyExit {
				return sum
			}
		}
	}
	return sum
}

// sadWithEarlyExit computes the grayscale SAD between tpl and the
// tw×th region of img at (x0, y0). Stops as soon as the running
// sum reaches earlyExit — the caller uses this to skip positions
// that can't beat the current best.
func sadWithEarlyExit(img image.Image, tpl [][]uint8, x0, y0, tw, th int, earlyExit float64) float64 {
	sum := 0.0
	for dy := 0; dy < th; dy++ {
		row := tpl[dy]
		yy := y0 + dy
		for dx := 0; dx < tw; dx++ {
			diff := float64(row[dx]) - float64(luma8(img.At(x0+dx, yy)))
			if diff < 0 {
				diff = -diff
			}
			sum += diff
			if sum >= earlyExit {
				return sum
			}
		}
	}
	return sum
}

// luma8 returns an 8-bit grayscale value (Rec.601-style weights) for
// any color. This is the canonical formula used across statusui
// (template load, runtime capture, and the slow path all go through
// here). Matches the formula in statusui's PreprocessImage so every
// grayscale conversion in the package agrees to ±1.
func luma8(c color.Color) uint8 {
	r, g, b, _ := c.RGBA()
	return uint8((r*299 + g*587 + b*114) / 1000 >> 8)
}

// luma8FromRGBA computes the same 8-bit grayscale for raw 8-bit RGB
// inputs read directly from an *image.RGBA Pix slice. Used by the
// fast path (precomputeImageGrayscale) to avoid the At()→RGBA() round
// trip. Assumes fully-opaque pixels — for non-opaque inputs the At()
// path (which goes through pre-multiplied 16-bit RGBA()) will differ
// from this by the alpha factor, so call the color.Color version
// instead.
func luma8FromRGBA(r, g, b uint8) uint8 {
	return uint8((uint32(r)*299 + uint32(g)*587 + uint32(b)*114) / 1000)
}
