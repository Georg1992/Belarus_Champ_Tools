//go:build windows

package main

import (
	"github.com/lxn/walk"
)

func (a *guiApp) buildControlPanel(parent walk.Container) error {
	hintFont, err := walk.NewFont("Segoe UI", 8, 0)
	if err != nil {
		return err
	}

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

	// ---- VIIPER block ----
	viiperPanel, err := walk.NewComposite(controlRow)
	if err != nil {
		return err
	}
	viiperVBox := walk.NewVBoxLayout()
	viiperVBox.SetSpacing(4)
	if err := viiperPanel.SetLayout(viiperVBox); err != nil {
		return err
	}

	a.viiperBadge, err = newViiperBadge(viiperPanel)
	if err != nil {
		return err
	}

	a.viiperStartBtn, err = walk.NewPushButton(viiperPanel)
	if err != nil {
		return err
	}
	if err := a.viiperStartBtn.SetText("Start VIIPER"); err != nil {
		return err
	}
	a.viiperStartBtn.Clicked().Attach(a.onStartViiper)

	viiperHint, err := walk.NewLabel(viiperPanel)
	if err != nil {
		return err
	}
	if err := viiperHint.SetText("VIIPER starts automatically."); err != nil {
		return err
	}
	viiperHint.SetFont(hintFont)

	if _, err := walk.NewHSpacer(controlRow); err != nil {
		return err
	}

	// ---- TOOLS block ----
	toolsPanel, err := walk.NewComposite(controlRow)
	if err != nil {
		return err
	}
	toolsVBox := walk.NewVBoxLayout()
	toolsVBox.SetSpacing(4)
	if err := toolsPanel.SetLayout(toolsVBox); err != nil {
		return err
	}

	a.toolsBadge, err = newToolsBadge(toolsPanel)
	if err != nil {
		return err
	}

	btnRow, err := walk.NewComposite(toolsPanel)
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

	toggleHint, err := walk.NewLabel(toolsPanel)
	if err != nil {
		return err
	}
	if err := toggleHint.SetText("Toggle: End"); err != nil {
		return err
	}
	toggleHint.SetFont(hintFont)

	// Initial state set in createWindow after all tabs are built.
	return nil
}
