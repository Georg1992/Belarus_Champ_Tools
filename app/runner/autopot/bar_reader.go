package autopot

import (
	"context"
	"fmt"
	"image"
	"time"

	win "belarus-champ-tools/runner/platform/windows"
	"belarus-champ-tools/runner/autopot/statusui"
)

// BarReadStatus distinguishes the semantic state of a BarReadResult.
type BarReadStatus int

const (
	StatusFound    BarReadStatus = iota // valid HP/SP data
	StatusNotFound                      // bars/panel not found on screen
	StatusInvalid                       // transient error (capture fail, etc.)
	StatusDead                          // character is dead (HP=1)
)

// BarReadResult is the unified HP/SP reading produced by any BarReader.
// HP and SP are 0-100 percentages. HPLow/SPLow are true when the relevant
// bar is below its threshold (for the pixel-bar reader this requires
// PotConfirmReads=3 consecutive low reads via the stabiliser; for the
// statusUI reader a single low parse suffices). Status discriminates the
// semantic state (found, not found, invalid, dead). Err carries the
// underlying error for logging when Status != StatusFound.
type BarReadResult struct {
	HP     float64
	SP     float64
	HPLow bool
	SPLow bool
	Status BarReadStatus
	Err   error
}

// BarReader produces HP/SP percentage readings. Two implementations exist:
//   - pixelBarReader — colour-based bar detection (always-available fallback)
//   - statusUIReader — OCR-based status panel reading (primary, higher precision)
//
// ReadValues blocks until a reading is available or ctx is cancelled.
// Name returns a short identifier for the overlay mode label.
type BarReader interface {
	ReadValues(ctx context.Context) BarReadResult
	Name() string
}

// pixelBarReader wraps the bar stabilisers and screen capture for
// pixel-based HP/SP reading. Tracks the last known bar position in
// screen coordinates so the search ROI can follow camera drift.
type pixelBarReader struct {
	hpStab  *BarStabilizer
	spStab  *BarStabilizer
	log     func(string)
	lastLog time.Time

	// lastScreenRect is the last known HP bar position in screen
	// coordinates. Used to centre the next search ROI so the detector
	// follows camera drift instead of always searching screen centre.
	lastScreenRect Rect
	// lostFrames counts consecutive frames where bars were not found.
	// After 3 lost frames, lastScreenRect is cleared so the search
	// falls back to screen centre (avoids getting stuck on stale pos).
	lostFrames int
}

func (r *pixelBarReader) Name() string { return "Pixel" }

func (r *pixelBarReader) ReadValues(ctx context.Context) BarReadResult {
	if ctx.Err() != nil {
		return BarReadResult{Status: StatusInvalid, Err: ctx.Err()}
	}

	sw, sh := win.ScreenSize()
	var rct Rect
	if r.lastScreenRect.W > 0 && r.lostFrames < 3 {
		// Centre the search ROI on the last known bar position so
		// the detector follows camera drift. Keep a generous margin
		// (mapROIHalfW × 2) so sudden movements are still captured.
		// Clamp to screen bounds to prevent negative coordinates.
		cx := r.lastScreenRect.X + r.lastScreenRect.W/2
		cy := r.lastScreenRect.Y + r.lastScreenRect.H/2
		rct = clampROI(image.Rect(0, 0, sw, sh), Rect{
			X: cx - mapROIHalfW,
			Y: cy - mapROIHalfH,
			W: mapROIHalfW * 2,
			H: mapROIHalfH * 2,
		})
	} else {
		rct = PlayerBarSearchROI(sw, sh)
	}

	roi := win.ScreenROI{X: rct.X, Y: rct.Y, W: rct.W, H: rct.H}
	img, err := win.CaptureScreenRegion(roi)
	if err != nil {
		r.debugf("pixel: capture failed, roi %d,%d %dx%d: %v", roi.X, roi.Y, roi.W, roi.H, err)
		return BarReadResult{Status: StatusInvalid, Err: err}
	}
	bounds := img.Bounds()
	mapped, pairOK := RefreshConsistentBarPair(img)
	if !pairOK {
		// Bars not found — increment lostFrames. Keep lastScreenRect
		// so the next iteration still searches near the last known
		// position (the camera may drift back). After 3 consecutive
		// lost frames, clear lastScreenRect to fall back to screen
		// centre (avoids getting stuck on a stale position).
		r.lostFrames++
		if r.lostFrames >= 3 {
			r.lastScreenRect = Rect{}
		}
		r.debugf("pixel: bars not found img=%dx%d roi %d,%d %dx%d", bounds.Dx(), bounds.Dy(), roi.X, roi.Y, roi.W, roi.H)
		return BarReadResult{Status: StatusNotFound, Err: fmt.Errorf("pixel bars not found (ROI %d,%d %dx%d)", roi.X, roi.Y, roi.W, roi.H)}
	}

	// Convert mapped bar position from image coords to screen coords
	// and store it so the next search follows camera drift.
	r.lastScreenRect = Rect{
		X: mapped.HP.X + roi.X,
		Y: mapped.HP.Y + roi.Y,
		W: mapped.HP.W,
		H: mapped.HP.H,
	}
	r.lostFrames = 0

	hp := r.hpStab.UpdatePair(img, true, mapped, pairOK)
	sp := r.spStab.UpdatePair(img, false, mapped, pairOK)
	r.debugf("pixel: HP=%.0f%% rect(%d,%d %dx%d) status=%d SP=%.0f%% rect(%d,%d %dx%d) status=%d mapped block(%d,%d %dx%d) score=%d img=%dx%d roi %d,%d %dx%d",
		hp.Percent, mapped.HP.X, mapped.HP.Y, mapped.HP.W, mapped.HP.H, hp.Status,
		sp.Percent, mapped.SP.X, mapped.SP.Y, mapped.SP.W, mapped.SP.H, sp.Status,
		mapped.Block.X, mapped.Block.Y, mapped.Block.W, mapped.Block.H, mapped.MapScore,
		bounds.Dx(), bounds.Dy(), roi.X, roi.Y, roi.W, roi.H)
	return BarReadResult{
		HP:     hp.Percent,
		SP:     sp.Percent,
		HPLow:  hp.Status == BarStatusLow,
		SPLow:  sp.Status == BarStatusLow,
		Status: StatusFound,
	}
}

