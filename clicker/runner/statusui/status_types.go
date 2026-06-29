package statusui

import "time"

// ResourceValue is an alias for NumericResourceRead so call sites can use either name.
type ResourceValue = NumericResourceRead

// StatusValues represents a complete status snapshot from the UI.
type StatusValues struct {
	HP         ResourceValue
	SP         ResourceValue
	UpdatedAt  time.Time
	Valid      bool
	Confidence float64
}
