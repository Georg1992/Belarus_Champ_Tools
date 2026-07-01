package statusui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type expectedStatus struct {
	hp    int
	hpMax int
	sp    int
	spMax int
}

func statusKnownCases() map[string]expectedStatus {
	return map[string]expectedStatus{
		"aa.png":      {hp: 751, hpMax: 1290, sp: 102, spMax: 201},
		"gg.png":      {hp: 411, hpMax: 1254, sp: 117, spMax: 195},
		"jj.png":      {hp: 120, hpMax: 1290, sp: 6, spMax: 201},
		"pp.png":      {hp: 1045, hpMax: 1290, sp: 66, spMax: 201},
		"tt.png":      {hp: 674, hpMax: 1290, sp: 18, spMax: 201},
		"drift1.png":  {hp: 1290, hpMax: 1290, sp: 201, spMax: 201},
		"drift2.png":  {hp: 1290, hpMax: 1290, sp: 201, spMax: 201},
		"drift3.png":  {hp: 1290, hpMax: 1290, sp: 201, spMax: 201},
		"drift4.png":  {hp: 1290, hpMax: 1290, sp: 201, spMax: 201},
		"drift5.png":  {hp: 639, hpMax: 1290, sp: 33, spMax: 201},
		"drift6.png":  {hp: 651, hpMax: 1290, sp: 57, spMax: 201},
		"Drift7.png":  {hp: 663, hpMax: 1290, sp: 93, spMax: 201},
		"Drift8.png":  {hp: 1290, hpMax: 1290, sp: 201, spMax: 201},
		"zoomed1.png": {hp: 675, hpMax: 1290, sp: 117, spMax: 201},
	}
}

func statusRootDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}

func statusGlyphsDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(statusRootDir(t), "glyphs")
}

func statusFixturesDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(statusRootDir(t), "..", "autopot", "testdata")
}

func loadPNGImage(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return img
}

func writePNGImage(t *testing.T, path string, img image.Image) {
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

func TestPipeline_EndToEnd_FixtureSet(t *testing.T) {
	pipeline, err := NewPipeline(statusGlyphsDir(t), 0.70)
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}

	outDir := t.TempDir()
	fixtures := statusKnownCases()

	for name, want := range fixtures {
		t.Run(name, func(t *testing.T) {
			src := loadPNGImage(t, filepath.Join(statusFixturesDir(t), name))
			got, err := pipeline.RecognizeScreen(src)
			if err != nil {
				t.Fatalf("RecognizeScreen: %v", err)
			}

			if got.PanelImage == nil {
				t.Fatal("panel image is nil")
			}
			if got.StripImage == nil {
				t.Fatal("strip image is nil")
			}
			if got.OverlayImage == nil {
				t.Fatal("overlay image is nil")
			}

			if got.PanelRect.Dx() != 218 || got.PanelRect.Dy() != 58 {
				t.Fatalf("panel rect dimensions %dx%d, want 218x58", got.PanelRect.Dx(), got.PanelRect.Dy())
			}
			if got.StripRect.Dx() != 200 || got.StripRect.Dy() != 11 {
				t.Fatalf("strip rect dimensions %dx%d, want 200x11", got.StripRect.Dx(), got.StripRect.Dy())
			}

			if got.ParseResult.HP != want.hp || got.ParseResult.HPMax != want.hpMax || got.ParseResult.SP != want.sp || got.ParseResult.SPMax != want.spMax {
				t.Fatalf("parsed values HP=%d/%d SP=%d/%d, want HP=%d/%d SP=%d/%d (text=%q conf=%.4f)",
					got.ParseResult.HP, got.ParseResult.HPMax,
					got.ParseResult.SP, got.ParseResult.SPMax,
					want.hp, want.hpMax, want.sp, want.spMax,
					got.ParseResult.Text, got.ParseResult.Confidence,
				)
			}

			base := name[:len(name)-4]
			panelPath := filepath.Join(outDir, fmt.Sprintf("%s_panel.png", base))
			stripPath := filepath.Join(outDir, fmt.Sprintf("%s_strip.png", base))
			overlayPath := filepath.Join(outDir, fmt.Sprintf("%s_overlay.png", base))
			writePNGImage(t, panelPath, got.PanelImage)
			writePNGImage(t, stripPath, got.StripImage)
			writePNGImage(t, overlayPath, got.OverlayImage)
		})
	}
}

func TestPipeline_ParseStrip_FromRecognizedStrip(t *testing.T) {
	pipeline, err := NewPipeline(statusGlyphsDir(t), 0.70)
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}
	src := loadPNGImage(t, filepath.Join(statusFixturesDir(t), "aa.png"))
	full, err := pipeline.RecognizeScreen(src)
	if err != nil {
		t.Fatalf("RecognizeScreen: %v", err)
	}
	fromStrip, err := pipeline.ParseStrip(full.StripImage)
	if err != nil {
		t.Fatalf("ParseStrip: %v", err)
	}
	if fromStrip.ParsedStatus != full.ParseResult.ParsedStatus {
		t.Fatalf("ParseStrip mismatch: strip=%+v full=%+v", fromStrip.ParsedStatus, full.ParseResult.ParsedStatus)
	}
}

func TestPipeline_VisualValidation_AAAndII(t *testing.T) {
	pipeline, err := NewPipeline(statusGlyphsDir(t), 0.70)
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}

	type tc struct {
		name string
		hp   int
		hpMx int
		sp   int
		spMx int
	}
	cases := []tc{
		{name: "aa.png", hp: 751, hpMx: 1290, sp: 102, spMx: 201},
		{name: "ii.png", hp: 1254, hpMx: 1254, sp: 195, spMx: 195},
	}

	outDir := filepath.Join(statusRootDir(t), "visual_validation", "aa_ii")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", outDir, err)
	}
	t.Logf("visual validation outputs: %s", outDir)

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := loadPNGImage(t, filepath.Join(statusFixturesDir(t), c.name))
			got, err := pipeline.RecognizeScreen(src)
			if err != nil {
				t.Fatalf("RecognizeScreen(%s): %v", c.name, err)
			}

			base := c.name[:len(c.name)-4]
			panelPath := filepath.Join(outDir, fmt.Sprintf("%s_panel.png", base))
			stripPath := filepath.Join(outDir, fmt.Sprintf("%s_strip.png", base))
			overlayPath := filepath.Join(outDir, fmt.Sprintf("%s_overlay.png", base))

			if got.PanelImage == nil || got.StripImage == nil || got.OverlayImage == nil {
				t.Fatalf("%s: missing one or more output images panel=%v strip=%v overlay=%v", c.name, got.PanelImage != nil, got.StripImage != nil, got.OverlayImage != nil)
			}
			writePNGImage(t, panelPath, got.PanelImage)
			writePNGImage(t, stripPath, got.StripImage)
			writePNGImage(t, overlayPath, got.OverlayImage)

			if got.ParseResult.HP != c.hp || got.ParseResult.HPMax != c.hpMx || got.ParseResult.SP != c.sp || got.ParseResult.SPMax != c.spMx {
				t.Fatalf("%s: parsed HP=%d/%d SP=%d/%d, want HP=%d/%d SP=%d/%d (text=%q conf=%.4f)",
					c.name,
					got.ParseResult.HP, got.ParseResult.HPMax,
					got.ParseResult.SP, got.ParseResult.SPMax,
					c.hp, c.hpMx, c.sp, c.spMx,
					got.ParseResult.Text, got.ParseResult.Confidence,
				)
			}
		})
	}
}
