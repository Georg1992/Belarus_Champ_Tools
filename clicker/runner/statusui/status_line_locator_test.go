package statusui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStatusLineLocator_ReturnsSingleRect verifies the locator's
// core contract: exactly ONE rectangle out, anchored at the panel
// top-left, sized according to OffsetX/OffsetY/Width/Height.
func TestStatusLineLocator_ReturnsSingleRect(t *testing.T) {
	loc := StatusLineLocator{OffsetX: 10, OffsetY: 33, Width: 200, Height: 11}
	panel := image.Rect(0, 0, 218, 58)
	got := loc.LocateStatusTextLine(panel)

	want := image.Rect(10, 33, 210, 44)
	if got != want {
		t.Errorf("LocateStatusTextLine = %v, want %v", got, want)
	}
}

// TestStatusLineLocator_PanelRelativeCoords verifies that the
// locator's output shifts with the detected panel — moving the
// panel by (dx, dy) moves the StatusTextLineRect by exactly
// (dx, dy).
func TestStatusLineLocator_PanelRelativeCoords(t *testing.T) {
	loc := StatusLineLocator{OffsetX: 10, OffsetY: 33, Width: 200, Height: 11}
	r1 := loc.LocateStatusTextLine(image.Rect(0, 0, 218, 58))
	r2 := loc.LocateStatusTextLine(image.Rect(1000, 500, 1218, 558))

	if got, want := r2.Min.X-r1.Min.X, 1000; got != want {
		t.Errorf("rect.Min.X delta = %d, want %d", got, want)
	}
	if got, want := r2.Min.Y-r1.Min.Y, 500; got != want {
		t.Errorf("rect.Min.Y delta = %d, want %d", got, want)
	}
	// Width/Height stay constant under panel translation.
	if r1.Dx() != r2.Dx() || r1.Dy() != r2.Dy() {
		t.Errorf("rect dimensions shifted under panel translation: r1=%v r2=%v", r1, r2)
	}
}

// TestStatusLineLocator_DefaultsTargetSecondLine verifies that
// DefaultStatusLineLocator's values land on the calibrated second
// text line band (panel-local x=10..210, y=46..55) for the
// standard 218×58 panel template. This pins the production
// defaults so any drift must be deliberate.
func TestStatusLineLocator_DefaultsTargetSecondLine(t *testing.T) {
	loc := DefaultStatusLineLocator()
	if loc.OffsetX != 10 {
		t.Errorf("Default OffsetX = %d, want 10 (start of text row)", loc.OffsetX)
	}
	if loc.OffsetY != 33 {
		t.Errorf("Default OffsetY = %d, want 33 (2 px above dense HP/SP band y=[35,42])", loc.OffsetY)
	}
	if loc.Width != 200 {
		t.Errorf("Default Width = %d, want 200 (full text-row span)", loc.Width)
	}
	if loc.Height != 11 {
		t.Errorf("Default Height = %d, want 11 (2 px below dense HP/SP band y=[35,42])", loc.Height)
	}

	rect := loc.LocateStatusTextLine(image.Rect(0, 0, 218, 58))
	if rect != (image.Rect(10, 33, 210, 44)) {
		t.Errorf("default Locator on (0,0,218,58) = %v, want (10,33,210,44)", rect)
	}
}

// TestStatusLineLocator_ExcludesTopAndBottom verifies the rect's
// vertical extent stays clear of the Lv/Exp row above and the
// HP/SP decorative bars below. With Y=46 starting AFTER Lv/Exp
// and Height=9 ending BEFORE the bars (panel y ≤ 55), the second
// line is fully enclosed.
func TestStatusLineLocator_ExcludesTopAndBottom(t *testing.T) {
	loc := DefaultStatusLineLocator()
	panel := image.Rect(0, 0, 218, 58)
	rect := loc.LocateStatusTextLine(panel)

	// Y must be at or below 33 (no Lv/Exp row leakage from above).
	if rect.Min.Y < 33 {
		t.Errorf("rect.Min.Y = %d, want ≥ 33 (Lv/Exp row excluded - upper text ends at y=26)", rect.Min.Y)
	}
	// Y must be at or above 44 (no HP/SP bar leakage from below).
	if rect.Max.Y > 44 {
		t.Errorf("rect.Max.Y = %d, want ≤ 44 (HP/SP bars excluded - bars begin at y=53)", rect.Max.Y)
	}
	// X must be at or below 10 (no chrome/title bar leakage).
	if rect.Min.X < 10 {
		t.Errorf("rect.Min.X = %d, want ≥ 10 (panel chrome excluded)", rect.Min.X)
	}
	// X must be at or above panel width minus the right border.
	if rect.Max.X > 210 {
		t.Errorf("rect.Max.X = %d, want ≤ 210 (panel right border excluded)", rect.Max.X)
	}
}

