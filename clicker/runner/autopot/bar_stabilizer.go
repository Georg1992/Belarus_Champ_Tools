package autopot

import (
	"image"
	"sync"
)

const (
	PotConfirmReads = 3
	PotUnlatchReads = 1 // single read to unlatch from full — faster reaction
)

type BarStatus int

const (
	BarStatusUnknown BarStatus = iota
	BarStatusFull
	BarStatusOK
	BarStatusLow
)

type StableBarRead struct {
	Found   bool
	Percent float64
	Status  BarStatus
	Rect    Rect
}

type BarStabilizer struct {
	hpBar     bool
	threshold int

	mu            sync.Mutex
	lastValidRect Rect
	fullLatched   bool
	notFullStreak int
	lowStreak     int
}

func NewBarStabilizer(hpBar bool, threshold int) *BarStabilizer {
	return &BarStabilizer{hpBar: hpBar, threshold: threshold}
}

func (s *BarStabilizer) SetThreshold(threshold int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if threshold == s.threshold {
		return
	}
	s.threshold = threshold
	s.lowStreak = 0
}

func (s *BarStabilizer) Reset() {
	s.mu.Lock()
	s.lastValidRect = Rect{}
	s.fullLatched = false
	s.notFullStreak = 0
	s.lowStreak = 0
	s.mu.Unlock()
}

func (s *BarStabilizer) UpdatePair(img image.Image, hpBar bool, mapped MappedBars, pairOK bool) StableBarRead {
	if hpBar != s.hpBar || img == nil {
		return s.readUnknown()
	}
	if !pairOK {
		return s.readUnknown()
	}

	hp, sp := ReadMappedBars(img, mapped)

	var read BarRead
	var rect Rect
	if s.hpBar {
		read, rect = hp, mapped.HP
	} else {
		read, rect = sp, mapped.SP
	}
	// Each stabiliser checks only its OWN bar. The old code required
	// BOTH bars to be found (!hp.Found || !sp.Found) which meant a
	// missing SP bar would reset the HP stabiliser's lowStreak too —
	// making pixel search autopot silently never heal.
	if !read.Found || !barReadConsistent(img, rect, s.hpBar, read) {
		return s.readUnknown()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.fullLatched {
		rectStable := s.lastValidRect.W < 1 || !rectDrifted(s.lastValidRect, rect, BarPositionMaxDrift)

		if BarLooksFull(img, rect, s.hpBar) || read.Percent >= 99.9 {
			s.lastValidRect = rect
			s.notFullStreak = 0
			s.lowStreak = 0
			return s.fullReadLocked(rect)
		}
		if rectStable && barConfirmedNotFull(img, rect, s.hpBar, read) {
			s.notFullStreak++
			if s.notFullStreak >= PotUnlatchReads {
				s.fullLatched = false
				s.notFullStreak = 0
				s.lastValidRect = rect
			}
		} else if !rectStable {
			s.notFullStreak = 0
			return s.readUnknownLocked()
		} else {
			s.notFullStreak = 0
			return s.readUnknownLocked()
		}
		if s.fullLatched {
			return s.readUnknownLocked()
		}
	}

	if s.lastValidRect.W >= 1 && rectDrifted(s.lastValidRect, rect, BarPositionMaxDrift) {
		s.lowStreak = 0
	}

	s.lastValidRect = rect
	return s.applyTrustedReadLocked(img, rect, read)
}

func (s *BarStabilizer) readUnknown() StableBarRead {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readUnknownLocked()
}

func (s *BarStabilizer) readUnknownLocked() StableBarRead {
	s.lowStreak = 0
	return StableBarRead{Status: BarStatusUnknown}
}

func (s *BarStabilizer) fullReadLocked(rect Rect) StableBarRead {
	return StableBarRead{
		Found:   true,
		Percent: 100,
		Status:  BarStatusFull,
		Rect:    rect,
	}
}

func (s *BarStabilizer) applyTrustedReadLocked(img image.Image, rect Rect, read BarRead) StableBarRead {
	if BarLooksFull(img, rect, s.hpBar) || read.Percent >= 99.9 {
		s.fullLatched = true
		s.notFullStreak = 0
		s.lowStreak = 0
		return s.fullReadLocked(rect)
	}

	if read.Percent < float64(s.threshold) {
		s.lowStreak++
	} else {
		s.lowStreak = 0
	}

	if s.lowStreak >= PotConfirmReads {
		return StableBarRead{
			Found:   true,
			Percent: read.Percent,
			Status:  BarStatusLow,
			Rect:    rect,
		}
	}

	return StableBarRead{
		Found:   true,
		Percent: read.Percent,
		Status:  BarStatusOK,
		Rect:    rect,
	}
}
