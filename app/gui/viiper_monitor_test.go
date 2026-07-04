//go:build windows

package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// viiperMonitor tests
//
// The monitor only fires onStatusChange on transitions (active↔inactive).
// It initialises wasActive=false, so if VIIPER is not running the first
// ping also yields inactive and no callback fires. Tests below respect
// this contract.
// ---------------------------------------------------------------------------

func TestStartViiperMonitor_StopDoesNotDeadlock(t *testing.T) {
	m := startViiperMonitor(context.Background(), func(bool) {})
	// Give the goroutine a moment to start.
	time.Sleep(10 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		m.stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("stop() did not return within 3s — deadlock or goroutine leak")
	}
}

func TestStartViiperMonitor_NoCallbacksWhenStatusStable(t *testing.T) {
	var callCount atomic.Int32

	m := startViiperMonitor(context.Background(), func(bool) {
		callCount.Add(1)
	})

	// Wait for at least two full ping cycles (each: ~instant ping + 2s sleep).
	// If VIIPER is not running: first ping returns inactive, matches
	// wasActive=false → no transition, 0 callbacks.
	// If VIIPER is running: first ping returns active → transition,
	// exactly 1 callback. After that, status is stable — no more callbacks.
	time.Sleep(4500 * time.Millisecond)

	if c := callCount.Load(); c > 1 {
		t.Errorf("expected at most 1 callback (initial transition only), got %d — status kept changing", c)
	}

	m.stop()
}

func TestStartViiperMonitor_SurvivesMultiplePingCycles(t *testing.T) {
	m := startViiperMonitor(context.Background(), func(bool) {})

	// Let several ping cycles complete.
	time.Sleep(4500 * time.Millisecond)

	// Stop must still return promptly.
	done := make(chan struct{})
	go func() {
		m.stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("stop() did not return after multiple ping cycles")
	}
}

func TestStartViiperMonitor_MultipleStartStopCycles(t *testing.T) {
	for i := 0; i < 3; i++ {
		m := startViiperMonitor(context.Background(), func(bool) {})
		time.Sleep(10 * time.Millisecond)
		m.stop()
	}
}

// ---------------------------------------------------------------------------
// viiperBadge tests
//
// The badge is a Walk CustomWidget and requires a running GUI to paint.
// These tests cover the pure-data surface (constants, maps, dimensions)
// and the SetStatus dedup guard without requiring Walk.
// ---------------------------------------------------------------------------

func TestViiperStatus_ConstantsHaveExpectedValues(t *testing.T) {
	if viiperInactive != 0 {
		t.Errorf("viiperInactive = %d, want 0", viiperInactive)
	}
	if viiperActive != 1 {
		t.Errorf("viiperActive = %d, want 1", viiperActive)
	}
}

func TestViiperColors_AllStatusesHaveEntries(t *testing.T) {
	statuses := []viiperStatus{viiperInactive, viiperActive}
	for _, s := range statuses {
		if _, ok := viiperColors[s]; !ok {
			t.Errorf("viiperColors missing entry for status %d", s)
		}
	}
}

func TestViiperTexts_AllStatusesHaveEntries(t *testing.T) {
	statuses := []viiperStatus{viiperInactive, viiperActive}
	for _, s := range statuses {
		if _, ok := viiperTexts[s]; !ok {
			t.Errorf("viiperTexts missing entry for status %d", s)
		}
	}
}

func TestViiperTexts_Content(t *testing.T) {
	if got := viiperTexts[viiperInactive]; got != "VIIPER OFF" {
		t.Errorf("inactive text = %q, want %q", got, "VIIPER OFF")
	}
	if got := viiperTexts[viiperActive]; got != "VIIPER ON" {
		t.Errorf("active text = %q, want %q", got, "VIIPER ON")
	}
}

func TestViiperDimensions_Positive(t *testing.T) {
	if viiperBadgeWidth <= 0 {
		t.Errorf("viiperBadgeWidth = %d, want > 0", viiperBadgeWidth)
	}
	if viiperBadgeHeight <= 0 {
		t.Errorf("viiperBadgeHeight = %d, want > 0", viiperBadgeHeight)
	}
}

// TestViiperBadge_SetStatusDedup verifies the guard clause without
// depending on a Walk canvas. The embedded *walk.CustomWidget is nil,
// so calling SetStatus panics on the final Invalidate() call when the
// status actually changes. We recover from that panic and verify that
// the status field was updated before the Invalidate call.
//
// The same-status path returns before Invalidate, so it doesn't panic.
func TestViiperBadge_SetStatusDedup(t *testing.T) {
	b := &viiperBadge{status: viiperInactive}

	// Same status: should return before Invalidate.
	b.SetStatus(viiperInactive)
	if b.status != viiperInactive {
		t.Errorf("expected status to remain inactive, got %d", b.status)
	}

	// Different status: updates the field, then panics on Invalidate
	// because CustomWidget is nil. We recover and verify the field.
	func() {
		defer func() { recover() }()
		b.SetStatus(viiperActive)
	}()
	if b.status != viiperActive {
		t.Errorf("expected status to become active after SetStatus, got %d", b.status)
	}

	// Back to inactive.
	func() {
		defer func() { recover() }()
		b.SetStatus(viiperInactive)
	}()
	if b.status != viiperInactive {
		t.Errorf("expected status to become inactive after SetStatus, got %d", b.status)
	}
}