// TestStatusLineLocator_SingleRectNotSplit verifies the locator
// returns ONE rectangle (not two side-by-side rects for HP and
// SP). A single read with a nonzero subdivided-x would mean the
// locator is splitting HP from SP — which the user explicitly
// forbade.
func TestStatusLineLocator_SingleRectNotSplit(t *testing.T) {
	loc := DefaultStatusLineLocator()
	rect := loc.LocateStatusTextLine(image.Rect(0, 0, 218, 58))

	// One rect, not two — width is positive but rect itself is
	// un-subdivided (no internal "seam" rectangles exposed).
	if rect.Dx() != 200 {
		t.Errorf("rect width = %d, want 200 (full text-row span, NOT split into 2 sub-rects)", rect.Dx())
	}
	if rect.Dy() != 11 {
		t.Errorf("rect height = %d, want 11 (single vertical band wrapping dense HP/SP text at y=[35,42] with 2 px padding each side)", rect.Dy())
	}
}

// TestStatusLineLocator_AcceptanceContractsRemoved: This test
// gathered the user's acceptance bullets into a single block, but
// the surrounding tests already cover its assertions:
//
//   - width = 200 (no HP/SP split) is asserted by
//     TestStatusLineLocator_SingleRectNotSplit.
//   - vertical bounds (Lv/Exp above, HP/SP bars below) are
//     asserted by TestStatusLineLocator_ExcludesTopAndBottom.
//   - actual intersect with foreground text is the more useful
//     contract and is asserted by
//     TestStatusLineLocator_TextIntersectsForeground.
//
// Keeping the standalone AcceptanceContracts would have only
// duplicated those assertions with overlapping error messages.
// Removed for clarity. The four acceptance bullets (panel-local
// rect (10, 33)–(210, 44); no Lv/Exp row; no HP/SP bars; single
// rect) remain covered by the named tests above.

// TestStatusLineLocator_TextIntersectsForeground is the
// fail-loud ground-truth check for the Y offsets. For each
// calibration fixture it:
//   1. locates the panel via FindStatusPanel,
//   2. computes the line rect via the default locator,
//   3. crops the line rect out of the panel,
//   4. runs PreprocessImage on the line crop,
//   5. counts what fraction of the line's columns contain at
//      least one foreground pixel,
//   6. asserts that fraction is >= 30%.
//
// Without this guard, the previous bug (line rect drawn BELOW the
// HP/SP text in the empty/bottom UI area, y=[46,55] in panel-local
// coords) would silently pass all structural assertions because
// the rect was still well-formed, just aimed at the wrong rows.
// The 30% rule was chosen because HP. xxx/xxx | SP. xxx/xxx fills
// 60–90 % of the 200-px width with foreground glyphs on healthy
// captures; an empty-area rect will have well under 5% coverage.
//
// On any regression the test prints the per-fixture ratio and
// fails loudly so the calibration owner can re-tune OffsetY/Height.
func TestStatusLineLocator_TextIntersectsForeground(t *testing.T) {
	fixtures := []string{
		"aa.png",
		"gg.png",
		"status_bar_drift1.png",
		"drift1.png",
	}
	const minForegroundFraction = 0.30

	tpl := DefaultStatusPanelTemplate()
	if tpl == nil {
		t.Fatal("missing embedded template")
	}
	loc := DefaultStatusLineLocator()

	for _, name := range fixtures {
		screen := loadRawFixture(t, name)
		panelRect, _, ok := FindStatusPanel(screen, tpl, FindStatusPanelOptions{})
		if !ok {
			t.Fatalf("%s: FindStatusPanel returned ok=false", name)
		}
		panel := ExtractROI(screen, panelRect)
		if panel == nil {
			t.Fatalf("%s: ExtractROI(%v) returned nil", name, panelRect)
		}
		lineRectScreen := loc.LocateStatusTextLine(panelRect)
		// Translate to panel-local coords so we can crop directly
		// out of the 218x58 panel image.
		lineRectPanelLocal := lineRectScreen.Sub(panelRect.Min)
		lineRectPanelLocal = lineRectPanelLocal.Intersect(panel.Bounds())
		if lineRectPanelLocal.Empty() {
			t.Fatalf("%s: line rect %v (panel-local) is empty", name, lineRectPanelLocal)
		}
		lineCrop := ExtractROI(panel, lineRectPanelLocal)
		if lineCrop == nil {
			t.Fatalf("%s: ExtractROI returned nil on line rect %v", name, lineRectPanelLocal)
		}
		binary := PreprocessImage(lineCrop)

		nCols := len(binary[0])
		nColsWithPixels := 0
		for x := 0; x < nCols; x++ {
			for y := 0; y < len(binary); y++ {
				if binary[y][x] {
					nColsWithPixels++
					break
				}
			}
		}
		ratio := float64(nColsWithPixels) / float64(nCols)
		t.Logf("%s: panel=%v line=%v (panel-local %v) → %d/%d cols with foreground (%.1f%%) want ≥%.0f%%",
			name, panelRect, lineRectScreen, lineRectPanelLocal,
			nColsWithPixels, nCols, ratio*100, minForegroundFraction*100)

		if ratio < minForegroundFraction {
			t.Errorf("%s: StatusTextLineRect %v (panel-local %v) intersects foreground in only %.1f%% of columns (want ≥%.0f%%). Rect is probably aimed at empty area, not the HP/SP text row. Move OffsetY upward and/or shrink Height until the dense text band sits inside the rect.",
				name, lineRectScreen, lineRectPanelLocal, ratio*100, minForegroundFraction*100)
		}
	}
}


