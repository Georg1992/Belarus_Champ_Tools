package autopot

// Color-detection constants used by HP/SP pixel predicates.
const (
	barBgR, barBgG, barBgB = 10, 10, 14
	barBgTol               = 28

	hpGreenR, hpGreenG, hpGreenB = 16, 238, 33
	hpRedR, hpRedG, hpRedB       = 255, 13, 0
	spBlueR, spBlueG, spBlueB    = 25, 101, 225
	fillTol                      = 50

	// HP fill detection thresholds
	hpFillMinGreen     = 35
	hpFillMinRed       = 50
	hpFillRedGreenDiff = 25

	// Color detection tolerances for isHPTrack
	hpTrackBgTol   = 8
	hpTrackSumMin  = 60
	hpTrackSumMax  = 210
	hpTrackDiffTol = 30

	// Color detection thresholds for bar background
	barBgNearBlackTol = 15
	barBgDarkSum      = 35

	// HP green detection thresholds
	hpGreenMinGreen    = 60
	hpGreenMaxRed      = 20
	hpGreenBlueGapTol  = 8
	hpGreenRedGapMin   = 10
	hpGreenAltMinGreen = 50
	hpGreenAltMinDiff  = 10
	hpGreenBrightMin   = 80

	// HP red detection thresholds
	hpRedMinRed    = 90
	hpRedGreenDiff = 25

	// HP yellow detection thresholds
	hpYellowMinRed        = 110
	hpYellowMinGreen      = 90
	hpYellowMaxBlue       = 90
	hpYellowRedBlueDiff   = 15
	hpYellowGreenBlueDiff = 10

	// SP blue detection thresholds
	spBlueMinBlue   = 90
	spBlueGreenDiff = 10
	spBlueRedDiff   = 18

	// SP fill detection thresholds
	spFillMinBlue  = 130
	spFillMinRed   = 12
	spFillBlueDiff = 20

	// SP cyan detection thresholds
	spCyanMinBlue   = 80
	spCyanMinGreen  = 60
	spCyanBlueDiff  = 10
	spCyanGreenDiff = 5
)

// IsHPPixel returns true if the pixel color is part of the HP bar (green, red, or yellow).
func IsHPPixel(r, g, b uint8) bool {
	return isHPGreen(r, g, b) || isHPRed(r, g, b) || isHPYellow(r, g, b)
}

// IsSPPixel returns true if the pixel color is part of the SP bar (blue or cyan).
func IsSPPixel(r, g, b uint8) bool {
	return isSPBlue(r, g, b) || isSPCyan(r, g, b)
}

// isHPFillRead returns true if the pixel is part of the HP bar fill,
// including mixed green-red pixels at the fill/unfilled boundary and
// red-dominant pixels below the isHPRed brightness threshold.
func isHPFillRead(r, g, b uint8) bool {
	if IsHPPixel(r, g, b) {
		return true
	}
	if isHPTrack(r, g, b) {
		return false
	}
	ri, gi, bi := int(r), int(g), int(b)
	// Mixed green-red pixels at the fill/unfilled boundary
	if gi >= hpFillMinGreen && ri >= hpFillMinRed && absInt(ri-gi) < hpFillRedGreenDiff {
		return true
	}
	// Red-dominant pixels: part of the HP bar fill that is red but below
	// the isHPRed brightness threshold (e.g. anti-aliased edges at low HP).
	// Must clearly be more red than green and blue to avoid false positives.
	if ri >= hpRedMinRed && ri > gi+hpRedGreenDiff && ri > bi+hpRedGreenDiff {
		return true
	}
	return false
}

// isSPFill returns true if the pixel is a blue SP fill pixel (bright blue
// used for filled portion of SP bar).
func isSPFill(r, g, b uint8) bool {
	if isBarBackground(r, g, b) {
		return false
	}
	ri, gi, bi := int(r), int(g), int(b)
	return bi >= spFillMinBlue && ri >= spFillMinRed && bi > gi+spFillBlueDiff && bi > ri+spFillBlueDiff
}

func isHPTrack(r, g, b uint8) bool {
	if IsHPPixel(r, g, b) || IsSPPixel(r, g, b) {
		return false
	}
	if colorNear(r, g, b, barBgR, barBgG, barBgB, hpTrackBgTol) {
		return true
	}
	ri, gi, bi := int(r), int(g), int(b)
	sum := ri + gi + bi
	if sum < hpTrackSumMin || sum > hpTrackSumMax {
		return false
	}
	return bi <= gi && absInt(ri-gi) < hpTrackDiffTol
}

func isBarBackground(r, g, b uint8) bool {
	if colorNear(r, g, b, barBgR, barBgG, barBgB, barBgTol) {
		return true
	}
	if colorNear(r, g, b, 0, 0, 5, barBgNearBlackTol) {
		return true
	}
	ri, gi, bi := int(r), int(g), int(b)
	return ri+gi+bi < barBgDarkSum
}

func isHPGreen(r, g, b uint8) bool {
	if isBarBackground(r, g, b) {
		return false
	}
	if colorNear(r, g, b, hpGreenR, hpGreenG, hpGreenB, fillTol) {
		return true
	}
	ri, gi, bi := int(r), int(g), int(b)
	if gi >= hpGreenMinGreen && ri <= hpGreenMaxRed && bi <= gi+hpGreenBlueGapTol && gi > ri+hpGreenRedGapMin {
		return true
	}
	if gi >= hpGreenAltMinGreen && gi > ri+hpGreenAltMinDiff && gi > bi {
		return true
	}
	return gi > hpGreenBrightMin && gi > ri && gi+ri > bi+hpGreenAltMinDiff*2
}

func isHPRed(r, g, b uint8) bool {
	if isBarBackground(r, g, b) {
		return false
	}
	if colorNear(r, g, b, hpRedR, hpRedG, hpRedB, fillTol) {
		return true
	}
	ri, gi, bi := int(r), int(g), int(b)
	return ri > hpRedMinRed && ri > gi+hpRedGreenDiff && ri > bi+hpRedGreenDiff
}

func isHPYellow(r, g, b uint8) bool {
	if isBarBackground(r, g, b) {
		return false
	}
	ri, gi, bi := int(r), int(g), int(b)
	return ri > hpYellowMinRed && gi > hpYellowMinGreen && bi < hpYellowMaxBlue &&
		ri > bi+hpYellowRedBlueDiff && gi > bi+hpYellowGreenBlueDiff
}

func isSPBlue(r, g, b uint8) bool {
	if isBarBackground(r, g, b) {
		return false
	}
	if colorNear(r, g, b, spBlueR, spBlueG, spBlueB, fillTol) {
		return true
	}
	ri, gi, bi := int(r), int(g), int(b)
	return bi >= spBlueMinBlue && bi > gi+spBlueGreenDiff && bi > ri+spBlueRedDiff
}

func isSPCyan(r, g, b uint8) bool {
	if isBarBackground(r, g, b) {
		return false
	}
	ri, gi, bi := int(r), int(g), int(b)
	return bi >= spCyanMinBlue && gi >= spCyanMinGreen && bi > ri+spCyanBlueDiff && gi > ri+spCyanGreenDiff
}

func colorNear(r, g, b, refR, refG, refB uint8, tol int) bool {
	return absInt(int(r)-int(refR)) <= tol &&
		absInt(int(g)-int(refG)) <= tol &&
		absInt(int(b)-int(refB)) <= tol
}
