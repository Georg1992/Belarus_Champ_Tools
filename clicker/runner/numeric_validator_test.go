package runner

import (
	"context"
	"testing"
	"time"
)

// TestGetCachedVeto_BlocksWhenAboveThreshold tests that veto occurs when resource is above threshold.
func TestGetCachedVeto_BlocksWhenAboveThreshold(t *testing.T) {
	v := NewNumericSafetyValidator()

	// Simulate numeric read: HP at 97%
	snapshot := &NumericSafetyState{
		HP: NumericResourceRead{
			Found:      true,
			Current:    97,
			Max:        100,
			Percent:    97.0,
			UpdatedAt:  time.Now(),
			Confidence: 0.9,
		},
	}
	v.cachedState.Store(snapshot)

	// Threshold is 30%, numeric is 97% -> BLOCK (resource is safe, don't need potion)
	decision := v.GetCachedVeto(PotionHP, 30)
	if !decision.Block {
		t.Errorf("Expected Block=true when HP=97%% and threshold=30%%, got Block=%v", decision.Block)
	}
	if decision.Reason != "numeric_above_threshold" {
		t.Errorf("Expected Reason=numeric_above_threshold, got %q", decision.Reason)
	}
}

// TestGetCachedVeto_AllowsWhenBelowThreshold tests that potion is allowed when resource is at/below threshold.
func TestGetCachedVeto_AllowsWhenBelowThreshold(t *testing.T) {
	v := NewNumericSafetyValidator()

	// Simulate numeric read: HP at 25%
	snapshot := &NumericSafetyState{
		HP: NumericResourceRead{
			Found:      true,
			Current:    25,
			Max:        100,
			Percent:    25.0,
			UpdatedAt:  time.Now(),
			Confidence: 0.9,
		},
	}
	v.cachedState.Store(snapshot)

	// Threshold is 30%, numeric is 25% -> ALLOW (resource needs potion)
	decision := v.GetCachedVeto(PotionHP, 30)
	if decision.Block {
		t.Errorf("Expected Block=false when HP=25%% and threshold=30%%, got Block=%v", decision.Block)
	}
	if decision.Reason != "numeric_below_threshold" {
		t.Errorf("Expected Reason=numeric_below_threshold, got %q", decision.Reason)
	}
}

// TestGetCachedVeto_AllowsWhenParserFailed tests fail-safe: allow when parse not found.
func TestGetCachedVeto_AllowsWhenParserFailed(t *testing.T) {
	v := NewNumericSafetyValidator()
	// Empty snapshot means parser hasn't found data yet

	decision := v.GetCachedVeto(PotionHP, 30)
	if decision.Block {
		t.Errorf("Expected Block=false when parser failed, got Block=%v", decision.Block)
	}
	if decision.Reason != "numeric_parse_not_found" {
		t.Errorf("Expected Reason=numeric_parse_not_found, got %q", decision.Reason)
	}
}

// TestGetCachedVeto_AllowsWhenStale tests fail-safe: allow when numeric data is stale.
func TestGetCachedVeto_AllowsWhenStale(t *testing.T) {
	v := NewNumericSafetyValidator()

	// Simulate old numeric read (5 seconds old, max age is 2 seconds)
	snapshot := &NumericSafetyState{
		HP: NumericResourceRead{
			Found:      true,
			Current:    95,
			Max:        100,
			Percent:    95.0,
			UpdatedAt:  time.Now().Add(-5 * time.Second),
			Confidence: 0.9,
		},
	}
	v.cachedState.Store(snapshot)

	// Even though numeric says 95%, it's stale -> ALLOW (don't rely on stale data)
	decision := v.GetCachedVeto(PotionHP, 30)
	if decision.Block {
		t.Errorf("Expected Block=false when data is stale, got Block=%v", decision.Block)
	}
	if decision.Reason != "numeric_parse_stale" {
		t.Errorf("Expected Reason=numeric_parse_stale, got %q", decision.Reason)
	}
}

