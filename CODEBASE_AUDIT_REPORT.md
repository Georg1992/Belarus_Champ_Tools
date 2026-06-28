# Comprehensive Codebase Audit Report
**Project:** Experimental Clicker (Belarus Champ Clicker)  
**Audit Date:** 2026-06-28  
**Scope:** `/clicker/gui/` and `/clicker/runner/` directories  

---

## Executive Summary
This audit identified **32+ issues** across the codebase spanning dead code, debug code, error handling, architectural concerns, and code quality improvements. The issues are categorized by severity and type.

---

## 1. DEBUG CODE & CONDITIONAL COMPILATION

### 1.1 Unused Debug Output Flag in AutoPot Runner
**File:** [runner/autopot.go](runner/autopot.go)  
**Lines:** 134, 157-162  
**Severity:** MEDIUM  
**Issue:** Debug mode flag `BAR_SEARCH_DEBUG` enables bar search visualization to disk, but this appears to be experimental/development code that should be cleaned up.

```go
debugSave := os.Getenv("BAR_SEARCH_DEBUG") != ""  // Line 134

if debugSave {
    bars, err := RefreshBarPair(img)
    if err == nil {
        hp, sp := ReadMappedBars(img, bars)
        _ = SaveMappedBarsDebug(img, bars, "bar_search_debug.png")
        cfg.Log(FormatMappedBarsLog(img, bars, hp, sp, true))
    }
}
```

**Recommendation:** Either make this a proper feature with settings, or remove it entirely. Debug code should not be in production paths.

---

### 1.2 Debug Visualization Functions Unused in Normal Flow
**File:** [runner/player_bars.go](runner/player_bars.go)  
**Lines:** 968-1180  
**Functions:** 
- `barPairDebugScan` (line 968)
- `scanBarPairDebug()` (line 976)
- `SaveMappedBarsDebug()` (line 1034)
- `drawCross()` (line 1060)
- `drawRunOutline()` (line 1090)
- `drawRectOutline()` (line 1098)
- `drawBarRectDebug()` (line 1108)
- `imageToRGBA()` (line 1158)

**Severity:** LOW  
**Issue:** Multiple debug visualization functions only called when `BAR_SEARCH_DEBUG` environment variable is set. These ~200 lines of code are essentially dead unless debugging.

**Recommendation:** Consider moving these to a separate `debug.go` file or a build tag like `// +build debug`.

---

## 2. ERROR HANDLING ISSUES

### 2.1 Ignored Error Returns (Underscore Assignments)
**File:** [gui/main.go](gui/main.go)  
**Lines:** 246, 248  
**Severity:** LOW  
**Issue:** UI update errors are silently ignored:

```go
_ = a.logList.SetModel(a.logItems)  // Line 246
_ = a.logList.SetCurrentIndex(len(a.logItems) - 1)  // Line 248
```

**Recommendation:** Log these errors or handle gracefully:
```go
if err := a.logList.SetModel(a.logItems); err != nil {
    // Handle error
}
```

### 2.2 Process Cleanup Errors Ignored
**File:** [gui/server.go](gui/server.go)  
**Lines:** 78, 114, 123, 130, 140  
**Severity:** LOW  
**Issue:** Multiple error returns from process cleanup operations are ignored:

```go
_, _ = cmd.Process.Wait()  // Lines 78, 114
_ = exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()  // Line 123
_ = os.RemoveAll(dir)  // Lines 130, 140
```

**Recommendation:** While these are cleanup operations, consider at minimum logging them in debug mode.

### 2.3 Ignored Brush Creation Error
**File:** [gui/keychain_arrows.go](gui/keychain_arrows.go)  
**Line:** 30  
**Severity:** LOW  
**Issue:**

```go
keyChainSurfaceOnce.Do(func() {
    keyChainSurfaceBrush, _ = walk.NewSystemColorBrush(walk.SysColorBtnFace)
})
```

**Recommendation:** At minimum, check the error in development. This is a one-time initialization, so it might be acceptable but is not ideal practice.

---

## 3. UNUSED IMPORTS & BLANK IMPORTS

### 3.1 Blank Import in autopot.go
**File:** [runner/autopot.go](runner/autopot.go)  
**Line:** 6  
**Severity:** LOW  
**Issue:** 
```go
import (
    "context"
    "fmt"
    "os"  // Used only for Getenv() which is only called in debug code
    "sync"
)
```

