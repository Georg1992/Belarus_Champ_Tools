// Command hpsp-read runs the hpspreader reader end-to-end on a
// Ragnarok Online screenshot, OR on a pre-cropped status strip.
// One CLI; two input kinds; auto-detected by default.
//
// Modes:
//
//	hpsp-read --image screenshot.png
//	    statusui.FindStatusPanel -> VerifyPanel -> ExtractStatusLineStrip
//	    -> Reader.Read. mode=screenshot.
//
//	hpsp-read --no-auto-find --image strip.png
//	    Skips FindStatusPanel and feeds the input directly to
//	    Reader.Read. mode=strip.
//
//	hpsp-read --image strip.png   (without --no-auto-find)
//	    FindStatusPanel rejects (panel template 218x58 won't fit
//	    a tight 200x11 strip) and the CLI exits with
//	    FAIL reason=panel_not_found. Use --no-auto-find for
//	    pre-cropped strips.
//
// Flags:
//
//	--image        path to PNG (REQUIRED)
//	--templates    directory containing templates/glyphs/*.png
//	               (default "clicker/runner/statusui/glyphs")
//	--debug        optional directory for diagnostic PNGs + recognized.txt
//	--min-glyph-score   minimum per-glyph match score (default 0.70)
//	--no-auto-find      skip FindStatusPanel; treat --image as a strip
//
// Output: single stdout line per invocation, plain text. Shell-grep
// and CI-assert-friendly. The full Reason taxonomy:
//
//	panel_not_found           FindStatusPanel returned ok=false (no panel match).
//	panel_verify_failed:...   VerifyPanel rejected the cropped panel;
//	                          trailing ":..." carries VerifyPanel's diagnostic.
//	strip_extraction_failed   ExtractStatusLineStrip returned nil.
//	no_components             Reader.Read on a strip-sized image produced
//	                          no measurable foreground.
//	low_glyph_score           Per-glyph silhouette score below min-glyph-score.
//	parse_failed              Assembled text didn't match the HP/SP regex.
//	value_validation_failed   Regex matched but hp/hpMax or sp/spMax invalid.
//
// Two example output shapes:
//
//	OK HP=1045/1290 SP=66/201 text="..." conf=0.96 mode=screenshot panel_score=0.05
//	FAIL reason="panel_not_found" conf=0.0000 mode=screenshot panel_score=0.18
//	FAIL reason="low_glyph_score" conf=0.86 mode=screenshot panel_score=0.05 text="..."
//
// Exit code 0 iff the read succeeded (Result.OK true); 1 on any
// pipeline failure or reader failure; 2 on usage error.
//
// This CLI is intentionally thin: it owns no parsing beyond what
// the Reader and statusui package supply. Higher-level consumers
// should import those packages directly.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"experimental-clicker/internal/vision/hpspreader"
	"experimental-clicker/runner/statusui"
)

func main() {
	var (
		imagePath = flag.String("image", "",
			"path to a screenshot OR a pre-cropped strip PNG (required)")
		templates = flag.String("templates",
			filepath.Join("runner", "statusui", "glyphs"),
			"directory containing templates/glyphs/*.png (default assumes cwd=clicker/)")
		debugDir = flag.String("debug", "",
			"optional directory to write diagnostic PNGs + recognized.txt (skipped when absent)")
		minScore = flag.Float64("min-glyph-score", 0.70,
			"minimum per-glyph match score (0..1); lower for noisier captures")
		noAutoFind = flag.Bool("no-auto-find", false,
			"skip FindStatusPanel; treat --image as a pre-cropped strip (mode=strip)")
	)
	flag.Parse()

	if *imagePath == "" {
		fmt.Fprintln(os.Stderr, "hpsp-read: --image is required")
		flag.Usage()
		os.Exit(2)
	}

	img, err := openPNG(*imagePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hpsp-read: open %s: %v\n", *imagePath, err)
		os.Exit(1)
	}

	reader, err := hpspreader.NewReader(*templates)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hpsp-read: load templates from %s: %v\n", *templates, err)
		os.Exit(1)
	}
	reader.MinGlyphScore = *minScore
	if *debugDir != "" {
		reader.Debug = true
		reader.DebugDir = *debugDir
	}

	strip, mode, panelScore, failReason := resolveStrip(img, *noAutoFind)
	if failReason != "" {
		// Upstream pipeline failure: do NOT invoke the Reader,
		// because feeding a full screenshot to it would produce a
		// meaningless FAIL reason="low_glyph_score" and obscure
		// the real failure mode (panel missing / panel shifted /
		// strip extraction bug). Surface the upstream reason on
		// stdout as a structured line so CI can grep it cleanly.
		printResult(hpspreader.Result{OK: false, Reason: failReason}, mode, panelScore)
		os.Exit(1)
	}

	res := reader.Read(strip)
	printResult(res, mode, panelScore)
	if !res.OK {
		os.Exit(1)
	}
}

