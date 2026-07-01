package hpspreader

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"experimental-clicker/runner/statusui"
)

// ----------------------------------------------------------------------------
// Self-contained 7×9 monospace bitmap font for the overlay band.
//
// Each rune is encoded as 9 bytes (one per row, top→bottom). Within a
// row, bit 6 is the LEFT column, bit 0 is the RIGHT column — same
// (x_col, y_row) convention as font9 used by the integration test, so
// the existing renderGlyph code in reader_test.go can be mimicked bit
// for bit.
//
// We deliberately keep this font SEPARATE from font9 because:
//
//   - font9 covers only the 16 runes the reader templates recognise
//     (digits 0-9, '.', '/', '|', 'H', 'P', 'S'). The overlay needs
//     'OK', 'FAIL', status codes, etc. — extra uppercase letters.
//   - font9 is asserted by TestReader_IntegrationSynthetic to round-trip
//     the 16 runes exactly; touching it would risk breaking that test.
//
// Drawing a hand-rolled bitmap here keeps the module's dependency
// graph free of golang.org/x/image/font (which the project's
// go.mod intentionally omits). The total glyph count is small enough
// (~30) to hand-design and verify by eye.
// ----------------------------------------------------------------------------

var overlayFont = map[rune][9]byte{
	// Digits — verbatim from font9 for symmetry with the values the
	// reader actually extracted from the strip.
	'0': {0b0111110, 0b1100011, 0b1100111, 0b1101011, 0b1101011, 0b1100111, 0b1100011, 0b0111110, 0b0000000},
	'1': {0b0001100, 0b0011100, 0b0111100, 0b0001100, 0b0001100, 0b0001100, 0b0001100, 0b0111111, 0b0000000},
	'2': {0b0111110, 0b1100011, 0b0000011, 0b0000110, 0b0001100, 0b0011000, 0b0110000, 0b1111111, 0b0000000},
	'3': {0b0111110, 0b1100011, 0b0000011, 0b0001110, 0b0000110, 0b0000011, 0b1100011, 0b0111110, 0b0000000},
	'4': {0b0000110, 0b0001110, 0b0011110, 0b0110110, 0b1100110, 0b1111111, 0b0000110, 0b0000110, 0b0000000},
	'5': {0b1111111, 0b1100000, 0b1111110, 0b0000011, 0b0000011, 0b0000011, 0b1100011, 0b0111110, 0b0000000},
	'6': {0b0011110, 0b0111000, 0b1100000, 0b1111110, 0b1100011, 0b1100011, 0b1100011, 0b0111110, 0b0000000},
	'7': {0b1111111, 0b0000011, 0b0000110, 0b0001100, 0b0011000, 0b0110000, 0b0110000, 0b0110000, 0b0000000},
	'8': {0b0111110, 0b1100011, 0b1100011, 0b0111110, 0b1100011, 0b1100011, 0b1100011, 0b0111110, 0b0000000},
	'9': {0b0111110, 0b1100011, 0b1100011, 0b0111111, 0b0000011, 0b0000011, 0b0000110, 0b0111100, 0b0000000},

	// Symbols / punctuation.
	'.': {0b0000000, 0b0000000, 0b0000000, 0b0000000, 0b0000000, 0b0000000, 0b0011000, 0b0011000, 0b0000000},
	'/': {0b0000011, 0b0000110, 0b0001100, 0b0011000, 0b0110000, 0b1100000, 0b1000000, 0b0000000, 0b0000000},
	':': {0b0000000, 0b0011100, 0b0011100, 0b0000000, 0b0000000, 0b0011100, 0b0011100, 0b0000000, 0b0000000},
	'=': {0b0000000, 0b0000000, 0b1111111, 0b0000000, 0b1111111, 0b0000000, 0b0000000, 0b0000000, 0b0000000},
	'_': {0b0000000, 0b0000000, 0b0000000, 0b0000000, 0b0000000, 0b0000000, 0b0000000, 0b1111111, 0b0000000},
	' ': {0b0000000, 0b0000000, 0b0000000, 0b0000000, 0b0000000, 0b0000000, 0b0000000, 0b0000000, 0b0000000},

	// Uppercase letters added for the overlay alphabet.
	'O': {0b0111110, 0b1100011, 0b1100011, 0b1100011, 0b1100011, 0b1100011, 0b1100011, 0b0111110, 0b0000000},
	'K': {0b1100011, 0b1100110, 0b1101100, 0b1111000, 0b1111000, 0b1101100, 0b1100110, 0b1100011, 0b0000000},
	'F': {0b1111111, 0b1100000, 0b1100000, 0b1111110, 0b1100000, 0b1100000, 0b1100000, 0b1100000, 0b0000000},
	'A': {0b0011100, 0b0110110, 0b0110110, 0b1100011, 0b1111111, 0b1100011, 0b1100011, 0b1100011, 0b0000000},
	'I': {0b0111111, 0b0011100, 0b0011100, 0b0011100, 0b0011100, 0b0011100, 0b0011100, 0b0111111, 0b0000000},
	'L': {0b1100000, 0b1100000, 0b1100000, 0b1100000, 0b1100000, 0b1100000, 0b1100000, 0b1111111, 0b0000000},
	'N': {0b1100011, 0b1110011, 0b1111011, 0b1111111, 0b1101111, 0b1100111, 0b1100011, 0b1100011, 0b0000000},
	'E': {0b1111111, 0b1100000, 0b1100000, 0b1111110, 0b1111110, 0b1100000, 0b1100000, 0b1111111, 0b0000000},
	'R': {0b1111110, 0b1100011, 0b1100011, 0b1111110, 0b1111100, 0b1100110, 0b1100011, 0b1100011, 0b0000000},
	'V': {0b1100011, 0b1100011, 0b1100011, 0b1100011, 0b0110110, 0b0110110, 0b0011100, 0b0011100, 0b0000000},
	'C': {0b0111110, 0b1100011, 0b1100011, 0b1100000, 0b1100000, 0b1100011, 0b1100011, 0b0111110, 0b0000000},
	'D': {0b1111110, 0b1100011, 0b1100011, 0b1100011, 0b1100011, 0b1100011, 0b1100011, 0b1111110, 0b0000000},
	'U': {0b1100011, 0b1100011, 0b1100011, 0b1100011, 0b1100011, 0b1100011, 0b1100011, 0b0111110, 0b0000000},
	'M': {0b1100011, 0b1110111, 0b1111111, 0b1111111, 0b1101011, 0b1100011, 0b1100011, 0b1100011, 0b0000000},
	'G': {0b0111110, 0b1100011, 0b1100000, 0b1101111, 0b1101111, 0b1100011, 0b1100011, 0b0111110, 0b0000000},
	'T': {0b1111111, 0b0011100, 0b0011100, 0b0011100, 0b0011100, 0b0011100, 0b0011100, 0b0011100, 0b0000000},
	'B': {0b1111110, 0b1100011, 0b1100011, 0b1111110, 0b1100011, 0b1100011, 0b1100011, 0b1111110, 0b0000000},
	'W': {0b1100011, 0b1100011, 0b1101011, 0b1101011, 0b1111111, 0b1110111, 0b1100011, 0b1100011, 0b0000000},
	'X': {0b1100011, 0b1100011, 0b0110110, 0b0011100, 0b0011100, 0b0110110, 0b1100011, 0b1100011, 0b0000000},
	'Y': {0b1100011, 0b1100011, 0b0110110, 0b0011100, 0b0011100, 0b0011100, 0b0011100, 0b0011100, 0b0000000},
	'Z': {0b1111111, 0b0000011, 0b0000110, 0b0001100, 0b0011000, 0b0110000, 0b1100000, 0b1111111, 0b0000000},
	'H': {0b1100011, 0b1100011, 0b1100011, 0b1111111, 0b1100011, 0b1100011, 0b1100011, 0b1100011, 0b0000000},
	'P': {0b1111110, 0b1100011, 0b1100011, 0b1111110, 0b1100000, 0b1100000, 0b1100000, 0b1100000, 0b0000000},
	'S': {0b0111111, 0b1100000, 0b1100000, 0b0111110, 0b0000011, 0b0000011, 0b0000011, 0b1111110, 0b0000000},
	'Q': {0b0111110, 0b1100011, 0b1100011, 0b1100011, 0b1101011, 0b1100111, 0b0111111, 0b0000011, 0b0000000},	'J': {0b0001111, 0b0000110,  0b0000110,  0b0000110,  0b0000110,  0b1100110,  0b0111100,  0b0000000,  0b0000000},
	// Defence-in-depth: renderOverlayText falls back to this
	// glyph on unknown runes so missing entries show up as a
	// visible '?' rather than silently disappearing. As of this
	// writing no overlay string uses a rune outside overlayFont.
	'?': {0b0111110, 0b1100011, 0b0000110, 0b0011100, 0b0110000, 0b0000000, 0b0011000, 0b0011000, 0b0000000},
}

