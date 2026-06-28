package runner

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"time"
)

// barPairDebugScan contains debug information about bar pair detection.
type barPairDebugScan struct {
	screenCX, screenCY int
	roi                Rect
	hpRuns, spRuns     []ColorRun
	hpRun, spRun       ColorRun
	pairCX, pairCY     int
}

// scanBarPairDebug returns detailed debug information about detected bar pairs.
func scanBarPairDebug(img image.Image) barPairDebugScan {
	b := img.Bounds()
	cx := b.Min.X + b.Dx()/2
	cy := b.Min.Y + b.Dy()/2
	roi := driftROI(b)
	roiCX := roi.X + roi.W/2
	roiCY := roi.Y + roi.H/2
	hpRuns := consolidateRuns(scanColorRuns(img, roi, IsHPPixel, "hp"))
	spRuns := consolidateRuns(scanColorRuns(img, roi, IsSPPixel, "sp"))
	hpRun, spRun, _, _ := findPlayerBarPair(hpRuns, spRuns, roi, roiCX, roiCY)
	pcx, pcy := pairCenter(hpRun, spRun)
	return barPairDebugScan{
		screenCX: cx,
		screenCY: cy,
		roi:      roi,
		hpRuns:   hpRuns,
		spRuns:   spRuns,
		hpRun:    hpRun,
		spRun:    spRun,
		pairCX:   pcx,
		pairCY:   pcy,
	}
}

// FormatMappedBarsLog returns a detailed debug log of mapped bar information.
func FormatMappedBarsLog(img image.Image, bars MappedBars, hp, sp BarRead, refreshed bool) string {
	scan := scanBarPairDebug(img)
	mode := "reused"
	if refreshed {
		mode = "refreshed"
	}
	age := time.Since(bars.LastMapped)
	return fmt.Sprintf(
		"driftROI x=%d y=%d w=%d h=%d\n"+
			"screenCenter x=%d y=%d\n"+
			"pairCenter x=%d y=%d\n"+
			"barPair=%s remapAge=%s\n"+
			"hpRuns=%d spRuns=%d\n"+
			"selectedHP x1=%d x2=%d y=%d w=%d\n"+
			"selectedSP x1=%d x2=%d y=%d w=%d\n"+
			"mapScore=%d\n"+
			"mapped HP rect x=%d y=%d w=%d h=%d\n"+
			"mapped SP rect x=%d y=%d w=%d h=%d\n"+
			"%s\n%s",
		scan.roi.X, scan.roi.Y, scan.roi.W, scan.roi.H,
		scan.screenCX, scan.screenCY,
		scan.pairCX, scan.pairCY,
		mode, formatDuration(age),
		len(scan.hpRuns), len(scan.spRuns),
		scan.hpRun.X1, scan.hpRun.X2, scan.hpRun.Y, scan.hpRun.Width,
		scan.spRun.X1, scan.spRun.X2, scan.spRun.Y, scan.spRun.Width,
		bars.MapScore,
		bars.HP.X, bars.HP.Y, bars.HP.W, bars.HP.H,
		bars.SP.X, bars.SP.Y, bars.SP.W, bars.SP.H,
		FormatBarReadLog("HP", bars.HP, hp),
		FormatBarReadLog("SP", bars.SP, sp),
	)
}

// FormatBarReadLog returns a debug log of bar read information.
func FormatBarReadLog(name string, r Rect, br BarRead) string {
	if !br.Found {
		return name + ": not found"
	}
	return fmt.Sprintf(
		"%s:\n"+
			"x=%d\n"+
			"y=%d\n"+
			"w=%d\n"+
			"h=%d\n"+
			"fillPx=%d\n"+
			"fullPx=%d\n"+
			"percent=%.0f%%",
		name,
		r.X, r.Y, r.W, r.H,
		br.FilledWidth, br.FullWidth,
		br.Percent,
	)
}

// formatDuration returns a human-readable duration string for debug output.
func formatDuration(d time.Duration) string {
	ms := d.Milliseconds()
	if ms < 0 {
		ms = 0
	}
	return fmt.Sprintf("%dms", ms)
}

