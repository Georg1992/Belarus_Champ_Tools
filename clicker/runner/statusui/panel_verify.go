package statusui

import (
	"errors"
	"fmt"
	"image"
	"math"
)

// PanelFeatures holds the three numeric signatures that distinguish a
// real status panel UI from a same-shaped region of unrelated in-game
// UI (skill hotbar, chat window, minimap trim, etc.).
//
// Computed from the 218×58 cropped panel that FindStatusPanel returns.
// All three values are 0..1 fractions except CornerContrast (avg of
// per-channel stddev in the top-right 12×12 patch, in 0..255 units).
type PanelFeatures struct {
	// BlueFraction is the share of pixels whose blue channel dominates
	// by a meaningful margin (B > R+25 && B > G+25 && B > 80). The real
	// status panel has bluish UI accents (decorative borders, character
	// info icons); the skill hotbar / generic window trim rarely does.
	BlueFraction float64

	// TopRightCornerContrast is the average per-channel stddev across
	// the top-right 12×12 pixel region of the panel. The real status
	// panel has a decorative emblem in the top-right corner with
	// strong color variation; the skill hotbar has a uniform gradient
	// in the same area.
	TopRightCornerContrast float64

	// EdgeFraction is the share of neighbour pairs (±2px straight) whose
	// luminance jump exceeds 80. The real status panel contains HP/SP
	// digit glyphs and the HP/SP bars, all of which create sharp
	// luminance edges; the false-positive regions are largely flat
	// backgrounds that produce very few such edges.
	EdgeFraction float64
}

// panelCornerPatch is the side length of the corner patch used to
// compute CornerContrast. 12px covers the decorative emblem on the
// real status panel without dragging in too much background.
const panelCornerPatch = 12

// panelVerifyWidth / panelVerifyHeight are the dimensions VerifyPanel
// requires of its input. They equal the embedded StatusPanel.png
// template size, which is what the corner-patch positioning
// (x ∈ [panelVerifyWidth - panelCornerPatch, panelVerifyWidth))
// assumes. Allowing anything other than 218×58 silently positions
// the stddev measurement at the wrong place; valid rejection.
const (
	panelVerifyWidth  = 218
	panelVerifyHeight = 58
)

// panelEdgeStep is the orthogonal neighbour distance used for edge
// counting. >1 is intentional: a single-pixel diagonal anti-alias
// fringe shouldn't count as an "edge", but a digit glyph or HP-bar
// edge jumps multiple luminance levels across 2-3px.
const panelEdgeStep = 2

// panelEdgeDiff is the minimum luminance difference for a neighbour
// pair to be counted as an "edge". 80 (out of 255) is calibrated to
// fire on digit/anti-aliased glyph edges but ignore intra-bar
// anti-aliasing and JPEG-like background blips.
const panelEdgeDiff = 80

// PanelVerifyOptions tunes the three-signal post-locate verification.
// Zero values fall back to Defaults. The three thresholds are tuned
// against a known set of true-positive status-panel crops and false-
// positive same-shape captures (skill hotbar, chat trim, zoomed UI).
type PanelVerifyOptions struct {
	// MinBlueFraction, default 0.08 (8%). Real panel ~15% blue; skill
	// hotbar ~0%. Tightened from the original 0.04 calibration so
	// UI drifts toward a flatter blue accent count are caught earlier
	// while still leaving the panel a 7pp safety margin over its
	// worst observed value.
	MinBlueFraction float64

	// MinCornerContrast, default 50 (avg per-channel stddev in 12×12
	// top-right patch, 0..255). Real panel ~65; skill hotbar ~25.
	// Tightened from the original 30 calibration — the observed TP
	// cluster sits around 65 and the worst observed false-positive
	// stddev is ~25, so 50 leaves a ~2× margin over the highest
	// false-positive in the calibration set.
	MinCornerContrast float64

	// MinEdgeFraction, default 0.07 (7%). Real panel ~11–12%; skill
	// hotbar ~0.3%. Tightened from the original 0.04 calibration —
	// the margin to false-positive is still >20× so it's safe, and
	// a degraded digit/bar rendering should fall through the
	// verifier quickly rather than silently pass.
	MinEdgeFraction float64
}

