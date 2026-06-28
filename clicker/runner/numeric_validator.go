package runner

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// PotionKind represents the type of potion (HP or SP).
type PotionKind int

const (
	PotionHP PotionKind = iota
	PotionSP
)

// VetoDecision is returned by GetCachedVeto to indicate whether to block a potion press.
// This is a fast, non-blocking read from the cached validator state.
type VetoDecision struct {
	Block      bool    // true = skip potion press (resource is safe), false = allow potion press
	Reason     string  // explanation of the decision
	Percent    float64 // numeric resource percentage (0-100), or 0 if not available
	Confidence float64 // parse confidence (0.0-1.0), or 0 if not available
	AgeMs      int64   // age of the numeric read in milliseconds
}

// NumericResourceRead holds the result of parsing numeric HP/SP from the status window.
type NumericResourceRead struct {
	Found      bool
	Current    int
	Max        int
	Percent    float64
	UpdatedAt  time.Time
	Confidence float64 // 0.0 to 1.0
}

// IsStale returns true if the read is older than the given duration.
func (r *NumericResourceRead) IsStale(maxAge time.Duration) bool {
	return time.Since(r.UpdatedAt) > maxAge
}

// Age returns the age of this read in milliseconds.
func (r *NumericResourceRead) Age() int64 {
	return int64(time.Since(r.UpdatedAt).Milliseconds())
}

// NumericSafetyState holds the latest validated numeric HP and SP reads.
// This struct is immutable after publication to atomic.Value.
type NumericSafetyState struct {
	HP NumericResourceRead
	SP NumericResourceRead
}

// NumericSafetyValidator runs in a background goroutine to validate HP/SP
// by periodically parsing numeric text from the status window.
// All parsing and capture happens asynchronously, published via atomic.Value.
// AutoPot reads the latest cached state via GetCachedVeto() with zero cost.
type NumericSafetyValidator struct {
	// Immutable configuration (set at creation)
	pollInterval  time.Duration
	maxStateAge   time.Duration
	minConfidence float64

	// Atomic cache: stores *NumericSafetyState
	// Parser publishes new snapshots here; AutoPot reads from here
	cachedState atomic.Value // *NumericSafetyState

	// Logging and config access (locked only for config changes, not hot path)
	mu  sync.RWMutex
	log func(string)
}

// NewNumericSafetyValidator creates a new numeric validator.
func NewNumericSafetyValidator() *NumericSafetyValidator {
	v := &NumericSafetyValidator{
		pollInterval:  750 * time.Millisecond,
		maxStateAge:   2 * time.Second,
		minConfidence: 0.7,
		log:           func(string) {},
	}
	// Initialize with empty state
	v.cachedState.Store(&NumericSafetyState{})
	return v
}

// SetLogFunc sets the logging function.
func (v *NumericSafetyValidator) SetLogFunc(fn func(string)) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.log = fn
}

// SetPollInterval sets how often to capture and parse numeric data.
func (v *NumericSafetyValidator) SetPollInterval(d time.Duration) {
	// Update affects next cycle only; current parse is not interrupted
	v.pollInterval = d
}

// SetMinConfidence sets the minimum confidence required to block a potion.
func (v *NumericSafetyValidator) SetMinConfidence(conf float64) {
	// Update affects next parse only
	v.minConfidence = conf
}

