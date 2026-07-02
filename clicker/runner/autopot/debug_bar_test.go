package autopot

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// drawRectOutline draws a 1px-thick rectangle outline on dst.
func drawRectOutline(dst draw.Image, r image.Rectangle, c color.Color) {
	if r.Empty() {
		return
	}
	b := dst.Bounds()
	x1 := clampTo(r.Min.X, b.Min.X, b.Max.X-1)
	y1 := clampTo(r.Min.Y, b.Min.Y, b.Max.Y-1)
	x2 := clampTo(r.Max.X-1, b.Min.X, b.Max.X-1)
	y2 := clampTo(r.Max.Y-1, b.Min.Y, b.Max.Y-1)
	for x := x1; x <= x2; x++ {
		dst.Set(x, y1, c)
		dst.Set(x, y2, c)
	}
	for y := y1; y <= y2; y++ {
		dst.Set(x1, y, c)
		dst.Set(x2, y, c)
	}
}

// drawCross draws a + crosshair at (cx, cy) with the given arm length.
func drawCross(dst draw.Image, cx, cy, size int, c color.Color) {
	b := dst.Bounds()
	for x := clampTo(cx-size, b.Min.X, b.Max.X-1); x <= clampTo(cx+size, b.Min.X, b.Max.X-1); x++ {
		if cy >= b.Min.Y && cy < b.Max.Y {
			dst.Set(x, cy, c)
		}
	}
	for y := clampTo(cy-size, b.Min.Y, b.Max.Y-1); y <= clampTo(cy+size, b.Min.Y, b.Max.Y-1); y++ {
		if cx >= b.Min.X && cx < b.Max.X {
			dst.Set(cx, y, c)
		}
	}
}

// drawFillLine draws a horizontal line from (x, y) to (x+width-1, y) showing
// the filled portion of the bar in bright colour and the unfilled portion in dim.
func drawFillLine(dst draw.Image, x, y, barW int, fillPct float64, bright, dim color.RGBA) {
	b := dst.Bounds()
	fillW := int(fillPct/100*float64(barW) + 0.5)
	for col := 0; col < barW; col++ {
		px := x + col
		if px < b.Min.X || px >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
			continue
		}
		if col < fillW {
			dst.Set(px, y, bright)
		} else {
			dst.Set(px, y, dim)
		}
	}
}