// SaveMappedBarsDebug saves a debug image showing bar detection visualization.
// path should be empty string for production; BAR_SEARCH_DEBUG env var enables this.
func SaveMappedBarsDebug(img image.Image, bars MappedBars, path string) error {
	if path == "" {
		return nil
	}
	scan := scanBarPairDebug(img)

	out := imageToRGBA(img)
	drawRectOutline(out, scan.roi, color.RGBA{R: 255, G: 255, B: 0, A: 255})
	drawCross(out, scan.screenCX, scan.screenCY, color.RGBA{R: 255, G: 0, B: 255, A: 255})
	drawCross(out, scan.pairCX, scan.pairCY, color.RGBA{R: 255, G: 128, B: 0, A: 255})
	for _, r := range scan.hpRuns {
		drawRunOutline(out, r, color.RGBA{R: 0, G: 180, B: 0, A: 255})
	}
	for _, r := range scan.spRuns {
		drawRunOutline(out, r, color.RGBA{R: 80, G: 160, B: 255, A: 255})
	}
	drawRunOutline(out, scan.hpRun, color.RGBA{R: 0, G: 255, B: 0, A: 255})
	drawRunOutline(out, scan.spRun, color.RGBA{R: 0, G: 200, B: 255, A: 255})

	hp := barFromRead(bars.HP, ReadHPFill(img, bars.HP))
	sp := barFromRead(bars.SP, ReadSPFill(img, bars.SP))
	drawBarRectDebug(out, hp, color.RGBA{R: 0, G: 255, B: 0, A: 255}, color.RGBA{R: 0, G: 200, B: 0, A: 255})
	drawBarRectDebug(out, sp, color.RGBA{R: 0, G: 128, B: 255, A: 255}, color.RGBA{R: 0, G: 200, B: 255, A: 255})

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, out)
}

// drawCross draws a crosshair at the given position for debug visualization.
func drawCross(img *image.RGBA, x, y int, c color.RGBA) {
	b := img.Bounds()
	for dx := -debugCrossSize; dx <= debugCrossSize; dx++ {
		px := x + dx
		if px >= b.Min.X && px < b.Max.X && y >= b.Min.Y && y < b.Max.Y {
			img.Set(px, y, c)
		}
	}
	for dy := -debugCrossSize; dy <= debugCrossSize; dy++ {
		py := y + dy
		if x >= b.Min.X && x < b.Max.X && py >= b.Min.Y && py < b.Max.Y {
			img.Set(x, py, c)
		}
	}
}

// drawRunOutline draws a horizontal line representing a color run.
func drawRunOutline(img *image.RGBA, r ColorRun, c color.RGBA) {
	for x := r.X1; x <= r.X2; x++ {
		img.Set(x, r.Y, c)
	}
}

// drawRectOutline draws the outline of a rectangle.
func drawRectOutline(img *image.RGBA, r Rect, c color.RGBA) {
	for x := r.X; x < r.X+r.W; x++ {
		img.Set(x, r.Y, c)
		img.Set(x, r.Y+r.H-1, c)
	}
	for y := r.Y; y < r.Y+r.H; y++ {
		img.Set(r.X, y, c)
		img.Set(r.X+r.W-1, y, c)
	}
}

// drawBarRectDebug draws a bar rectangle showing outline and fill level.
func drawBarRectDebug(img *image.RGBA, bar Bar, outline, fill color.RGBA) {
	if !bar.Found {
		return
	}
	h := bar.Height
	if h < 1 {
		h = barRowHeight
	}
	for dy := 0; dy < h; dy++ {
		y := bar.Y + dy
		img.Set(bar.Left, y, outline)
		img.Set(bar.Right, y, outline)
	}
	for dx := 0; dx < bar.Width; dx++ {
		x := bar.Left + dx
		img.Set(x, bar.Y, outline)
		img.Set(x, bar.Y+h-1, outline)
	}
	for dx := 0; dx < bar.FilledWidth; dx++ {
		x := bar.Left + dx
		for dy := 0; dy < h; dy++ {
			img.Set(x, bar.Y+dy, fill)
		}
	}
}

// imageToRGBA converts any image to RGBA format for manipulation.
func imageToRGBA(img image.Image) *image.RGBA {
	bounds := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := bounds.Min.Y; y < bounds.Min.Y+bounds.Dy(); y++ {
		for x := bounds.Min.X; x < bounds.Min.X+bounds.Dx(); x++ {
			out.Set(x-bounds.Min.X, y-bounds.Min.Y, img.At(x, y))
		}
	}
	return out
}
