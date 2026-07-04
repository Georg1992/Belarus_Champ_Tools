// Package autopot is the HP/SP auto-potion runner.
//
// Architecture:
//
//   - BarReader interface — HP/SP detectors produce percentage readings.
//     Two implementations: pixelBarReader (colour-based, always-available)
//     and statusUIReader (OCR-based, higher precision).
//
//   - AutoPotRunner.run() — orchestrator that picks the active reader,
//     reads HP/SP, and calls healUntil when values drop below thresholds.
//
//   - healUntil() — unified heal loop that presses a potion key and
//     spin-reads via the active BarReader until the stat rises above
//     threshold. Replaces two duplicate ~140-line heal functions.
//
// Lifecycle bookkeeping is in internal/lifecycle; timing constants in
// internal/timing; InputSession interface in internal/session.
package autopot

import (
	"context"
	"fmt"
	"time"

	"belarus-champ-tools/runner/autopot/statusui"

	"belarus-champ-tools/runner/internal/lifecycle"
	"belarus-champ-tools/runner/internal/session"
	"belarus-champ-tools/runner/internal/timing"
)

// AutoPotConfig is what gui/main.go passes to NewAutoPot.
type AutoPotConfig struct {
	Session        session.InputSession
	HPThreshold    int
	SPThreshold    int
	HPKeyVK        int32
	SPKeyVK        int32
	HPKeyName      string // human-readable key name for overlay (e.g. "F1")
	SPKeyName      string
	HPEnabled      bool
	SPEnabled      bool
	Log            func(string)
	OnStatusParsed func(hp, hpMax, sp, spMax, stripX, stripY, stripW, stripH int)
	OnStatusUIMode func(mode string)
}

// AutoPotRunner heals HP/SP based on readings from the active BarReader.
type AutoPotRunner struct {
	lc *lifecycle.Lifecycle[AutoPotConfig]

	hpStabilizer *BarStabilizer
	spStabilizer *BarStabilizer
}

// NewAutoPot constructs an AutoPotRunner with the given initial config.
func NewAutoPot(cfg AutoPotConfig) *AutoPotRunner {
	return &AutoPotRunner{
		lc: lifecycle.New(
			cfg,
			func(c AutoPotConfig) error {
				if c.Session == nil {
					return fmt.Errorf("input session is required")
				}
				if c.Log == nil {
					return fmt.Errorf("log callback is required")
				}
				if c.HPEnabled && c.HPKeyVK == 0 {
					return fmt.Errorf("HP potion key is not set")
				}
				if c.SPEnabled && c.SPKeyVK == 0 {
					return fmt.Errorf("SP potion key is not set")
				}
				return nil
			},
			nil, // cleanup is handled by defer resetStabilizers() inside run()
		),
		hpStabilizer: NewBarStabilizer(true, cfg.HPThreshold),
		spStabilizer: NewBarStabilizer(false, cfg.SPThreshold),
	}
}

// Running reports whether the heal loop is currently active.
func (a *AutoPotRunner) Running() bool { return a.lc.Running() }

// UpdateSettings propagates new settings to the stabilisers.
//
// IMPORTANT: Log, OnStatusParsed, OnStatusUIMode, and Session are
// preserved from the existing config. The GUI layer passes bare
// callbacks (a.appendLog, a.onStatusParsed) without Synchronize;
// the initial startup replaces them with Synchronize-wrapped
// versions. We must keep those wrappers so UI calls from the
// autopot goroutine always marshal to the GUI thread.
func (a *AutoPotRunner) UpdateSettings(cfg AutoPotConfig) {
	old := a.settings()
	cfg.Log = old.Log
	cfg.OnStatusParsed = old.OnStatusParsed
	cfg.OnStatusUIMode = old.OnStatusUIMode
	cfg.Session = old.Session
	a.lc.UpdateSettings(cfg)
	a.hpStabilizer.SetThreshold(cfg.HPThreshold)
	a.spStabilizer.SetThreshold(cfg.SPThreshold)
}

