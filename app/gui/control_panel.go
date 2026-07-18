//go:build windows

package main

import (
	"github.com/lxn/walk"
)

func (a *guiApp) buildControlPanel(parent walk.Container) error {
	runGB, err := walk.NewGroupBox(parent)
	if err != nil {
		return err
	}
	if err := runGB.SetTitle("1. Tools Control"); err != nil {
		return err
	}
	runLayout := walk.NewVBoxLayout()
	runLayout.SetSpacing(8)
	if err := runGB.SetLayout(runLayout); err != nil {
		return err
	}

	controlRow, err := walk.NewComposite(runGB)
	if err != nil {
		return err
	}
	controlHBox := walk.NewHBoxLayout()
	controlHBox.SetSpacing(16)
	if err := controlRow.SetLayout(controlHBox); err != nil {
		return err
	}

	if err := a.buildViiperSection(controlRow); err != nil {
		return err
	}
	if _, err := walk.NewHSpacer(controlRow); err != nil {
		return err
	}
	return a.buildToolsSection(controlRow)
}

// buildViiperSection creates the VIIPER status badge, Start button, and hint.
func (a *guiApp) buildViiperSection(parent walk.Container) error {
	hintFont, err := walk.NewFont("Segoe UI", 8, 0)
	if err != nil {
		return err
	}

	panel, err := walk.NewComposite(parent)
	if err != nil {
		return err
	}
	vbox := walk.NewVBoxLayout()
	vbox.SetSpacing(4)
	if err := panel.SetLayout(vbox); err != nil {
		return err
	}

	a.viiperBadge, err = newViiperBadge(panel)
	if err != nil {
		return err
	}

	a.viiperStartBtn, err = walk.NewPushButton(panel)
	if err != nil {
		return err
	}
	if err := a.viiperStartBtn.SetText("Start VIIPER"); err != nil {
		return err
	}
	a.viiperStartBtn.Clicked().Attach(a.onStartViiper)

	hint, err := walk.NewLabel(panel)
	if err != nil {
		return err
	}
	if err := hint.SetText("VIIPER starts automatically."); err != nil {
		return err
	}
	hint.SetFont(hintFont)
	return nil
}

// buildToolsSection creates the TOOLS status badge, Start/Stop buttons, and hint.
func (a *guiApp) buildToolsSection(parent walk.Container) error {
	hintFont, err := walk.NewFont("Segoe UI", 8, 0)
	if err != nil {
		return err
	}

	panel, err := walk.NewComposite(parent)
	if err != nil {
		return err
	}
	vbox := walk.NewVBoxLayout()
	vbox.SetSpacing(4)
	if err := panel.SetLayout(vbox); err != nil {
		return err
	}

	a.toolsBadge, err = newToolsBadge(panel)
	if err != nil {
		return err
	}

	btnRow, err := walk.NewComposite(panel)
	if err != nil {
		return err
	}
	btnHBox := walk.NewHBoxLayout()
	btnHBox.SetSpacing(10)
	if err := btnRow.SetLayout(btnHBox); err != nil {
		return err
	}

	a.startBtn, err = walk.NewPushButton(btnRow)
	if err != nil {
		return err
	}
	if err := a.startBtn.SetText("Start"); err != nil {
		return err
	}
	a.startBtn.Clicked().Attach(a.onStart)

	a.stopBtn, err = walk.NewPushButton(btnRow)
	if err != nil {
		return err
	}
	if err := a.stopBtn.SetText("Stop"); err != nil {
		return err
	}
	a.stopBtn.SetEnabled(false)
	a.stopBtn.Clicked().Attach(a.onStop)

	toggleHint, err := walk.NewLabel(panel)
	if err != nil {
		return err
	}
	if err := toggleHint.SetText("Toggle: End / F12"); err != nil {
		return err
	}
	toggleHint.SetFont(hintFont)
	return nil
}
