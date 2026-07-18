package runner

import (
	"sync"
	"testing"
	"time"
)

// recordingSession records TapKey VKs for clicker unit tests.
type recordingSession struct {
	mu   sync.Mutex
	taps []int32
}

func (s *recordingSession) Paused() bool { return false }
func (s *recordingSession) TapKey(vk int32, _ time.Duration) error {
	s.mu.Lock()
	s.taps = append(s.taps, vk)
	s.mu.Unlock()
	return nil
}
func (s *recordingSession) MouseClick(_ time.Duration) error { return nil }
func (s *recordingSession) snapshot() []int32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]int32(nil), s.taps...)
}

func TestClicker_OnlyFiresOwnKey(t *testing.T) {
	orig := PhysicalKeyDown
	defer func() { PhysicalKeyDown = orig }()

	PhysicalKeyDown = func(vk int32) bool { return vk == 'D' }

	sess := &recordingSession{}
	r := New(Config{
		Session: sess,
		Slots: [ClickerSlotCount]ClickerSlot{
			{TriggerVK: 'D', DelayMs: 5, MouseClick: true},
			{TriggerVK: 'T', DelayMs: 5, MouseClick: false},
		},
		Log: func(string) {},
	})
	if err := r.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		r.Stop()
		r.Wait()
	}()

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		taps := sess.snapshot()
		if len(taps) >= 1 {
			for _, vk := range taps {
				if vk != 'D' {
					t.Fatalf("expected only D taps while D held; taps=%v", taps)
				}
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("expected at least one D tap")
}

func TestClicker_SlotsActIndependently(t *testing.T) {
	orig := PhysicalKeyDown
	defer func() { PhysicalKeyDown = orig }()

	PhysicalKeyDown = func(vk int32) bool { return vk == 'D' || vk == 'T' }

	sess := &recordingSession{}
	r := New(Config{
		Session: sess,
		Slots: [ClickerSlotCount]ClickerSlot{
			{TriggerVK: 'D', DelayMs: 5, MouseClick: true},
			{TriggerVK: 'T', DelayMs: 5, MouseClick: false},
		},
		Log: func(string) {},
	})
	if err := r.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		r.Stop()
		r.Wait()
	}()

	deadline := time.Now().Add(300 * time.Millisecond)
	sawD, sawT := false, false
	for time.Now().Before(deadline) {
		for _, vk := range sess.snapshot() {
			if vk == 'D' {
				sawD = true
			}
			if vk == 'T' {
				sawT = true
			}
		}
		if sawD && sawT {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected both slots to fire; sawD=%v sawT=%v taps=%v", sawD, sawT, sess.snapshot())
}