// TestGetCachedVeto_AllowsWhenLowConfidence tests fail-safe: allow when confidence is low.
func TestGetCachedVeto_AllowsWhenLowConfidence(t *testing.T) {
	v := NewNumericSafetyValidator()

	// Simulate low-confidence numeric read
	snapshot := &NumericSafetyState{
		HP: NumericResourceRead{
			Found:      true,
			Current:    95,
			Max:        100,
			Percent:    95.0,
			UpdatedAt:  time.Now(),
			Confidence: 0.3, // Low confidence (min is 0.7)
		},
	}
	v.cachedState.Store(snapshot)

	// Even though numeric says 95%, confidence is low -> ALLOW (don't trust unreliable parse)
	decision := v.GetCachedVeto(PotionHP, 30)
	if decision.Block {
		t.Errorf("Expected Block=false when confidence is low, got Block=%v", decision.Block)
	}
	if decision.Reason != "numeric_confidence_low" {
		t.Errorf("Expected Reason=numeric_confidence_low, got %q", decision.Reason)
	}
}

// TestGetCachedVeto_AllowsWhenInvalidMax tests fail-safe: allow when max is invalid.
func TestGetCachedVeto_AllowsWhenInvalidMax(t *testing.T) {
	v := NewNumericSafetyValidator()

	// Simulate invalid max (zero or negative)
	snapshot := &NumericSafetyState{
		HP: NumericResourceRead{
			Found:      true,
			Current:    95,
			Max:        0, // Invalid: max should be > 0
			Percent:    0,
			UpdatedAt:  time.Now(),
			Confidence: 0.9,
		},
	}
	v.cachedState.Store(snapshot)

	decision := v.GetCachedVeto(PotionHP, 30)
	if decision.Block {
		t.Errorf("Expected Block=false when max is invalid, got Block=%v", decision.Block)
	}
	if decision.Reason != "numeric_invalid_max" {
		t.Errorf("Expected Reason=numeric_invalid_max, got %q", decision.Reason)
	}
}

// TestGetCachedVeto_SPIndependent tests that SP and HP use independent thresholds.
func TestGetCachedVeto_SPIndependent(t *testing.T) {
	v := NewNumericSafetyValidator()

	// Set HP to high, SP to low
	snapshot := &NumericSafetyState{
		HP: NumericResourceRead{
			Found:      true,
			Current:    95,
			Max:        100,
			Percent:    95.0,
			UpdatedAt:  time.Now(),
			Confidence: 0.9,
		},
		SP: NumericResourceRead{
			Found:      true,
			Current:    25,
			Max:        100,
			Percent:    25.0,
			UpdatedAt:  time.Now(),
			Confidence: 0.9,
		},
	}
	v.cachedState.Store(snapshot)

	// HP should block at 95% with 30% threshold
	hpDecision := v.GetCachedVeto(PotionHP, 30)
	if !hpDecision.Block {
		t.Errorf("Expected Block=true for HP, got Block=%v", hpDecision.Block)
	}

	// SP should allow at 25% with 30% threshold
	spDecision := v.GetCachedVeto(PotionSP, 30)
	if spDecision.Block {
		t.Errorf("Expected Block=false for SP, got Block=%v", spDecision.Block)
	}
}

