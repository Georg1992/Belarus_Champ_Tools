package runner

import (
	"image"
	"image/color"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsHPFillColors(t *testing.T) {
	if !isHPGreen(16, 238, 33) {
		t.Fatal("exact HP green should match")
	}
	if !isHPRed(255, 13, 0) {
		t.Fatal("exact HP red should match")
	}
	if !isHPFill(255, 13, 0) {
		t.Fatal("HP fill should include red")
	}
	if !isSPFill(25, 101, 225) {
		t.Fatal("exact SP blue should match")
	}
}

func TestFindHPBarSyntheticFull(t *testing.T) {
	img := newBarTestImage(100, 50, false)
	hp := FindHPBar(img)
	sp := FindSPBar(img)
	if !hp.Found || !sp.Found {
		t.Fatalf("bars not found hp=%v sp=%v", hp.Found, sp.Found)
	}
	if hp.Percent < 95 {
		t.Fatalf("HP %.1f%% want ~100", hp.Percent)
	}
	if sp.Percent < 40 || sp.Percent > 60 {
		t.Fatalf("SP %.1f%% want ~50", sp.Percent)
	}
}

func TestFindHPBarSyntheticLow(t *testing.T) {
	img := newBarTestImage(12, 40, true)
	hp := FindHPBar(img)
	if !hp.Found {
		t.Fatal("HP bar not found")
	}
	if hp.Percent < 8 || hp.Percent > 20 {
		t.Fatalf("HP %.1f%% want ~12", hp.Percent)
	}
}

func TestFindHPBarScreenFull(t *testing.T) {
	crop := cropPlayerBarSearch(loadFixture(t, "screen_full_bars.jpg"))
	hp := FindHPBar(crop)
	sp := FindSPBar(crop)
	if !hp.Found || !sp.Found {
		t.Fatalf("bars not found hp=%v sp=%v", hp.Found, sp.Found)
	}
	if hp.Percent < 95 {
		t.Fatalf("HP %.1f%% want ~100", hp.Percent)
	}
	if sp.Percent < 95 {
		t.Fatalf("SP %.1f%% want ~100", sp.Percent)
	}
	t.Logf("HP width=%d fill=%d %.1f%%", hp.Width, hp.FilledWidth, hp.Percent)
}

func TestFindHPBarScreenLowHP(t *testing.T) {
	crop := cropPlayerBarSearch(loadFixture(t, "screen_low_hp.jpg"))
	hp := FindHPBar(crop)
	sp := FindSPBar(crop)
	if !hp.Found || !sp.Found {
		t.Fatalf("bars not found hp=%v sp=%v", hp.Found, sp.Found)
	}
	if hp.Percent > 35 {
		t.Fatalf("HP %.1f%% want low", hp.Percent)
	}
	t.Logf("HP %.1f%% SP %.1f%%", hp.Percent, sp.Percent)
}

func TestFindHPBarScreenZoomed(t *testing.T) {
	crop := cropPlayerBarSearch(loadFixture(t, "screen_zoomed_full.jpg"))
	hp := FindHPBar(crop)
	sp := FindSPBar(crop)
	if !hp.Found || !sp.Found {
		t.Fatalf("bars not found hp=%v sp=%v", hp.Found, sp.Found)
	}
	if hp.Percent < 95 {
		t.Fatalf("HP %.1f%% want ~100", hp.Percent)
	}
	if sp.Percent < 95 {
		t.Fatalf("SP %.1f%% want ~100", sp.Percent)
	}
}

func loadFixture(t *testing.T, name string) image.Image {
	t.Helper()
	path := filepath.Join(testdataDir(t), name)
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return img
}

func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "testdata")
}

func newBarTestImage(hpPct, spPct int, hpRed bool) *image.RGBA {
	w := searchHalfWidth * 2
	h := searchBottomOff - searchTopOffset
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	floor := color.RGBA{R: 30, G: 30, B: 35, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, floor)
		}
	}
	hpFill := color.RGBA{R: hpGreenR, G: hpGreenG, B: hpGreenB, A: 255}
	if hpRed {
		hpFill = color.RGBA{R: hpRedR, G: hpRedG, B: hpRedB, A: 255}
	}
	drawTestBar(img, 65, 38, hpPct, hpFill)
	drawTestBar(img, 65, 42, spPct, color.RGBA{R: spBarR, G: spBarG, B: spBarB, A: 255})
	return img
}

func drawTestBar(img *image.RGBA, x, y, fillPct int, fill color.RGBA) {
	track := color.RGBA{R: 10, G: 10, B: 14, A: 255}
	barW := 31
	fillW := barW * fillPct / 100
	for dy := 0; dy < 4; dy++ {
		for dx := 0; dx < barW; dx++ {
			c := track
			if dx < fillW {
				c = fill
			}
			img.Set(x+dx, y+dy, c)
		}
	}
}

func cropPlayerBarSearch(full image.Image) *image.RGBA {
	bounds := full.Bounds()
	sw, sh := bounds.Dx(), bounds.Dy()
	roi := PlayerBarSearchROI(sw, sh)
	out := image.NewRGBA(image.Rect(0, 0, roi.W, roi.H))
	for y := 0; y < roi.H; y++ {
		for x := 0; x < roi.W; x++ {
			out.Set(x, y, full.At(bounds.Min.X+roi.X+x, bounds.Min.Y+roi.Y+y))
		}
	}
	return out
}
