package autopot

import "testing"

// Rect helper for tests — zero-value W means degenerate.
var r = func(x, y, w, h int) Rect { return Rect{X: x, Y: y, W: w, H: h} }

func TestRectDrifted_SameRect(t *testing.T) {
	a := r(100, 200, 60, 3)
	if rectDrifted(a, a, 8) {
		t.Error("same rect should not be drifted")
	}
}

func TestRectDrifted_XBoundary(t *testing.T) {
	base := r(100, 200, 60, 3)

	tests := []struct {
		name     string
		other    Rect
		max      int
		want     bool
	}{
		// diff ≤ max → within tolerance → not drifted
		{"same_x", r(100, 200, 60, 3), 8, false},
		{"x_diff_7", r(107, 200, 60, 3), 8, false},
		{"x_diff_8_eq_max", r(108, 200, 60, 3), 8, false},
		// diff > max → drifted
		{"x_diff_9", r(109, 200, 60, 3), 8, true},
		{"x_diff_50", r(150, 200, 60, 3), 8, true},
		// negative drift
		{"x_diff_neg_9", r(91, 200, 60, 3), 8, true},
		{"x_diff_neg_8", r(92, 200, 60, 3), 8, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rectDrifted(base, tt.other, tt.max)
			if got != tt.want {
				t.Errorf("rectDrifted(base=%+v, other=%+v, max=%d) = %v; want %v",
					base, tt.other, tt.max, got, tt.want)
			}
		})
	}
}

func TestRectDrifted_YBoundary(t *testing.T) {
	base := r(100, 200, 60, 3)

	tests := []struct {
		name     string
		other    Rect
		max      int
		want     bool
	}{
		{"y_diff_7", r(100, 207, 60, 3), 8, false},
		{"y_diff_8_eq_max", r(100, 208, 60, 3), 8, false},
		{"y_diff_9", r(100, 209, 60, 3), 8, true},
		{"y_diff_neg_9", r(100, 191, 60, 3), 8, true},
		{"y_diff_neg_8", r(100, 192, 60, 3), 8, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rectDrifted(base, tt.other, tt.max)
			if got != tt.want {
				t.Errorf("rectDrifted(base=%+v, other=%+v, max=%d) = %v; want %v",
					base, tt.other, tt.max, got, tt.want)
			}
		})
	}
}

func TestRectDrifted_WBoundary(t *testing.T) {
	base := r(100, 200, 60, 3)

	tests := []struct {
		name     string
		other    Rect
		max      int
		want     bool
	}{
		{"w_diff_7", r(100, 200, 67, 3), 8, false},
		{"w_diff_8_eq_max", r(100, 200, 68, 3), 8, false},
		{"w_diff_9", r(100, 200, 69, 3), 8, true},
		{"w_shrink_7", r(100, 200, 53, 3), 8, false},
		{"w_shrink_8", r(100, 200, 52, 3), 8, false},
		{"w_shrink_9", r(100, 200, 51, 3), 8, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rectDrifted(base, tt.other, tt.max)
			if got != tt.want {
				t.Errorf("rectDrifted(base=%+v, other=%+v, max=%d) = %v; want %v",
					base, tt.other, tt.max, got, tt.want)
			}
		})
	}
}

func TestRectDrifted_Degenerate(t *testing.T) {
	a := r(100, 200, 60, 3)
	b := r(100, 200, 0, 3)  // W=0 → degenerate
	if !rectDrifted(a, b, 8) {
		t.Error("rect with W=0 should be considered drifted")
	}
	if !rectDrifted(b, a, 8) {
		t.Error("rect with W=0 (first arg) should be considered drifted")
	}
}

func TestRectDrifted_CustomMax(t *testing.T) {
	base := r(100, 200, 60, 3)

	tests := []struct {
		name     string
		other    Rect
		max      int
		want     bool
	}{
		{"custom_max_4_diff_4", r(104, 200, 60, 3), 4, false},
		{"custom_max_4_diff_5", r(105, 200, 60, 3), 4, true},
		{"custom_max_10_diff_10", r(110, 200, 60, 3), 10, false},
		{"custom_max_10_diff_11", r(111, 200, 60, 3), 10, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rectDrifted(base, tt.other, tt.max)
			if got != tt.want {
				t.Errorf("rectDrifted(base=%+v, other=%+v, max=%d) = %v; want %v",
					base, tt.other, tt.max, got, tt.want)
			}
		})
	}
}

func TestRectDrifted_MultipleAxes(t *testing.T) {
	base := r(100, 200, 60, 3)

	// Both X and Y within tolerance → not drifted
	if rectDrifted(base, r(105, 205, 60, 3), 8) {
		t.Error("X=5, Y=5 (both within max=8) should not be drifted")
	}
	// X within, W within → not drifted
	if rectDrifted(base, r(105, 200, 65, 3), 8) {
		t.Error("X=5, W=5 (both within max=8) should not be drifted")
	}
	// Both X and Y exceeded → drifted
	if !rectDrifted(base, r(110, 210, 60, 3), 8) {
		t.Error("X=10, Y=10 (both > max=8) should be drifted")
	}
	// X within, Y exceeded → drifted (one axis exceeding is enough)
	if !rectDrifted(base, r(105, 210, 60, 3), 8) {
		t.Error("X=5 within, Y=10 > max — should be drifted")
	}
}