**Analysis:** The `os` package is only used for `os.Getenv("BAR_SEARCH_DEBUG")` which is conditional debug code. If debug code is removed, this import becomes unused.

**Recommendation:** When refactoring debug code, remove or conditionally compile this import.

---

## 4. MAGIC NUMBERS MISSING CONSTANTS

### 4.1 Magic Numbers in player_bars.go Color Detection
**File:** [runner/player_bars.go](runner/player_bars.go)  
**Lines:** Throughout (examples: 199, 491, 555, 562, 563, 600, 618, 832)  
**Severity:** MEDIUM  
**Issue:** Many hardcoded numeric thresholds for color detection should be named constants:

```go
// Line 199
return gi >= 35 && ri >= 50 && absInt(ri-gi) < 25;

// Line 491
if absInt(hpRun.X1-spRun.X1) > 6 {

// Line 555
centerDist := absInt(midX-cx)*3 + absInt(midY-cy)*4;

// Line 562
gapPenalty := absInt((sp.Y-hp.Y)-expectedBarGap) * 8;

// Line 600
return r.Width*3 - absInt(mx-cx)*3 - absInt(r.Y-cy)*2;

// Line 618
return absInt(mx-cx) + absInt(r.Y-cy);
```

**Examples of constants that should be defined:**
```go
const (
    HPGreenMinIntensity = 35
    HPRedMinIntensity = 50
    HPRedGreenMaxDiff = 25
    BarXAlignmentTolerance = 6
    CenterDistXWeight = 3
    CenterDistYWeight = 4
    GapPenaltyWeight = 8
    BarRunWidthWeight = 3
    BarRunXDistWeight = 3
    BarRunYDistWeight = 2
)
```

**Recommendation:** Replace all magic numbers with named constants for maintainability.

### 4.2 Magic Numbers in player_bars.go Color Value Thresholds
**File:** [runner/player_bars.go](runner/player_bars.go)  
**Constants already defined but many magic numbers still exist:**

Examples needing extraction:
- Line 199: `35`, `50`, `25` (HP color thresholds)
- Line 818: `60`, `210` (color sum bounds)
- Line 847: `130`, `25` (Red channel thresholds)
- Line 874: `110`, `90`, `90`, `15`, `10` (Yellow detection)
- Line 899: `90`, `10`, `18` (Blue detection)
- Line 912: `130`, `12`, `20`, `20` (Fill detection)
- Line 938: `80`, `60`, `10`, `5` (Cyan detection)

**Recommendation:** Create a constants section specifically for color thresholds.

### 4.3 Magic Number in keychain_arrows.go
**File:** [gui/keychain_arrows.go](gui/keychain_arrows.go)  
**Line:** 50, 56  
**Severity:** LOW  
**Issue:**
```go
size := 3.5  // Line 50 - Arrow head size
// And arrow drawing calculations with hardcoded offsets
for dx := -6; dx <= 6; dx++ {  // Line 57 - Cross size
for dy := -6; dy <= 6; dy++ {  // Line 63
```

**Recommendation:** Extract to named constants.

---

## 5. ARCHITECTURAL CONCERNS

### 5.1 Mixed Concerns in Main Window Creation
**File:** [gui/main.go](gui/main.go)  
**Lines:** 88-240  
**Severity:** MEDIUM  
**Issue:** The `createWindow()` function does multiple things:
1. Creates main window
2. Sets title, size, icon
3. Builds all tabs (Clicker, AutoPot, KeyChain)
4. Creates log panel
5. Wires event handlers
6. Manages window lifecycle

This violates Single Responsibility Principle. The function is ~150 lines and handles window, UI building, and lifecycle.

**Recommendation:** Break into smaller functions:
- `setupMainWindow()`
- `buildUITabs()`
- `setupLogPanel()`
- `wireEventHandlers()`

### 5.2 VIIPER Server Management is Overly Complex
**File:** [gui/server.go](gui/server.go)  
**Lines:** 28-140  
**Severity:** MEDIUM  
**Issue:** The server management uses global mutable state with mutex:
```go
var (
    serverMu    sync.Mutex
    serverCmd   *exec.Cmd
    serverStarted bool
    serverPID   int
    viiperTempDir string
)
```

This is thread-safe but global state management makes testing difficult and state tracking fragile.

**Recommendation:** Encapsulate in a `ViiperServerManager` struct with proper lifecycle management.

