package autopot

// Game-provided exact bar colors (from game client data):
//
//	HP green fill:  #21DE42  (33, 222, 66)
//	HP red fill:    #E21019  (226, 16, 25)
//	SP blue fill:   #225FDE  (34, 95, 222)
//	Bar background: #423F46  (66, 63, 70)
//
// Pixels vary slightly due to anti-aliasing and rendering; tol handles
// that range without being so broad it matches unrelated game pixels.
const (
	hpFillGreenR, hpFillGreenG, hpFillGreenB = 33, 222, 66   // #21DE42
	hpFillRedR, hpFillRedG, hpFillRedB       = 226, 16, 25   // #E21019
	spFillBlueR, spFillBlueG, spFillBlueB    = 34, 95, 222   // #225FDE
	barTrackR, barTrackG, barTrackB          = 66, 63, 70    // #423F46

	// Tolerance for colour variation from anti-aliasing / rendering.
	// Actual pixels in-game vary by ~40-50 per channel from the exact
	// reference colours due to rendering and compression.
	colorTol = 55
)

// IsHPPixel returns true if the pixel matches HP bar fill (green or red).
func IsHPPixel(r, g, b uint8) bool {
	return isHPFillRead(r, g, b)
}

// IsSPPixel returns true if the pixel matches SP bar fill (blue).
func IsSPPixel(r, g, b uint8) bool {
	return isSPFill(r, g, b)
}

// isHPFillRead returns true if the pixel is HP fill (green or red).
func isHPFillRead(r, g, b uint8) bool {
	if isBarBackground(r, g, b) {
		return false
	}
	return colorNear(r, g, b, hpFillGreenR, hpFillGreenG, hpFillGreenB, colorTol) ||
		colorNear(r, g, b, hpFillRedR, hpFillRedG, hpFillRedB, colorTol)
}

// isSPFill returns true if the pixel is SP fill (blue).
func isSPFill(r, g, b uint8) bool {
	if isBarBackground(r, g, b) {
		return false
	}
	return colorNear(r, g, b, spFillBlueR, spFillBlueG, spFillBlueB, colorTol)
}

// isHPTrack returns true if the pixel matches the bar's dark background
// track colour #423F46.
func isHPTrack(r, g, b uint8) bool {
	return colorNear(r, g, b, barTrackR, barTrackG, barTrackB, colorTol)
}

// isBarBackground returns true for very dark pixels (near-black) or the
// bar track colour. This function identifies areas that are NOT fill.
func isBarBackground(r, g, b uint8) bool {
	// Near-black pixels: each channel within ~20 of 0, or total brightness < 35.
	// The sum check catches pixels where one channel is slightly above 20 but
	// the overall brightness is still effectively black (e.g. 21,5,5).
	if colorNear(r, g, b, 0, 0, 0, 20) {
		return true
	}
	ri, gi, bi := int(r), int(g), int(b)
	if ri+gi+bi < 35 {
		return true
	}
	return isHPTrack(r, g, b)
}

// colorNear checks whether each channel of (r,g,b) is within tol of (refR,refG,refB).
func colorNear(r, g, b, refR, refG, refB uint8, tol int) bool {
	return absInt(int(r)-int(refR)) <= tol &&
		absInt(int(g)-int(refG)) <= tol &&
		absInt(int(b)-int(refB)) <= tol
}
