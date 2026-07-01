package statusui

import (
	"image"
	"sync"
)

// panelDriftSlackPx is how much we expand the cached panel rect before
// seeding the next FindStatusPanel's TopLeftRegion search area.
const panelDriftSlackPx = 40

// PanelRecognizer is the LAYOUT-only layer of status recognition. It
// locates the status panel rectangle on a screen capture using
// FindStatusPanel and caches the result so subsequent calls can skip
// the expensive full-screen template scan.
//
// This file is intentionally OCR-free. The recognition pipeline is:
//
//	FindStatusPanel (gray SAD template match)  → image.Rectangle
//	VerifyPanel (3-signal post-filter)         → ok / not-a-panel
//	ExtractStatusLineStrip (in panel_verify.go)→ image.Image
//
// The downstream consumer (a sibling app, a future neural network,
// a human eye, anything) handles OCR on the strip image. This package
// does NOT parse digits, match exemplars, or produce HP/SP values.
type PanelRecognizer struct {
	mu        sync.Mutex
	lastRect  image.Rectangle
	lastFound bool
}

// NewPanelRecognizer returns a recognizer with an empty cache.
func NewPanelRecognizer() *PanelRecognizer { return &PanelRecognizer{} }

// LastRect returns the most recently located panel rect. Empty until
// the first successful FindStatusPanel call.
func (p *PanelRecognizer) LastRect() image.Rectangle {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastRect
}

// Reset clears the cache so the next call falls back to the full
// default TopLeftRegion search.
func (p *PanelRecognizer) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastRect = image.Rectangle{}
	p.lastFound = false
}

// FindStatusPanel locates the status panel inside screenImg using
// the cached lastRect (expanded by panelDriftSlackPx to tolerate
// small inter-frame drift) as the TopLeftRegion. On hit, the cache
// is updated; on miss, it is cleared so the next call falls back to
// the default TopLeftRegion.
//
// Returns (rect, score, ok): ok=true iff a panel-shaped region was
// found at a score below maxScore (default 0.15). The returned rect
// matches the template's dimensions.
func (p *PanelRecognizer) FindStatusPanel(screenImg, template image.Image) (image.Rectangle, float64, bool) {
	if screenImg == nil || template == nil {
		return image.Rectangle{}, 0, false
	}
	p.mu.Lock()
	region := expandRectForDrift(p.lastRect, panelDriftSlackPx)
	p.mu.Unlock()

	rect, score, ok := FindStatusPanel(screenImg, template, FindStatusPanelOptions{
		TopLeftRegion: region,
	})

	p.mu.Lock()
	if ok {
		p.lastRect = rect
		p.lastFound = true
	} else {
		p.lastFound = false
		p.lastRect = image.Rectangle{}
	}
	p.mu.Unlock()
	return rect, score, ok
}

// expandRectForDrift returns r padded by pad pixels on each side,
// clamped to non-negative coordinates. An empty rect passes through
// unchanged so the caller can detect "no prior cache" naturally.
func expandRectForDrift(r image.Rectangle, pad int) image.Rectangle {
	if r.Empty() {
		return r
	}
	minX := r.Min.X - pad
	if minX < 0 {
		minX = 0
	}
	minY := r.Min.Y - pad
	if minY < 0 {
		minY = 0
	}
	maxX := r.Max.X + pad
	maxY := r.Max.Y + pad
	return image.Rect(minX, minY, maxX, maxY)
}