### 5.3 Inconsistent Error Handling Between Runners
**File:** [runner/runner.go](runner/runner.go), [runner/autopot.go](runner/autopot.go), [runner/keychain.go](runner/keychain.go), [runner/timer_key.go](runner/timer_key.go)  
**Issue:** Different runners have slightly different error validation patterns:

**In Runner.Start()** (runner.go:71-78):
```go
if r.running {
    r.mu.Unlock()
    return fmt.Errorf("clicker already running")
}
if r.cfg.Session == nil {
    r.mu.Unlock()
    return fmt.Errorf("input session is required")
}
```

**In AutoPotRunner.Start()** (autopot.go:60-80):
```go
if a.running {
    a.mu.Unlock()
    return fmt.Errorf("autopot already running")
}
cfg := a.settings()
if cfg.HPEnabled && cfg.HPKeyVK == 0 {
    a.mu.Unlock()
    return fmt.Errorf("HP potion key is not set")
}
// ... repeated unlocks
```

**Recommendation:** Create a base runner interface or helper function for consistent error handling.

### 5.4 Duplicate Pause/Resume Logic
**File:** [runner/viiper_session.go](runner/viiper_session.go)  
**Lines:** 127-150  
**Severity:** LOW  
**Issue:** Pause detection logic is hardcoded in `ViiperSession`. If multiple runners exist, they all use the same pause watcher. This is actually good design, but the pause key is defined globally as a constant in runner.go which couples the session to the runner business logic.

**Recommendation:** Consider making pause key configurable.

---

## 6. UNUSED/UNTESTED CODE PATHS

### 6.1 Unused Test Fixtures (Partial)
**File:** [runner/screen_fixtures_test.go](runner/screen_fixtures_test.go)  
**Lines:** 24-39  
**Severity:** LOW  
**Issue:** Test fixtures are defined but some may not be exercised by all test cases:
```go
testFixtures := map[string]barTestFixture{
    "aa.png":       {...},
    "gg.png":       {...},
    // ... 17 more fixtures
}
```

**Recommendation:** Add a test that verifies all fixtures are tested or document which are intentionally untested.

### 6.2 Unused Bar struct fields
**File:** [runner/player_bars.go](runner/player_bars.go)  
**Line:** 76  
**Severity:** LOW  
**Issue:** The `Bar` struct has `Left` and `Right` fields:
```go
type Bar struct {
    Left, Right int  // Computed from X and W, only used in debug
    Y           int
    Width       int
    Height      int
    FilledWidth int
    Percent     float64
    Found       bool
}
```

These are only computed in `barFromRead()` and used in debug visualization. Not needed for production logic.

**Recommendation:** Move `Bar` struct into debug code or document why it's in main.

---

## 7. STALE/EXPERIMENTAL CODE

### 7.1 Experimental Bar Pair Search Debug
**File:** [runner/player_bars.go](runner/player_bars.go)  
**Function:** `scanBarPairDebug()` (line 976)  
**Severity:** LOW  
**Issue:** This function and related debug visualization appears to be development tooling that somehow made it into production code:
```go
type barPairDebugScan struct {
    screenCX, screenCY int
    roi                Rect
    hpRuns, spRuns     []ColorRun
    hpRun, spRun       ColorRun
    pairCX, pairCY     int
}
```

**Recommendation:** Move to separate file or ensure it's only compiled with debug tag.

---

## 8. CODE QUALITY ISSUES

### 8.1 Inconsistent Error Variable Naming
**Files:** Multiple runner files  
**Issue:** Error variables use different patterns:
- Most use `err` 
- Some use `_, err` (ignoring one return value)
- Some use `_, _ = ...` (ignoring both)

**Recommendation:** Standardize error handling patterns across codebase.

### 8.2 String Concatenation Instead of fmt
**File:** [runner/player_bars.go](runner/player_bars.go)  
**Lines:** Throughout (examples: 1007-1021)  
**Severity:** LOW  
**Issue:** Heavy use of string concatenation for logging:
```go
return "driftROI x=" + itoa(scan.roi.X) + " y=" + itoa(scan.roi.Y) +
    " w=" + itoa(scan.roi.W) + " h=" + itoa(scan.roi.H) + "\n" +
    "screenCenter x=" + itoa(scan.screenCX) + " y=" + itoa(scan.screenCY) + "\n" +
    // ... 10+ more lines
```