// loadRawFixture loads a screen-capture PNG from the autopot
// testdata directory (the same fixture set the recognition tests
// iterate) and returns it as an image.Image. Used by the
// locator-acceptance tests below that need a real screenshot to
// drive FindStatusPanel against.
//
// Thin wrapper over panel_finder_test.go's testdataPath + loadPNG —
// those helpers already anchor via runtime.Caller(0) and handle
// open/decode, so this just routes to them to avoid duplicating
// the path-resolution and PNG-decoding logic in two places.
func loadRawFixture(t *testing.T, name string) image.Image {
	t.Helper()
	return loadPNG(t, testdataPath(t, name))
}

// TestExportStatusLineLocator_DebugImages runs FindStatusPanel on
// each calibration fixture, runs the default StatusLineLocator on
// the detected panel rect, and exports a debug PNG to
// clicker/runner/statusui/debug/status_line_locator/<fixture>_line_locator.png
// showing the full screen with:
//   - green outline: detected StatusPanelRect
//   - red outline:   StatusTextLineRect
//
// Use for visual confirmation of the rect positioning on the four
// calibration fixtures and to spot-check the 2-px vertical padding
// above and below the dense HP/SP text band at panel-local y=[35, 42].
func TestExportStatusLineLocator_DebugImages(t *testing.T) {
	fixtures := []string{
		"aa.png",
		"gg.png",
		"status_bar_drift1.png",
		"drift1.png",
	}
	debugDir := filepath.Join("debug", "status_line_locator")
	if err := os.RemoveAll(debugDir); err != nil {
		t.Fatalf("clean %s: %v", debugDir, err)
	}
	if err := os.MkdirAll(debugDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", debugDir, err)
	}

	tpl := DefaultStatusPanelTemplate()
	if tpl == nil {
		t.Fatal("missing embedded template")
	}

	loc := DefaultStatusLineLocator()
	for _, name := range fixtures {
		screen := loadRawFixture(t, name)
		panelRect, score, ok := FindStatusPanel(screen, tpl, FindStatusPanelOptions{})
		if !ok {
			t.Fatalf("FindStatusPanel on %s: not found (score=%.4f)", name, score)
		}
		lineRect := loc.LocateStatusTextLine(panelRect)
		annotated := loc.RenderDebug(screen, panelRect, lineRect)

		baseName := strings.TrimSuffix(name, ".png")
		out := filepath.Join(debugDir, baseName+"_line_locator.png")
		f, err := os.Create(out)
		if err != nil {
			t.Fatalf("create %s: %v", out, err)
		}
		if err := png.Encode(f, annotated); err != nil {
			f.Close()
			t.Fatalf("encode %s: %v", out, err)
		}
		f.Close()
		t.Logf("wrote %s — panel=%v line=%v score=%.4f", out, panelRect, lineRect, score)
	}

	t.Logf("wrote %d status-line-locator debug PNGs to %s — compare red outline with the dense HP/SP text band",
		len(fixtures), debugDir)
}

