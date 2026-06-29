package autopot

import (
	"image"
	"testing"
)

func newTestStabilizers(threshold int) (*BarStabilizer, *BarStabilizer) {
	return NewBarStabilizer(true, threshold), NewBarStabilizer(false, threshold)
}

func stabRead(img image.Image, stab *BarStabilizer, hpBar bool) StableBarRead {
	mapped, ok := RefreshStableBarPair(img)
	if !ok {
		return stab.UpdatePair(img, hpBar, mapped, false)
	}
	return stab.UpdatePair(img, hpBar, mapped, ok)
}

func TestAutoPotUpdateSettings(t *testing.T) {
	ap := NewAutoPot(AutoPotConfig{
		HPEnabled:   true,
		HPKeyVK:     'W',
		HPThreshold: 50,
		SPThreshold: 30,
		Log:         func(string) {},
	})

	ap.UpdateSettings(AutoPotConfig{
		HPEnabled:   true,
		HPKeyVK:     'W',
		HPThreshold: 60,
		SPThreshold: 30,
		Log:         func(string) {},
	})
	cfg := ap.settings()
	if cfg.HPThreshold != 60 || cfg.SPThreshold != 30 {
		t.Fatalf("after HP edit cfg=%d/%d want 60/30", cfg.HPThreshold, cfg.SPThreshold)
	}

	ap.UpdateSettings(AutoPotConfig{
		HPEnabled:   true,
		HPKeyVK:     'W',
		HPThreshold: 75,
		SPEnabled:   true,
		SPKeyVK:     'E',
		SPThreshold: 50,
		Log:         func(string) {},
	})
	cfg = ap.settings()
	if cfg.HPThreshold != 75 || cfg.SPThreshold != 50 {
		t.Fatalf("after SP edit cfg=%d/%d want 75/50", cfg.HPThreshold, cfg.SPThreshold)
	}
}

func TestStabilizerRejectsStaleOffset(t *testing.T) {
	img := loadFixture(t, "aa.png")
	mapped, err := RefreshBarPair(img)
	if err != nil {
		t.Fatal(err)
	}
	fresh, _ := ReadMappedBars(img, mapped)

	stale := mapped
	stale.HP.X += 10
	stale.HP.Y += 2
	stale.SP.X += 10
	stale.SP.Y += 2
	staleHP, _ := ReadMappedBars(img, stale)
	if staleHP.Percent >= 50 {
		t.Fatalf("setup: stale read %.1f%% should be below 50%%", staleHP.Percent)
	}

	hpStab, _ := newTestStabilizers(50)
	read := stabRead(img, hpStab, true)
	if !read.Found || read.Percent < fresh.Percent-3 {
		t.Fatalf("Update=%.1f%% want ~%.1f%% (stale was %.1f%%)", read.Percent, fresh.Percent, staleHP.Percent)
	}
	if read.Status == BarStatusLow {
		t.Fatalf("Update=%.1f%% should not be low at 50%% threshold", read.Percent)
	}
}

func TestStabilizerDetectsLowHP(t *testing.T) {
	img := loadFixture(t, "jj.png")
	hpStab, _ := newTestStabilizers(50)

	var read StableBarRead
	for i := 0; i < PotConfirmReads; i++ {
		read = stabRead(img, hpStab, true)
	}
	if !read.Found || read.Percent > 15 {
		t.Fatalf("low HP read %.1f%%", read.Percent)
	}
	if read.Status != BarStatusLow {
		t.Fatalf("low HP %.1f%% want Status=Low after %d reads, got %v", read.Percent, PotConfirmReads, read.Status)
	}
}

func TestStabilizerFullAfterStablePair(t *testing.T) {
	img := loadFixture(t, "drift1.2.png")
	hpStab, spStab := newTestStabilizers(50)

	hp := stabRead(img, hpStab, true)
	sp := stabRead(img, spStab, false)
	if hp.Status != BarStatusFull || hp.Percent < 99.9 {
		t.Fatalf("full HP hp=%.1f%% status=%v", hp.Percent, hp.Status)
	}
	if sp.Status != BarStatusFull || sp.Percent < 99.9 {
		t.Fatalf("full SP sp=%.1f%% status=%v", sp.Percent, sp.Status)
	}
}

