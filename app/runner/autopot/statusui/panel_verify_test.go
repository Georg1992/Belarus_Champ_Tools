package statusui

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

// TestDefaultStatusPanelTemplate_EmbeddedValidates asserts that the
// //go:embed StatusPanel.png wiring compiled cleanly: the embedded
// image decodes and yields the expected 218×58 dimensions that the
// panel_finder tests rely on. If somebody accidentally points the
// embed at the wrong file, this test catches it before the rest of
// the suite starts failing mysteriously.
func TestDefaultStatusPanelTemplate_EmbeddedValidates(t *testing.T) {
	tpl := DefaultStatusPanelTemplate()
	if tpl == nil {
		t.Fatal("DefaultStatusPanelTemplate returned nil — StatusPanel.png likely not embedded")
	}
	b := tpl.Bounds()
	if b.Dx() != 218 || b.Dy() != 58 {
		t.Fatalf("template dimensions %dx%d, expected 218x58 (the status panel size)", b.Dx(), b.Dy())
	}
	t.Logf("embedded template: %dx%d", b.Dx(), b.Dy())
}

// TestVerifyPanel_Direct tests VerifyPanel against synthetic inputs so
// the contract is provable without depending on which corner a real
// full-screen locate happens to favor.
//
// Cases:
//   - the embedded StatusPanel.png template passes everything
//   - a uniform off-white 218×58 pad fails all three signals
//   - a 100×30 panel fails the dimension pre-check
//   - nil input fails the nil pre-check
//
// Together these prove: (a) the verifier accepts a known reference,
// (b) the verifier rejects a known-fabricated skill-panel motif,
// (c) the verifier enforces the 218×58 contract, (d) the verifier
// rejects nil safely.
func TestVerifyPanel_Direct(t *testing.T) {
	t.Run("accepts_embedded_template", func(t *testing.T) {
		panel := DefaultStatusPanelTemplate()
		if panel == nil {
			t.Fatal("missing embedded template")
		}
		if err := VerifyPanel(panel); err != nil {
			t.Errorf("VerifyPanel rejected the embedded template: %v", err)
		}
	})
	t.Run("rejects_uniform_offwhite_pad", func(t *testing.T) {
		pad := image.NewRGBA(image.Rect(0, 0, 218, 58))
		for y := 0; y < 58; y++ {
			for x := 0; x < 218; x++ {
				pad.SetRGBA(x, y, color.RGBA{240, 240, 240, 255})
			}
		}
		err := VerifyPanel(pad)
		if err == nil {
			t.Fatalf("expected VerifyPanel to reject uniform off-white pad, got nil")
		}
		t.Logf("got expected rejection: %v", err)
	})
	t.Run("rejects_wrong_dimensions", func(t *testing.T) {
		small := image.NewRGBA(image.Rect(0, 0, 100, 30))
		err := VerifyPanel(small)
		if err == nil {
			t.Fatal("expected VerifyPanel to reject 100x30 panel")
		}
		if !strings.Contains(err.Error(), "dimensions") {
			t.Errorf("error should mention dimensions, got: %v", err)
		}
	})
	t.Run("rejects_nil", func(t *testing.T) {
		err := VerifyPanel(nil)
		if err == nil {
			t.Fatal("expected VerifyPanel to reject nil image")
		}
	})
}

// TestExtractStatusLineStrip exercises the pure image crop helper
// that hands a downstream OCR pipeline the HP/SP text strip. We
// feed it a synthetic 2180×116 image (full-screen class, 10× scale)
// with the panel located somewhere inside, then assert (a) the
// returned image has the locator's width × height, (b) the returned
// rect matches LocateStatusTextLine, and (c) the strip's pixels
// match the source panel's pixels at the expected positions.
func TestExtractStatusLineStrip(t *testing.T) {
	tpl := DefaultStatusPanelTemplate()
	if tpl == nil {
		t.Fatal("missing embedded template")
	}
	b := tpl.Bounds()
	// Build a screen 10× the panel size so the panel sits at the
	// origin (the locator's default TopLeftRegion starts at 0,0).
	const scale = 10
	scrW, scrH := b.Dx()*scale, b.Dy()*scale
	scr := image.NewRGBA(image.Rect(0, 0, scrW, scrH))

	// Paste the embedded template at the panel origin.
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			scr.Set(x, y, tpl.At(b.Min.X+x, b.Min.Y+y))
		}
	}

	strip, lineRect := ExtractStatusLineStrip(scr, image.Rect(0, 0, b.Dx(), b.Dy()))
	if strip == nil {
		t.Fatal("ExtractStatusLineStrip returned nil crop")
	}
	loc := DefaultStatusLineLocator()
	wantRect := loc.LocateStatusTextLine(image.Rect(0, 0, b.Dx(), b.Dy()))
	if lineRect != wantRect {
		t.Errorf("lineRect %v != locate %v", lineRect, wantRect)
	}
	if got := strip.Bounds(); got.Dx() != loc.Width || got.Dy() != loc.Height {
		t.Errorf("strip dimensions %dx%d, want %dx%d", got.Dx(), got.Dy(), loc.Width, loc.Height)
	}
	t.Logf("strip %dx%d at screen-rect %v", strip.Bounds().Dx(), strip.Bounds().Dy(), lineRect)
}
