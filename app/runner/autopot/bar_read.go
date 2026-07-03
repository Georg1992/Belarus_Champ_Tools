package autopot

import "image"

// BarRead holds the fill percentage and pixel counts for a single bar.
type BarRead struct {
	Percent     float64
	FilledWidth int
	FullWidth   int
	Found       bool
}

// Bar is used by debug visualization output.
type Bar struct {
	Left, Right int
	Y           int
	Width       int
	Height      int
	FilledWidth int
	Percent     float64
	Found       bool
}

// ReadMappedBars reads the fill percentage of HP and SP bars from the given image.
// Uses cached bar rectangles from the last successful pair detection.
func ReadMappedBars(img image.Image, bars MappedBars) (hp BarRead, sp BarRead) {
	if !bars.Valid {
		return BarRead{}, BarRead{}
	}
	if bars.HP.W < 1 || bars.SP.W < 1 {
		return BarRead{FullWidth: bars.HP.W}, BarRead{FullWidth: bars.SP.W}
	}
	hp = ReadHPFill(img, bars.HP)
	sp = ReadSPFill(img, bars.SP)
	return hp, sp
}

// ReadHPFill reads the fill percentage of an HP bar from the image.
// Returns a BarRead with the fill percentage and pixel counts.
// When no fill pixels are found and the bar has no HP-colored pixels at all
// (player is dead, bar is empty), returns Found=false to prevent potion spam.
func ReadHPFill(img image.Image, hp Rect) BarRead {
	if hp.W < 1 || hp.H < 1 {
		return BarRead{Found: false}
	}
	hp = trimBarEdges(img, hp, true)
	if hp.W < 1 {
		return BarRead{Found: false}
	}
	best := BarRead{Found: true, FullWidth: hp.W}
	for row := 0; row < hp.H; row++ {
		br := readBarFillSingleRow(img, hp.X, hp.Y+row, hp.W, isHPFillRead)
		if br.FilledWidth > best.FilledWidth {
			best = br
		}
	}
	if best.FilledWidth == 0 {
		if barHasNoColorPixels(img, hp, true) {
			return BarRead{Found: false, FullWidth: hp.W}
		}
		return normalizeBarRead(img, hp, true, readBarFillSingleRow(img, hp.X, hp.Y, hp.W, isHPFillRead))
	}
	return normalizeBarRead(img, hp, true, best)
}

// ReadSPFill reads the fill percentage of an SP bar from the image.
// Similar to ReadHPFill but uses SP color detection.
// When no fill pixels are found and the bar has no SP-colored pixels at all,
// returns Found=false to prevent potion spam.
func ReadSPFill(img image.Image, sp Rect) BarRead {
	if sp.W < 1 || sp.H < 1 {
		return BarRead{Found: false}
	}
	sp = trimBarEdges(img, sp, false)
	if sp.W < 1 {
		return BarRead{Found: false}
	}
	if sp.H >= 3 {
		br := readBarFillSingleRow(img, sp.X, sp.Y+1, sp.W, isSPFill)
		if br.FilledWidth == 0 && barHasNoColorPixels(img, sp, false) {
			return BarRead{Found: false, FullWidth: sp.W}
		}
		return normalizeBarRead(img, sp, false, br)
	}
	best := BarRead{Found: true, FullWidth: sp.W}
	for row := 0; row < sp.H; row++ {
		br := readBarFillSingleRow(img, sp.X, sp.Y+row, sp.W, isSPFill)
		if br.FilledWidth > best.FilledWidth {
			best = br
		}
	}
	if best.FilledWidth == 0 && barHasNoColorPixels(img, sp, false) {
		return BarRead{Found: false, FullWidth: sp.W}
	}
	return normalizeBarRead(img, sp, false, best)
}

func readBarFillSingleRow(img image.Image, x0, y, w int, isPixel func(r, g, b uint8) bool) BarRead {
	filled := 0
	for col := 0; col < w; col++ {
		rp, gp, bp := pixelAt(img, x0+col, y)
		if isPixel(rp, gp, bp) {
			filled++
			continue
		}
		if filled > 0 {
			break
		}
	}
	return barReadFromFill(filled, w)
}

func barReadFromFill(filled, full int) BarRead {
	pct := float64(filled) * 100 / float64(full)
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return BarRead{
		Percent:     pct,
		FilledWidth: filled,
		FullWidth:   full,
		Found:       true,
	}
}

func trimBarEdges(img image.Image, r Rect, hpBar bool) Rect {
	y := r.Y + r.H/2
	for r.W > 0 {
		rp, gp, bp := pixelAt(img, r.X, y)
		if barEdgePixel(rp, gp, bp, hpBar) {
			break
		}
		r.X++
		r.W--
	}
	for r.W > 0 {
		rp, gp, bp := pixelAt(img, r.X+r.W-1, y)
		if barEdgePixel(rp, gp, bp, hpBar) {
			break
		}
		r.W--
	}
	return r
}

