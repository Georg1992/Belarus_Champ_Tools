package autopot

import (
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDebugFalseZeroTriggerPixels(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "testdata", "false_zero_trigger.png")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	bounds := img.Bounds()
	t.Logf("Image: %dx%d", bounds.Dx(), bounds.Dy())

	cx := bounds.Dx() / 2
	cy := bounds.Dy()/2 + mapROICenterYOffset
	halfW, halfH := mapROIHalfW, mapROIHalfH

	t.Logf("Search ROI center (%d,%d) %dx%d", cx, cy, halfW*2, halfH*2)

	// Find HP and SP color-run rows
	hpRows := map[int]int{}
	spRows := map[int]int{}
	for y := cy - halfH; y < cy+halfH; y++ {
		hc, sc := 0, 0
		for x := cx - halfW; x < cx+halfW; x++ {
			r, g, b := pixelAt(img, x, y)
			if IsHPPixel(r, g, b) {
				hc++
			}
			if IsSPPixel(r, g, b) {
				sc++
			}
		}
		if hc > 0 {
			hpRows[y] = hc
		}
		if sc > 0 {
			spRows[y] = sc
		}
	}

	// Log HP row summary
	var hpSummaries, spSummaries []string
	for y := cy - halfH; y < cy+halfH; y++ {
		if hc, ok := hpRows[y]; ok {
			// Show a sample pixel in this row
			var sampleR, sampleG, sampleB uint8
			for x := cx - halfW; x < cx+halfW; x++ {
				r, g, b := pixelAt(img, x, y)
				if IsHPPixel(r, g, b) {
					sampleR, sampleG, sampleB = r, g, b
					break
				}
			}
			hpSummaries = append(hpSummaries, fmt.Sprintf("Y=%d:%d(%d,%d,%d)", y, hc, sampleR, sampleG, sampleB))
		}
		if sc, ok := spRows[y]; ok {
			var sampleR, sampleG, sampleB uint8
			for x := cx - halfW; x < cx+halfW; x++ {
				r, g, b := pixelAt(img, x, y)
				if IsSPPixel(r, g, b) {
					sampleR, sampleG, sampleB = r, g, b
					break
				}
			}
			spSummaries = append(spSummaries, fmt.Sprintf("Y=%d:%d(%d,%d,%d)", y, sc, sampleR, sampleG, sampleB))
		}
	}
	t.Logf("HP rows: %s", strings.Join(hpSummaries, " "))
	t.Logf("SP rows: %s", strings.Join(spSummaries, " "))

	// Show the bar detection result
	mapped, err := RefreshBarPair(img)
	if err != nil {
		t.Logf("RefreshBarPair FAILED: %v", err)
		return
	}
	t.Logf("Bar found: HP rect(%d,%d %dx%d) SP rect(%d,%d %dx%d) score=%d",
		mapped.HP.X, mapped.HP.Y, mapped.HP.W, mapped.HP.H,
		mapped.SP.X, mapped.SP.Y, mapped.SP.W, mapped.SP.H, mapped.MapScore)

	hp := ReadHPFill(img, mapped.HP)
	sp := ReadSPFill(img, mapped.SP)
	t.Logf("HP=%.1f%% found=%v filledW=%d fullW=%d", hp.Percent, hp.Found, hp.FilledWidth, hp.FullWidth)
	t.Logf("SP=%.1f%% found=%v filledW=%d fullW=%d", sp.Percent, sp.Found, sp.FilledWidth, sp.FullWidth)

	// Find the BEST HP bar position by scanning the actual HP pixels
	// The HP bar should be where the most HP pixels are concentrated
	bestHPY := 0
	bestHPCount := 0
	for y, hc := range hpRows {
		if hc > bestHPCount {
			bestHPCount = hc
			bestHPY = y
		}
	}
	t.Logf("Peak HP pixel row: Y=%d (%d HP pixels)", bestHPY, bestHPCount)

	// Now try reading HP fill from the correct position - scan a 3px window around each potential HP bar position
	t.Logf("\nHP fill scan at various Y positions (corrected rect placement):")
	correctedHP := Rect{X: mapped.HP.X, W: mapped.HP.W, H: 3}
	for hpY := cy - halfH; hpY < cy+halfH; hpY++ {
		correctedHP.Y = hpY
		// Trim edges
		trimmed := trimBarEdges(img, correctedHP, true)
		if trimmed.W < 5 {
			continue
		}
		hpRead := ReadHPFill(img, correctedHP)
		if hpRead.Found && hpRead.FilledWidth > 0 && hpRead.Percent > 0 {
			t.Logf("  HP rect Y=%d: %.1f%% fill=%d/%d", hpY, hpRead.Percent, hpRead.FilledWidth, hpRead.FullWidth)
		}
	}
}
