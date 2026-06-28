package runner

import (
	"fmt"
	"testing"
)

// TestUnifiedNormalizationPipeline tests the unified preprocessing pipeline.
func TestUnifiedNormalizationPipeline(t *testing.T) {
	// Test 1: Verify foreground polarity
	t.Run("foreground_polarity", func(t *testing.T) {
		// Create test image: black digit on white background
		// Pattern: a vertical line (should be foreground)
		binary := [][]bool{
			{false, false, false, false, false},
			{false, true, true, false, false},
			{false, true, true, false, false},
			{false, true, true, false, false},
			{false, false, false, false, false},
		}

		ng := PreprocessGlyph(binary)
		if len(ng.Pattern) == 0 {
			t.Fatal("PreprocessGlyph returned empty pattern")
		}

		// Should have non-zero foreground bits
		foregroundCount := 0
		for _, bit := range ng.Pattern {
			if bit == '1' {
				foregroundCount++
			}
		}

		if foregroundCount == 0 {
			t.Error("No foreground pixels found - polarity might be inverted")
		}

		t.Logf("✓ Foreground polarity correct: %d/%d bits are foreground",
			foregroundCount, len(ng.Pattern))
	})

	// Test 2: Self-match test for templates
	t.Run("template_self_match", func(t *testing.T) {
		lib := NewGlyphExemplarLibrary()

		for char, exemplar := range lib.exemplars {
			// Re-process the exemplar through the same pipeline
			// Simulate by creating a binary image from the exemplar pattern
			binaryFromPattern := patternToBinary(exemplar.Pattern, exemplar.Width, exemplar.Height)
			reprocessed := PreprocessGlyph(binaryFromPattern)

			// Should match itself with distance ~0
			distance := GlyphHammingDistance(exemplar, reprocessed)
			errorDistance := 1.0 - distance

			if errorDistance > 0.01 {
				t.Errorf("'%c' self-match distance too high: %.3f", char, errorDistance)
			} else {
				t.Logf("✓ '%c' self-match: distance %.4f", char, errorDistance)
			}
		}
	})

	// Test 3: Trim function works identically
	t.Run("trim_symmetry", func(t *testing.T) {
		// Create binary with borders
		binary := [][]bool{
			{false, false, false, false, false},
			{false, true, true, false, false},
			{false, true, true, false, false},
			{false, false, false, false, false},
		}

		trimmed := trimToForegroundBounds(binary)
		if len(trimmed) != 2 || len(trimmed[0]) != 2 {
			t.Errorf("Trim failed: expected 2x2, got %dx%d",
				len(trimmed[0]), len(trimmed))
		} else {
			t.Log("✓ Trim function correctly removes white borders")
		}
	})

	// Test 4: NormalizeGlyph with different aspect ratios
	t.Run("normalize_aspect_preservation", func(t *testing.T) {
		// Wide glyph: 40x10
		pattern := ""
		for i := 0; i < 400; i++ {
			if i < 100 || (i >= 200 && i < 300) {
				pattern += "1"
			} else {
				pattern += "0"
			}
		}

		ng := NormalizeGlyph(pattern, 40, 10, CanonicalWidth, CanonicalHeight)
		if len(ng.Pattern) != CanonicalBits {
			t.Errorf("Normalization failed: expected %d bits, got %d",
				CanonicalBits, len(ng.Pattern))
		}

		foreground := 0
		for _, bit := range ng.Pattern {
			if bit == '1' {
				foreground++
			}
		}

		if foreground == 0 {
			t.Error("Normalized glyph has no foreground")
		} else {
			t.Logf("✓ Wide glyph normalized: %d foreground bits", foreground)
		}
	})

	// Test 5: ASCII visualization
	t.Run("visualization", func(t *testing.T) {
		// Simple test pattern
		binary := [][]bool{
			{true, true, false},
			{true, false, false},
			{false, false, false},
		}

		ng := PreprocessGlyph(binary)
		vis := VisualizeGlyph(ng)

		if len(vis) == 0 {
			t.Error("Visualization failed")
		} else {
			t.Logf("✓ Visualization works:\n%s", vis)
		}
	})
}