// overlayGlyphW / overlayGlyphH are the dimensions of one glyph in
// the overlay font. Both equal font9's dimensions so the two fonts
// could share a single ruler if we ever merged them.
const (
	overlayGlyphW = 7
	overlayGlyphH = 9
)

// renderOverlayText draws s onto dst starting at (x, y) using a
// 1-pixel gap between glyphs. Glyphs outside dst.Bounds() are
// silently clipped by image.Set so the caller doesn't need to bounds-
// check. Unknown runes are rendered as the literal '?' placeholder
// so a missing glyph doesn't disappear silently — a human opening
// the annotated PNG immediately sees a stray '?' and can add the
// missing map entry.
func renderOverlayText(dst *image.RGBA, s string, x, y int, fg color.RGBA) {
	xCursor := x
	for _, r := range s {
		if r == ' ' {
			xCursor += overlayGlyphW + 1
			continue
		}
		glyph, ok := overlayFont[r]
		if !ok {
			glyph = overlayFont['?']
		}
		for row := 0; row < overlayGlyphH; row++ {
			rowByte := glyph[row]
			for col := 0; col < overlayGlyphW; col++ {
				// Same column/polarity convention as font9:
				// bit (6-col) of rowByte is column col.
				if (rowByte>>(6-col))&1 == 1 {
					px := xCursor + col
					py := y + row
					if (image.Point{X: px, Y: py}).In(dst.Bounds()) {
						dst.SetRGBA(px, py, fg)
					}
				}
			}
		}
		xCursor += overlayGlyphW + 1
	}
}

