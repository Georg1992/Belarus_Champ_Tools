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
// Following the user's AHK pattern: opens a handle, reads, and closes
// on every ReadValues call. This avoids stale-handle issues entirely.
type addressReader struct {
	pid     uint32
	profile profiles.Profile

	liveConfig func() AutoPotConfig
	onParsed   func(hp, hpMax, sp, spMax, stripX, stripY, stripW, stripH int)

	lastLog  time.Time
	log      func(string)
	loggedFirstFail bool
}

func (r *addressReader) Name() string { return "Address" }

func (r *addressReader) ReadValues(ctx context.Context) BarReadResult {
	if ctx.Err() != nil {
		return BarReadResult{Status: StatusInvalid, Err: ctx.Err()}
	}
	if r.pid == 0 {
		return BarReadResult{Status: StatusInvalid, Err: fmt.Errorf("address reader: no process selected (PID=0)")}
	}

	// Read HP values — opens handle, reads, closes on each call (like AHK pattern).
	curHP, err := win.ReadProcessUint32ByPID(r.pid, r.profile.CurrentHPAddr)
	if err != nil {
		r.logFirstFail("address: %v", err)
		return BarReadResult{Status: StatusInvalid, Err: err}
	}
	maxHP, err := win.ReadProcessUint32ByPID(r.pid, r.profile.MaxHPAddr)
	if err != nil {
		r.resetFirstFail()
		r.logFirstFail("address: %v", err)
		return BarReadResult{Status: StatusInvalid, Err: err}
	}

	// Read SP values.
	curSP, err := win.ReadProcessUint32ByPID(r.pid, r.profile.CurrentSPAddr)
	if err != nil {
		r.resetFirstFail()
		r.logFirstFail("address: %v", err)
		return BarReadResult{Status: StatusInvalid, Err: err}
	}
	maxSP, err := win.ReadProcessUint32ByPID(r.pid, r.profile.MaxSPAddr)
	if err != nil {
		r.resetFirstFail()
		r.logFirstFail("address: %v", err)
		return BarReadResult{Status: StatusInvalid, Err: err}
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

	r.debugf("address: HP=%d/%d (%.0f%%) SP=%d/%d (%.0f%%)",
		curHP, maxHP, hpPct, curSP, maxSP, spPct)

	return BarReadResult{
		HP:     hpPct,
		SP:     spPct,
		HPLow:  hpLow,
		SPLow:  spLow,
		Status: StatusFound,
	}
}

// logFirstFail logs the first failure immediately, then rate-limits to
// once per 10 seconds so the user sees the error without spamming.
func (r *addressReader) logFirstFail(format string, args ...interface{}) {
	if r.log == nil {
		return
	}
	now := time.Now()
	if !r.loggedFirstFail || now.Sub(r.lastLog) > 10*time.Second {
		r.log(fmt.Sprintf(format, args...))
		r.loggedFirstFail = true
		r.lastLog = now
	}
}

// resetFirstFail allows the next failure to log immediately (used after
// a successful read followed by a new failure).
func (r *addressReader) resetFirstFail() {
	r.loggedFirstFail = false
}

// debugf logs at most once per 2 seconds (kept for non-critical messages).
func (r *addressReader) debugf(format string, args ...interface{}) {
	if r.log == nil {
		return
	}
	now := time.Now()
	if now.Sub(r.lastLog) < 2*time.Second {
		return
	}
	r.lastLog = now
	r.log(fmt.Sprintf(format, args...))
}