// DefaultPanelVerifyOptions returns the calibrated defaults. These
// are documented per-signal above so a future operator can read the
// thresholds against a known-good capture and decide whether they've
// drifted enough to warrant re-tuning.
func DefaultPanelVerifyOptions() PanelVerifyOptions {
	return PanelVerifyOptions{
		MinBlueFraction:   0.08,
		MinCornerContrast: 50,
		MinEdgeFraction:   0.07,
	}
}

// ComputePanelFeatures walks the panel image once and returns the
// three signatures used by VerifyPanel. Errors out only on nil input;
// wrong dimensions are tolerated (the per-feature loops just slice
// within image.Bounds()).
//
// The implementation does a single linear pass per signature so 218×58
// costs well under a millisecond — the verification is cheap enough
// to run on every successful locate without affecting the 500ms
// recognition cadence.
func ComputePanelFeatures(panel image.Image) (PanelFeatures, error) {
	if panel == nil {
		return PanelFeatures{}, errors.New("nil panel image")
	}
	b := panel.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return PanelFeatures{}, errors.New("empty panel bounds")
	}
	totalPx := float64(w * h)

	// Pass 1 — colour buckets + top-right corner patch capture. The
	// corner patch is captured inline to avoid re-decoding pixels.
	blue := 0
	patchW := panelCornerPatch
	patchH := panelCornerPatch
	patchX0 := w - patchW
	patchY0 := 0
	if patchX0 < 0 {
		patchX0 = 0
	}
	patchSize := patchW * patchH
	if patchSize > w*h {
		patchSize = w * h
	}
	patchR := make([]uint8, 0, patchSize)
	patchG := make([]uint8, 0, patchSize)
	patchB := make([]uint8, 0, patchSize)
	luma := make([]uint16, h*w) // 16-bit so cumulative addition doesn't overflow
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := panel.At(b.Min.X+x, b.Min.Y+y).RGBA()
			// 16-bit RGBA() returns pre-multiplied, 0..0xffff. Convert
			// to 8-bit post-pre-multiply (q>>8).
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(bl >> 8)
			if b8 > r8+25 && b8 > g8+25 && b8 > 80 {
				blue++
			}
			// Rec.601 luma, same formula the rest of statusui uses
			// (luma8 in panel_finder.go) — agreement to ±1.
			luma[y*w+x] = uint16((uint32(r8)*299 + uint32(g8)*587 + uint32(b8)*114) / 1000)
			if x >= patchX0 && y >= patchY0 && x < patchX0+patchW && y < patchY0+patchH {
				patchR = append(patchR, r8)
				patchG = append(patchG, g8)
				patchB = append(patchB, b8)
			}
		}
	}

	// Pass 2 — edge density. Sample stride >= 2 from each interior
	// pixel (right + below + left + above, 4 strains).
	edges := 0
	strains := 0
	for y := panelEdgeStep; y < h-panelEdgeStep; y++ {
		for x := panelEdgeStep; x < w-panelEdgeStep; x++ {
			c := luma[y*w+x]
			if absU16(c, luma[y*w+(x+panelEdgeStep)]) > panelEdgeDiff {
				edges++
			}
			if absU16(c, luma[(y+panelEdgeStep)*w+x]) > panelEdgeDiff {
				edges++
			}
			if absU16(c, luma[y*w+(x-panelEdgeStep)]) > panelEdgeDiff {
				edges++
			}
			if absU16(c, luma[(y-panelEdgeStep)*w+x]) > panelEdgeDiff {
				edges++
			}
			strains += 4
		}
	}

	// Patch contrast — average per-channel stddev. Empty patch on tiny
	// panels collapses to 0 (no signal) and Verify will fail loudly
	// rather than pass a false negative.
	contrast := channelAvgStddev(patchR, patchG, patchB)
	frac := func(n int) float64 {
		if totalPx == 0 {
			return 0
		}
		return float64(n) / totalPx
	}
	edgeFrac := 0.0
	if strains > 0 {
		edgeFrac = float64(edges) / float64(strains)
	}
	return PanelFeatures{
		BlueFraction:          frac(blue),
		TopRightCornerContrast: contrast,
		EdgeFraction:          edgeFrac,
	}, nil
}

