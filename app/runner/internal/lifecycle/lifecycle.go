// Package lifecycle is a tiny generic helper that drives the Start/Stop/
// Wait/Running/UpdateSettings/Settings lifecycle every runner needs. By
// keeping the goroutine bookkeeping in one place we avoid four near-identical
// ~150-line Runner implementations.
package lifecycle

import (
	"context"
	"errors"
	"sync"
)

// Lifecycle is the common state shared by every Runner that holds a long-
// running goroutine. Generic over C so each runner keeps Config as its own
// type without us reaching into fields.
type Lifecycle[C any] struct {
	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	running bool

	liveMu sync.RWMutex
	live   C

	// validate is checked at Start() time and must succeed before the
	// goroutine spins up. The lifecycle does not interpret the cfg.
	validate func(C) error

	// onStop, if non-nil, runs inside the goroutine right after run()
	// returns. Useful for finalizing side-channel state (e.g. clearing a
	// stabilizer).
	onStop func(C)
}

// New constructs a Lifecycle seeded with cfg. validate and onStop are
// optional; pass nil for either if they don't apply.
func New[C any](cfg C, validate func(C) error, onStop func(C)) *Lifecycle[C] {
	if validate == nil {
		validate = func(C) error { return nil }
	}
	return &Lifecycle[C]{live: cfg, validate: validate, onStop: onStop}
}

// Running reports whether the goroutine is currently active.
func (l *Lifecycle[C]) Running() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running
}

// UpdateSettings replaces the live configuration. Updates are visible to
// run() on the next Settings() read.
func (l *Lifecycle[C]) UpdateSettings(cfg C) {
	l.liveMu.Lock()
	l.live = cfg
	l.liveMu.Unlock()
}

// Settings returns a snapshot of the current live configuration. Safe to
// call from any goroutine.
func (l *Lifecycle[C]) Settings() C {
	l.liveMu.RLock()
	defer l.liveMu.RUnlock()
	return l.live
}

// Start validates cfg and spawns run() in a background goroutine. Returns an
// error if already running or if validate rejects the cfg.
func (l *Lifecycle[C]) Start(run func(context.Context, C)) error {
	l.mu.Lock()
	if l.running {
		l.mu.Unlock()
		return errors.New("already running")
	}
	cfg := l.Settings()
	if err := l.validate(cfg); err != nil {
		l.mu.Unlock()
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	l.cancel = cancel
	l.running = true
	l.done = make(chan struct{})
	onStop := l.onStop
	l.mu.Unlock()

	go func() {
		defer close(l.done)
		defer func() {
			l.mu.Lock()
			l.running = false
			l.cancel = nil
			l.mu.Unlock()
		}()
		defer func() {
			if onStop != nil {
				onStop(cfg)
			}
		}()
		run(ctx, cfg)
	}()
	return nil
}

// Stop signals the goroutine to exit. Safe to call when not running.
func (l *Lifecycle[C]) Stop() {
	l.mu.Lock()
	cancel := l.cancel
	l.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Wait blocks until the goroutine has exited.
func (l *Lifecycle[C]) Wait() {
	l.mu.Lock()
	done := l.done
	l.mu.Unlock()
	if done != nil {
		<-done
	}
}
