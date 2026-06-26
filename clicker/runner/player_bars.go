package runner

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

const (
	searchHalfWidth = 80
	searchTopOffset = -30
	searchBottomOff = 120

	hpGreenR, hpGreenG, hpGreenB = 16, 238, 33
	hpRedR, hpRedG, hpRedB       = 255, 13, 0
	spBarR, spBarG, spBarB       = 25, 101, 225

	fillTol = 45
)

type BarROI struct {
	X, Y, W, H int
}

type Bar struct {
	Left, Right int
	Y           int
	Width       int
	FilledWidth int
	Percent     float64
	Found       bool
}

func PlayerBarSearchROI(screenW, screenH int) BarROI {
	cx := screenW / 2
	cy := screenH / 2
	return BarROI{
		X: cx - searchHalfWidth,
		Y: cy + searchTopOffset,
		W: searchHalfWidth * 2,
		H: searchBottomOff - searchTopOffset,
	}
}

func isHPGreen(r, g, b uint8) bool {
	if colorNear(r, g, b, hpGreenR, hpGreenG, hpGreenB, fillTol) {
		return true
	}
	ri, gi, bi := int(r), int(g), int(b)
	return gi > 130 && gi > ri+15 && gi > bi+15
}

func isHPRed(r, g, b uint8) bool {
	ri, gi, bi := int(r), int(g), int(b)
	if gi > 60 || ri < 90 {
		return false
	}
	if ri > gi+15 && ri > bi {
		return true
	}
	return colorNear(r, g, b, hpRedR, hpRedG, hpRedB, fillTol)
}

func isHPFill(r, g, b uint8) bool {
	return isHPGreen(r, g, b) || isHPRed(r, g, b)
}

func isSPFill(r, g, b uint8) bool {
	return colorNear(r, g, b, spBarR, spBarG, spBarB, fillTol)
}

func colorNear(r, g, b, refR, refG, refB uint8, tol int) bool {
	return absInt(int(r)-int(refR)) <= tol &&
		absInt(int(g)-int(refG)) <= tol &&
		absInt(int(b)-int(refB)) <= tol
}

func isBarTrack(r, g, b uint8) bool {
	ri, gi, bi := int(r), int(g), int(b)
	max := ri
	if gi > max {
		max = gi
	}
	if bi > max {
		max = bi
	}
	if max > 22 {
		return false
	}
	if ri >= 18 && gi >= 18 && bi >= 18 {
		return false
	}
	return true
}

func FindHPBar(img image.Image) Bar {
	sp := FindSPBar(img)
	yMin := img.Bounds().Min.Y
	yMax := img.Bounds().Max.Y
	if sp.Found {
		yMax = sp.Y
		yMin = sp.Y - 8
		if yMin < img.Bounds().Min.Y {
			yMin = img.Bounds().Min.Y
		}
	}
	hpY := bestFillRow(img, isHPFill, yMin, yMax)
	if hpY < 0 {
		return Bar{}
	}
	fillLeft, fillRight := fillBoundsInBand(img, hpY, isHPFill)
	if fillLeft < 0 {
		return Bar{}
	}

	left, right, width := sp.Left, sp.Right, sp.Width
	if !sp.Found {
		left, right = expandBarBounds(img, hpY, fillLeft, fillRight, isHPFill)
		width = right - left + 1
	}
	if width <= 0 {
		return Bar{}
	}

	filledWidth := fillRight - left + 1
	if filledWidth < 0 {
		filledWidth = 0
	}
	if filledWidth > width {
		filledWidth = width
	}
	return Bar{
		Left: left, Right: right, Y: hpY, Width: width, FilledWidth: filledWidth,
		Percent: float64(filledWidth) * 100 / float64(width), Found: true,
	}
}

