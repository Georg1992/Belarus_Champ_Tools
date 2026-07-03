//go:build windows

package main

import (
	"testing"
)

func TestOverlayUpdate_Sentinel(t *testing.T) {
	ovl, err := newStatusOverlay()
	if err != nil {
		t.Fatal(err)
	}
	defer ovl.Destroy()

	ovl.Update(-1, 0, -1, 0, 0, 0, 0, 0)

	ovl.mu.Lock()
	got := ovl.text
	ovl.mu.Unlock()

	want := "error: Pixelsearch is used"
	if got != want {
		t.Errorf("Update(-1,0,-1,0) text = %q; want %q", got, want)
	}
}

func TestOverlayUpdate_FullValues(t *testing.T) {
	ovl, err := newStatusOverlay()
	if err != nil {
		t.Fatal(err)
	}
	defer ovl.Destroy()

	ovl.Update(750, 1290, 100, 200, 0, 0, 0, 0)

	ovl.mu.Lock()
	got := ovl.text
	ovl.mu.Unlock()

	want := "HP 750/1290  SP 100/200"
	if got != want {
		t.Errorf("Update(750,1290,100,200) text = %q; want %q", got, want)
	}
}

func TestOverlayUpdate_OnlyHPMax(t *testing.T) {
	ovl, err := newStatusOverlay()
	if err != nil {
		t.Fatal(err)
	}
	defer ovl.Destroy()

	ovl.Update(500, 1000, 60, 0, 0, 0, 0, 0)

	ovl.mu.Lock()
	got := ovl.text
	ovl.mu.Unlock()

	want := "HP 500/1000"
	if got != want {
		t.Errorf("Update(500,1000,60,0) text = %q; want %q", got, want)
	}
}

func TestOverlayUpdate_NoMax(t *testing.T) {
	ovl, err := newStatusOverlay()
	if err != nil {
		t.Fatal(err)
	}
	defer ovl.Destroy()

	ovl.Update(80, 0, 30, 0, 0, 0, 0, 0)

	ovl.mu.Lock()
	got := ovl.text
	ovl.mu.Unlock()

	want := "HP 80  SP 30"
	if got != want {
		t.Errorf("Update(80,0,30,0) text = %q; want %q", got, want)
	}
}

func TestOverlayUpdate_Reposition(t *testing.T) {
	ovl, err := newStatusOverlay()
	if err != nil {
		t.Fatal(err)
	}
	defer ovl.Destroy()

	// stripW>0, stripH>0 → SetWindowPos is called with these strip coords.
	// We can't easily verify the HWND position from here, but we can verify
	// the text was set correctly (the reposition path doesn't affect text).
	ovl.Update(750, 1290, 100, 200, 100, 500, 218, 58)

	ovl.mu.Lock()
	got := ovl.text
	ovl.mu.Unlock()

	want := "HP 750/1290  SP 100/200"
	if got != want {
		t.Errorf("Update with strip text = %q; want %q", got, want)
	}
}

func TestOverlayUpdate_ZeroValues(t *testing.T) {
	ovl, err := newStatusOverlay()
	if err != nil {
		t.Fatal(err)
	}
	defer ovl.Destroy()

	// Zero HP/SP — not a sentinel, should show "HP 0 SP 0".
	ovl.Update(0, 0, 0, 0, 0, 0, 0, 0)

	ovl.mu.Lock()
	got := ovl.text
	ovl.mu.Unlock()

	want := "HP 0  SP 0"
	if got != want {
		t.Errorf("Update(0,0,0,0) text = %q; want %q", got, want)
	}
}

func TestOverlayUpdate_OnlyOneNegative(t *testing.T) {
	ovl, err := newStatusOverlay()
	if err != nil {
		t.Fatal(err)
	}
	defer ovl.Destroy()

	// hp=-1 only — should still trigger sentinel (hp < 0).
	ovl.Update(-1, 0, 50, 100, 0, 0, 0, 0)

	ovl.mu.Lock()
	got := ovl.text
	ovl.mu.Unlock()

	want := "error: Pixelsearch is used"
	if got != want {
		t.Errorf("Update(-1,0,50,100) text = %q; want %q", got, want)
	}
}

func TestOverlaySetMode(t *testing.T) {
	ovl, err := newStatusOverlay()
	if err != nil {
		t.Fatal(err)
	}
	defer ovl.Destroy()

	ovl.SetMode("Pixel-bar")

	ovl.mu.Lock()
	got := ovl.mode
	ovl.mu.Unlock()

	want := "Pixel-bar"
	if got != want {
		t.Errorf("SetMode('Pixel-bar') mode = %q; want %q", got, want)
	}
}