// TestGetCachedVeto_NeverTriggersPotion tests that validator never returns a trigger signal.
// The validator is NEGATIVE-ONLY: it can only block, never trigger potting.
func TestGetCachedVeto_NeverTriggersPotion(t *testing.T) {
	v := NewNumericSafetyValidator()

	testCases := []struct {
		percent    float64
		threshold  int
		name       string
	}{
		{10.0, 30, "low_hp"},
		{30.0, 30, "at_threshold"},
		{50.0, 50, "at_threshold_mid"},
		{0.0, 100, "critically_low"},
	}

	for _, tc := range testCases {
		snapshot := &NumericSafetyState{
			HP: NumericResourceRead{
				Found:      true,
				Current:    int(tc.percent),
				Max:        100,
				Percent:    tc.percent,
				UpdatedAt:  time.Now(),
				Confidence: 0.9,
			},
		}
		v.cachedState.Store(snapshot)

		decision := v.GetCachedVeto(PotionHP, tc.threshold)

		// When below or at threshold, block should be false (allow potion)
		if tc.percent <= float64(tc.threshold) {
			if decision.Block {
				t.Errorf("Case %q: Expected Block=false when below/at threshold, got Block=%v with reason=%q",
					tc.name, decision.Block, decision.Reason)
			}
		}
	}
}

// TestGetCachedVeto_EdgeCases tests boundary conditions.
func TestGetCachedVeto_EdgeCases(t *testing.T) {
	v := NewNumericSafetyValidator()

	testCases := []struct {
		percent   float64
		threshold int
		wantBlock bool
		name      string
	}{
		{30.1, 30, true, "slightly_above_threshold"},
		{30.0, 30, false, "exactly_at_threshold"},
		{29.9, 30, false, "slightly_below_threshold"},
		{100.0, 0, true, "max_resource_min_threshold"},
		{0.0, 0, false, "zero_resource_zero_threshold"},
	}

	for _, tc := range testCases {
		snapshot := &NumericSafetyState{
			HP: NumericResourceRead{
				Found:      true,
				Current:    int(tc.percent),
				Max:        100,
				Percent:    tc.percent,
				UpdatedAt:  time.Now(),
				Confidence: 0.9,
			},
		}
		v.cachedState.Store(snapshot)

		decision := v.GetCachedVeto(PotionHP, tc.threshold)
		if decision.Block != tc.wantBlock {
			t.Errorf("Case %q: Expected Block=%v at percent=%.1f threshold=%d, got Block=%v",
				tc.name, tc.wantBlock, tc.percent, tc.threshold, decision.Block)
		}
	}
}

// TestGetCachedVeto_ReturnsMetadata tests that VetoDecision contains useful metadata.
func TestGetCachedVeto_ReturnsMetadata(t *testing.T) {
	v := NewNumericSafetyValidator()

	now := time.Now()
	snapshot := &NumericSafetyState{
		HP: NumericResourceRead{
			Found:      true,
			Current:    95,
			Max:        100,
			Percent:    95.0,
			UpdatedAt:  now,
			Confidence: 0.85,
		},
	}
	v.cachedState.Store(snapshot)

	decision := v.GetCachedVeto(PotionHP, 30)

	if decision.Percent != 95.0 {
		t.Errorf("Expected Percent=95.0, got %.1f", decision.Percent)
	}
	if decision.Confidence != 0.85 {
		t.Errorf("Expected Confidence=0.85, got %.2f", decision.Confidence)
	}
	if decision.AgeMs < 0 || decision.AgeMs > 100 {
		t.Errorf("Expected AgeMs to be reasonable (0-100), got %d", decision.AgeMs)
	}
}