// absU16 is integer |a-b| in two's complement, equivalent to math.Abs
// without the float conversion (cheap because luma is small).
func absU16(a, b uint16) uint16 {
	if a > b {
		return a - b
	}
	return b - a
}

// channelAvgStddev computes the average per-channel stddev across
// three parallel slices (R/G/B). Uses sample stddev (n-1 denominator)
// for compatibility with how the calibration data was collected
// (Pillow's statistics.stdev — same formula). Returns 0 on empty input.
func channelAvgStddev(rs, gs, bs []uint8) float64 {
	n := len(rs)
	if n == 0 {
		return 0
	}
	sr := stddevSample(rs)
	sg := stddevSample(gs)
	sb := stddevSample(bs)
	return (sr + sg + sb) / 3
}

// stddevSample is sample stddev (Bessel-corrected). Implemented
// inline because math import otherwise pulls in trigonometry we don't
// need. ~70ns per call on 144-element patches — comfortable.
func stddevSample(xs []uint8) float64 {
	n := len(xs)
	if n <= 1 {
		return 0
	}
	var sumU64 float64
	for _, x := range xs {
		sumU64 += float64(x)
	}
	mean := sumU64 / float64(n)
	var sqDiff float64
	for _, x := range xs {
		d := float64(x) - mean
		sqDiff += d * d
	}
	variance := sqDiff / float64(n-1)
	return math.Sqrt(variance)
}

// ExtractROI crops a rectangular sub-image from src. The returned
// image's origin is (0, 0); coordinates in the ROI are translated by
// the ROI's top-left. ROI is clipped to src.Bounds() so out-of-range
// calls silently produce a smaller (or empty) crop rather than panic.
//
// Pure image utility — no OCR, no matching, no exemplar library.
// Moved here from numeric_parser.go which is being retired; the
// recognition pipeline is the only surviving caller.
func ExtractROI(src image.Image, roi image.Rectangle) image.Image {
	if src == nil {
		return nil
	}
	roi = roi.Intersect(src.Bounds())
	if roi.Empty() {
		return nil
	}
	out := image.NewRGBA(image.Rect(0, 0, roi.Dx(), roi.Dy()))
	for y := 0; y < roi.Dy(); y++ {
		for x := 0; x < roi.Dx(); x++ {
			out.Set(x, y, src.At(roi.Min.X+x, roi.Min.Y+y))
		}
	}
	return out
}

// ExtractStatusLineStrip crops the HP/SP text row out of a screen
// capture given a previously-located status panel rect. Returns both
// the cropped image (origin (0, 0)) and its exact screen-space
// rectangle — the caller hands the image to whatever downstream
// OCR pipeline it owns (a sibling app, a human eye, a future
// neural network) and the rect so the receiver can map result
// coordinates back to screen pixels.
//
// Pure helper: locating the panel is the caller's job (use
// FindStatusPanel + VerifyPanel), the strip rect is computed by
// DefaultStatusLineLocator, and this function just composes the
// two via ExtractROI. Returns (nil, zero Rectangle) on bad input
// rather than panicking.
func ExtractStatusLineStrip(screenImg image.Image, panelRect image.Rectangle) (image.Image, image.Rectangle) {
	if screenImg == nil || panelRect.Empty() {
		return nil, image.Rectangle{}
	}
	locator := DefaultStatusLineLocator()
	lineRect := locator.LocateStatusTextLine(panelRect)
	strip := ExtractROI(screenImg, lineRect)
	if strip == nil {
		return nil, image.Rectangle{}
	}
	return strip, lineRect
}