// Start launches the healer.
func (a *AutoPotRunner) Start() error {
	if err := a.lc.Start(a.run); err != nil {
		return fmt.Errorf("autopot: %w", err)
	}
	return nil
}

// Stop signals the healer to exit.
func (a *AutoPotRunner) Stop() { a.lc.Stop() }

// Wait blocks until the healer goroutine has exited.
func (a *AutoPotRunner) Wait() { a.lc.Wait() }

func (a *AutoPotRunner) settings() AutoPotConfig { return a.lc.Settings() }

func (a *AutoPotRunner) resetStabilizers() {
	a.hpStabilizer.Reset()
	a.spStabilizer.Reset()
}

// pixelModeSentinel is passed to OnStatusParsed when switching from OCR
// to pixel mode. The -1 values signal "no OCR data available" to the
// overlay, which displays an appropriate fallback indicator.
const (
	pixelModeSentinel      = -1
	ocrProbeInterval       = 2 * time.Second
	statusUIRetryInterval  = 5 * time.Second
)

// run is the main autopot loop.
//
//  1. Try to build the OCR reader (statusUIReader). If it succeeds,
//     start in OCR mode; otherwise start in pixel-bar mode.
//  2. Each tick: read HP/SP from the active reader. If HP or SP drops
//     below its threshold, call healUntil to press the potion key and
//     wait for the value to rise.
//  3. If the OCR reader fails (panel lost, parse error), switch
//     immediately to pixel-bar. Every 5 s, probe the OCR reader and
//     switch back if it recovers.
func (a *AutoPotRunner) run(ctx context.Context, cfg AutoPotConfig) {
	defer a.resetStabilizers()

	pixel := &pixelBarReader{
		hpStab: a.hpStabilizer,
		spStab: a.spStabilizer,
		log:    cfg.Log,
	}

	pipeline, err := statusui.NewDefaultPipeline()
	hasOCR := err == nil
	var ocr *statusUIReader
	if hasOCR {
		ocr = &statusUIReader{
			poller:       statusui.NewStripPoller(pipeline),
			onModeChange: cfg.OnStatusUIMode,
			onParsed:     cfg.OnStatusParsed,
			log:          cfg.Log,
			settings:     a.settings,
		}
	}

	var reader BarReader
	if hasOCR {
		reader = ocr
		setMode(cfg.OnStatusUIMode, "Searching...")
	} else {
		reader = pixel
		if cfg.OnStatusParsed != nil {
			cfg.OnStatusParsed(pixelModeSentinel, 0, pixelModeSentinel, 0, 0, 0, 0, 0)
		}
	}

	nextOCRRetry := time.Time{}
	loggedPixelFail := false
	pixelFailStart := time.Time{}
	dead := false

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		cfg = a.settings()
		if cfg.Session == nil {
			timing.Sleep(ctx, timing.PollInterval)
			continue
		}

		result := reader.ReadValues(ctx)

		if a.handleDead(ctx, cfg, result, &dead) {
			continue
		}

		// Character respawned — restore the normal mode label.
		if dead && result.Status == StatusFound {
			dead = false
			setMode(cfg.OnStatusUIMode, reader.Name())
		}

		// Dispatch to the active reader's handler.
		// OCR path: handle OCR reader failures (switch to pixel).
		// Pixel path: handle OCR recovery probe and pixel failures.
		if reader == ocr {
			if a.handleOCR(cfg, &reader, pixel, result, &nextOCRRetry) {
				continue
			}
		} else {
			if a.handlePixel(ctx, cfg, &reader, pixel, ocr, &result, &nextOCRRetry, &pixelFailStart, &loggedPixelFail) {
				continue
			}
		}

		// Normal processing — result is valid (StatusFound).
		pixelFailStart = time.Time{}
		loggedPixelFail = false

		if cfg.HPEnabled && result.HPLow {
			a.healUntil(ctx, reader, true)
			continue
		}
		if cfg.SPEnabled && result.SPLow {
			a.healUntil(ctx, reader, false)
			continue
		}

		timing.Sleep(ctx, timing.CaptureRetryDelay)
	}
}