// debugf logs at most once per 2 seconds to avoid GUI log spam.
func (r *pixelBarReader) debugf(format string, args ...interface{}) {
	if r.log == nil {
		return
	}
	now := time.Now()
	if now.Sub(r.lastLog) < 2*time.Second {
		return
	}
	r.lastLog = now
	r.log(fmt.Sprintf(format, args...))
}

// statusUIReader wraps the StripPoller for OCR-based HP/SP reading.
// It handles panel validation, debounced logging, overlay mode transitions,
// and the OnStatusParsed overlay callback — all as side-effects of ReadValues.
// The settings function provides access to live thresholds (which can change
// via UpdateSettings mid-run) so HPLow/SPLow are computed correctly.
type statusUIReader struct {
	poller        *statusui.StripPoller
	wasPanelFound bool
	onModeChange  func(string)
	onParsed      func(hp, hpMax, sp, spMax, stripX, stripY, stripW, stripH int)
	log           func(string)
	settings      func() AutoPotConfig
}

func (r *statusUIReader) Name() string { return "OCR" }

func (r *statusUIReader) ReadValues(ctx context.Context) BarReadResult {
	if ctx.Err() != nil {
		return BarReadResult{Status: StatusInvalid, Err: ctx.Err()}
	}
	if r.poller.NeedsValidation() {
		if err := r.validate(); err != nil {
			return BarReadResult{Status: StatusNotFound, Err: err}
		}
	}
	status, err := r.captureAndParse()
	if err != nil {
		// Parse failed — trigger ONE instant panel re-search before
		// giving up. Invalidate forces NeedsValidation() on the next
		// attempt, and we validate immediately so the orchestrator
		// doesn't have to switch to pixel on a single transient error.
		r.poller.Invalidate()
		if valErr := r.validate(); valErr != nil {
			return BarReadResult{Status: StatusNotFound, Err: valErr}
		}
		status, err = r.captureAndParse()
		if err != nil {
			return BarReadResult{Status: StatusInvalid, Err: err}
		}
	}
	// Always notify the overlay first so it shows HP=1 when dead.
	r.notifyParsed(status)

	// HP==1 means the character is dead in the game engine. Don't
	// heal — return an error so the main loop retries or switches to
	// pixel. When the character respawns (HP > 1), parsing succeeds
	// and healing resumes.
	if status.HP == 1 {
		return BarReadResult{Status: StatusDead, Err: fmt.Errorf("character dead (HP=1)")}
	}

	hpPct := 0.0
	spPct := 0.0
	if status.HPMax > 0 {
		hpPct = float64(status.HP) * 100 / float64(status.HPMax)
	}
	if status.SPMax > 0 {
		spPct = float64(status.SP) * 100 / float64(status.SPMax)
	}

	cfg := r.settings()
	return BarReadResult{
		HP:     hpPct,
		SP:     spPct,
		HPLow:  hpPct < float64(cfg.HPThreshold),
		SPLow:  spPct < float64(cfg.SPThreshold),
		Status: StatusFound,
	}
}

// validate captures a full screenshot and runs panel detection.
// Logs failures only on state transitions (panel lost / found) to
// avoid GUI spam on repeated retries. Screen capture failures
// are logged once then suppressed until a successful capture.
func (r *statusUIReader) validate() error {
	screen, err := win.CaptureFullScreen()
	if err != nil {
		if r.wasPanelFound && r.log != nil {
			r.log(fmt.Sprintf("autopot statusui: screen capture failed: %v", err))
		}
		return err
	}
	if err := r.poller.Validate(screen); err != nil {
		if r.wasPanelFound {
			if r.log != nil {
				r.log("autopot statusui: status panel lost, searching...")
			}
			r.wasPanelFound = false
			if r.onModeChange != nil {
				r.onModeChange("Searching...")
			}
		}
		return err
	}
	if !r.wasPanelFound {
		if r.log != nil {
			r.log("autopot statusui: status panel found")
		}
		r.wasPanelFound = true
		if r.onModeChange != nil {
			r.onModeChange("OCR")
		}
	}
	return nil
}

// captureAndParse captures the cached strip region and parses HP/SP values.
func (r *statusUIReader) captureAndParse() (statusui.ParsedStatus, error) {
	strip := r.poller.StripRect()
	if strip.Empty() {
		return statusui.ParsedStatus{}, fmt.Errorf("strip rect not yet validated")
	}
	img, err := win.CaptureScreenRegion(win.ScreenROI{
		X: strip.Min.X, Y: strip.Min.Y,
		W: strip.Dx(), H: strip.Dy(),
	})
	if err != nil {
		return statusui.ParsedStatus{}, err
	}
	return r.poller.Parse(img)
}

func (r *statusUIReader) notifyParsed(s statusui.ParsedStatus) {
	if r.onParsed == nil {
		return
	}
	strip := r.poller.StripRect()
	r.onParsed(s.HP, s.HPMax, s.SP, s.SPMax, strip.Min.X, strip.Min.Y, strip.Dx(), strip.Dy())
}