// overlayBandH is how tall the top annotation strip is. Glyph height
// (9) + 1 px padding above + 1 px below = 11.
const overlayBandH = overlayGlyphH + 2

// annotateOverlay fills a horizontal band across the top of dst with
// bg, then renders text on top in fg. The band is full-width so the
// text always sits on a uniformly-coloured backdrop regardless of
// what game pixels were originally at y < overlayBandH.
//
// Status colour mapping is intentionally hard-coded so the test
// output is self-explanatory at a glance:
//
//   - colorOK   → green   strip with black text  (Read OK)
//   - colorFail → red     strip with white text  (Read FAIL)
//   - colorLost → gray    strip with black text  (panel not found /
//                                                 not verified)
type annotation struct {
	text string
	bg   color.RGBA
	fg   color.RGBA
}

var (
	colorOKBand   = color.RGBA{R: 60, G: 200, B: 60, A: 255}
	colorFailBand = color.RGBA{R: 220, G: 30, B: 30, A: 255}
	colorLostBand = color.RGBA{R: 220, G: 220, B: 220, A: 255}
	colorOKText   = color.RGBA{R: 0, G: 0, B: 0, A: 255}
	colorFailText = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	colorLostText = color.RGBA{R: 0, G: 0, B: 0, A: 255}
)

func annotateOverlay(dst *image.RGBA, ann annotation) {
	b := dst.Bounds()
	band := image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+overlayBandH)
	draw.Draw(dst, band, &image.Uniform{C: ann.bg}, image.Point{}, draw.Src)
	renderOverlayText(dst, ann.text, b.Min.X+4, b.Min.Y+1, ann.fg)
}

// encodePNGToFile encodes img as PNG and writes to path. Returns an
// error only on disk I/O — image.Encode failures are treated as bugs
// in the caller so we surface them via t.Fatal at the call site.
func encodePNGToFile(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
}

// loadScreenshot decodes path as PNG. Surfaces a t.Skip if the file
// is missing (so a developer who wipes testdata quietly hears about
// it from the test name, not a Fatal) and a t.Fatal on decode errors.
func loadScreenshot(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("fixture missing %s (%v) — skip", filepath.Base(path), err)
		return nil
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return img
}

// screenFixturesDir resolves the screen-fixtures directory relative
// to this test file (uses runtime.Caller(0) so it works regardless of
// the working directory `go test` is invoked from).
//
// Layout from the test file:
//
//	clicker/internal/vision/hpspreader/end_to_end_test.go
//
//	to reach:
//
//	clicker/runner/autopot/testdata/
//
//stepping up three directories from this file's dir.
func screenFixturesDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot locate testdata")
	}
	d := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "runner", "autopot", "testdata")
	abs, err := filepath.Abs(d)
	if err != nil {
		t.Fatalf("filepath.Abs(%q): %v", d, err)
	}
	return abs
}

// annotatedOutDir resolves the directory annotated PNGs land in:
//
//	clicker/internal/vision/hpspreader/debug/annotated_screenshots/
func annotatedOutDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot locate output dir")
	}
	d := filepath.Join(filepath.Dir(thisFile), "debug", "annotated_screenshots")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", d, err)
	}
	return d
}