// resolveStrip auto-detects whether img is a Ragnarok screenshot
// containing the status panel, OR a pre-cropped strip.
//
// Returns:
//
//	strip        image the Reader will be called on (nil on upstream failure).
//	mode         "screenshot" if FindStatusPanel matched; "strip" otherwise.
//	panelScore   FindStatusPanel's best score (0 in strip mode).
//	failReason   "" on success; one of "panel_not_found",
//	             "panel_verify_failed[:<detail>]", "strip_extraction_failed"
//	             for distinct upstream failures.
//
// --no-auto-find            -> (img, "strip", 0, "").
// tpl == nil                -> (img, "strip", 0, "") for build-without-template cases.
// FindStatusPanel miss      -> (nil, "screenshot", score, "panel_not_found").
// VerifyPanel reject        -> (nil, "screenshot", score, "panel_verify_failed: <err>").
// ExtractStatusLineStrip=?nil-> (nil, "screenshot", score, "strip_extraction_failed").
// All OK                    -> (extracted, "screenshot", score, "").
//
// Hard-fail semantics: when failReason != "", Reader.Read is NOT
// called. This avoids a known regression where feeding the full
// screenshot to the Reader under auto-detect failure produced a
// misleading FAIL reason="low_glyph_score" indistinguishable from
// a true OCR failure.
func resolveStrip(img image.Image, noAutoFind bool) (strip image.Image, mode string, panelScore float64, failReason string) {
	if noAutoFind {
		return img, "strip", 0, ""
	}

	tpl := statusui.DefaultStatusPanelTemplate()
	if tpl == nil {
		// Embedded StatusPanel.png missing or unreadable; can't
		// auto-detect. Surface loudly because silent fallback
		// hides an asset build regression — users in a broken
		// build would otherwise see "panel_not_found" forever
		// with no obvious explanation. After warning, treat as
		// strip mode so users with pre-cropped strips can still
		// proceed (with --no-auto-find or this fallback).
		fmt.Fprintln(os.Stderr,
			"hpsp-read: WARNING StatusPanel template unavailable; auto-detection disabled. Pass --no-auto-find to read raw strips.")
		return img, "strip", 0, ""
	}

	panelRect, score, ok := statusui.FindStatusPanel(img, tpl, statusui.FindStatusPanelOptions{})
	if !ok {
		return nil, "screenshot", score, "panel_not_found"
	}
	panelImg := statusui.ExtractROI(img, panelRect)
	if err := statusui.VerifyPanel(panelImg); err != nil {
		// VerifyPanel's err message names which aspect of the
		// panel template didn't match (likely "panel shifted" or
		// "panel antialiasing shifted"). Surface it.
		return nil, "screenshot", score, "panel_verify_failed: " + err.Error()
	}
	extracted, _ := statusui.ExtractStatusLineStrip(img, panelRect)
	if extracted == nil {
		return nil, "screenshot", score, "strip_extraction_failed"
	}
	return extracted, "screenshot", score, ""
}

func openPNG(path string) (image.Image, error) {
	// png.Decode returns image.Image directly. We open
	// ourselves rather than calling image.RegisterFormat so the
	// error path is explicit and the deferred Close is local.
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}

// printResult emits a single OK / FAIL line to stdout. The line is
// a stable, parseable contract — shell scripts / CI asserts may
// rely on its field set. Always write Reason as a quoted string
// (printf %q) so values that contain colons or spaces (e.g.
// "panel_verify_failed: panel shifted") survive grep intact.
func printResult(r hpspreader.Result, mode string, panelScore float64) {
	if r.OK {
		switch mode {
		case "screenshot":
			fmt.Printf("OK HP=%d/%d SP=%d/%d text=%q conf=%.4f mode=%s panel_score=%.4f\n",
				r.HP, r.HPMax, r.SP, r.SPMax, r.Text, r.Confidence, mode, panelScore)
		default:
			fmt.Printf("OK HP=%d/%d SP=%d/%d text=%q conf=%.4f mode=%s\n",
				r.HP, r.HPMax, r.SP, r.SPMax, r.Text, r.Confidence, mode)
		}
		return
	}
	parts := fmt.Sprintf("FAIL reason=%q", r.Reason)
	if r.Confidence != 0 {
		parts += fmt.Sprintf(" conf=%.4f", r.Confidence)
	}
	if r.Text != "" {
		parts += fmt.Sprintf(" text=%q", r.Text)
	}
	parts += fmt.Sprintf(" mode=%s", mode)
	if mode == "screenshot" {
		parts += fmt.Sprintf(" panel_score=%.4f", panelScore)
	}
	fmt.Println(parts)
}
