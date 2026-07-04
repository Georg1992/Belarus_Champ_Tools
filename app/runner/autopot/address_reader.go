package autopot

import (
	"context"
	"fmt"
	"time"

	win "belarus-champ-tools/runner/platform/windows"
	"belarus-champ-tools/runner/profiles"
)

// addressReader is a BarReader that reads HP/SP values from a game
// process's memory at the offsets defined by a server profile.
//
// Opens a fresh process handle on every ReadValues() call and closes it
// after all 4 values are read. This avoids the stale-handle problem that
// occurs when anti-cheat invalidates persistent handles.
//
// When reads fail persistently, the reader attempts to auto-reconnect by
// finding a window whose title matches processTitle and resolving its PID.
type addressReader struct {
	pid          uint32
	profile      profiles.Profile
	processTitle string // window title for auto-reconnect
	moduleBase   uintptr // base address of the exe in the target process

	liveConfig   func() AutoPotConfig
	onParsed     func(hp, hpMax, sp, spMax, stripX, stripY, stripW, stripH int)
	onModeChange func(string)

	lastLog  time.Time
	log      func(string)
	loggedFirstFail bool

	hadError      bool       // true when the last ReadValues returned an error
	lastReconn    time.Time  // last auto-reconnect attempt time
}

// Close is a no-op — there is no persistent handle to close. Handles are
// opened and closed inside each ReadValues call.
func (r *addressReader) Close() {}

// reconnectInterval is how long the reader waits before trying to find
// the process again after persistent read failures.
const reconnectInterval = 5 * time.Second

func (r *addressReader) Name() string { return "Address" }

func (r *addressReader) ReadValues(ctx context.Context) BarReadResult {
	if ctx.Err() != nil {
		return BarReadResult{Status: StatusInvalid, Err: ctx.Err()}
	}
	if r.pid == 0 {
		return BarReadResult{Status: StatusInvalid, Err: fmt.Errorf("address reader: no process selected (PID=0)")}
	}

	// Open a fresh handle every time — avoids stale-handle issues.
	h, err := win.OpenProcessHandle(r.pid)
	if err != nil {
		r.setError("address: OpenProcess(%d) failed: %v", r.pid, err)
		return BarReadResult{Status: StatusInvalid, Err: err}
	}
	defer win.CloseProcessHandle(h)

	// Profile stores module-relative offsets. Add the exe base address
	// to get absolute virtual addresses (ASLR-safe).
	base := r.moduleBase

	// Read HP values.
	curHP, err := win.ReadProcessUint32ByHandle(h, base+r.profile.CurrentHPAddr)
	if err != nil {
		r.setError("address: %v", err)
		return BarReadResult{Status: StatusInvalid, Err: err}
	}
	maxHP, err := win.ReadProcessUint32ByHandle(h, base+r.profile.MaxHPAddr)
	if err != nil {
		r.setError("address: %v", err)
		return BarReadResult{Status: StatusInvalid, Err: err}
	}

	// Read SP values.
	curSP, err := win.ReadProcessUint32ByHandle(h, base+r.profile.CurrentSPAddr)
	if err != nil {
		r.setError("address: %v", err)
		return BarReadResult{Status: StatusInvalid, Err: err}
	}
	maxSP, err := win.ReadProcessUint32ByHandle(h, base+r.profile.MaxSPAddr)
	if err != nil {
		r.setError("address: %v", err)
		return BarReadResult{Status: StatusInvalid, Err: err}
	}

	// All reads succeeded — clear error state and restore normal mode.
	if r.hadError {
		r.hadError = false
		if r.onModeChange != nil {
			r.onModeChange("Address reading")
		}
	}

	// Convert to percentages.
	hpPct := 0.0
	if maxHP > 0 {
		hpPct = float64(curHP) * 100.0 / float64(maxHP)
	}
	spPct := 0.0
	if maxSP > 0 {
		spPct = float64(curSP) * 100.0 / float64(maxSP)
	}

	// HP=1 means dead in Ragnarok Online.
	if curHP == 1 {
		return BarReadResult{
			HP:     hpPct,
			SP:     spPct,
			Status: StatusDead,
			Err:    fmt.Errorf("character dead (HP=1)"),
		}
	}

	// (Debug logging for HP/SP values removed — addresses confirmed working.)

	// Forward raw values to the overlay (same as OCR reader does).
	if r.onParsed != nil {
		r.onParsed(int(curHP), int(maxHP), int(curSP), int(maxSP), 0, 0, 0, 0)
	}

	// Compute HPLow/SPLow against the current thresholds from live config.
	hpLow := false
	spLow := false
	if r.liveConfig != nil {
		cfg := r.liveConfig()
		hpLow = hpPct < float64(cfg.HPThreshold)
		spLow = spPct < float64(cfg.SPThreshold)
	}

	return BarReadResult{
		HP:     hpPct,
		SP:     spPct,
		HPLow:  hpLow,
		SPLow:  spLow,
		Status: StatusFound,
	}
}

// setError marks the reader as in error state, updates the overlay mode
// to "Address err" on first failure, logs the error (rate-limited), and
// periodically attempts to auto-reconnect to the game process by searching
// for a visible window whose title matches processTitle.
func (r *addressReader) setError(format string, args ...interface{}) {
	now := time.Now()

	// First failure after a success — switch overlay to error indicator.
	if !r.hadError {
		r.hadError = true
		if r.onModeChange != nil {
			r.onModeChange("Address err")
		}
	}

	// Auto-reconnect: try to find a new PID by window title periodically.
	if r.pid != 0 && now.Sub(r.lastReconn) >= reconnectInterval {
		r.lastReconn = now

		newPID := r.pid
		if r.processTitle != "" {
			if found := win.FindVisibleWindowPID(r.processTitle); found != 0 {
				newPID = found
			}
		}

		if newPID != r.pid {
			r.pid = newPID
			r.log(fmt.Sprintf("address: reconnected to PID %d", newPID))
			// Reconnect succeeded — clear error and restore normal mode.
			// The next ReadValues will open a fresh handle to the new PID.
			r.hadError = false
			r.loggedFirstFail = false
			if r.onModeChange != nil {
				r.onModeChange("Address reading")
			}
			return
		}
	}

	// Log the error (rate-limited to once per 10s).
	if r.log == nil {
		return
	}
	if !r.loggedFirstFail || now.Sub(r.lastLog) > 10*time.Second {
		r.log(fmt.Sprintf(format, args...))
		r.loggedFirstFail = true
		r.lastLog = now
	}
}