// statusCodeFor returns the 1-character code for a Reader result
// reason so the overlay can show "FAIL: N", "FAIL: L", "FAIL: P",
// "FAIL: V" without having to render full English words (and the
// accompanying font alphabet expansion).
//
//	1 = no_components
//	2 = low_glyph_score
//	3 = parse_failed
//	4 = value_validation_failed
func statusCodeFor(reason string) string {
	switch reason {
	case "no_components":
		return "N"
	case "low_glyph_score":
		return "L"
	case "parse_failed":
		return "P"
	case "value_validation_failed":
		return "V"
	}
	return "?"
}

// composeOverlayText builds the annotation text for a single screenshot
// given the four outcomes the test pipeline can observe. It always
// includes the parsed/residual Text where available so an operator can
// see what the reader assembled even on failure.
//
// Result.OK: "OK  HP=X/Y  SP=Z/W"
// Read FAIL: "FAIL_<code>  TXT=<text>"
// Locate FAIL: "FAIL PNL=NONE  SCORE=X.XXXX"
// Verify FAIL: "FAIL VRFY  <blame>"
func composeOverlayText(p stageOutcome) annotation {
	switch {
	case p.readOK:
		txt := fmt.Sprintf("OK   HP=%d/%d  SP=%d/%d  CONF=%.2f",
			p.result.HP, p.result.HPMax,
			p.result.SP, p.result.SPMax,
			p.result.Confidence)
		return annotation{text: txt, bg: colorOKBand, fg: colorOKText}

	case p.result.Text != "":
		// Reader returned a non-OK Result with residual Text.
		code := statusCodeFor(p.result.Reason)
		txt := fmt.Sprintf("FAIL_%s  TXT=%s  CONF=%.2f",
			code, p.result.Text, p.result.Confidence)
		return annotation{text: txt, bg: colorFailBand, fg: colorFailText}

	case p.reasonCode != "":
		// Reader returned a reason but no text (e.g. no_components).
		txt := fmt.Sprintf("FAIL_%s  NOPX", p.reasonCode)
		return annotation{text: txt, bg: colorFailBand, fg: colorFailText}

	case p.reasonText != "":
		// FindStatusPanel found a candidate but VerifyPanel rejected it.
		txt := fmt.Sprintf("FAIL_VRFY  SCORE=%.4f", p.panelScore)
		return annotation{text: txt, bg: colorLostBand, fg: colorLostText}

	default:
		// FindStatusPanel returned no candidate within maxScore.
		txt := fmt.Sprintf("FAIL_PNL  SCORE=%.4f", p.panelScore)
		return annotation{text: txt, bg: colorLostBand, fg: colorLostText}
	}
}

// stageOutcome captures the per-stage outcome of running the
// FindStatusPanel → VerifyPanel → ExtractStatusLineStrip →
// Reader.Read() pipeline on a single screenshot. The overlay text
// composer reads whichever fields are populated; the rest stay zero.
type stageOutcome struct {
	readOK    bool     // true if Reader.Read returned OK=true
	result    Result   // raw Reader result (zero except on read stage)
	reasonCode string  // populated when Reader had no Text
	reasonText string  // populated when VerifyPanel rejected
	panelScore float64 // score returned by FindStatusPanel
}

// runPipeline walks one screenshot through the full pipeline and
// returns the per-stage outcome. It always returns a non-nil
// stageOutcome even on early-stage failures (FindStatusPanel miss,
// VerifyPanel rejection) so the caller can always produce an
// annotated PNG. Each stage failure is logged via t.Logf so the run
// output makes the failure mode explicit.
func runPipeline(t *testing.T, screenImg image.Image, reader *Reader) stageOutcome {
	t.Helper()
	tpl := statusui.DefaultStatusPanelTemplate()
	if tpl == nil {
		t.Fatal("DefaultStatusPanelTemplate returned nil — StatusPanel.png not embedded")
	}

	panelRect, score, ok := statusui.FindStatusPanel(screenImg, tpl, statusui.FindStatusPanelOptions{})
	if !ok {
		t.Logf("FindStatusPanel miss: score=%.4f", score)
		return stageOutcome{panelScore: score}
	}

	panelImg := statusui.ExtractROI(screenImg, panelRect)
	if err := statusui.VerifyPanel(panelImg); err != nil {
		t.Logf("VerifyPanel reject: %v", err)
		return stageOutcome{panelScore: score, reasonText: err.Error()}
	}

	strip, stripRect := statusui.ExtractStatusLineStrip(screenImg, panelRect)
	if strip == nil {
		t.Logf("ExtractStatusLineStrip returned nil: rect=%v", stripRect)
		return stageOutcome{panelScore: score}
	}
	res := reader.Read(strip)
	// Strip the "<reject>" sentinel that reader.Read appends to
	// res.Text on low_glyph_score to mark the failing component —
	// the overlay banner would otherwise carry that token as ugly
	// noise. Reason is preserved so the overlay's status code + colour
	// stay semantically correct after the trim.
	res.Text = strings.TrimSuffix(res.Text, "<reject>")
	if res.OK {
		t.Logf("Read OK: HP=%d/%d SP=%d/%d conf=%.4f text=%q",
			res.HP, res.HPMax, res.SP, res.SPMax, res.Confidence, res.Text)
		return stageOutcome{readOK: true, result: res, panelScore: score}
	}
	if res.Reason != "" {
		t.Logf("Read FAIL: reason=%q text=%q conf=%.4f", res.Reason, res.Text, res.Confidence)
		return stageOutcome{result: res, panelScore: score, reasonCode: statusCodeFor(res.Reason)}
	}
	return stageOutcome{result: res, panelScore: score}
}