func FindSPBar(img image.Image) Bar {
	hpY := bestFillRow(img, isHPFill, img.Bounds().Min.Y, img.Bounds().Max.Y)
	yMin := img.Bounds().Min.Y
	if hpY >= 0 {
		yMin = hpY + 1
	}
	spY := bestFillRow(img, isSPFill, yMin, img.Bounds().Max.Y)
	if spY < 0 {
		return Bar{}
	}
	fillLeft, fillRight := fillBoundsInBand(img, spY, isSPFill)
	if fillLeft < 0 {
		return Bar{}
	}
	left, right := expandBarBounds(img, spY, fillLeft, fillRight, isSPFill)
	width := right - left + 1
	if width <= 0 {
		return Bar{}
	}
	filledWidth := fillRight - left + 1
	if filledWidth < 0 {
		filledWidth = 0
	}
	if filledWidth > width {
		filledWidth = width
	}
	return Bar{
		Left: left, Right: right, Y: spY, Width: width, FilledWidth: filledWidth,
		Percent: float64(filledWidth) * 100 / float64(width), Found: true,
	}
}

func bestFillRow(img image.Image, isFill func(r, g, b uint8) bool, yMin, yMax int) int {
	bestY, bestSpan := -1, 0
	for y := yMin; y < yMax; y++ {
		span := rowFillSpan(img, y, isFill)
		if span < 1 {
			continue
		}
		if span > bestSpan || (span == bestSpan && (bestY < 0 || y < bestY)) {
			bestSpan = span
			bestY = y
		}
	}
	return bestY
}

func fillBoundsInBand(img image.Image, centerY int, isFill func(r, g, b uint8) bool) (left, right int) {
	left = -1
	right = -1
	for _, y := range barBandRows(img, centerY) {
		l, r := rowFillBounds(img, y, isFill)
		if l < 0 {
			continue
		}
		if left < 0 || l < left {
			left = l
		}
		if r > right {
			right = r
		}
	}
	return left, right
}

func barBandRows(img image.Image, centerY int) []int {
	b := img.Bounds()
	rows := []int{centerY - 1, centerY, centerY + 1, centerY + 2}
	out := make([]int, 0, 4)
	for _, y := range rows {
		if y >= b.Min.Y && y < b.Max.Y {
			out = append(out, y)
		}
	}
	return out
}

func expandBarBounds(img image.Image, y, fillLeft, fillRight int, isFill func(r, g, b uint8) bool) (left, right int) {
	bounds := img.Bounds()
	left = fillLeft
	for x := fillLeft - 1; x >= bounds.Min.X; x-- {
		if barColumnOK(img, x, y, isFill) {
			left = x
			continue
		}
		break
	}
	right = fillRight
	for x := fillRight + 1; x < bounds.Max.X; x++ {
		if barColumnOK(img, x, y, isFill) {
			right = x
			continue
		}
		break
	}
	if right == fillRight && needsGapScan(img, y, fillRight, isFill) {
		if edge := findTrackEdgeAfterGap(img, y, fillRight); edge > right {
			right = edge
		}
	}
	return left, right
}

func needsGapScan(img image.Image, y, fillRight int, isFill func(r, g, b uint8) bool) bool {
	x := fillRight + 1
	if x >= img.Bounds().Max.X {
		return false
	}
	r, g, b := pixelAt(img, x, y)
	if isFill(r, g, b) {
		return false
	}
	if isBarTrack(r, g, b) && !barColumnOK(img, x, y, isFill) {
		return false
	}
	return true
}

func findTrackEdgeAfterGap(img image.Image, y, fillRight int) int {
	bounds := img.Bounds()
	for x := fillRight + 1; x < bounds.Max.X; x++ {
		r, g, b := pixelAt(img, x, y)
		if isUIFloor(r, g, b) {
			break
		}
		if trackColumnOK(img, x, y) {
			return x - 1
		}
	}
	return fillRight
}

