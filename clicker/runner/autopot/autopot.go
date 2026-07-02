// Package autopot is the HP/SP auto-potion runner.
//
// Lifecycle bookkeeping (Start/Stop/Wait/Running/UpdateSettings) lives in
// internal/lifecycle; timing constants in internal/timing; the
// InputSession interface in internal/session. autopot does not import the
// parent runner package (to keep the import graph cycle-free) so it
// composes Lifecycle, session.InputSession, and timing.* from internal/.
package autopot

import (
	"context"
	"fmt"
	"time"

	win "experimental-clicker/runner/platform/windows"
	"experimental-clicker/runner/statusui"

	"experimental-clicker/runner/internal/lifecycle"
	"experimental-clicker/runner/internal/session"
	"experimental-clicker/runner/internal/timing"
)

// AutoPotConfig is what gui/main.go passes to NewAutoPot.
type AutoPotConfig struct {
	Session     session.InputSession
	HPThreshold int
	SPThreshold int
	HPKeyVK     int32
	SPKeyVK     int32
	HPEnabled   bool
	SPEnabled   bool
	Log         func(string)
	// OnStatusParsed is called from the statusui loop after each successful
	// HP/SP parse. stripX/Y/W/H are the screen-space coordinates of the text
	// strip used to read the values. May be nil.
	OnStatusParsed func(hp, hpMax, sp, spMax, stripX, stripY, stripW, stripH int)
	// OnStatusUIMode is called when the autopot switches between the statusui
	// OCR reader and the pixel-bar fallback so the overlay can display the
	// current mode. May be nil.
	OnStatusUIMode func(mode string)
}

// AutoPotRunner heals HP/SP based on bar-fill reading. Embeds a Lifecycle so
// the goroutine bookkeeping isn't reimplemented.
type AutoPotRunner struct {
	lc *lifecycle.Lifecycle[AutoPotConfig]

	hpStabilizer *BarStabilizer
	spStabilizer *BarStabilizer

	// wasPanelFound tracks whether the status panel was successfully
	// located at least once. Used by validateWithLog to debounce log
	// messages: failures are only logged on a state transition
	// (found→lost, lost→found), not on every retry.
	wasPanelFound bool
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
			func(c AutoPotConfig) {
				// On stop, reset stabilizers so a future Start begins clean.
				_ = c // stabilizer.Reset is on the runner; called in Stop hook below
			},
		),
		hpStabilizer: NewBarStabilizer(true, cfg.HPThreshold),
		spStabilizer: NewBarStabilizer(false, cfg.SPThreshold),
	}
}

// Running reports whether the heal loop is currently active.
func (a *AutoPotRunner) Running() bool { return a.lc.Running() }

// UpdateSettings propagates new settings to the stabilizers.
// Settings applied after Start() take effect on the next poll.
func (a *AutoPotRunner) UpdateSettings(cfg AutoPotConfig) {
	// Preserve OnStatusUIMode from the existing config if the incoming
	// config doesn't set it (e.g. settings sync from the GUI).
	if cfg.OnStatusUIMode == nil {
		cfg.OnStatusUIMode = a.settings().OnStatusUIMode
	}
	if cfg.OnStatusParsed == nil {
		cfg.OnStatusParsed = a.settings().OnStatusParsed
	}
	a.lc.UpdateSettings(cfg)
	a.hpStabilizer.SetThreshold(cfg.HPThreshold)
	a.spStabilizer.SetThreshold(cfg.SPThreshold)
}

// Start launches the healer. Returns an error if validation fails or the
// runner is already active.
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

// settings returns a snapshot of the live config.
func (a *AutoPotRunner) settings() AutoPotConfig { return a.lc.Settings() }

// resetStabilizers is called after a Stop completes (or on Start).
func (a *AutoPotRunner) resetStabilizers() {
	a.hpStabilizer.Reset()
	a.spStabilizer.Reset()
	a.wasPanelFound = false
}

// statusUIRetryInterval is how often the pixel-bar loop attempts to
// switch back to the status UI after falling back.
const statusUIRetryInterval = 30 * time.Second

