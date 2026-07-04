package autopot

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sys/windows"

	win "belarus-champ-tools/runner/platform/windows"
	"belarus-champ-tools/runner/profiles"
)

// addressReader is a BarReader that reads HP/SP values from a game
// process's memory at the offsets defined by a server profile.
//
// Uses kernel32 ReadProcessMemory via win.ReadProcessUint32ByHandle with
// a persistent process handle (opened once in autopot.go, reused for all
// reads, closed on shutdown). This avoids the open/close-per-read pattern
// that can trigger anti-cheat heuristics.
//
// When reads fail persistently, the reader attempts to auto-reconnect by
// finding a window whose title matches processTitle and resolving its PID.
type addressReader struct {
	pid          uint32
	procHandle   windows.Handle
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

// Close closes the persistent process handle. Should be called when the
// reader is no longer needed (e.g. on autopot shutdown).
func (r *addressReader) Close() {
	if r.procHandle != windows.InvalidHandle && r.procHandle != 0 {
		win.CloseProcessHandle(r.procHandle)
		r.procHandle = windows.InvalidHandle
	}
}

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

	// Profile stores module-relative offsets. Add the exe base address
	// to get absolute virtual addresses (ASLR-safe).
	base := r.moduleBase

	// Read HP values using the persistent process handle via kernel32
	// ReadProcessMemory. Handle opened once in autopot.go.
	curHP, err := win.ReadProcessUint32ByHandle(r.procHandle, base+r.profile.CurrentHPAddr)
	if err != nil {
		r.setError("address: %v", err)
		return BarReadResult{Status: StatusInvalid, Err: err}
	}
	maxHP, err := win.ReadProcessUint32ByHandle(r.procHandle, base+r.profile.MaxHPAddr)
	if err != nil {
		r.setError("address: %v", err)
		return BarReadResult{Status: StatusInvalid, Err: err}
	}

	// Read SP values.
	curSP, err := win.ReadProcessUint32ByHandle(r.procHandle, base+r.profile.CurrentSPAddr)
	if err != nil {
		r.setError("address: %v", err)
		return BarReadResult{Status: StatusInvalid, Err: err}
	}
	maxSP, err := win.ReadProcessUint32ByHandle(r.procHandle, base+r.profile.MaxSPAddr)
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

	// Auto-reconnect: try to find the process by window title periodically.
	// On reconnect, close the old handle and open a new one for the new PID.
	if r.processTitle != "" && now.Sub(r.lastReconn) >= reconnectInterval {
		r.lastReconn = now
		newPID := win.FindVisibleWindowPID(r.processTitle)
		if newPID != 0 && newPID != r.pid {
			// Close old handle, open new one for the new PID.
			r.Close()
			newHandle, hErr := win.OpenProcessHandle(newPID)
			if hErr != nil {
				r.log(fmt.Sprintf("address: reconnected to PID %d but OpenProcess failed: %v", newPID, hErr))
				return
			}
			r.pid = newPID
			r.procHandle = newHandle
			r.log(fmt.Sprintf("address: reconnected to PID %d via window %q", newPID, r.processTitle))
			// Reconnect succeeded — clear error and restore normal mode.
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