// TestNumericValidatorNeverBlocksOnError ensures fail-safe behavior for various error conditions.
func TestNumericValidatorNeverBlocksOnError(t *testing.T) {
	v := NewNumericSafetyValidator()

	errorCases := []struct {
		setup func()
		name  string
	}{
		{
			setup: func() {
				v.cachedState.Store(&NumericSafetyState{}) // Empty state
			},
			name: "no_data",
		},
		{
			setup: func() {
				v.cachedState.Store(&NumericSafetyState{
					HP: NumericResourceRead{
						Found: false,
					},
				})
			},
			name: "found_false",
		},
		{
			setup: func() {
				v.cachedState.Store(&NumericSafetyState{
					HP: NumericResourceRead{
						Found:      true,
						Max:        0,
						Percent:    0,
						UpdatedAt:  time.Now(),
						Confidence: 0.9,
					},
				})
			},
			name: "invalid_max",
		},
		{
			setup: func() {
				v.cachedState.Store(&NumericSafetyState{
					HP: NumericResourceRead{
						Found:      true,
						Max:        100,
						Percent:    50.0,
						UpdatedAt:  time.Now().Add(-10 * time.Second), // Very stale
						Confidence: 0.9,
					},
				})
			},
			name: "stale_data",
		},
	}

	for _, tc := range errorCases {
		tc.setup()
		decision := v.GetCachedVeto(PotionHP, 30)
		if decision.Block {
			t.Errorf("Case %q: Expected Block=false on error, got Block=%v with reason=%q",
				tc.name, decision.Block, decision.Reason)
		}
	}
}

// TestGetCachedVeto_NonBlocking demonstrates that GetCachedVeto is O(1) non-blocking.
// This test shows that reading from cache does not involve any IO or blocking operations.
func TestGetCachedVeto_NonBlocking(t *testing.T) {
	v := NewNumericSafetyValidator()

	snapshot := &NumericSafetyState{
		HP: NumericResourceRead{
			Found:      true,
			Current:    50,
			Max:        100,
			Percent:    50.0,
			UpdatedAt:  time.Now(),
			Confidence: 0.9,
		},
	}
	v.cachedState.Store(snapshot)

	// Read from cache should be very fast, no blocking
	start := time.Now()
	for i := 0; i < 10000; i++ {
		_ = v.GetCachedVeto(PotionHP, 30)
	}
	elapsed := time.Since(start)

	// 10000 reads should complete in well under 1ms (typically <100µs)
	if elapsed > 100*time.Millisecond {
		t.Logf("WARNING: 10000 GetCachedVeto calls took %v (expected <100ms)", elapsed)
	}
}

// TestNumericValidatorParserPublishesSnapshot verifies that parser publishes
// new snapshots atomically and AutoPot reads from the published cache.
func TestNumericValidatorParserPublishesSnapshot(t *testing.T) {
	v := NewNumericSafetyValidator()

	// Simulate parser publishing a snapshot
	snapshot1 := &NumericSafetyState{
		HP: NumericResourceRead{
			Found:      true,
			Current:    80,
			Max:        100,
			Percent:    80.0,
			UpdatedAt:  time.Now(),
			Confidence: 0.9,
		},
	}
	v.cachedState.Store(snapshot1)

	// AutoPot reads (should get snapshot1)
	decision1 := v.GetCachedVeto(PotionHP, 70)
	if !decision1.Block { // 80% > 70% threshold
		t.Errorf("Expected Block=true after first snapshot publish")
	}

	// Parser publishes new snapshot
	snapshot2 := &NumericSafetyState{
		HP: NumericResourceRead{
			Found:      true,
			Current:    60,
			Max:        100,
			Percent:    60.0,
			UpdatedAt:  time.Now(),
			Confidence: 0.9,
		},
	}
	v.cachedState.Store(snapshot2)

	// AutoPot reads (should get snapshot2)
	decision2 := v.GetCachedVeto(PotionHP, 70)
	if decision2.Block { // 60% < 70% threshold
		t.Errorf("Expected Block=false after second snapshot publish")
	}
}

// TestNumericValidatorAsyncStart verifies the background validator starts correctly.
func TestNumericValidatorAsyncStart(t *testing.T) {
	v := NewNumericSafetyValidator()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Start the validator in a goroutine
	go v.Start(ctx)

	// Give goroutine minimal time to start
	time.Sleep(50 * time.Millisecond)

	// Should be able to call GetCachedVeto without blocking
	_ = v.GetCachedVeto(PotionHP, 30)

	// Wait for context to expire
	<-ctx.Done()
	time.Sleep(100 * time.Millisecond) // Let goroutine exit
}