// run is the main autopot loop. It alternates between the statusui OCR
// reader (primary) and the pixel-bar reader (fallback). When the status
// UI encounters any issue (panel not found, parse error, pipeline failure),
// it falls back to pixel-bar immediately. Every 30 s the pixel-bar loop
// probes whether the status UI has recovered and switches back if so.
func (a *AutoPotRunner) run(ctx context.Context, cfg AutoPotConfig) {
	defer a.resetStabilizers()

	// Build the statusui pipeline once; reuse across switches.
	pipeline, err := statusui.NewDefaultPipeline()
	hasPipeline := err == nil
	var poller *statusui.StripPoller
	if hasPipeline {
		poller = statusui.NewStripPoller(pipeline)
	}

	useStatusUI := hasPipeline
	if useStatusUI {
		a.setMode("OCR", cfg)
	} else {
		a.setMode("Pixel-bar", cfg)
	}

	for {
		// === Status UI (OCR) mode ===================================
		if useStatusUI {
			if err := a.runStatusUI(ctx, poller); err != nil {
				cfg.Log(fmt.Sprintf("autopot: statusui issue, switching to pixel-bar: %v", err))
				useStatusUI = false
				a.setMode("Pixel-bar", cfg)
				// fall through to pixel-bar below
			} else {
				return // normal Stop via ctx cancel
			}
		}

		// === Pixel-bar fallback ======================================
		nextStatusUIRetry := time.Now().Add(statusUIRetryInterval)

		for !useStatusUI {
			select {
			case <-ctx.Done():
				return
			default:
			}

			cfg = a.settings()
			session := cfg.Session
			if session == nil || session.Paused() {
				timing.Sleep(ctx, timing.PollInterval)
				continue
			}

			// Periodic probe: can we switch back to status UI?
			if hasPipeline && time.Now().After(nextStatusUIRetry) {
				nextStatusUIRetry = time.Now().Add(statusUIRetryInterval)
				if a.tryStatusUIOnce(poller) == nil {
					cfg.Log("autopot: statusui recovered, switching back")
					useStatusUI = true
					a.setMode("OCR", cfg)
					break // exit pixel-bar loop, back to status UI
				}
			}

			// Normal pixel-bar tick.
			img, _, err := win.CapturePlayerBarSearch()
			if err != nil {
				timing.Sleep(ctx, timing.CaptureRetryDelay)
				continue
			}

			mapped, pairOK := RefreshStableBarPair(img)

			hp := a.hpStabilizer.UpdatePair(img, true, mapped, pairOK)
			if cfg.HPEnabled && hp.Status == BarStatusLow {
				a.healUntil(ctx, session, true)
				continue
			}

			sp := a.spStabilizer.UpdatePair(img, false, mapped, pairOK)
			if cfg.SPEnabled && sp.Status == BarStatusLow {
				a.healUntil(ctx, session, false)
				continue
			}

			timing.Sleep(ctx, timing.KeyTapHold)
		}
	}
}

// tryStatusUIOnce performs a single validate+parse cycle to check whether
// the status UI is currently viable. It primes the poller with a fresh
// strip rect on success so the next runStatusUI call can skip validation.
// Returns nil only if both panel detection and OCR parsing succeed.
func (a *AutoPotRunner) tryStatusUIOnce(poller *statusui.StripPoller) error {
	screen, err := win.CaptureFullScreen()
	if err != nil {
		return err
	}
	if err := poller.Validate(screen); err != nil {
		return err
	}
	// Validate succeeded — poller now has a fresh strip rect and
	// lastValidate timestamp, so runStatusUI can skip to parsing.
	_, err = captureAndParse(poller)
	return err
}

// setMode fires cfg.OnStatusUIMode if set.
func (a *AutoPotRunner) setMode(mode string, cfg AutoPotConfig) {
	if cfg.OnStatusUIMode != nil {
		cfg.OnStatusUIMode(mode)
	}
}

func (a *AutoPotRunner) healUntil(ctx context.Context, session session.InputSession, hpBar bool) {
	stabilizer := a.spStabilizer
	if hpBar {
		stabilizer = a.hpStabilizer
	}

	for {
		if ctx.Err() != nil {
			return
		}
		if session.Paused() {
			timing.Sleep(ctx, timing.PollInterval)
			continue
		}
		cfg := a.settings()
		vk, ok := healTarget(cfg, hpBar)
		if !ok {
			return
		}

		img, _, err := win.CapturePlayerBarSearch()
		if err != nil {
			timing.Sleep(ctx, timing.PollInterval)
			continue
		}
		mapped, pairOK := RefreshStableBarPair(img)
		read := stabilizer.UpdatePair(img, hpBar, mapped, pairOK)
		if read.Status != BarStatusLow {
			return
		}
		before := read.Percent

		if err := session.TapKey(vk, timing.KeyTapHold); err != nil {
			cfg.Log(fmt.Sprintf("Key VK_0x%02X failed: %v", vk, err))
			return
		}
		for {
			if ctx.Err() != nil {
				return
			}
			if session.Paused() {
				timing.Sleep(ctx, timing.PollInterval)
				continue
			}
			cfg = a.settings()
			if _, ok := healTarget(cfg, hpBar); !ok {
				return
			}
			img, _, err := win.CapturePlayerBarSearch()
			if err != nil {
				continue
			}
			mapped, pairOK := RefreshStableBarPair(img)
			read := stabilizer.UpdatePair(img, hpBar, mapped, pairOK)
			if read.Status != BarStatusLow {
				return
			}
			if read.Percent > before {
				break
			}
		}
	}
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