// handleDead handles the StatusDead case. Returns true if the iteration was
// consumed (caller should continue the loop). The dead block has already slept.
func (a *AutoPotRunner) handleDead(ctx context.Context, cfg AutoPotConfig, result BarReadResult, dead *bool) bool {
	if result.Status != StatusDead {
		return false
	}
	if !*dead {
		cfg.Log("autopot: character dead (HP=1)")
		*dead = true
	}
	setMode(cfg.OnStatusUIMode, "Dead")
	if cfg.HPEnabled && cfg.HPKeyVK != 0 {
		if err := cfg.Session.TapKey(cfg.HPKeyVK, timing.KeyTapHold); err != nil {
			cfg.Log(fmt.Sprintf("Key VK_0x%02X failed: %v", cfg.HPKeyVK, err))
		}
	}
	timing.Sleep(ctx, potsEndedDelay)
	return true
}

// handleOCR handles the case when the OCR reader is active and the result is
// not StatusDead. If the result is valid (StatusFound), returns false so the
// main loop proceeds to normal processing. If the OCR reader failed, switches
// to pixel-bar mode, sends the sentinel to the overlay, and returns true.
func (a *AutoPotRunner) handleOCR(cfg AutoPotConfig, reader *BarReader, pixel BarReader, result BarReadResult, nextOCRRetry *time.Time) bool {
	if result.Status == StatusFound {
		return false
	}
	// OCR reader failed — switch to pixel-bar fallback.
	cfg.Log(fmt.Sprintf("autopot: statusui issue, switching to pixel-bar: %v", result.Err))
	*reader = pixel
	setMode(cfg.OnStatusUIMode, "Pixel-bar")
	if cfg.OnStatusParsed != nil {
		cfg.OnStatusParsed(pixelModeSentinel, 0, pixelModeSentinel, 0, 0, 0, 0, 0)
	}
	*nextOCRRetry = time.Now().Add(ocrProbeInterval)
	return true
}

// handlePixel handles the case when the pixel reader is active and the result
// is not StatusDead. It performs two tasks:
//
//  1. OCR recovery probe: periodically probes the OCR reader. If OCR recovers
//     (StatusFound or StatusDead), switches back to OCR and updates *result.
//     For StatusDead, returns true so the next iteration handles dead state.
//     For StatusFound, returns false so normal processing uses the probe data.
//
//  2. Pixel failure: if bars are not found, tracks failure duration. For the
//     first 5 seconds polls fast (50ms); after 5s slows to 5s polling. Logs
//     the transition once.
//
// Returns true if the iteration was consumed (caller should continue loop).
func (a *AutoPotRunner) handlePixel(ctx context.Context, cfg AutoPotConfig, reader *BarReader, pixel BarReader, ocr *statusUIReader, result *BarReadResult, nextOCRRetry *time.Time, pixelFailStart *time.Time, loggedPixelFail *bool) bool {
	// OCR recovery probe — runs every ocrProbeInterval.
	if ocr != nil && time.Now().After(*nextOCRRetry) {
		*nextOCRRetry = time.Now().Add(ocrProbeInterval)
		probe := ocr.ReadValues(ctx)
		if probe.Status == StatusFound {
			cfg.Log("autopot: statusui recovered, switching back")
			*reader = ocr
			setMode(cfg.OnStatusUIMode, "OCR")
			*loggedPixelFail = false
			*result = probe
			return false // fall through to normal processing
		}
		if probe.Status == StatusDead {
			cfg.Log("autopot: statusui recovered (dead)")
			*reader = ocr
			*loggedPixelFail = false
			*result = probe
			return true // next iteration: StatusDead check at top
		}
		// Probe didn't find anything — continue with pixel.
	}

	if result.Status == StatusFound {
		return false // normal processing
	}

	// Pixel bars not found.
	if pixelFailStart.IsZero() {
		*pixelFailStart = time.Now()
	}
	if time.Since(*pixelFailStart) < statusUIRetryInterval {
		timing.Sleep(ctx, timing.CaptureRetryDelay)
		return true
	}
	if !*loggedPixelFail {
		cfg.Log(fmt.Sprintf("autopot: pixel bars not found for 5s — retrying every 5s: %v", result.Err))
		*loggedPixelFail = true
	}
	timing.Sleep(ctx, statusUIRetryInterval)
	return true
}

