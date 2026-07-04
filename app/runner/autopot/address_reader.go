package autopot

import (
	"context"
	"fmt"
	"time"

	win "belarus-champ-tools/runner/platform/windows"
	"belarus-champ-tools/runner/profiles"

	"golang.org/x/sys/windows"
)

// addressReader is a BarReader that reads HP/SP values from a game
// process's memory at the offsets defined by a server profile.
//
// The process handle must be opened externally and remain valid for
// the reader's lifetime. ReadValues returns StatusInvalid if the
// handle is invalid or the process has exited.
type addressReader struct {
	handle  windows.Handle
	profile profiles.Profile

	// liveConfig returns the current AutoPotConfig so the reader can
	// compute HPLow/SPLow against the latest threshold values.
	liveConfig func() AutoPotConfig

	// onParsed forwards the raw HP/SP values to the overlay, matching
	// the signature used by the OCR reader for the same purpose.
	// strip coordinates are set to 0 since address mode has no screen panel.
	onParsed func(hp, hpMax, sp, spMax, stripX, stripY, stripW, stripH int)

	lastLog time.Time
	log     func(string)
}

func (r *addressReader) Name() string { return "Address" }

func (r *addressReader) ReadValues(ctx context.Context) BarReadResult {
	if ctx.Err() != nil {
		return BarReadResult{Status: StatusInvalid, Err: ctx.Err()}
	}
	if r.handle == windows.InvalidHandle || r.handle == 0 {
		return BarReadResult{Status: StatusInvalid, Err: fmt.Errorf("address reader: invalid process handle")}
	}

	// Read HP values.
	curHP, err := win.ReadProcessUint32(r.handle, 0, r.profile.CurrentHPAddr)
	if err != nil {
		r.debugf("address: read HP failed: %v", err)
		return BarReadResult{Status: StatusInvalid, Err: err}
	}
	maxHP, err := win.ReadProcessUint32(r.handle, 0, r.profile.MaxHPAddr)
	if err != nil {
		r.debugf("address: read MaxHP failed: %v", err)
		return BarReadResult{Status: StatusInvalid, Err: err}
	}

	// Read SP values.
	curSP, err := win.ReadProcessUint32(r.handle, 0, r.profile.CurrentSPAddr)
	if err != nil {
		r.debugf("address: read SP failed: %v", err)
		return BarReadResult{Status: StatusInvalid, Err: err}
	}
	maxSP, err := win.ReadProcessUint32(r.handle, 0, r.profile.MaxSPAddr)
	if err != nil {
		r.debugf("address: read MaxSP failed: %v", err)
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

// debugf logs at most once per 2 seconds.
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
