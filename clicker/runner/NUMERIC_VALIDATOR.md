# Numeric HP/SP Safety Validator

## Overview

The Numeric Safety Validator is a parallel, non-blocking safety mechanism that prevents false positive potions triggered by the bar detector.

### Key Principle
**The numeric validator ONLY blocks potting. It NEVER triggers potting.**

The bar detector remains the sole source of truth for potion decisions. The numeric validator provides an additional safety gate that prevents potting when it confidently knows HP/SP is safe.

## Architecture

### Fail-Safe Design
If numeric parsing fails, is stale, or has low confidence, the validator returns `false` (don't block), allowing the bar detector to make the decision.

This ensures numeric validation never causes death due to OCR failure.

### Thread-Safe Operation
- Runs in a separate background goroutine
- Atomic state updates via mutex
- Non-blocking reads via `State()`
- No synchronous waits in the hot path

## Data Structures

```go
type NumericResourceRead struct {
    Found      bool
    Current    int
    Max        int
    Percent    float64
    UpdatedAt  time.Time
    Confidence float64 // 0.0 to 1.0
}

type NumericSafetyState struct {
    HP NumericResourceRead
    SP NumericResourceRead
}
```

## Public API

```go
func (v *NumericSafetyValidator) Start(ctx context.Context)
func (v *NumericSafetyValidator) State() NumericSafetyState
func (v *NumericSafetyValidator) ShouldBlockHP(threshold int) bool
func (v *NumericSafetyValidator) ShouldBlockSP(threshold int) bool
```

## Blocking Logic

Returns `true` (block potting) ONLY if ALL conditions are met:

1. **Data Found**: Numeric parse was successful
2. **Data Fresh**: Within 2 seconds old
3. **Confidence High**: >= 0.7
4. **Value Above Threshold**: `numeric_percent >= (threshold + safetyMargin)`

Otherwise returns `false` (don't block, let bar detector decide).

### Safety Margin
- Default: 4%
- Example with threshold=70:
  - Numeric=74% → **BLOCK** (74 >= 70+4)
  - Numeric=73% → **ALLOW** (73 < 70+4)
  - Parse fails → **ALLOW** (fail-safe)
  - Data stale → **ALLOW** (fail-safe)
  - Low confidence → **ALLOW** (fail-safe)

## Integration Points

### Startup
```go
a.numericValidator = NewNumericSafetyValidator()
a.numericValidator.SetLogFunc(cfg.Log)
a.numericValidator.Start(ctx)
```

### Before Potting (healUntil)
```go
if hpBar && a.numericValidator.ShouldBlockHP(cfg.HPThreshold) {
    cfg.Log("HP pot blocked by numeric safety validator")
    return
}
if !hpBar && a.numericValidator.ShouldBlockSP(cfg.SPThreshold) {
    cfg.Log("SP pot blocked by numeric safety validator")
    return
}
```

## Performance Characteristics

- **Poll Interval**: 500-1500ms (configurable, default 750ms)
- **Background**: Does not block main potion loop
- **Memory**: Small fixed overhead (~500 bytes)
- **Latency**: None (non-blocking reads)

## OCR Implementation

Current placeholder in `extractNumericHPSP()`:

```go
func extractNumericHPSP(img image.Image, isHP bool) (current, max int, confidence float64, ok bool) {
    // TODO: Implement with tesseract or similar OCR library
    // Parse numeric text from image: "current / max"
    // Return confidence based on OCR quality
    return 0, 0, 0, false
}
```

Future implementation should:
- Capture status window ROI
- Extract numeric text (e.g., "542 / 800")
- Parse current/max values
- Return confidence score based on character recognition quality

## Test Coverage

✅ Blocks when numeric is above threshold + margin  
✅ Doesn't block when numeric is below threshold + margin  
✅ Doesn't block when parse failed (Found=false)  
✅ Doesn't block when data is stale  
✅ Doesn't block when confidence is low  
✅ Never triggers potting (only blocks)  
✅ HP and SP blocking work independently  
✅ Thread-safe state copying  
✅ Edge cases at exactly threshold boundaries  
✅ Staleness boundary conditions  

## Configuration

```go
// Set custom poll interval
v.SetPollInterval(1000 * time.Millisecond)

// Set custom safety margin (percentage points)
v.SetSafetyMargin(5)

// Set custom log function
v.SetLogFunc(myLogFn)
```

Default configuration is conservative and fail-safe:
- Poll every 750ms
- Accept data up to 2 seconds old
- Require confidence >= 0.7
- Safety margin of 4%

## Logging

Blocks are logged when they occur:
```
"HP pot blocked by numeric safety validator"
"SP pot blocked by numeric safety validator"
```

Debug information available via `State()`:
```go
state := validator.State()
fmt.Printf("HP: %d/%d (%.0f%%) confidence=%.1f\n", 
    state.HP.Current, state.HP.Max, state.HP.Percent, state.HP.Confidence)
```

## Known Limitations

1. **OCR Placeholder**: Current implementation returns `Found=false`
2. **ROI Hardcoded**: Assumes specific status window location
3. **No Digit Recognition**: Actual OCR library needed
4. **No Error Logging**: Parse failures are silent (by design)

## Future Enhancements

1. Implement real OCR with tesseract
2. Configurable ROI for different resolutions
3. Multiple parsing strategies (digit recognition, pixel matching)
4. Historical tracking (confirm trends)
5. Adaptive confidence thresholds

## Design Decisions

### Why Separate Goroutine?
- Parsing may be slow
- No need to block potion decisions
- Can run independently at different cadence

### Why Fail-Safe?
- Numeric parsing can fail or produce wrong results
- Never want to skip a potion due to OCR error
- Bar detector is trusted, numeric is advisory

### Why Atomic State?
- Safe concurrent reads from multiple goroutines
- No locks in the hot potting path
- Minimal contention

### Why Safety Margin?
- Accounts for numeric/bar detector drift
- Reduces false blocks from marginal cases
- Configurable for different risk profiles