func barEdgePixel(r, g, b uint8, hpBar bool) bool {
	if hpBar {
		return IsHPPixel(r, g, b) || isHPTrack(r, g, b)
	}
	return IsSPPixel(r, g, b) || isSPFill(r, g, b) || isHPTrack(r, g, b)
}

func normalizeBarRead(img image.Image, r Rect, hpBar bool, read BarRead) BarRead {
	if !read.Found || r.W < 2 {
		return read
	}
	if !BarLooksFull(img, r, hpBar) {
		return read
	}
	if read.FullWidth < 1 {
		read.FullWidth = r.W
	}
	read.FilledWidth = read.FullWidth
	read.Percent = 100
	read.Found = true
	return read
}

// BarLooksFull reports whether the bar in rect appears to be at 100% fill.
// Exported (uppercase) so the runner package can re-export it for tests.
func BarLooksFull(img image.Image, r Rect, hpBar bool) bool {
	if r.W < 2 {
		return false
	}
	if barRightHasEmptyTrack(img, r, hpBar) {
		return false
	}
	return bestFillWidth(img, r, hpBar) >= r.W-2
}

func bestFillWidth(img image.Image, r Rect, hpBar bool) int {
	if !hpBar && r.H >= 3 {
		return readBarFillSingleRow(img, r.X, r.Y+1, r.W, isSPFill).FilledWidth
	}
	isPixel := isHPFillRead
	if !hpBar {
		isPixel = isSPFill
	}
	best := 0
	for row := 0; row < r.H; row++ {
		br := readBarFillSingleRow(img, r.X, r.Y+row, r.W, isPixel)
		if br.FilledWidth > best {
			best = br.FilledWidth
		}
	}
	return best
}

func barConfirmedNotFull(img image.Image, r Rect, hpBar bool, read BarRead) bool {
	if !read.Found || !barReadConsistent(img, r, hpBar, read) {
		return false
	}
	if BarLooksFull(img, r, hpBar) || read.Percent >= 99 {
		return false
	}
	return barRightHasEmptyTrack(img, r, hpBar)
}

func barReadConsistent(img image.Image, r Rect, hpBar bool, read BarRead) bool {
	if !read.Found || r.W < 2 {
		return false
	}
	if BarLooksFull(img, r, hpBar) {
		return true
	}
	fillW := bestFillWidth(img, r, hpBar)
	if fillW == 0 {
		return read.FilledWidth == 0
	}
	if read.FilledWidth == 0 {
		return false
	}
	if read.FilledWidth < fillW-2 {
		return false
	}
	return read.FilledWidth <= fillW+1
}

func barHasNoColorPixels(img image.Image, r Rect, hpBar bool) bool {
	isPixel := IsHPPixel
	if !hpBar {
		isPixel = isSPFill
	}
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			rp, gp, bp := pixelAt(img, x, y)
			if isPixel(rp, gp, bp) {
				return false
			}
		}
	}
	return true
}

func barRightHasEmptyTrack(img image.Image, r Rect, hpBar bool) bool {
	if r.W < 4 || r.H < 1 {
		return false
	}
	checkCols := r.W / 5
	if checkCols < 3 {
		checkCols = 3
	}
	if checkCols > 8 {
		checkCols = 8
	}
	for row := 0; row < r.H; row++ {
		y := r.Y + row
		empty := 0
		for col := r.W - checkCols; col < r.W; col++ {
			rp, gp, bp := pixelAt(img, r.X+col, y)
			if isBarEmptyPixel(rp, gp, bp, hpBar) {
				empty++
			}
		}
		if empty >= 2 {
			return true
		}
	}
	return false
}

func isBarEmptyPixel(r, g, b uint8, hpBar bool) bool {
	if hpBar {
		if IsHPPixel(r, g, b) || isHPFillRead(r, g, b) {
			return false
		}
	} else if IsSPPixel(r, g, b) || isSPFill(r, g, b) {
		return false
	}
	return isHPTrack(r, g, b) || isBarBackground(r, g, b)
}

// barFromRead creates a debug visualization Bar from a Rect and BarRead.
func barFromRead(r Rect, br BarRead) Bar {
	return Bar{
		Left:        r.X,
		Right:       r.X + r.W - 1,
		Y:           r.Y,
		Width:       r.W,
		Height:      r.H,
		FilledWidth: br.FilledWidth,
		Percent:     br.Percent,
		Found:       br.Found,
	}
}
