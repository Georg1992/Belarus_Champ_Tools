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
		// Mode label hidden — the sentinel text already shows the error.
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

		// StatusDead: character is dead — show "[Dead]" in overlay,
		// try the HP potion every 1s like pots-ended (in case a
		// revival item is bound). Don't switch to pixel-bar.
		// Re-apply the mode every iteration so OCR validation
		// ("Searching..." / "OCR") cannot overwrite it.
		if result.Status == StatusDead {
			if !dead {
				cfg.Log("autopot: character dead (HP=1)")
				dead = true
			}
			setMode(cfg.OnStatusUIMode, "Dead")
			if cfg.HPEnabled && cfg.HPKeyVK != 0 {
				if err := cfg.Session.TapKey(cfg.HPKeyVK, timing.KeyTapHold); err != nil {
					cfg.Log(fmt.Sprintf("Key VK_0x%02X failed: %v", cfg.HPKeyVK, err))
				}
			}
			timing.Sleep(ctx, potsEndedDelay)
			continue
		}

		// Character respawned — restore the normal mode label.
		if dead && result.Status == StatusFound {
			dead = false
			setMode(cfg.OnStatusUIMode, reader.Name())
		}

		// SINGLE OCR recovery probe — runs every iteration when pixel is
		// active. Also handles StatusDead: when the character dies while the
		// pixel reader is active (e.g. OCR failed during death animation),
		// the probe detects HP=1, switches back to OCR, and the main loop's
		// StatusDead block handles it on the next iteration.
		if reader == pixel && ocr != nil && time.Now().After(nextOCRRetry) {
			nextOCRRetry = time.Now().Add(ocrProbeInterval)
			probe := ocr.ReadValues(ctx)
			if probe.Status == StatusFound {
				cfg.Log("autopot: statusui recovered, switching back")
				reader = ocr
				setMode(cfg.OnStatusUIMode, "OCR")
				loggedPixelFail = false
				result = probe
			} else if probe.Status == StatusDead {
				// Character is dead — switch back to OCR so the main
				// loop's StatusDead block handles it properly.
				cfg.Log("autopot: statusui recovered (dead)")
				reader = ocr
				loggedPixelFail = false
				result = probe
				continue // next iteration: StatusDead check at top
			}
		}

		if result.Status != StatusFound {
			if reader == ocr {
				cfg.Log(fmt.Sprintf("autopot: statusui issue, switching to pixel-bar: %v", result.Err))
				reader = pixel
				setMode(cfg.OnStatusUIMode, "Pixel-bar")
				// pixelModeSentinel → overlay shows "error: Pixelsearch is used".
				if cfg.OnStatusParsed != nil {
					cfg.OnStatusParsed(pixelModeSentinel, 0, pixelModeSentinel, 0, 0, 0, 0, 0)
				}
				nextOCRRetry = time.Now().Add(ocrProbeInterval)
				continue
			}
			// Pixel reader failed. Track how long it's been continuously failing;
			// only slow to 5s polling after 5 consecutive seconds without bars.
			// This gives the game a window to launch without delay.
			if pixelFailStart.IsZero() {
				pixelFailStart = time.Now()
			}
			if time.Since(pixelFailStart) < statusUIRetryInterval {
				// First 5 seconds — fast polling (50ms) for responsive startup.
				timing.Sleep(ctx, timing.CaptureRetryDelay)
				continue
			}
			// 5+ seconds without bars — slow to 5s polling. The OCR recovery
			// probe above tries OCR every 5s; if it recovers we switch back.
			if !loggedPixelFail {
				cfg.Log(fmt.Sprintf("autopot: pixel bars not found for 5s — retrying every 5s: %v", result.Err))
				loggedPixelFail = true
			}
			timing.Sleep(ctx, statusUIRetryInterval)
			continue
		}

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

		// 50ms between reads during idle — fast enough to react to
		// damage while keeping CPU usage low, especially in pixel
		// mode (which is very fast per-iteration). During healing
		// (healUntil) the polling runs at PollInterval (10ms).
		timing.Sleep(ctx, timing.CaptureRetryDelay)
	}
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
// overlay shows "HP pots ended" / "SP pots ended". If the value eventually
// changes (a potion took effect), we exit the slow state immediately.
func (a *AutoPotRunner) healUntil(ctx context.Context, reader BarReader, hpBar bool) {
	var (
		healStart  time.Time
		lastPct    = -1.0
		potsEnded  bool
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
			a.clearPotsEndedMode(cfg.OnStatusUIMode, potsEnded)
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
			a.clearPotsEndedMode(cfg.OnStatusUIMode, potsEnded)
			return
		}

		// Initialise tracking on first iteration.
		if healStart.IsZero() {
			healStart = time.Now()
			lastPct = pct
		}

		elapsed := time.Since(healStart)

		// Detect pots ended: 3+ seconds of spamming with no value change.
		// Re-apply the label on EVERY iteration while potsEnded is true so
		// OCR validation (panel lost→found→"Searching..."→"OCR") cannot
		// overwrite it. Only log on first entry.
		if !potsEnded && elapsed >= noChangeTimeout && absPctDiff(pct, lastPct) < valueChangeTol {
			potsEnded = true
			cfg.Log(fmt.Sprintf("autopot: %s — slowing to 1s taps", potsEndedLabel(cfg, hpBar)))
		}
		if potsEnded {
			// Re-apply every iteration so OCR validation doesn't overwrite.
			setMode(cfg.OnStatusUIMode, potsEndedLabel(cfg, hpBar))
		}

		// Exit pots-ended when value finally changes (potion took effect).
		// Reset healStart so the 3s timeout starts fresh in the new state.
		// Skip the TapKey on this iteration — the potion is already working
		// from the previous tap (1s ago), tapping again wastes a potion.
		if potsEnded && absPctDiff(pct, lastPct) >= valueChangeTol {
			cfg.Log("autopot: potion took effect, resuming normal speed")
			setMode(cfg.OnStatusUIMode, "") // clear pots-ended label
			potsEnded = false
			healStart = time.Now()
			continue
		}

		lastPct = pct

		if err := cfg.Session.TapKey(vk, timing.KeyTapHold); err != nil {
			cfg.Log(fmt.Sprintf("Key VK_0x%02X failed: %v", vk, err))
			return
		}

		if potsEnded {
			// Slow taps — the potion stack appears empty, just retry
			// every 1s in case the player looted more.
			timing.Sleep(ctx, potsEndedDelay)
		} else {
			// Rapid taps — the game animation is the bottleneck.
			timing.Sleep(ctx, timing.PollInterval)
		}
	}
}

// clearPotsEndedMode clears the "Pots ended" overlay mode when leaving
// healUntil (regardless of exit reason: recovered, reader failed, ctx done).
func (a *AutoPotRunner) clearPotsEndedMode(fn func(string), potsEnded bool) {
	if potsEnded {
		setMode(fn, "")
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