// TestExportStripDebugImages generates the HP/SP text-strip crops
// from EVERY screenshot in runner/autopot/testdata so they can be
// visually validated. For each fixture it runs:
//   FindStatusPanel → VerifyPanel → ExtractStatusLineStrip,
// and writes two PNGs to runner/statusui/debug/strip_export/:
//   - <name>_strip.png     — the bare 200×11 HP/SP text row crop;
//                            visually inspectable on its own to
//                            confirm "HP. cur/max | SP. cur/max"
//                            glyphs are clean and complete.
//   - <name>_annotated.png — full screen with the green panel
//                            outline and red strip outline from
//                            StatusLineLocator.RenderDebug, so
//                            you can confirm the strip rect is
//                            well-placed relative to the panel
//                            on every capture.
//
// Fixture list is auto-discovered via filepath.Glob so dropping a
// new PNG into runner/autopot/testdata automatically picks it up
// on the next run — no test update required. Glob returns
// alphabetically sorted names, so the log is deterministic.
//
// Fixtures where FindStatusPanel or VerifyPanel fail are logged
// as SKIP and contribute no PNGs — this is intentional: the SKIP
// log line itself becomes a diagnostic that surfaces which
// captures the locator currently can't handle (e.g. non-status-bar
// captures like the skill-tree or zoomed UI never expected to
// match).
//
// Note: this test does not assert a strict pass/fail rate — it
// always exits cleanly so the runs-with-skipped-fixtures case
// still produces a partial output set the user can validate.
// Add a future regression test (e.g. asserting >=80% fixtures
// produce a strip) once the locate pipeline is stable enough
// for a hard floor.
func TestExportStripDebugImages(t *testing.T) {
	// Auto-discover every PNG in the autopot testdata dir so
	// adding a new screenshot doesn't silently miss export. The
	// glob result is sorted alphabetically (filepath.Glob
	// guarantee), giving deterministic log ordering.
	fixturePaths, err := filepath.Glob(testdataPath(t, "*.png"))
	if err != nil {
		t.Fatalf("glob testdata/*.png: %v", err)
	}
	if len(fixturePaths) == 0 {
		t.Fatal("no PNG fixtures found in runner/autopot/testdata — check testdataPath() anchor")
	}
	fixtures := make([]string, 0, len(fixturePaths))
	for _, p := range fixturePaths {
		fixtures = append(fixtures, filepath.Base(p))
	}
	t.Logf("auto-discovered %d fixtures via filepath.Glob", len(fixtures))

	debugDir := filepath.Join("debug", "strip_export")
	if err := os.RemoveAll(debugDir); err != nil {
		t.Fatalf("clean %s: %v", debugDir, err)
	}
	if err := os.MkdirAll(debugDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", debugDir, err)
	}

	tpl := DefaultStatusPanelTemplate()
	if tpl == nil {
		t.Fatal("missing embedded template")
	}
	loc := DefaultStatusLineLocator()

	var succeeded, skipped int
	for _, name := range fixtures {
		screen := loadRawFixture(t, name)
		panelRect, score, ok := FindStatusPanel(screen, tpl, FindStatusPanelOptions{})
		if !ok {
			t.Logf("SKIP %s: FindStatusPanel failed (score=%.4f)", name, score)
			skipped++
			continue
		}
		panelImg := ExtractROI(screen, panelRect)
		if panelImg == nil {
			t.Logf("SKIP %s: ExtractROI(%v) returned nil", name, panelRect)
			skipped++
			continue
		}
		if err := VerifyPanel(panelImg); err != nil {
			t.Logf("SKIP %s: VerifyPanel: %v", name, err)
			skipped++
			continue
		}
		strip, lineRect := ExtractStatusLineStrip(screen, panelRect)
		if strip == nil {
			t.Logf("SKIP %s: ExtractStatusLineStrip returned nil (lineRect=%v)", name, lineRect)
			skipped++
			continue
		}

		baseName := strings.TrimSuffix(name, ".png")

		// 1) Bare strip crop — the primary validation artefact.
		stripPath := filepath.Join(debugDir, baseName+"_strip.png")
		sf, err := os.Create(stripPath)
		if err != nil {
			t.Fatalf("create %s: %v", stripPath, err)
		}
		if err := png.Encode(sf, strip); err != nil {
			sf.Close()
			t.Fatalf("encode %s: %v", stripPath, err)
		}
		sf.Close()

		// 2) Annotated screen overlay — shows the strip rect
		// positioning on the full capture so you can spot drift.
		annotated := loc.RenderDebug(screen, panelRect, lineRect)
		annotatedPath := filepath.Join(debugDir, baseName+"_annotated.png")
		af, err := os.Create(annotatedPath)
		if err != nil {
			t.Fatalf("create %s: %v", annotatedPath, err)
		}
		if err := png.Encode(af, annotated); err != nil {
			af.Close()
			t.Fatalf("encode %s: %v", annotatedPath, err)
		}
		af.Close()

		t.Logf("OK   %s: panel=%v line=%v score=%.4f → %s + %s",
			name, panelRect, lineRect, score, stripPath, annotatedPath)
		succeeded++
	}

	t.Logf("strip export: %d succeeded, %d skipped → %s (compare <name>_strip.png with the dense HP/SP text band, and check <name>_annotated.png to confirm the red rect sits on the panel second-line row)",
		succeeded, skipped, debugDir)
}
