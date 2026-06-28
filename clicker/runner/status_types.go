package runner

import "time"

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

// NumericRead holds parsed HP and SP values from the status window.
type NumericRead struct {
	HP NumericResourceRead
	SP NumericResourceRead
}
