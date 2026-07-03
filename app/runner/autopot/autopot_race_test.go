package autopot

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"belarus-champ-tools/runner/internal/session"
)

// mockSession is a session.InputSession for the autopot stress test.
type mockSession struct {
	mu       sync.Mutex
	paused   bool
	tapCount atomic.Int64
}

func (m *mockSession) Paused() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.paused
}

func (m *mockSession) SetPaused(p bool) {
	m.mu.Lock()
	m.paused = p
	m.mu.Unlock()
}

func (m *mockSession) TapKey(vk int32, hold time.Duration) error {
	m.tapCount.Add(1)
	time.Sleep(hold)
	return nil
}

func (m *mockSession) MouseDown() error { return nil }
func (m *mockSession) MouseUp() error   { return nil }

// TestAutoPotRunnerStress starts a real AutoPotRunner. The run() loop
// calls win.CapturePlayerBarSearch(), which fails in a non-game test env
// and triggers the `continue` branch. That branch still exercises:
//   - a.settings()        (lifecycle.Settings, RLock on liveMu)
//   - session.Paused()    (InputSession.RLock in real ViiperSession)
//   - timing.Sleep        (ctx-aware sleep — Stop works)
//
// and the spawned numericValidator goroutine also loops, calling
// SetThresholds on the validator and atomic Store of
// cachedSafety. Hammering UpdateSettings from outside covers the same
// surface the healUntil() hot path reads.
func TestAutoPotRunnerStress(t *testing.T) {
	sess := &mockSession{}
	cfg := AutoPotConfig{
		Session:     sess,
		HPThreshold: 50,
		SPThreshold: 50,
		HPKeyVK:     'Q',
		SPKeyVK:     'W',
		HPEnabled:   true,
		SPEnabled:   true,
		Log:         func(string) {},
	}
	ap := NewAutoPot(cfg)
	if err := ap.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			n := seed
			for {
				select {
				case <-stop:
					return
				default:
					ap.UpdateSettings(AutoPotConfig{
						Session:     sess,
						HPThreshold: 40 + n%40,
						SPThreshold: 40 + n%40,
						HPKeyVK:     'Q',
						SPKeyVK:     'W',
						HPEnabled:   n%2 == 0,
						SPEnabled:   true,
						Log:         func(string) {},
					})
					n++
				}
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		on := false
		for {
			select {
			case <-stop:
				return
			case <-time.After(3 * time.Millisecond):
				on = !on
				sess.SetPaused(on)
			}
		}
	}()
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = ap.Running()
				}
			}
		}()
	}

	time.Sleep(250 * time.Millisecond)
	close(stop)
	wg.Wait()
	ap.Stop()
	ap.Wait()
	if ap.Running() {
		t.Fatal("still running after Stop+Wait")
	}
}

var _ session.InputSession = (*mockSession)(nil)

// mockFlakyReader is a BarReader that can fail transiently on demand.
// Returns hpValue (default 30, below threshold) on success so that
// healUntil keeps pressing the potion key.
type mockFlakyReader struct {
	mu        sync.Mutex
	failNext  int
	callCount int
	hpValue   float64
}

func (r *mockFlakyReader) setFailNext(n int) {
	r.mu.Lock()
	r.failNext = n
	r.mu.Unlock()
}

func (r *mockFlakyReader) ReadBars(ctx context.Context) BarReadResult {
	r.mu.Lock()
	r.callCount++
	if r.failNext > 0 {
		r.failNext--
		r.mu.Unlock()
		return BarReadResult{Status: StatusInvalid, Err: fmt.Errorf("transient mock failure")}
	}
	r.mu.Unlock()
	return BarReadResult{
		Status: StatusFound,
		HP:     r.hpValue,
		SP:     80,
		HPLow:  r.hpValue < 50,
		SPLow:  false,
	}
}

func (r *mockFlakyReader) Name() string { return "mockFlaky" }

// TestAutoPotHealUntilErrorAborts verifies that healUntil returns
// on reader failure instead of retrying forever. The main loop
// (run()) handles mode switching (OCR→pixel) when the reader fails,
// so healUntil must return promptly to give control back.
//
// The mock reader starts with HP=30 (below threshold) and succeeds
// initially so healUntil presses keys. Then it permanently fails,
// and healUntil must return within one read cycle.
//
// NOTE: We do NOT call ap.Start() — the lifecycle's Settings() uses
// liveMu.RLock and works from the initial config stored by New().
// Starting the lifecycle would spawn the run() goroutine which tries
// to initialize statusui.NewDefaultPipeline(), which can hang in the
// test environment.
func TestAutoPotHealUntilErrorAborts(t *testing.T) {
	sess := &mockSession{}
	cfg := AutoPotConfig{
		Session:     sess,
		HPThreshold: 50,
		HPKeyVK:     'Q',
		HPEnabled:   true,
		Log:         func(string) {},
	}
	ap := NewAutoPot(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	reader := &mockFlakyReader{hpValue: 30, failNext: 1} // fail on first call

	start := time.Now()
	ap.healUntil(ctx, reader, true)
	elapsed := time.Since(start)

	// healUntil should return quickly — the reader fails on first call.
	if elapsed > 200*time.Millisecond {
		t.Errorf("healUntil took %v to abort on reader failure (expected < 200ms)",
			elapsed.Round(time.Millisecond))
	}

	tapCount := sess.tapCount.Load()
	if tapCount > 0 {
		t.Errorf("healUntil pressed %d keys despite reader failing on first call", tapCount)
	}

	t.Logf("healUntil aborted after %v, TapKey calls=%d, reader calls=%d",
		elapsed.Round(time.Millisecond), tapCount, reader.callCount)
}

// TestAutoPotHealUntilSucceeds verifies that healUntil presses keys
// and exits when the value rises above threshold.
func TestAutoPotHealUntilSucceeds(t *testing.T) {
	sess := &mockSession{}
	// Start below threshold, then healUntil reads will see the value
	// cross above threshold and return.
	cfg := AutoPotConfig{
		Session:     sess,
		HPThreshold: 50,
		HPKeyVK:     'Q',
		HPEnabled:   true,
		Log:         func(string) {},
	}
	ap := NewAutoPot(cfg)

	// Reader returns values that start below threshold then rise.
	reader := &risingReader{
		hpValues: []float64{30, 30, 30, 60, 80},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	start := time.Now()
	ap.healUntil(ctx, reader, true)
	elapsed := time.Since(start)

	if elapsed > 500*time.Millisecond {
		t.Errorf("healUntil took %v to complete (expected < 500ms)",
			elapsed.Round(time.Millisecond))
	}

	tapCount := sess.tapCount.Load()
	if tapCount == 0 {
		t.Error("healUntil did not press any keys")
	}

	t.Logf("healUntil completed after %v, TapKey calls=%d, reader calls=%d",
		elapsed.Round(time.Millisecond), tapCount, reader.callCount)
}

// risingReader returns the next HP value from a pre-defined sequence,
// cycling the last value for any extra calls.
type risingReader struct {
	mu        sync.Mutex
	callCount int
	hpValues  []float64
}

func (r *risingReader) ReadBars(ctx context.Context) BarReadResult {
	r.mu.Lock()
	idx := r.callCount
	if idx >= len(r.hpValues) {
		idx = len(r.hpValues) - 1
	}
	hp := r.hpValues[idx]
	r.callCount++
	r.mu.Unlock()
	return BarReadResult{
		Status: StatusFound,
		HP:     hp,
		SP:     80,
		HPLow:  hp < 50,
		SPLow:  false,
	}
}

func (r *risingReader) Name() string { return "rising" }
