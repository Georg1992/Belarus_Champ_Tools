// Package runner's clicker runner: while a physical key is held, emit
// clicks (mouse) or key taps (keyboard). Its lifecycle is driven by
// internal/lifecycle; timing uses internal/timing; the session interface
// is internal/session.InputSession.
//
// Clicker's public UpdateSettings(slots [ClickerSlotCount]ClickerSlot) is
// preserved (matches what gui/main.go calls). Under the hood we build a
// full Config from lc.Settings() and swap in the new slots.
package runner

import (
	"context"
	"fmt"
	"time"

	"belarus-champ-tools/runner/internal/lifecycle"
	"belarus-champ-tools/runner/internal/session"
	"belarus-champ-tools/runner/internal/timing"
)

const (
	ClickerSlotCount = 5
	DefaultDelayMs   = 50
)

// ClickerSlot is one independent clicker: hold TriggerVK to tap that key
// (and optionally mouse-click). Unused slots have TriggerVK == 0.
type ClickerSlot struct {
	TriggerVK  int32
	DelayMs    int
	MouseClick bool
}

// Config holds every mutable thing the clicker loop needs.
type Config struct {
	Session session.InputSession
	Log     func(string)
	Slots   [ClickerSlotCount]ClickerSlot
}

// Runner watches trigger keys and emits clicks.
type Runner struct {
	lc *lifecycle.Lifecycle[Config]
}

// New constructs a Runner backed by a Lifecycle. The Log callback is
// defaulted to a no-op so callers don't have to.
func New(cfg Config) *Runner {
	if cfg.Log == nil {
		cfg.Log = func(string) {}
	}
	r := &Runner{}
	r.lc = lifecycle.New[Config](
		cfg,
		func(c Config) error {
			if c.Session == nil {
				return fmt.Errorf("input session is required")
			}
			return nil
		},
		nil,
	)
	return r
}

// Running reports whether the clicker loop is currently active.
func (r *Runner) Running() bool { return r.lc.Running() }

// UpdateSettings merges the new slots into the live config (preserving
// Session/Log captured at Start() time) so callers can push just the part
// of the cfg they're editing.
func (r *Runner) UpdateSettings(slots [ClickerSlotCount]ClickerSlot) {
	cfg := r.lc.Settings()
	cfg.Slots = slots
	r.lc.UpdateSettings(cfg)
}

func (r *Runner) settings() Config { return r.lc.Settings() }

// Start launches the clicker loop.
func (r *Runner) Start() error {
	if err := r.lc.Start(r.run); err != nil {
		return fmt.Errorf("clicker: %w", err)
	}
	return nil
}

// Stop signals the clicker loop to exit.
func (r *Runner) Stop() { r.lc.Stop() }

// Wait blocks until the clicker goroutine has exited.
func (r *Runner) Wait() { r.lc.Wait() }

func (r *Runner) run(ctx context.Context, _ Config) {
	var nextDue [ClickerSlotCount]time.Time

	for {
		if ctx.Err() != nil {
			return
		}
		current := r.settings()
		now := time.Now()
		anyMapped := false
		anyHeld := false
		var earliest time.Time

		for i := range current.Slots {
			slot := current.Slots[i]
			if slot.TriggerVK == 0 {
				nextDue[i] = time.Time{}
				continue
			}
			anyMapped = true

			if !PhysicalKeyDown(slot.TriggerVK) {
				nextDue[i] = time.Time{}
				continue
			}
			anyHeld = true

			delayMs := slot.DelayMs
			if delayMs <= 0 {
				delayMs = DefaultDelayMs
			}
			delay := time.Duration(delayMs) * time.Millisecond

			if nextDue[i].IsZero() || !now.Before(nextDue[i]) {
				if err := r.fireSlot(current.Session, slot); err != nil {
					if ctx.Err() != nil {
						return
					}
					current.Log(err.Error())
				}
				nextDue[i] = now.Add(delay)
			}
			if earliest.IsZero() || nextDue[i].Before(earliest) {
				earliest = nextDue[i]
			}
		}

		if !anyMapped {
			timing.Sleep(ctx, timing.CaptureRetryDelay)
			continue
		}
		if !anyHeld {
			timing.Sleep(ctx, timing.PollInterval)
			continue
		}

		wait := time.Until(earliest)
		if wait < timing.MinPollWait {
			wait = timing.MinPollWait
		}
		if wait > timing.PollInterval {
			wait = timing.PollInterval
		}
		timing.Sleep(ctx, wait)
	}
}

func (r *Runner) fireSlot(sess session.InputSession, slot ClickerSlot) error {
	if err := sess.TapKey(slot.TriggerVK, timing.KeyTapHold); err != nil {
		return fmt.Errorf("clicker key %s failed: %v", KeyName(slot.TriggerVK), err)
	}
	if slot.MouseClick {
		if err := sess.MouseClick(timing.MouseClickHold); err != nil {
			return fmt.Errorf("clicker mouse click failed: %v", err)
		}
	}
	return nil
}