// GetCachedVeto reads the latest published numeric state and determines veto.
// This is the main entry point called by AutoPot before each potion keypress.
// O(1), non-blocking, fail-open: returns veto decision from atomic cache.
// Returns Block=true ONLY if numeric validator cached data is confident
// that the resource is above the threshold. Otherwise returns Block=false (fail-safe).
func (v *NumericSafetyValidator) GetCachedVeto(kind PotionKind, thresholdPercent int) VetoDecision {
	// Read cached state (atomic, non-blocking, O(1))
	cachedState := v.cachedState.Load().(*NumericSafetyState)

	var read NumericResourceRead
	if kind == PotionHP {
		read = cachedState.HP
	} else {
		read = cachedState.SP
	}

	// Fail-safe: no parse data means allow potting
	if !read.Found {
		return VetoDecision{
			Block:  false,
			Reason: "numeric_parse_not_found",
			AgeMs:  read.Age(),
		}
	}

	// Fail-safe: stale data means allow potting
	if read.IsStale(v.maxStateAge) {
		return VetoDecision{
			Block:      false,
			Reason:     "numeric_parse_stale",
			Percent:    read.Percent,
			Confidence: read.Confidence,
			AgeMs:      read.Age(),
		}
	}

	// Fail-safe: low confidence means allow potting
	if read.Confidence < v.minConfidence {
		return VetoDecision{
			Block:      false,
			Reason:     "numeric_confidence_low",
			Percent:    read.Percent,
			Confidence: read.Confidence,
			AgeMs:      read.Age(),
		}
	}

	// Fail-safe: invalid max value means allow potting
	if read.Max <= 0 {
		return VetoDecision{
			Block:      false,
			Reason:     "numeric_invalid_max",
			Percent:    read.Percent,
			Confidence: read.Confidence,
			AgeMs:      read.Age(),
		}
	}

	// Core veto decision:
	// Block ONLY if numeric percent is ABOVE threshold (resource is safe, don't need potion)
	if read.Percent > float64(thresholdPercent) {
		return VetoDecision{
			Block:      true,
			Reason:     "numeric_above_threshold",
			Percent:    read.Percent,
			Confidence: read.Confidence,
			AgeMs:      read.Age(),
		}
	}

	// Resource is at or below threshold, allow autopot to pot normally
	return VetoDecision{
		Block:      false,
		Reason:     "numeric_below_threshold",
		Percent:    read.Percent,
		Confidence: read.Confidence,
		AgeMs:      read.Age(),
	}
}

// State returns a copy of the current cached numeric state.
// Note: This is a blocking read for diagnostics; AutoPot must use GetCachedVeto.
func (v *NumericSafetyValidator) State() NumericSafetyState {
	return *v.cachedState.Load().(*NumericSafetyState)
}

// Start launches the background numeric parsing goroutine.
// The goroutine runs independently, capturing and parsing as fast as possible.
func (v *NumericSafetyValidator) Start(ctx context.Context) {
	go v.run(ctx)
}

// run is the main loop that periodically captures and parses numeric HP/SP.
// It publishes new snapshots to cachedState via atomic.Store.
func (v *NumericSafetyValidator) run(ctx context.Context) {
	ticker := time.NewTicker(v.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			v.captureAndParse()
		}
	}
}

// captureAndParse captures the status window, parses numeric HP/SP, and publishes result.
// This method runs asynchronously and independently; no blocking on AutoPot side.
func (v *NumericSafetyValidator) captureAndParse() {
	// Capture screen for status window ROI
	img, _, err := CapturePlayerBarSearch()
	if err != nil {
		// Capture failed: publish empty state or keep existing cached state
		// For fail-safe: we either keep old cache (which will become stale)
		// or publish fresh empty state. Choosing to keep existing to avoid churn.
		return
	}

	// Parse numeric HP/SP from the status window using deterministic bitmap matching
	numRead, err := ParseNumericResources(img)
	if err != nil {
		// Parsing failed: publish empty state to allow fallback
		v.cachedState.Store(&NumericSafetyState{})
		return
	}

	// Build new snapshot locally before publishing
	snapshot := NumericSafetyState{HP: numRead.HP, SP: numRead.SP}

	// Update HP with timestamp
	if snapshot.HP.Found {
		snapshot.HP.UpdatedAt = time.Now()
		// Apply validator's confidence threshold
		if snapshot.HP.Confidence < v.minConfidence {
			snapshot.HP.Found = false
		}
	} else {
		snapshot.HP.UpdatedAt = time.Now()
	}

	// Update SP with timestamp
	if snapshot.SP.Found {
		snapshot.SP.UpdatedAt = time.Now()
		// Apply validator's confidence threshold
		if snapshot.SP.Confidence < v.minConfidence {
			snapshot.SP.Found = false
		}
	} else {
		snapshot.SP.UpdatedAt = time.Now()
	}

	// Atomically publish the new snapshot
	// AutoPot reads from this without waiting
	v.cachedState.Store(&snapshot)
}