// PreprocessImage converts an image to a binary grid using the
// package-canonical gray<150 threshold and Rec.601 luma weights.
// TRUE = foreground (dark pixel), FALSE = background (light pixel).
//
// Pure image utility — used by layout tests to compute the
// foreground-pixel fraction of the line strip (verifying the
// locator targets the dense HP/SP text row and not empty area).
// NOT part of any OCR pipeline; downstream OCR comes from
// whatever the caller hands the strip image to.
func PreprocessImage(img image.Image) [][]bool {
	if img == nil {
		return nil
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width == 0 || height == 0 {
		return nil
	}
	binary := make([][]bool, height)
	for y := 0; y < height; y++ {
		binary[y] = make([]bool, width)
		for x := 0; x < width; x++ {
			r, g, bl, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			gray := uint8((r*299 + g*587 + bl*114) / 1000 >> 8)
			binary[y][x] = gray < 150
		}
	}
	return binary
}

// VerifyPanel runs the three-signal check on the panel and returns
// nil if it looks like a real status panel, or a descriptive error
// otherwise. Cheap enough to run on every successful locate — the
// caller should drop the rect from cache on error rather than retry.
//
// Pre-flight: VerifyPanel requires its input to be exactly 218×58 —
// the calibrated dimensions of the embedded StatusPanel.png template.
// Anywhere else (1440p scaled UI, a stray crop rectangle, an
// extracted sub-region) the corner patch positions the stddev check
// in the wrong place and the calibration is silently wrong. Better to
// fail loud than to silently let a wrong-sized input through.
//
// Reasons for failure (returned as distinct error strings so callers
// can log them) include "wrong dimensions", "insufficient blue
// accents", "uniform top-right corner (no emblem)", and "no
// digit/anti-aliased content".
func VerifyPanel(panel image.Image) error {
	return VerifyPanelWith(panel, DefaultPanelVerifyOptions())
}

// VerifyPanelWith is VerifyPanel with explicit thresholds. Empty
// option fields (zero) are substituted from DefaultPanelVerifyOptions
// — callers can override any subset of the three signals without
// touching the rest. The 218×58 dimension check is enforced
// regardless of options, since the corner-patch positioning only
// makes sense at that size.
func VerifyPanelWith(panel image.Image, opts PanelVerifyOptions) error {
	if panel == nil {
		return errors.New("verify: nil panel image")
	}
	b := panel.Bounds()
	if b.Dx() != panelVerifyWidth || b.Dy() != panelVerifyHeight {
		return fmt.Errorf("verify: panel dimensions %dx%d, want %dx%d (corner-patch stddev is calibrated for the embedded StatusPanel.png size)",
			b.Dx(), b.Dy(), panelVerifyWidth, panelVerifyHeight)
	}
	feats, err := ComputePanelFeatures(panel)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	def := DefaultPanelVerifyOptions()
	if opts.MinBlueFraction == 0 {
		opts.MinBlueFraction = def.MinBlueFraction
	}
	if opts.MinCornerContrast == 0 {
		opts.MinCornerContrast = def.MinCornerContrast
	}
	if opts.MinEdgeFraction == 0 {
		opts.MinEdgeFraction = def.MinEdgeFraction
	}
	if feats.BlueFraction < opts.MinBlueFraction {
		return fmt.Errorf("verify: insufficient blue accents (got %.3f, want >= %.3f) — region lacks status-panel UI colour signature (likely a skill hotbar or other neutral UI)",
			feats.BlueFraction, opts.MinBlueFraction)
	}
	if feats.TopRightCornerContrast < opts.MinCornerContrast {
		return fmt.Errorf("verify: top-right corner uniform (got %.1f, want >= %.1f stddev) — region lacks the decorative emblem the status panel has",
			feats.TopRightCornerContrast, opts.MinCornerContrast)
	}
	if feats.EdgeFraction < opts.MinEdgeFraction {
		return fmt.Errorf("verify: no digit/anti-aliased content (got %.3f, want >= %.3f) — region is flat, no HP/SP text or bar edges detected",
			feats.EdgeFraction, opts.MinEdgeFraction)
	}
	return nil
}