const (
	potsEndedDelay    = 1 * time.Second  // tap interval when pots appear empty
	noChangeTimeout   = 3 * time.Second  // no value change → assume pots ended
	valueChangeTol    = 1.0              // tolerance for value-change detection (%)
)

// healUntil presses the potion key and keeps pressing until the relevant
// stat rises above threshold or ctx is cancelled.
//
// Pots-ended detection: if the stat doesn't change for `noChangeTimeout`
// while we're spamming below threshold, assume the potion stack is empty.
// In this state the tap interval slows to `potsEndedDelay` (1s) and the
// overlay shows "HP pots ended" / "SP pots ended".
//
// Recovery in slow mode is checked immediately after each tap: we read the
// value before the tap, tap the key, then read the value again (before the
// 1s sleep). If the value changed >= 1%, the potion took effect and we
// exit slow mode instantly — no need to wait for the next loop iteration.
func (a *AutoPotRunner) healUntil(ctx context.Context, reader BarReader, hpBar bool) {
	var (
		healStart time.Time
		lastPct   = -1.0
		potsEnded bool
		recovered bool
	)

	for {
		if ctx.Err() != nil {
			return
		}

		cfg := a.settings()
		if cfg.Session == nil {
			timing.Sleep(ctx, timing.PollInterval)
			continue
		}

		vk, ok := healTarget(cfg, hpBar)
		if !ok {
			return
		}

		result := reader.ReadValues(ctx)
		if result.Status != StatusFound {
			a.healExit(cfg, potsEnded)
			// Return to the main loop — the main loop handles mode
			// switching (OCR→pixel when OCR fails). Retrying in place
			// would keep healing stuck forever on a dead OCR reader
			// while the main loop never gets control to switch to
			// pixel fallback.
			return
		}

		pct := result.HP
		threshold := float64(cfg.HPThreshold)
		if !hpBar {
			pct = result.SP
			threshold = float64(cfg.SPThreshold)
		}
		if pct >= threshold {
			a.healExit(cfg, potsEnded)
			return
		}

		// Initialise tracking on first iteration.
		if healStart.IsZero() {
			healStart = time.Now()
			lastPct = pct
		}

		// Manage pots-ended state: detection, label re-apply, recovery.
		elapsed := time.Since(healStart)
		potsEnded, recovered, healStart = a.handlePotsEnded(cfg, hpBar, elapsed, pct, lastPct, potsEnded, healStart)
		if recovered {
			timing.Sleep(ctx, potsEndedDelay)
			continue
		}

		lastPct = pct

		// In slow mode: check immediately after the tap if the potion worked.
		// Read before, tap, read after — if the value changed >= 1%, recovery.
		if potsEnded {
			beforePct := pct
			if err := cfg.Session.TapKey(vk, timing.KeyTapHold); err != nil {
				cfg.Log(fmt.Sprintf("Key VK_0x%02X failed: %v", vk, err))
				return
			}
			afterResult := reader.ReadValues(ctx)
			if afterResult.Status == StatusFound {
				afterPct := afterResult.HP
				if !hpBar {
					afterPct = afterResult.SP
				}
				if absPctDiff(afterPct, beforePct) >= valueChangeTol {
					cfg.Log("autopot: potion took effect, resuming normal speed")
					setMode(cfg.OnStatusUIMode, "")
					potsEnded = false
					healStart = time.Now()
					continue
				}
			}
			// No recovery — sleep the slow interval and loop again.
			timing.Sleep(ctx, potsEndedDelay)
		} else {
			if !a.healTap(ctx, cfg, vk, false) {
				return
			}
		}
	}
}

