package runner

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	DefaultAPIAddr = "localhost:3242"
	DefaultDelayMs = 50
	StepHoldMs     = 20 // minimum gap so virtual HID events register
	PauseVK        = 0x23 // End
)

type Config struct {
	Session    *ViiperSession
	TriggerVKs []int32
	DelayMs    int
	MouseClick bool
	Log        func(string)
}

func (c *Config) applyDefaults() {
	if c.DelayMs <= 0 {
		c.DelayMs = DefaultDelayMs
	}
	if c.Log == nil {
		c.Log = func(string) {}
	}
}

type Runner struct {
	cfg Config

	mu             sync.Mutex
	cancel         context.CancelFunc
	done           chan struct{}
	running        bool
	liveMu         sync.RWMutex
	liveTriggerVKs []int32
	liveDelayMs    int
	liveMouseClick bool
}

func New(cfg Config) *Runner {
	cfg.applyDefaults()
	return &Runner{cfg: cfg}
}

func (r *Runner) Running() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

func (r *Runner) Start() error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("clicker already running")
	}
	if r.cfg.Session == nil {
		r.mu.Unlock()
		return fmt.Errorf("input session is required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.running = true
	r.liveTriggerVKs = append([]int32(nil), r.cfg.TriggerVKs...)
	r.liveDelayMs = r.cfg.DelayMs
	r.liveMouseClick = r.cfg.MouseClick
	r.done = make(chan struct{})
	r.mu.Unlock()

	go func() {
		defer close(r.done)
		defer func() {
			r.mu.Lock()
			r.running = false
			r.cancel = nil
			r.mu.Unlock()
		}()
		r.run(ctx)
	}()

	return nil
}

func (r *Runner) Stop() {
	r.mu.Lock()
	cancel := r.cancel
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (r *Runner) Wait() {
	r.mu.Lock()
	done := r.done
	r.mu.Unlock()
	if done != nil {
		<-done
	}
}

func (r *Runner) UpdateSettings(triggerVKs []int32, delayMs int, mouseClick bool) {
	r.liveMu.Lock()
	r.liveTriggerVKs = append([]int32(nil), triggerVKs...)
	if delayMs > 0 {
		r.liveDelayMs = delayMs
	}
	r.liveMouseClick = mouseClick
	r.liveMu.Unlock()
}

func (r *Runner) settings() ([]int32, time.Duration, bool) {
	r.liveMu.RLock()
	delayMs := r.liveDelayMs
	triggerVKs := append([]int32(nil), r.liveTriggerVKs...)
	mouseClick := r.liveMouseClick
	r.liveMu.RUnlock()
	return triggerVKs, time.Duration(delayMs) * time.Millisecond, mouseClick
}

func (r *Runner) log(msg string) {
	r.cfg.Log(msg)
}

func (r *Runner) run(ctx context.Context) {
	session := r.cfg.Session

	for {
		if ctx.Err() != nil {
			return
		}

		if session.Paused() {
			time.Sleep(PollInterval)
			continue
		}

		triggerVKs, delay, mouseClick := r.settings()
		if !TriggerHeld(triggerVKs) {
			time.Sleep(PollInterval)
			continue
		}

		triggerVK, _ := ActiveTrigger(triggerVKs)

		for TriggerHeld(triggerVKs) && !session.Paused() {
			if ctx.Err() != nil {
				return
			}

			triggerVKs, delay, mouseClick = r.settings()
			triggerVK, _ = ActiveTrigger(triggerVKs)
			if err := runCycle(ctx, session, triggerVK, triggerVKs, delay, mouseClick); err != nil {
				if ctx.Err() != nil {
					return
				}
				r.log(fmt.Sprintf("Clicker loop failed: %v", err))
				return
			}
			if !TriggerHeld(triggerVKs) || session.Paused() {
				break
			}
		}
	}
}

func runCycle(ctx context.Context, session *ViiperSession, vk int32, triggerVKs []int32, delay time.Duration, mouseClick bool) error {
	defer session.ReleaseAll()

	step := time.Duration(StepHoldMs) * time.Millisecond

	if err := session.KeyDown(vk); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if !waitDelay(ctx, triggerVKs, delay, session.Paused) {
		return ctx.Err()
	}

	if mouseClick {
		if err := session.MouseDown(); err != nil {
			return err
		}
		sleep(ctx, step)
	}

	if err := session.KeyUp(); err != nil {
		return err
	}

	if mouseClick {
		sleep(ctx, step)
		if err := session.MouseUp(); err != nil {
			return err
		}
	}
	return nil
}

// waitDelay sleeps for delay. Trigger release ends the wait early but the cycle continues.
// Returns false only when the clicker is stopped (context cancelled).
func waitDelay(ctx context.Context, triggerVKs []int32, d time.Duration, paused func() bool) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return false
		}
		if paused != nil && paused() {
			return true
		}
		if len(triggerVKs) > 0 && !TriggerHeld(triggerVKs) {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return ctx.Err() == nil
}

func sleep(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