func clampTo(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// debugDrawBars draws the detected HP/SP bars onto the source image.
// Green = HP, Blue = SP, Yellow = merged block.
// Below the bars, a 3px-tall fill-visualisation line shows fill% (bright = filled).
func debugDrawBars(src image.Image, mapped MappedBars, hp, sp BarRead) image.Image {
	dst := image.NewRGBA(src.Bounds())
	draw.Draw(dst, dst.Bounds(), src, image.Point{}, draw.Src)

	green := color.RGBA{R: 0, G: 255, B: 0, A: 255}
	blue := color.RGBA{R: 50, G: 100, B: 255, A: 255}
	yellow := color.RGBA{R: 255, G: 255, B: 0, A: 255}
	red := color.RGBA{R: 255, G: 40, B: 40, A: 255}

	hpR := image.Rect(mapped.HP.X, mapped.HP.Y, mapped.HP.X+mapped.HP.W, mapped.HP.Y+mapped.HP.H)
	spR := image.Rect(mapped.SP.X, mapped.SP.Y, mapped.SP.X+mapped.SP.W, mapped.SP.Y+mapped.SP.H)
	blockR := image.Rect(mapped.Block.X, mapped.Block.Y, mapped.Block.X+mapped.Block.W, mapped.Block.Y+mapped.Block.H)

	// Outline the merged block in yellow.
	drawRectOutline(dst, blockR, yellow)

	// Outline HP and SP bar rects.
	drawRectOutline(dst, hpR, green)
	drawRectOutline(dst, spR, blue)

	// Crosshairs at bar centres.
	hpCX := mapped.HP.X + mapped.HP.W/2
	hpCY := mapped.HP.Y + mapped.HP.H/2
	spCX := mapped.SP.X + mapped.SP.W/2
	spCY := mapped.SP.Y + mapped.SP.H/2
	drawCross(dst, hpCX, hpCY, 5, green)
	drawCross(dst, spCX, spCY, 5, blue)

	// Fill-visualisation line below each bar.
	if hp.Found {
		hpFillY := mapped.HP.Y + mapped.HP.H + 1
		drawFillLine(dst, mapped.HP.X, hpFillY, mapped.HP.W, hp.Percent, green, dimColor(green))
	}
	if sp.Found {
		spFillY := mapped.SP.Y + mapped.SP.H + 1
		drawFillLine(dst, mapped.SP.X, spFillY, mapped.SP.W, sp.Percent, blue, dimColor(blue))
	}

	// Legend bar at top-left.
	drawLegend(dst, map[string]color.RGBA{
		"HP bar":    green,
		"SP bar":    blue,
		"Block":     yellow,
		"HP fill":   green,
		"SP fill":   blue,
		"Not found": red,
	})

	// If bars not found, draw a red crosshair at the expected search centre.
	if !mapped.Valid || !hp.Found || !sp.Found {
		cx := src.Bounds().Dx() / 2
		cy := src.Bounds().Dy()/2 + mapROICenterYOffset
		drawCross(dst, cx, cy, 8, red)
		drawRectOutline(dst, image.Rect(cx-mapROIHalfW, cy-mapROIHalfH, cx+mapROIHalfW, cy+mapROIHalfH), red)
	}

	return dst
}

func dimColor(c color.RGBA) color.RGBA {
	return color.RGBA{R: uint8(int(c.R) / 3), G: uint8(int(c.G) / 3), B: uint8(int(c.B) / 3), A: c.A}
}

func drawLegend(dst draw.Image, items map[string]color.RGBA) {
	x, y := 6, 6
	swatchW := 12
	swatchH := 4
	gap := 10
	labels := make([]string, 0, len(items))
	for k := range items {
		labels = append(labels, k)
	}
	sort.Strings(labels)
	for _, label := range labels {
		c := items[label]
		// Small swatch rectangle.
		for px := x; px < x+swatchW; px++ {
			for py := y; py < y+swatchH; py++ {
				if px < dst.Bounds().Max.X && py < dst.Bounds().Max.Y {
					dst.Set(px, py, c)
				}
			}
		}
		y += gap
	}
}

// TestDebugBarMappingOnAllFixtures runs RefreshBarPair + ReadHPFill/ReadSPFill
// on every PNG in testdata/ and writes a debug screenshot with bar overlays
// to testdata/debug_output/.
func TestDebugBarMappingOnAllFixtures(t *testing.T) {
	dir := testdataDir(t)
	outDir := filepath.Join(dir, "debug_output")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("mkdir debug_output: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read testdata dir: %v", err)
	}

	pngs := make([]string, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".png") {
			pngs = append(pngs, e.Name())
		}
	}
	sort.Strings(pngs)

	if len(pngs) == 0 {
		t.Fatal("no PNG fixtures found in testdata")
	}

	t.Logf("Processing %d PNG fixtures → %s", len(pngs), outDir)

	for _, name := range pngs {
		t.Run(name, func(t *testing.T) {
			img := loadFixture(t, name)
			bounds := img.Bounds()
			t.Logf("  %s: %dx%d", name, bounds.Dx(), bounds.Dy())

			// Locate bar pair.
			mapped, err := RefreshBarPair(img)
			if err != nil {
				t.Logf("  %s: RefreshBarPair FAILED: %v", name, err)
				// Still produce a debug image showing the search ROI.
				mapped = MappedBars{
					Block: Rect{
						X: bounds.Dx()/2 - mapROIHalfW,
						Y: bounds.Dy()/2 + mapROICenterYOffset - mapROIHalfH,
						W: mapROIHalfW * 2,
						H: mapROIHalfH * 2,
					},
					HP: Rect{X: bounds.Dx() / 2, Y: bounds.Dy()/2 + mapROICenterYOffset, W: 0, H: 0},
					SP: Rect{X: bounds.Dx() / 2, Y: bounds.Dy()/2 + mapROICenterYOffset, W: 0, H: 0},
				}
			} else {
				hp := ReadHPFill(img, mapped.HP)
				sp := ReadSPFill(img, mapped.SP)
				t.Logf("  %s: HP=%.1f%% (found=%v) rect(%d,%d %dx%d) SP=%.1f%% (found=%v) rect(%d,%d %dx%d) score=%d",
					name,
					hp.Percent, hp.Found, mapped.HP.X, mapped.HP.Y, mapped.HP.W, mapped.HP.H,
					sp.Percent, sp.Found, mapped.SP.X, mapped.SP.Y, mapped.SP.W, mapped.SP.H,
					mapped.MapScore)
			}

			// Read fills even on partial success.
			hp := BarRead{}
			sp := BarRead{}
			if mapped.HP.W > 0 {
				hp = ReadHPFill(img, mapped.HP)
			}
			if mapped.SP.W > 0 {
				sp = ReadSPFill(img, mapped.SP)
			}

			// Produce debug image.
			debug := debugDrawBars(img, mapped, hp, sp)

			base := strings.TrimSuffix(name, filepath.Ext(name))
			outPath := filepath.Join(outDir, base+"_debug.png")
		f, err := os.Create(outPath)
		if err != nil {
			t.Fatalf("create %s: %v", outPath, err)
		}
		defer f.Close()
		if err := png.Encode(f, debug); err != nil {
			t.Fatalf("encode %s: %v", outPath, err)
		}
			t.Logf("  → wrote %s", outPath)
		})
	}
}