func TestFullLatchHoldsWhileFull(t *testing.T) {
	img := loadFixture(t, "Drift8.png")
	hpStab, _ := newTestStabilizers(80)

	read := stabRead(img, hpStab, true)
	if read.Status != BarStatusFull {
		t.Fatal("full read should latch")
	}
	read = stabRead(img, hpStab, true)
	if read.Status != BarStatusFull || read.Percent < 99.9 {
		t.Fatalf("latched read %.1f%% status=%v should stay full", read.Percent, read.Status)
	}
}

func TestFullLatchSurvivesRectDrift(t *testing.T) {
	img := loadFixture(t, "ii.png")
	mapped, err := RefreshBarPair(img)
	if err != nil {
		t.Fatal(err)
	}
	hpStab, _ := newTestStabilizers(50)

	read := hpStab.UpdatePair(img, true, mapped, true)
	if read.Status != BarStatusFull {
		t.Fatalf("setup want full, got %v", read.Status)
	}

	drifted := mapped
	drifted.HP.X += 12
	drifted.SP.X += 12
	read = hpStab.UpdatePair(img, true, drifted, true)
	if read.Status == BarStatusLow {
		t.Fatalf("full HP must not go low after rect drift, got %.1f%%", read.Percent)
	}
	if read.Status == BarStatusFull {
		t.Fatalf("drifted rect must not assert full without visual proof, got full")
	}

	read = hpStab.UpdatePair(img, true, mapped, true)
	if read.Status != BarStatusFull {
		t.Fatalf("full HP latch should survive drift, got %v at %.1f%%", read.Status, read.Percent)
	}
}

func TestFullLatchUnknownOnFailedPair(t *testing.T) {
	img := loadFixture(t, "Drift8.png")
	mapped, err := RefreshBarPair(img)
	if err != nil {
		t.Fatal(err)
	}
	hpStab, _ := newTestStabilizers(50)

	read := hpStab.UpdatePair(img, true, mapped, true)
	if read.Status != BarStatusFull {
		t.Fatal("setup want full")
	}

	read = hpStab.UpdatePair(img, true, mapped, false)
	if read.Status != BarStatusUnknown {
		t.Fatalf("failed pair want unknown, got %v", read.Status)
	}
	if read.Status == BarStatusLow {
		t.Fatal("failed pair must not report low")
	}
}

func TestFullLatchClearsOnStableDamageRead(t *testing.T) {
	img := loadFixture(t, "jj.png")
	mapped, err := RefreshBarPair(img)
	if err != nil {
		t.Fatal(err)
	}
	hpStab, _ := newTestStabilizers(50)

	hpStab.mu.Lock()
	hpStab.fullLatched = true
	hpStab.lastValidRect = mapped.HP
	hpStab.mu.Unlock()

	var read StableBarRead
	for i := 0; i < PotUnlatchReads; i++ {
		read = hpStab.UpdatePair(img, true, mapped, true)
	}
	if read.Status == BarStatusFull {
		t.Fatal("partial HP should unlatch false full")
	}

	for i := 0; i < PotConfirmReads; i++ {
		read = hpStab.UpdatePair(img, true, mapped, true)
	}
	if read.Status != BarStatusLow {
		t.Fatalf("partial HP want low, got %v at %.1f%%", read.Status, read.Percent)
	}
}

func TestPartialBarsNotDetectedAsFull(t *testing.T) {
	partials := []string{"aa.png", "jj.png", "pp.png", "drift5.png", "Drift7.png"}
	for _, name := range partials {
		t.Run(name, func(t *testing.T) {
			img := loadFixture(t, name)
			mapped, err := RefreshBarPair(img)
			if err != nil {
				t.Fatal(err)
			}
			hp, sp := ReadMappedBars(img, mapped)
			tc := knownBarCases()[name]
			if tc.hpPct < 99.9 && BarLooksFull(img, mapped.HP, true) {
				t.Fatalf("HP %.1f%% (game %.1f%%) falsely detected full", hp.Percent, tc.hpPct)
			}
			if tc.spPct < 99.9 && BarLooksFull(img, mapped.SP, false) {
				t.Fatalf("SP %.1f%% (game %.1f%%) falsely detected full", sp.Percent, tc.spPct)
			}
		})
	}
}