// TestEndToEnd_AnnotateScreenshots loops every PNG in
// runner/autopot/testdata/, runs the recognition pipeline against
// each, renders a high-contrast annotation band on top of the
// original screenshot describing the parsed (or failed) values, and
// writes the result to:
//
//	internal/vision/hpspreader/debug/annotated_screenshots/<name>-annotated.png
//
// The test never t.Fatal()s on individual screenshot outcomes — it
// only Fatal()s when the *test infrastructure* itself fails (missing
// fixture, missing template, NewReader failure). Per-screenshot
// outcomes are reported via t.Logf so a single bad fixture doesn't
// cause the whole test to fail (and so the human reviewing the test
// output sees exactly which fixtures parsed, which were rejected).
//
// The assertion is that annotated PNGs land on disk for every input
// fixture; this guarantees the overlay pipeline is exercised against
// the full real-game drift / zoom / reproduce capture set rather
// than just the happy-path 'aa/gg/ii' round.
// ----------------------------------------------------------------------------

func TestEndToEnd_AnnotateScreenshots(t *testing.T) {
	reader, err := NewReader(glyphsDir(t))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	testdata := screenFixturesDir(t)
	files, err := filepath.Glob(filepath.Join(testdata, "*.png"))
	if err != nil {
		t.Fatalf("glob %s: %v", testdata, err)
	}
	if len(files) == 0 {
		t.Fatalf("no fixtures in %s (testdata wiped?)", testdata)
	}

	outDir := annotatedOutDir(t)

	for _, fpath := range files {
		name := filepath.Base(fpath)
		t.Run(strings.TrimSuffix(name, ".png"), func(t *testing.T) {
			screenImg := loadScreenshot(t, fpath)
			if screenImg == nil {
				return // loadScreenshot called t.Skip on missing file
			}

			// Clone the screen onto an *image.RGBA so the
			// overlay band can call SetRGBA. The original
			// screen may be *image.NRGBA / *image.Paletted
			// etc.; cloning first guarantees a known surface.
			b := screenImg.Bounds()
			work := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
			draw.Draw(work, work.Bounds(), screenImg, b.Min, draw.Src)

			outcome := runPipeline(t, screenImg, reader)
			ann := composeOverlayText(outcome)
			annotateOverlay(work, ann)

			outPath := filepath.Join(outDir, strings.TrimSuffix(name, ".png")+"-annotated.png")
			encodePNGToFile(t, outPath, work)
			t.Logf("wrote %s", outPath)
		})
	}

	// Guardrail: the spirit of this test is that EVERY fixture ends
	// up as an annotated PNG. If fixture-side t.Skip()s happened
	// (missing PNGs) and the loop finished with no outputs, surface
	// that loudly instead of passing vacuously.
	written, err := filepath.Glob(filepath.Join(outDir, "*.png"))
	if err != nil {
		t.Fatalf("glob %s: %v", outDir, err)
	}
	if len(written) == 0 {
		t.Fatalf("no annotated PNGs were written to %s — overlay pipeline produced nothing", outDir)
	}
	if len(written) < len(files) {
		t.Errorf("only %d/%d annotated PNGs (some fixtures skipped due to missing file?)", len(written), len(files))
	}
}

// glyphsDir resolves the templates directory using runtime.Caller(0)
// so the test is independent of the cwd `go test` is invoked from.
//
// Layout from the test file:
//
//	clicker/internal/vision/hpspreader/end_to_end_test.go
//
//	to reach:
//
//	clicker/runner/statusui/glyphs/
func glyphsDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot locate glyphs dir")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "runner", "statusui", "glyphs")
}
