package statusui

import (
	"image"
	"image/color"
	"image/draw"
)

// StatusLineLocator returns the rectangular region of the second
// text line of a detected StatusPanel. The line carries both HP
// and SP together, e.g. "HP. 100/100 | SP. 50/100", so the locator
// returns a SINGLE rectangle — not per-column rects.
//
// Architecture (per the project's design document):
//
//	StatusPanelLocator (FindStatusPanel) → StatusPanelRect
//	StatusLineLocator (this)            → StatusTextLineRect
//	NumericParser (numeric_parser.go)   → HP/SP values from crop
//
// StatusLineLocator is a PURE LAYOUT component anchored at the
// detected StatusPanelRect. It adds (OffsetX, OffsetY, Width,
// Height) to the panel's top-left. It does NOT OCR, search for
// digits, search for HP/SP labels, or template-match anything.
// NumericParser owns the job of understanding the line content.
type StatusLineLocator struct {
	OffsetX int
	OffsetY int
	Width   int
	Height  int
}

// DefaultStatusLineLocator returns the production-tuned locator.
//
// Ground-truth calibration against the aa/gg/drift1/status_bar_drift1
// fixtures (PreprocessImage threshold gray<150) shows the HP/SP text
// glyphs sit on a dense band at PANEL-LOCAL y=[35, 42] (peak row
// density 44–55 dark px/row on full HP/SP labels, vs. <10 dark px
// on the decorative bars at y=[53, 55] and vs. the upper Lv/Exp
// text band at y=[19, 26]). The 2-px vertical breathing room above
// and below absorbs anti-alias fringe pixels (which fall below the
// gray<150 threshold and would otherwise crop the visible text) and
// keeps clear separation from the Lv/Exp text row above and the
// HP/SP colored bars below.
//
// Default offsets:
//
//	OffsetX = 10   → start of the panel-internal text-row width.
//	OffsetY = 33   → 2 px above the dense text band (y=35 starts
//	                 the actual glyphs; the gap above is a clear
//	                 separator from the Lv/Exp text row ending at
//	                 y=26, with 2 px of breathing room for the top
//	                 anti-alias fringe).
//	Width   = 200  → covers the full panel-internal text span
//	                 (panel-local x ∈ [10, 210]). Includes the
//	                 "HP." label, "|" separator, and "SP." label
//	                 by design — NumericParser handles splitting.
//	Height  = 11   → 2 px below the dense text band (y=42 ends the
//	                 glyphs; the gap from y=45 down to y=52 is a
//	                 clear separator from the HP/SP bar at y=53+,
//	                 with 2 px of breathing room for the bottom
//	                 anti-alias fringe).
//
// Resulting rect: panel-local (10, 33)–(210, 44) — a tight 200×11
// strip surrounding the dense HP/SP text bands with 2 px of
// vertical breathing room on each side and healthy gaps (6–8 px
// and 8–9 px) above/below to the Lv/Exp row and HP/SP colored
// bars respectively. Re-tune via struct-literal config if the
// template's font height shifts.
func DefaultStatusLineLocator() StatusLineLocator {
	return StatusLineLocator{
		OffsetX: 10,
		OffsetY: 33,
		Width:   200,
		Height:  11,
	}
}

// LocateStatusTextLine returns the screen-space rectangle inside
// the given detected status panel that contains the second text
// line. The rect is anchored at the panel's top-left:
//
//	panel.Min.X + OffsetX  →  rect.Min.X
//	panel.Min.Y + OffsetY  →  rect.Min.Y
//	... + Width            →  rect.Max.X
//	... + Height           →  rect.Max.Y
//
// Coordinate space: output rect is in screen-space, anchored at
// the FindStatusPanel result. The returned rect is suitable for
// handing to the NumericParser as a full second-line crop.
func (l StatusLineLocator) LocateStatusTextLine(panel image.Rectangle) image.Rectangle {
	return image.Rect(
		panel.Min.X+l.OffsetX,
		panel.Min.Y+l.OffsetY,
		panel.Min.X+l.OffsetX+l.Width,
		panel.Min.Y+l.OffsetY+l.Height,
	)
}

// RenderDebug composites the given screen image with two outlined
// rectangles so an operator can visually verify the locator:
//
//	green outline = StatusPanelRect (entire detected panel)
//	red   outline = StatusTextLineRect (second text line)
//
// Acceptance check (per the user's contract):
//
//	red rect contains exactly the whole line "HP. xxx/xxx | SP. xxx/xxx"
//	it may include 1–2 px of padding
//	it must NOT include the first Lv/Exp row above
//	it must NOT include the bottom HP/SP bars below
//	it must NOT split HP from SP
//
// Coordinate space: panelRect and lineRect must both be in
// SCREEN-SPACE coordinates (anchored at FindStatusPanel result).
// Caller typically obtains both via FindStatusPanel +
// LocateStatusTextLine.
func (l StatusLineLocator) RenderDebug(screenImg image.Image, panelRect, lineRect image.Rectangle) image.Image {
	dst := image.NewRGBA(screenImg.Bounds())
	draw.Draw(dst, dst.Bounds(), screenImg, image.Point{}, draw.Src)

	drawRectOutline(dst, panelRect, color.RGBA{R: 0, G: 200, B: 0, A: 255})
	drawRectOutline(dst, lineRect, color.RGBA{R: 230, G: 30, B: 30, A: 255})

	return dst
}

// drawRectOutline draws a 1-pixel-thick rectangle outline on dst.
// dst is any draw.Image (which embeds image.Image and therefore
// exposes the per-pixel Set method). Coordinates outside
// dst.Bounds() are silently clamped by Set, so this helper is
// safe for arbitrarily large rects.
func drawRectOutline(dst draw.Image, r image.Rectangle, c color.Color) {
	if r.Empty() {
		return
	}
	for x := r.Min.X; x < r.Max.X; x++ {
		dst.Set(x, r.Min.Y, c)
		dst.Set(x, r.Max.Y-1, c)
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		dst.Set(r.Min.X, y, c)
		dst.Set(r.Max.X-1, y, c)
	}
}