**Recommendation:** Use `fmt.Sprintf()` or `strings.Builder` for better readability:
```go
return fmt.Sprintf(
    "driftROI x=%d y=%d w=%d h=%d\nscreenCenter x=%d y=%d\n...",
    scan.roi.X, scan.roi.Y, scan.roi.W, scan.roi.H,
    scan.screenCX, scan.screenCY, ...)
```

### 8.3 Helper Functions at End of File
**File:** [runner/player_bars.go](runner/player_bars.go)  
**Lines:** 1125-1200  
**Severity:** LOW  
**Issue:** Utility functions are scattered at the end:
```go
func absInt(v int) int { ... }
func itoa(v int) string { ... }
func ftoa(v float64) string { ... }
func imageToRGBA(img image.Image) *image.RGBA { ... }
```

These should be in a separate `util.go` file or marked as internal helpers.

**Recommendation:** Create `player_bars_util.go` or similar for helper functions.

---

## 9. POTENTIAL UNUSED CONSTANTS

### 9.1 Color Detection Constants - Usage Analysis
**File:** [runner/player_bars.go](runner/player_bars.go)  
**Lines:** 12-40  
**Severity:** LOW  
**Issue:** Some constants defined but used inconsistently. For example:
- `barBgTol = 28` defined but also `8` used inline at line 820
- Multiple `*Col*` constants for RGB colors that are duplicated as magic numbers

**Recommendation:** Audit all constant definitions and usage to ensure no duplication.

---

## 10. DUPLICATE IMPLEMENTATIONS

### 10.1 Similar Bar Reading Logic (HP vs SP)
**File:** [runner/player_bars.go](runner/player_bars.go)  
**Lines:** 173-230  
**Severity:** LOW  
**Issue:** `ReadHPFill()` and `ReadSPFill()` have similar structures but different implementations:

```go
func ReadHPFill(img image.Image, hp Rect) BarRead { ... }  // 63 lines
func ReadSPFill(img image.Image, sp Rect) BarRead { ... }  // 45 lines
```

The functions could potentially be consolidated with a parameter specifying HP vs SP, though the different logic may be intentional based on bar appearance differences.

**Recommendation:** Document why HP and SP reading differs, or consolidate if possible.

---

## 11. MISSING DOCUMENTATION

### 11.1 Complex Algorithms Without Comments
**File:** [runner/player_bars.go](runner/player_bars.go)  
**Functions with complex logic but minimal documentation:**
- `scoreBarPair()` (line 554) - Complex scoring algorithm
- `findPlayerBarPair()` (line 469) - Bar pair detection algorithm
- `deriveBarRects()` (line 648) - Rectangle derivation logic

**Severity:** MEDIUM  
**Recommendation:** Add doc comments explaining the algorithm rationale.

---

## SUMMARY BY CATEGORY

| Category | Count | Severity |
|----------|-------|----------|
| Debug Code | 8 | MEDIUM |
| Error Handling | 5 | LOW |
| Magic Numbers | 25+ | MEDIUM |
| Architectural Issues | 4 | MEDIUM |
| Code Quality | 4 | LOW |
| Unused Code | 3 | LOW |
| Documentation | 3 | MEDIUM |
| **TOTAL** | **52+** | Mixed |

---

## RECOMMENDATIONS PRIORITY

### HIGH PRIORITY (Address First)
1. Extract magic numbers to named constants
2. Move debug code to separate file or conditional compilation
3. Consolidate error handling patterns
4. Break down large functions (e.g., `createWindow()`)

### MEDIUM PRIORITY
1. Improve error logging (don't use `_ =`)
2. Add documentation for complex algorithms
3. Fix architectural issues (VIIPER server management)
4. Refactor string concatenation to `fmt.Sprintf()`

### LOW PRIORITY
1. Move utility functions to separate files
2. Standardize variable naming
3. Add comments to helper functions
4. Document test fixture coverage

---

## CLEANUP CHECKLIST

- [ ] Move debug code to `debug.go` or remove entirely
- [ ] Extract all magic numbers to named constants
- [ ] Add error logging for all `_ =` assignments
- [ ] Refactor `createWindow()` into smaller functions
- [ ] Consolidate runner error handling patterns
- [ ] Add doc comments to complex functions
- [ ] Move helper functions to `_util.go` files
- [ ] Replace string concatenation with `fmt.Sprintf()`
- [ ] Create `ViiperServerManager` struct
- [ ] Add build tags for debug code

---

**Report Generated:** 2026-06-28  
**Total Issues Found:** 52+  
**Estimated Cleanup Time:** 4-6 hours