// TestTemplateLoading tests that templates are loaded and normalized correctly.
func TestTemplateLoading(t *testing.T) {
	lib := NewGlyphExemplarLibrary()

	if len(lib.exemplars) == 0 {
		t.Fatal("No templates loaded")
	}

	t.Logf("Loaded %d templates", len(lib.exemplars))

	for char, exemplar := range lib.exemplars {
		// Check canonical size
		if exemplar.Width != CanonicalWidth || exemplar.Height != CanonicalHeight {
			t.Errorf("'%c' wrong size: %dx%d (expected %dx%d)",
				char, exemplar.Width, exemplar.Height, CanonicalWidth, CanonicalHeight)
		}

		// Check pattern length
		if len(exemplar.Pattern) != CanonicalBits {
			t.Errorf("'%c' wrong pattern length: %d (expected %d)",
				char, len(exemplar.Pattern), CanonicalBits)
		}

		// Count foreground
		foreground := 0
		for _, bit := range exemplar.Pattern {
			if bit == '1' {
				foreground++
			}
		}

		if foreground == 0 {
			t.Errorf("'%c' has no foreground pixels", char)
		}

		t.Logf("✓ '%c': %d foreground bits", char, foreground)
	}
}

// TestSelfMatchDebug provides visual debugging for template self-match.
func TestSelfMatchDebug(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping verbose debug test")
	}

	lib := NewGlyphExemplarLibrary()

	// Test first two templates: 0 and 1
	for _, char := range []rune{'0', '1'} {
		exemplar, ok := lib.exemplars[char]
		if !ok {
			t.Logf("Template '%c' not found", char)
			continue
		}

		// Re-process to get "runtime" version
		binaryFromPattern := patternToBinary(exemplar.Pattern, exemplar.Width, exemplar.Height)
		runtime := PreprocessGlyph(binaryFromPattern)

		// Compare
		dist := GlyphHammingDistance(exemplar, runtime)
		errorDistance := 1.0 - dist

		comparison := CompareGlyphsVisualized(
			fmt.Sprintf("template_%c", char),
			exemplar,
			fmt.Sprintf("runtime_%c", char),
			runtime)

		t.Logf("Self-match for '%c' (distance: %.4f):\n%s",
			char, errorDistance, comparison)
	}
}

// TestMatchingAccuracy tests that matching works with real runtime glyphs.
func TestMatchingAccuracy(t *testing.T) {
	lib := NewGlyphExemplarLibrary()

	// Create synthetic runtime glyphs by corrupting templates slightly
	for char := rune('0'); char <= rune('2'); char++ {
		exemplar, ok := lib.exemplars[char]
		if !ok {
			continue
		}

		// Create noisy version (flip some bits)
		noisyPattern := exemplar.Pattern
		// Flip first few bits to simulate noise
		if len(noisyPattern) > 10 {
			bits := []rune(noisyPattern)
			for i := 0; i < 5; i++ {
				if bits[i] == '0' {
					bits[i] = '1'
				} else {
					bits[i] = '0'
				}
			}
			noisyPattern = string(bits)
		}

		noisyGlyph := NormalizedGlyph{
			Width:   exemplar.Width,
			Height:  exemplar.Height,
			Pattern: noisyPattern,
		}

		// Match
		matched, distance, _, conf := lib.MatchGlyph(noisyGlyph)

		if matched != char {
			t.Errorf("'%c' matched to '%c' (distance: %.3f)", char, matched, distance)
		} else {
			t.Logf("✓ '%c' correctly recognized (distance: %.3f, confidence: %.3f)",
				char, distance, conf)
		}
	}
}

// patternToBinary converts a pattern string back to binary for testing.
func patternToBinary(pattern string, width, height int) [][]bool {
	binary := make([][]bool, height)
	for y := 0; y < height; y++ {
		binary[y] = make([]bool, width)
		for x := 0; x < width; x++ {
			idx := y*width + x
			if idx < len(pattern) {
				binary[y][x] = pattern[idx] == '1'
			}
		}
	}
	return binary
}