// handlePotsEnded manages the pots-ended state machine within healUntil.
// It detects when pots are empty (no pct change after noChangeTimeout),
// re-applies the overlay label every iteration while in the state,
// and detects recovery when the value finally changes.
//
// Returns (newPotsEnded, recovered, newHealStart). When recovered=true,
// the caller should skip the TapKey on this iteration (potion already
// working from the previous 1s-apart tap).
func (a *AutoPotRunner) handlePotsEnded(cfg AutoPotConfig, hpBar bool, elapsed time.Duration, pct, lastPct float64, potsEnded bool, healStart time.Time) (bool, bool, time.Time) {
	// Detect transition into pots-ended mode.
	if !potsEnded && elapsed >= noChangeTimeout && absPctDiff(pct, lastPct) < valueChangeTol {
		cfg.Log(fmt.Sprintf("autopot: %s — slowing to 1s taps", potsEndedLabel(cfg, hpBar)))
		potsEnded = true
	}

	if !potsEnded {
		return false, false, healStart
	}

	// Pots-ended mode is active: check for recovery.
	if absPctDiff(pct, lastPct) >= valueChangeTol {
		cfg.Log("autopot: potion took effect, resuming normal speed")
		setMode(cfg.OnStatusUIMode, "")
		return false, true, time.Now()
	}

	// Re-apply label every iteration so OCR validation (panel
	// lost→found→"Searching..."→"OCR") cannot overwrite it.
	setMode(cfg.OnStatusUIMode, potsEndedLabel(cfg, hpBar))
	return true, false, healStart
}

// healTap presses the potion key and sleeps for the appropriate interval
// based on whether pots-ended mode is active. Returns true on success;
// false on TapKey error or nil session (caller should return from healUntil).
func (a *AutoPotRunner) healTap(ctx context.Context, cfg AutoPotConfig, vk int32, potsEnded bool) bool {
	if cfg.Session == nil {
		return false
	}
	if err := cfg.Session.TapKey(vk, timing.KeyTapHold); err != nil {
		cfg.Log(fmt.Sprintf("Key VK_0x%02X failed: %v", vk, err))
		return false
	}
	if potsEnded {
		timing.Sleep(ctx, potsEndedDelay)
	} else {
		timing.Sleep(ctx, timing.PollInterval)
	}
	return true
}

// healExit clears the pots-ended overlay mode when leaving healUntil.
// Called before every return in the heal loop.
func (a *AutoPotRunner) healExit(cfg AutoPotConfig, potsEnded bool) {
	a.clearPotsEndedMode(cfg.OnStatusUIMode, potsEnded)
}

// clearPotsEndedMode clears the "Pots ended" overlay mode when leaving
// healUntil (regardless of exit reason: recovered, reader failed, ctx done).
func (a *AutoPotRunner) clearPotsEndedMode(fn func(string), potsEnded bool) {
	if potsEnded && fn != nil {
		fn("")
	}
}

func potsEndedLabel(cfg AutoPotConfig, hpBar bool) string {
	label := "HP pots ended"
	keyName := cfg.HPKeyName
	if !hpBar {
		label = "SP pots ended"
		keyName = cfg.SPKeyName
	}
	if keyName != "" {
		label += " on " + keyName
	}
	return label
}

func absPctDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

func healTarget(cfg AutoPotConfig, hpBar bool) (vk int32, ok bool) {
	if hpBar {
		if !cfg.HPEnabled || cfg.HPKeyVK == 0 {
			return 0, false
		}
		return cfg.HPKeyVK, true
	}
	if !cfg.SPEnabled || cfg.SPKeyVK == 0 {
		return 0, false
	}
	return cfg.SPKeyVK, true
}

func setMode(fn func(string), mode string) {
	if fn != nil {
		fn(mode)
	}
}
