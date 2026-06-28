package runner

import "image"

// SegmentGlyphs finds connected components in a binary image and converts them to glyph bounding boxes.
// This is the ONLY module responsible for glyph segmentation.
func SegmentGlyphs(binary [][]bool, roi image.Rectangle, mergeThreshold int) []image.Rectangle {
	// Step 1: Find connected components
	components := FindConnectedComponents(binary, roi)

	// Step 2: Convert to glyph bounding boxes with merging
	return BoundingBoxesToGlyphs(components, mergeThreshold)
}

// ExtractBinaryROI extracts a binary region from a larger binary image.
// Used by glyph recognition to isolate individual glyphs.
func ExtractBinaryROI(binary [][]bool, roi image.Rectangle) [][]bool {
	height := len(binary)
	width := 0
	if height > 0 {
		width = len(binary[0])
	}

	// Clip ROI to image bounds
	minX := roi.Min.X
	minY := roi.Min.Y
	maxX := roi.Max.X
	maxY := roi.Max.Y

	if minX < 0 {
		minX = 0
	}
	if minY < 0 {
		minY = 0
	}
	if maxX > width {
		maxX = width
	}
	if maxY > height {
		maxY = height
	}

	if minX >= maxX || minY >= maxY {
		return nil
	}

	// Extract region
	extracted := make([][]bool, maxY-minY)
	for y := minY; y < maxY; y++ {
		row := make([]bool, maxX-minX)
		for x := minX; x < maxX; x++ {
			if y < len(binary) && x < len(binary[y]) {
				row[x-minX] = binary[y][x]
			}
		}
		extracted[y-minY] = row
	}

	return extracted
}
