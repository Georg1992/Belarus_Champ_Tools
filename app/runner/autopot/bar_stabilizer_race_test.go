package autopot

import (
	"image"
	"sync"
	"testing"
	"time"
)

// TestBarStabilizerStreakPreservedOnTransientFail verifies that transient
// consistency failures in UpdatePair do not reset lowStreak. This is the
// fix that prevents pixel autopot from silently never healing during
// sustained combat — without it, fill-measurement noise kept resetting
// the low-read counter and healing never triggered.
//
// Phases:
//  1. Build lowStreak to PotConfirmReads using the jj.png fixture
//     (HP ≈ 9.3%, SP ≈ 3%) — every read is well below threshold.
//  2. Inject 10 transient consistency failures using a tiny black
//     canvas with a MappedBars rect that has zero HP/SP pixels,
//     forcing the !read.Found → readUnknownPreserveStreak() path.
//  3. One more consistent low read on the real fixture immediately
//     returns BarStatusLow because lowStreak survived.
//
// Both HP and SP stabilizers tested independently.
func TestBarStabilizerStreakPreservedOnTransientFail(t *testing.T) {
	img := loadFixture(t, "jj.png") // HP ≈ 9.3%, SP ≈ 3% → well below threshold
	mapped, err := RefreshBarPair(img)
	if err != nil {
		t.Fatal(err)
	}

	// A tiny black canvas — no HP/SP pixels anywhere.
	// Passing a MappedBars with rects in this canvas guarantees
	// ReadHPFill/ReadSPFill return Found: false, which triggers
	// readUnknownPreserveStreak() in the stabiliser.
	blackImg := image.NewRGBA(image.Rect(0, 0, 10, 10))
	blackMapped := MappedBars{
		HP:    Rect{X: 0, Y: 0, W: 5, H: 5},
		SP:    Rect{X: 0, Y: 0, W: 5, H: 5},
		Valid: true,
	}

	for _, stab := range []*BarStabilizer{
		NewBarStabilizer(true, 50),
		NewBarStabilizer(false, 50),
	} {
		t.Run(map[bool]string{true: "HP", false: "SP"}[stab.hpBar], func(t *testing.T) {
			// Phase 1: Build lowStreak to PotConfirmReads.
			// Must use stab.hpBar as the hpBar parameter.
			var last StableBarRead
			for i := 0; i < 20; i++ {
				last = stab.UpdatePair(img, stab.hpBar, mapped, true)
				if last.Status == BarStatusLow {
					break
				}
			}
			if last.Status != BarStatusLow {
				t.Fatalf("expected BarStatusLow on jj.png, got status=%d", last.Status)
			}

			// Phase 2: Inject transient consistency failures via the
			// black canvas. read.Found will be false → triggers
			// readUnknownPreserveStreak() which preserves lowStreak.
			for i := 0; i < 10; i++ {
				result := stab.UpdatePair(blackImg, stab.hpBar, blackMapped, true)
				if result.Status != BarStatusUnknown {
					t.Fatalf("injection round %d: expected BarStatusUnknown, got status=%d", i, result.Status)
				}
			}

			// Phase 3: After transient failures, one more consistent
			// low read should immediately return BarStatusLow because
			// lowStreak survived across all black-canvas calls.
			result := stab.UpdatePair(img, stab.hpBar, mapped, true)
			if result.Status != BarStatusLow {
				t.Errorf("expected BarStatusLow after transient failures (lowStreak preserved), got status=%d (percent=%.0f)",
					result.Status, result.Percent)
			}
		})
	}
}

// TestBarStabilizerConcurrentUpdates stresses the BarStabilizer's internal
// mu by hammering UpdatePair (mutates fullLatched / lastValidRect / lowStreak
// / notFullStreak) concurrently with SetThreshold and Reset. The stabilizer
// is read+mutated on every poll by autopot.run and autopot.healUntil, so
// this is on the auto-pot hot path.
func TestBarStabilizerConcurrentUpdates(t *testing.T) {
	img := loadFixture(t, "jj.png")
	mapped, err := RefreshBarPair(img)
	if err != nil {
		t.Fatal(err)
	}

	stab := NewBarStabilizer(true, 50)
	const duration = 250 * time.Millisecond
	stop := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = stab.UpdatePair(img, true, mapped, true)
				}
			}
		}()
	}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			n := seed
			for {
				select {
				case <-stop:
					return
				default:
					stab.SetThreshold(1 + n%99)
					n++
				}
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				stab.Reset()
			}
		}
	}()

	time.Sleep(duration)
	close(stop)
	wg.Wait()
}