func TestFullBarNeverReportsLow(t *testing.T) {
	img := loadFixture(t, "Drift8.png")
	hpStab, _ := newTestStabilizers(80)

	for i := 0; i < PotConfirmReads; i++ {
		read := stabRead(img, hpStab, true)
		if read.Status == BarStatusLow {
			t.Fatalf("full bar must not report low on read %d", i+1)
		}
	}
}

func TestFullBarStableAcrossShiftedRects(t *testing.T) {
	img := loadFixture(t, "Drift8.png")
	for dx := -50; dx <= 50; dx += 5 {
		if dx == 0 {
			continue
		}
		mapped, err := RefreshBarPair(img)
		if err != nil {
			t.Fatal(err)
		}
		mapped.HP.X += dx
		mapped.SP.X += dx
		if BarLooksFull(img, mapped.HP, true) {
			t.Fatalf("dx=%+d shifted rect must not look full", dx)
		}
	}
}

func TestDriftFullBarsNeverNeedPotion(t *testing.T) {
	for _, file := range []string{"drift1.2.png", "Drift8.png", "ii.png"} {
		t.Run(file, func(t *testing.T) {
			img := loadFixture(t, file)
			hpStab, spStab := newTestStabilizers(99)

			for _, threshold := range []int{1, 50, 80, 99} {
				hpStab.SetThreshold(threshold)
				spStab.SetThreshold(threshold)

				for i := 0; i < PotConfirmReads; i++ {
					hp := stabRead(img, hpStab, true)
					if hp.Status == BarStatusLow {
						t.Fatalf("threshold %d: HP %.1f%% must not be low", threshold, hp.Percent)
					}
					sp := stabRead(img, spStab, false)
					if sp.Status == BarStatusLow {
						t.Fatalf("threshold %d: SP %.1f%% must not be low", threshold, sp.Percent)
					}
				}
			}
		})
	}
}

func TestStabilizerSetThresholdResetsLowStreak(t *testing.T) {
	img := loadFixture(t, "jj.png")
	hpStab, _ := newTestStabilizers(50)

	for i := 0; i < PotConfirmReads-1; i++ {
		stabRead(img, hpStab, true)
	}
	hpStab.SetThreshold(51)
	read := stabRead(img, hpStab, true)
	if read.Status == BarStatusLow {
		t.Fatalf("threshold change should reset low streak, got low at %.1f%%", read.Percent)
	}

	hpStab.SetThreshold(80)
	for i := 0; i < PotConfirmReads; i++ {
		read = stabRead(img, hpStab, true)
	}
	if read.Status != BarStatusLow {
		t.Fatalf("threshold 80 want low at %.1f%%, got %v", read.Percent, read.Status)
	}
}

func TestStabilizerUnknownNeverLow(t *testing.T) {
	stab := NewBarStabilizer(true, 50)
	read := stab.UpdatePair(nil, true, MappedBars{}, false)
	if read.Status == BarStatusLow {
		t.Fatal("invalid input must not return low")
	}
	if read.Status != BarStatusUnknown {
		t.Fatalf("invalid input want unknown, got %v", read.Status)
	}
}

func TestLatchedFullNeverLowWithoutVerifiedRead(t *testing.T) {
	img := loadFixture(t, "Drift8.png")
	mapped, err := RefreshBarPair(img)
	if err != nil {
		t.Fatal(err)
	}
	hpStab, _ := newTestStabilizers(50)

	read := hpStab.UpdatePair(img, true, mapped, true)
	if read.Status != BarStatusFull {
		t.Fatal("setup want full")
	}

	read = hpStab.UpdatePair(img, true, mapped, false)
	if read.Status == BarStatusLow || read.Status == BarStatusFull {
		t.Fatalf("unverified frame want unknown, got %v", read.Status)
	}
}
