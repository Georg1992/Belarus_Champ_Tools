// Command ii-panel-check produces visual artifacts to inspect what
// FindStatusPanel + VerifyPanel settle on for a single screenshot.
//
// Usage:
//
//	ii-panel-check <input.png> <out-dir>
//
// Writes to <out-dir>:
//
//	<basename>_panel_pass1.png    The 218x58 crop at the rect FindStatusPanel returned.
//	<basename>_annotated.png      The original screenshot with a yellow 2-px box drawn around that rect.
//	recognized.txt                A short text summary (pass1 stats, verify result).
//
// This is a one-shot diagnostic — not exported, no flags, no test
// integration. The intent is that a human opens the two PNGs in an
// image viewer and compares them side by side with the input.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"

	"experimental-clicker/runner/statusui"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: ii-panel-check <input.png> <out-dir>")
		os.Exit(2)
	}
	inPath := os.Args[1]
	outDir := os.Args[2]
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fail("mkdir %s: %v", outDir, err)
	}
	base := stripExt(filepath.Base(inPath))

	src := mustLoadPNG(inPath)
	tpl := statusui.DefaultStatusPanelTemplate()
	if tpl == nil {
		fail("DefaultStatusPanelTemplate returned nil")
	}

	// Pass 1 — the single FindStatusPanel call we want to inspect.
	rect, score, ok := statusui.FindStatusPanel(src, tpl, statusui.FindStatusPanelOptions{})
	fmt.Printf("FIND         ok=%v  rect=%v  score=%.4f\n", ok, rect, score)

	// Crop the rect. Even on miss we want to see what was at that location.
	panelImg := statusui.ExtractROI(src, rect)
	if panelImg == nil {
		// 0-size crop couldn't even be drawn — write a sentinel image so
		// the operator can tell at a glance that nothing was found.
		panelImg = image.NewRGBA(image.Rect(0, 0, 1, 1))
		fmt.Fprintln(os.Stderr, "WARN: ExtractROI returned nil — no panel crop available")
	}
	savePNG(filepath.Join(outDir, base+"_panel_pass1.png"), panelImg)

	// Verify signal so the operator can see the verifier's verdict
	// without having to rerun the CLI.
	verifyErr := statusui.VerifyPanel(panelImg)
	if verifyErr == nil {
		fmt.Println("VERIFY       OK")
	} else {
		fmt.Printf("VERIFY       FAIL  %v\n", verifyErr)
	}

	// Annotated screenshot: the original image with a 2-px yellow box
	// around the detected rect. On miss the box is at the zero rect
	// (so it doesn't draw anything) — the operator sees the original.
	annotated := image.NewRGBA(src.Bounds())
	draw.Draw(annotated, annotated.Bounds(), src, image.Point{}, draw.Src)
	if ok && !rect.Empty() {
		drawRectOutline(annotated, rect, color.RGBA{R: 255, G: 255, B: 0, A: 255}, 2)
	}
	savePNG(filepath.Join(outDir, base+"_annotated.png"), annotated)

	// Brief text dump so the operator has something grep-able alongside
	// the PNGs.
	summary := fmt.Sprintf("input=%s\nrect=%v\nscore=%.4f\nok=%v\nverify=%v\n",
		inPath, rect, score, ok, verifyErr)
	if err := os.WriteFile(filepath.Join(outDir, "recognized.txt"), []byte(summary), 0o644); err != nil {
		fail("write recognized.txt: %v", err)
	}
	fmt.Printf("OUTPUT       %s\n", outDir)
}

// drawRectOutline strokes a thickness-px border around r on img. Edges
// are inclusive so a 2-px border at r is actually 2 pixels wide
// regardless of whether r's bounds are inside a larger RGBA buffer.
func drawRectOutline(img *image.RGBA, r image.Rectangle, c color.RGBA, thickness int) {
	if thickness < 1 {
		thickness = 1
	}
	for t := 0; t < thickness; t++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.Set(x, r.Min.Y+t, c)
			img.Set(x, r.Max.Y-1-t, c)
		}
		for y := r.Min.Y; y < r.Max.Y; y++ {
			img.Set(r.Min.X+t, y, c)
			img.Set(r.Max.X-1-t, y, c)
		}
	}
}

func mustLoadPNG(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		fail("open %s: %v", path, err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		fail("decode %s: %v", path, err)
	}
	return img
}

func savePNG(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		fail("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		fail("encode %s: %v", path, err)
	}
}

func stripExt(s string) string {
	if ext := filepath.Ext(s); ext != "" {
		return s[:len(s)-len(ext)]
	}
	return s
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
	os.Exit(1)
}