func TestOverlayUpdate_PreservesMode(t *testing.T) {
	ovl, err := newStatusOverlay()
	if err != nil {
		t.Fatal(err)
	}
	defer ovl.Destroy()

	ovl.SetMode("OCR")
	ovl.Update(-1, 0, -1, 0, 0, 0, 0, 0)

	ovl.mu.Lock()
	mode := ovl.mode
	text := ovl.text
	ovl.mu.Unlock()

	if mode != "OCR" {
		t.Errorf("mode after sentinel = %q; want %q", mode, "OCR")
	}
	if text != "error: Pixelsearch is used" {
		t.Errorf("text after sentinel = %q; want %q", text, "error: Pixelsearch is used")
	}
}

func TestOverlaySetMode_Empty(t *testing.T) {
	ovl, err := newStatusOverlay()
	if err != nil {
		t.Fatal(err)
	}
	defer ovl.Destroy()

	ovl.SetMode("")

	ovl.mu.Lock()
	got := ovl.mode
	ovl.mu.Unlock()

	if got != "" {
		t.Errorf("SetMode('') mode = %q; want empty", got)
	}
}

func TestOverlaySetMode_Replaces(t *testing.T) {
	ovl, err := newStatusOverlay()
	if err != nil {
		t.Fatal(err)
	}
	defer ovl.Destroy()

	ovl.SetMode("OCR")
	ovl.SetMode("Pixel-bar")

	ovl.mu.Lock()
	got := ovl.mode
	ovl.mu.Unlock()

	want := "Pixel-bar"
	if got != want {
		t.Errorf("SetMode OCR→Pixel-bar mode = %q; want %q", got, want)
	}
}

func TestOverlaySetMode_DeadAlert(t *testing.T) {
	ovl, err := newStatusOverlay()
	if err != nil {
		t.Fatal(err)
	}
	defer ovl.Destroy()

	ovl.Update(1, 1290, 0, 201, 100, 500, 218, 58)
	ovl.SetMode("Dead")

	ovl.mu.Lock()
	gotMode := ovl.mode
	gotText := ovl.text
	ovl.mu.Unlock()

	if gotMode != "Dead" {
		t.Errorf("SetMode('Dead') mode = %q; want %q", gotMode, "Dead")
	}
	if gotText != "HP 1/1290  SP 0/201" {
		t.Errorf("text after SetMode('Dead') = %q; want %q", gotText, "HP 1/1290  SP 0/201")
	}
}

func TestOverlaySetMode_PotsEnded(t *testing.T) {
	ovl, err := newStatusOverlay()
	if err != nil {
		t.Fatal(err)
	}
	defer ovl.Destroy()

	ovl.Update(350, 1290, 107, 201, 100, 500, 218, 58)
	ovl.SetMode("HP pots ended on F9")

	ovl.mu.Lock()
	gotMode := ovl.mode
	gotText := ovl.text
	ovl.mu.Unlock()

	if gotMode != "HP pots ended on F9" {
		t.Errorf("SetMode('HP pots ended on F9') mode = %q; want %q", gotMode, "HP pots ended on F9")
	}
	if gotText != "HP 350/1290  SP 107/201" {
		t.Errorf("text after pots-ended = %q; want %q", gotText, "HP 350/1290  SP 107/201")
	}
}

func TestOverlayShowStopped(t *testing.T) {
	ovl, err := newStatusOverlay()
	if err != nil {
		t.Fatal(err)
	}
	defer ovl.Destroy()

	// Set some values first.
	ovl.Update(750, 1290, 100, 200, 0, 0, 0, 0)
	ovl.SetMode("OCR")

	// Now show stopped.
	ovl.ShowStopped()

	ovl.mu.Lock()
	gotMode := ovl.mode
	gotText := ovl.text
	ovl.mu.Unlock()

	if gotMode != "Stopped" {
		t.Errorf("ShowStopped() mode = %q; want %q", gotMode, "Stopped")
	}
	if gotText != "" {
		t.Errorf("ShowStopped() text = %q; want empty", gotText)
	}
}

func TestOverlayShowStopped_ThenUpdateRestoresText(t *testing.T) {
	ovl, err := newStatusOverlay()
	if err != nil {
		t.Fatal(err)
	}
	defer ovl.Destroy()

	ovl.ShowStopped()
	ovl.Update(750, 1290, 100, 200, 0, 0, 0, 0)

	ovl.mu.Lock()
	gotText := ovl.text
	gotMode := ovl.mode
	ovl.mu.Unlock()

	if gotText != "HP 750/1290  SP 100/200" {
		t.Errorf("Update after ShowStopped text = %q; want %q", gotText, "HP 750/1290  SP 100/200")
	}
	// Update does NOT clear mode.
	if gotMode != "Stopped" {
		t.Errorf("mode after Update = %q; want %q", gotMode, "Stopped")
	}
}

func TestOverlayHide(t *testing.T) {
	ovl, err := newStatusOverlay()
	if err != nil {
		t.Fatal(err)
	}
	defer ovl.Destroy()

	// Just ensure it doesn't panic — no return value to assert.
	ovl.Hide()
}