func isUIFloor(r, g, b uint8) bool {
	ri, gi, bi := int(r), int(g), int(b)
	if ri < 18 || gi < 18 || bi < 18 {
		return false
	}
	max := ri
	if gi > max {
		max = gi
	}
	if bi > max {
		max = bi
	}
	return max <= 35
}

func barColumnOK(img image.Image, x, y int, isFill func(r, g, b uint8) bool) bool {
	r, g, b := pixelAt(img, x, y)
	if !isFill(r, g, b) && !isBarTrack(r, g, b) {
		return false
	}
	ny := y + 1
	if ny >= img.Bounds().Max.Y {
		return false
	}
	r2, g2, b2 := pixelAt(img, x, ny)
	if isFill(r2, g2, b2) {
		return true
	}
	return isBarTrackGap(r2, g2, b2)
}

func trackColumnOK(img image.Image, x, y int) bool {
	for _, ny := range []int{y, y + 1} {
		if ny >= img.Bounds().Max.Y {
			return false
		}
		r, g, b := pixelAt(img, x, ny)
		if !isBarTrackGap(r, g, b) {
			return false
		}
	}
	return true
}

func isBarTrackGap(r, g, b uint8) bool {
	ri, gi, bi := int(r), int(g), int(b)
	max := ri
	if gi > max {
		max = gi
	}
	if bi > max {
		max = bi
	}
	return max <= 16
}

func rowFillSpan(img image.Image, y int, isFill func(r, g, b uint8) bool) int {
	left, right := rowFillBounds(img, y, isFill)
	if left < 0 {
		return 0
	}
	return right - left + 1
}

func rowFillBounds(img image.Image, y int, isFill func(r, g, b uint8) bool) (left, right int) {
	bounds := img.Bounds()
	left = -1
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		r, g, b := pixelAt(img, x, y)
		if isFill(r, g, b) {
			if left < 0 {
				left = x
			}
			right = x
		}
	}
	return left, right
}

func pixelAt(img image.Image, x, y int) (r, g, b uint8) {
	c := img.At(x, y)
	rgba := color.RGBAModel.Convert(c).(color.RGBA)
	return rgba.R, rgba.G, rgba.B
}

func SaveBarSearchDebug(img image.Image, hp, sp Bar, path string) error {
	if path == "" {
		return nil
	}
	out := imageToRGBA(img)
	drawBarDebug(out, hp, color.RGBA{R: 0, G: 255, B: 0, A: 255}, color.RGBA{R: 0, G: 180, B: 0, A: 255})
	drawBarDebug(out, sp, color.RGBA{R: 0, G: 128, B: 255, A: 255}, color.RGBA{R: 0, G: 200, B: 255, A: 255})
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, out)
}

func drawBarDebug(img *image.RGBA, bar Bar, outline, fill color.RGBA) {
	if !bar.Found {
		return
	}
	for x := bar.Left; x <= bar.Right; x++ {
		img.Set(x, bar.Y, outline)
	}
	fillEnd := bar.Left + bar.FilledWidth - 1
	if fillEnd > bar.Right {
		fillEnd = bar.Right
	}
	for x := bar.Left; x <= fillEnd; x++ {
		img.Set(x, bar.Y+1, fill)
	}
}

func FormatBarLog(name string, bar Bar) string {
	if !bar.Found {
		return name + ": not found"
	}
	return name + ":\n" +
		"left=" + itoa(bar.Left) + "\n" +
		"right=" + itoa(bar.Right) + "\n" +
		"fullWidth=" + itoa(bar.Width) + "\n" +
		"filledWidth=" + itoa(bar.FilledWidth) + "\n" +
		"percent=" + ftoa(bar.Percent)
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [12]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func ftoa(v float64) string {
	return itoa(int(v+0.5)) + "%"
}

func imageToRGBA(img image.Image) *image.RGBA {
	bounds := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			out.Set(x-bounds.Min.X, y-bounds.Min.Y, img.At(x, y))
		}
	}
	return out
}
